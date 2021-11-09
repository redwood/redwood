package braidhttp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/swarm"
	"redwood.dev/tree"
)

var altSvcRegexp1 = regexp.MustCompile(`\s*(\w+)="([^"]+)"\s*(;[^,]*)?`)
var altSvcRegexp2 = regexp.MustCompile(`\s*;\s*(\w+)=(\w+)`)

func forEachAltSvcHeaderPeer(header string, fn func(transportName, dialAddr string, metadata map[string]string)) {
	result := altSvcRegexp1.FindAllStringSubmatch(header, -1)
	for i := range result {
		transportName := result[i][1]
		dialAddr := result[i][2]
		metadata := make(map[string]string)
		if result[i][3] != "" {
			result2 := altSvcRegexp2.FindAllStringSubmatch(result[i][3], -1)
			for i := range result2 {
				key := result2[i][1]
				val := result2[i][2]
				metadata[key] = val
			}
		}
		fn(transportName, dialAddr, metadata)
	}
}

func makeAltSvcHeader(peerDialInfos map[swarm.PeerDialInfo]struct{}) string {
	var others []string
	for dialInfo := range peerDialInfos {
		others = append(others, fmt.Sprintf(`%s="%s"`, dialInfo.TransportName, dialInfo.DialAddr))
	}
	return strings.Join(others, ", ")
}

func parseRawParam(r *http.Request) (bool, error) {
	rawStr := r.URL.Query().Get("raw")
	if rawStr == "" {
		return false, nil
	}
	raw, err := strconv.ParseBool(rawStr)
	if err != nil {
		return false, errors.New("invalid raw param")
	}
	return raw, nil
}

func parseIndexParams(r *http.Request) (string, string) {
	indexName := r.URL.Query().Get("index")
	indexArg := r.URL.Query().Get("index_arg")
	return indexName, indexArg
}

// Creates an *http.Request representing the given Tx that follows the Braid-HTTP
// specification for sending transactions/patches to peers. If the transaction is
// public, the `senderEncKeypair` and `recipientEncPubkey` parameters may be nil.
func putRequestFromTx(
	requestContext context.Context,
	tx tree.Tx,
	dialAddr string,
) (*http.Request, error) {
	var parentStrs []string
	for _, parent := range tx.Parents {
		parentStrs = append(parentStrs, parent.Hex())
	}

	var body bytes.Buffer
	for _, patch := range tx.Patches {
		_, err := body.Write([]byte(patch.String() + "\n"))
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	req, err := http.NewRequest("PUT", dialAddr, &body)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	req.Header.Set("Version", tx.ID.Hex())
	req.Header.Set("State-URI", tx.StateURI)
	req.Header.Set("Signature", tx.Sig.Hex())
	req.Header.Set("Parents", strings.Join(parentStrs, ","))
	if tx.Checkpoint {
		req.Header.Set("Checkpoint", "true")
	}
	return req, nil
}
