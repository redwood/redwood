package redwood

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type HTTPClient interface {
	Authorize() error
	Subscribe(ctx context.Context, stateURI string) (chan MaybeTx, error)
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	Get(stateURI string, version *types.ID, keypath tree.Keypath, rng *tree.Range, raw bool) (io.ReadCloser, int64, []types.ID, error)
	Put(tx *Tx) error
	StoreRef(file io.Reader) (types.Hash, error)
}

type httpClient struct {
	peerReachableAddress string
	sigkeys              *SigningKeypair
	cookieJar            http.CookieJar
	tls                  bool
}

func NewHTTPClient(peerReachableAddress string, sigkeys *SigningKeypair, tls bool) (HTTPClient, error) {
	cookieJar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	return &httpClient{
		peerReachableAddress: peerReachableAddress,
		sigkeys:              sigkeys,
		cookieJar:            cookieJar,
		tls:                  tls,
	}, nil
}

func (c *httpClient) client() *http.Client {
	var tlsConfig *tls.Config
	if c.tls {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Jar: c.cookieJar, Transport: tr}
}

func (c *httpClient) Authorize() error {
	client := c.client()

	req, err := http.NewRequest("AUTHORIZE", c.peerReachableAddress, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	} else if resp.StatusCode != 200 {
		return errors.Errorf("error verifying peer address: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	challengeHex, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.WithStack(err)
	}

	challenge, err := hex.DecodeString(string(challengeHex))
	if err != nil {
		return errors.WithStack(err)
	}

	sig, err := c.sigkeys.SignHash(types.HashBytes(challenge))
	if err != nil {
		return errors.WithStack(err)
	}

	sigHex := hex.EncodeToString(sig)

	req2, err := http.NewRequest("AUTHORIZE", c.peerReachableAddress, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Set("Response", sigHex)
	resp2, err := client.Do(req2)
	if err != nil {
		return errors.WithStack(err)
	} else if resp2.StatusCode != 200 {
		return errors.Errorf("error verifying peer address: (%v) %v", resp2.StatusCode, resp2.Status)
	}
	defer resp2.Body.Close()

	return nil
}

type MaybeTx struct {
	*Tx
	Err error
}

func (c *httpClient) Subscribe(ctx context.Context, stateURI string) (chan MaybeTx, error) {
	client := c.client()

	req, err := http.NewRequest("GET", c.peerReachableAddress, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	req.Header.Set("Subscribe", "true")
	req.Header.Set("State-URI", stateURI)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	} else if resp.StatusCode != 200 {
		return nil, errors.Errorf("error subscribing: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	ch := make(chan MaybeTx)
	go func() {
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var tx Tx
			r := bufio.NewReader(resp.Body)
			bs, err := r.ReadBytes(byte('\n'))
			if err != nil {
				ch <- MaybeTx{Err: err}
				continue
			}
			bs = bytes.Trim(bs, "\n ")

			err = json.Unmarshal(bs, &tx)
			if err != nil {
				ch <- MaybeTx{Err: err}
				continue
			}

			ch <- MaybeTx{Tx: &tx}
		}
	}()
	return ch, nil
}

func (c *httpClient) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	client := c.client()
	req, err := http.NewRequest("GET", c.peerReachableAddress+"/__tx/"+txID.Hex(), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	req.Header.Set("State-URI", stateURI)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	} else if resp.StatusCode == 404 {
		return nil, types.Err404
	} else if resp.StatusCode != 200 {
		return nil, errors.Errorf("error fetching tx: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	var tx Tx
	err = json.NewDecoder(resp.Body).Decode(&tx)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (c *httpClient) Get(stateURI string, version *types.ID, keypath tree.Keypath, rng *tree.Range, raw bool) (io.ReadCloser, int64, []types.ID, error) {
	client := c.client()
	url := c.peerReachableAddress + "/" + string(keypath)
	if raw {
		url += "?raw=true"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, nil, errors.WithStack(err)
	}

	if stateURI != "" {
		req.Header.Set("State-URI", stateURI)
	}
	if version != nil {
		req.Header.Set("Version", version.Hex())
	}
	if rng != nil {
		req.Header.Set("Range", fmt.Sprintf("json=%d:%d", rng[0], rng[1]))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, errors.WithStack(err)
	} else if resp.StatusCode != 200 {
		if version != nil {
			return nil, 0, nil, errors.Errorf("error getting state@%v: (%v) %v", version.Hex(), resp.StatusCode, resp.Status)
		}
		return nil, 0, nil, errors.Errorf("error getting state@HEAD (%v) %v", resp.StatusCode, resp.Status)
	}

	var contentLength int
	if contentLengthStr := resp.Header.Get("Content-Length"); contentLengthStr != "" {
		contentLength, err = strconv.Atoi(contentLengthStr)
		if err != nil {
			return nil, 0, nil, err
		}
	}

	var parents []types.ID
	if parentsHeader := resp.Header.Get("Parents"); parentsHeader != "" {
		parentStrs := strings.Split(parentsHeader, ",")
		for _, pstr := range parentStrs {
			pstr = strings.TrimSpace(pstr)
			pid, err := types.IDFromHex(pstr)
			if err != nil {
				return nil, 0, nil, errors.New("bad parents header")
			}
			parents = append(parents, pid)
		}
	}

	return resp.Body, int64(contentLength), parents, nil
}

func (c *httpClient) Put(tx *Tx) error {
	if len(tx.Sig) == 0 {
		sig, err := c.sigkeys.SignHash(tx.Hash())
		if err != nil {
			return errors.WithStack(err)
		}
		tx.Sig = sig
	}

	var parentStrs []string
	for _, parent := range tx.Parents {
		parentStrs = append(parentStrs, parent.Hex())
	}

	var body bytes.Buffer
	for _, patch := range tx.Patches {
		_, err := body.Write([]byte(patch.String() + "\n"))
		if err != nil {
			return errors.WithStack(err)
		}
	}

	client := c.client()
	req, err := http.NewRequest("PUT", c.peerReachableAddress, &body)
	if err != nil {
		return errors.WithStack(err)
	}

	req.Header.Set("Version", tx.ID.Hex())
	req.Header.Set("State-URI", tx.URL)
	req.Header.Set("Signature", tx.Sig.Hex())
	req.Header.Set("Parents", strings.Join(parentStrs, ","))
	if tx.Checkpoint {
		req.Header.Set("Checkpoint", "true")
	}

	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	} else if resp.StatusCode != 200 {
		return errors.Errorf("error putting tx: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()
	return nil
}

func (c *httpClient) StoreRef(file io.Reader) (types.Hash, error) {
	client := c.client()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "ref", "ref"))
	h.Set("Content-Type", "application/octet-stream")
	fileWriter, err := w.CreatePart(h)
	if err != nil {
		return types.Hash{}, errors.WithStack(err)
	}

	// @@TODO: streaming?
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return types.Hash{}, errors.WithStack(err)
	}
	w.Close()

	req, err := http.NewRequest("PUT", c.peerReachableAddress, &buf)
	if err != nil {
		return types.Hash{}, errors.WithStack(err)
	}
	req.Header.Set("Ref", "true")
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return types.Hash{}, errors.WithStack(err)
	} else if resp.StatusCode != 200 {
		return types.Hash{}, errors.Errorf("error verifying peer address: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	var body StoreRefResponse
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return types.Hash{}, errors.WithStack(err)
	}
	return body.Hash, nil
}
