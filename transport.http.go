package redwood

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type httpTransport struct {
	*ctx.Context

	address         types.Address
	controller      Metacontroller
	defaultStateURI string
	ownURL          string
	listenAddr      string
	sigkeys         *SigningKeypair
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	cookieJar       http.CookieJar
	devMode         bool

	pendingAuthorizations map[types.ID][]byte

	subscriptionsIn   map[string]map[*httpSubscriptionIn]struct{}
	subscriptionsInMu sync.RWMutex

	host      Host
	refStore  RefStore
	peerStore PeerStore
}

func NewHTTPTransport(
	addr types.Address,
	listenAddr string,
	defaultStateURI string,
	controller Metacontroller,
	refStore RefStore,
	peerStore PeerStore,
	sigkeys *SigningKeypair,
	cookieSecret [32]byte,
	tlsCertFilename, tlsKeyFilename string,
	devMode bool,
) (Transport, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	ownURL := listenAddr
	if len(listenAddr) > 0 && listenAddr[0] == ':' {
		ownURL = "localhost" + listenAddr
	}

	t := &httpTransport{
		Context:               &ctx.Context{},
		address:               addr,
		subscriptionsIn:       make(map[string]map[*httpSubscriptionIn]struct{}),
		controller:            controller,
		listenAddr:            listenAddr,
		defaultStateURI:       defaultStateURI,
		sigkeys:               sigkeys,
		cookieSecret:          cookieSecret,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		devMode:               devMode,
		cookieJar:             jar,
		pendingAuthorizations: make(map[types.ID][]byte),
		ownURL:                ownURL,
		refStore:              refStore,
		peerStore:             peerStore,
	}
	return t, nil
}

func (t *httpTransport) Start() error {
	return t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel(t.address.Pretty() + " transport")
			t.Infof(0, "opening http transport at %v", t.listenAddr)

			if t.cookieSecret == [32]byte{} {
				_, err := rand.Read(t.cookieSecret[:])
				if err != nil {
					return err
				}
			}

			t.peerStore.AddReachableAddresses(t.Name(), NewStringSet([]string{t.ownURL}))

			go func() {
				if !t.devMode {
					srv := &http.Server{
						Addr:      t.listenAddr,
						Handler:   t,
						TLSConfig: &tls.Config{},
					}
					err := srv.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
					if err != nil {
						fmt.Printf("%+v\n", err.Error())
						panic("http transport failed to start")
					}
				} else {
					srv := &http.Server{
						Addr:    t.listenAddr,
						Handler: t,
					}
					err := srv.ListenAndServe()
					if err != nil {
						fmt.Printf("%+v\n", err.Error())
						panic("http transport failed to start")
					}
				}
			}()

			return nil
		},
		nil,
		nil,
		// on shutdown
		nil,
	)
}

type httpSubscriptionIn struct {
	io.Writer
	http.Flusher
	address          types.Address
	chDoneCatchingUp chan struct{}
	chDone           chan struct{}
}

func (s *httpSubscriptionIn) Close() error {
	close(s.chDone)
	return nil
}

func (t *httpTransport) Name() string {
	return "http"
}

var altSvcRegexp1 = regexp.MustCompile(`\s*(\w+)="([^"]+)"\s*(;[^,]*)?`)
var altSvcRegexp2 = regexp.MustCompile(`\s*;\s*(\w+)=(\w+)`)

func forEachAltSvcHeaderPeer(header string, fn func(transportName, reachableAt string, metadata map[string]string)) {
	result := altSvcRegexp1.FindAllStringSubmatch(header, -1)
	for i := range result {
		transportName := result[i][1]
		reachableAt := result[i][2]
		metadata := make(map[string]string)
		if result[i][3] != "" {
			result2 := altSvcRegexp2.FindAllStringSubmatch(result[i][3], -1)
			for i := range result2 {
				key := result2[i][1]
				val := result2[i][2]
				metadata[key] = val
			}
		}
		fn(transportName, reachableAt, metadata)
	}
}

func (t *httpTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if t.devMode {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	sessionID, err := t.ensureSessionIDCookie(w, r)
	if err != nil {
		t.Errorf("error reading sessionID cookie: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	address := t.addressFromCookie(r)

	// Peer discovery
	{
		// On every incoming request, advertise other peers via the Alt-Svc header
		var others []string
		for _, tuple := range t.peerStore.PeerTuples() {
			others = append(others, fmt.Sprintf(`%s="%s"`, tuple.TransportName, tuple.ReachableAt))
		}
		w.Header().Set("Alt-Svc", strings.Join(others, ", "))

		// Similarly, if other peers give us Alt-Svc headers, track them
		if altSvcHeader := r.Header.Get("Alt-Svc"); altSvcHeader != "" {
			forEachAltSvcHeaderPeer(altSvcHeader, func(transportName, reachableAt string, metadata map[string]string) {
				t.peerStore.AddReachableAddresses(transportName, NewStringSet([]string{reachableAt}))
			})
		}
	}

	switch r.Method {
	case "HEAD":
		// This is mainly used to poll for new peers

	case "OPTIONS":
		w.Header().Set("Access-Control-Allow-Headers", "State-URI")

	case "AUTHORIZE":
		// Address verification requests (2 kinds)
		if challengeMsgHex := r.Header.Get("Challenge"); challengeMsgHex != "" {
			// 1. Remote node is reaching out to us with a challenge, and we respond
			t.serveAuthorizeSelf(w, r, address, challengeMsgHex)
		} else {
			responseHex := r.Header.Get("Response")
			if responseHex == "" {
				// 2a. Remote node wants a challenge, so we send it
				t.serveAuthorizeOtherIssueChallenge(w, r, sessionID)
			} else {
				// 2b. Remote node wanted a challenge, this is their response
				t.serveAuthorizeOtherCheckResponse(w, r, sessionID, responseHex)
			}
		}

	case "GET":
		if r.Header.Get("Subscribe") != "" {
			t.serveSubscription(w, r, address)
		} else {
			if r.URL.Path == "/braid.js" {
				// @@TODO: this is hacky
				t.serveBraidJS(w, r)
			} else if strings.HasPrefix(r.URL.Path, "/__tx/") {
				t.serveGetTx(w, r)
			} else {
				t.serveGetState(w, r)
			}
		}

	case "ACK":
		t.serveAck(w, r, address)

	case "PUT":
		if r.Header.Get("Private") == "true" {
			t.servePostPrivateTx(w, r, address)

		} else if r.Header.Get("Ref") == "true" {
			t.servePostRef(w, r)

		} else {
			t.servePostTx(w, r, address)
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (t *httpTransport) serveAuthorizeSelf(w http.ResponseWriter, r *http.Request, address types.Address, challengeMsgHex string) {
	challengeMsg, err := hex.DecodeString(challengeMsgHex)
	if err != nil {
		http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
		return
	}

	peer := t.makePeerWithAddress(w, nil, address)
	err = t.host.OnVerifyAddressReceived([]byte(challengeMsg), peer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (t *httpTransport) serveAuthorizeOtherIssueChallenge(w http.ResponseWriter, r *http.Request, sessionID types.ID) {
	challenge, err := types.GenerateChallengeMsg()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.pendingAuthorizations[sessionID] = challenge

	challengeHex := hex.EncodeToString(challenge)
	_, err = w.Write([]byte(challengeHex))
	if err != nil {
		t.Errorf("error writing challenge message: %v", err)
		return
	}
}

func (t *httpTransport) serveAuthorizeOtherCheckResponse(w http.ResponseWriter, r *http.Request, sessionID types.ID, responseHex string) {
	challenge, exists := t.pendingAuthorizations[sessionID]
	if !exists {
		http.Error(w, "no pending authorization", http.StatusBadRequest)
		return
	}

	sig, err := hex.DecodeString(responseHex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sigpubkey, err := RecoverSigningPubkey(types.HashBytes(challenge), sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	addr := sigpubkey.Address()
	err = t.setSignedCookie(w, "address", addr[:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	delete(t.pendingAuthorizations, sessionID) // @@TODO: expiration/garbage collection for failed auths
}

func (t *httpTransport) serveSubscription(w http.ResponseWriter, r *http.Request, address types.Address) {
	t.Infof(0, "incoming subscription (address: %v)", address)

	// @@TODO: ensure we actually have this stateURI
	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		stateURI = t.defaultStateURI
	}

	// Make sure that the writer supports flushing.
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	sub := &httpSubscriptionIn{w, f, address, make(chan struct{}), make(chan struct{})}

	t.trackSubscription(stateURI, sub)

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		t.Infof(0, "http connection closed")
		t.untrackSubscription(stateURI, sub)
	}()

	f.Flush()

	parentsHeader := r.Header.Get("Parents")
	if parentsHeader != "" {
		var parents []types.ID
		parentStrs := strings.Split(parentsHeader, ",")
		for _, parentStr := range parentStrs {
			parent, err := types.IDFromHex(parentStr)
			if err != nil {
				// @@TODO: what to do?
				parents = []types.ID{GenesisTxID}
				break
			}
			parents = append(parents, parent)
		}

		var toVersion types.ID
		if versionHeader := r.Header.Get("Version"); versionHeader != "" {
			var err error
			toVersion, err = types.IDFromHex(versionHeader)
			if err != nil {
				// @@TODO: what to do?
			}
		}

		err := t.host.OnFetchHistoryRequestReceived(stateURI, parents, toVersion, t.makePeerWithAddress(w, f, address))
		if err != nil {
			t.Errorf("error fetching history: %v", err)
			// @@TODO: close subscription?
		}
	}

	close(sub.chDoneCatchingUp)

	// Block until the subscription is canceled so that net/http doesn't close the connection
	<-sub.chDone
}

func (t *httpTransport) serveBraidJS(w http.ResponseWriter, r *http.Request) {
	var filename string
	if fileExists("./braid.js") {
		filename = "./braid.js"
	}
	f, err := os.Open(filename)
	if err != nil {
		http.Error(w, "can't find braidjs", http.StatusNotFound)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, "./braidjs/braid-dist.js", time.Now(), f)
	return
}

func (t *httpTransport) serveGetTx(w http.ResponseWriter, r *http.Request) {
	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		http.Error(w, "missing State-URI header", http.StatusBadRequest)
		return
	}

	parts := strings.Split(r.URL.Path[1:], "/")
	txIDStr := parts[1]
	txID, err := types.IDFromHex(txIDStr)
	if err != nil {
		http.Error(w, "bad tx id", http.StatusBadRequest)
		return
	}

	tx, err := t.controller.FetchTx(stateURI, txID)
	if err != nil {
		http.Error(w, fmt.Sprintf("not found: %v", err), http.StatusNotFound)
		return
	}

	respondJSON(w, tx)
}

func (t *httpTransport) serveGetState(w http.ResponseWriter, r *http.Request) {

	keypathStrs := filterEmptyStrings(strings.Split(r.URL.Path[1:], "/"))

	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		queryStateURI := r.URL.Query().Get("state_uri")
		if queryStateURI != "" {
			stateURI = queryStateURI
		} else {
			stateURI = t.defaultStateURI
		}
	}

	keypathStr := strings.Join(keypathStrs, string(tree.KeypathSeparator))
	keypath := tree.Keypath(keypathStr)

	parts := keypath.Parts()
	newParts := make([]tree.Keypath, 0, len(parts))
	for _, part := range parts {
		if idx := part.IndexByte('['); idx > -1 {
			newParts = append(newParts, part[:idx])
			x, err := strconv.ParseInt(string(part[idx+1:len(part)-1]), 10, 64)
			if err != nil {
				http.Error(w, "bad slice index", http.StatusBadRequest)
				return
			}
			newParts = append(newParts, tree.EncodeSliceIndex(uint64(x)))
		} else {
			newParts = append(newParts, part)
		}
	}
	keypath = tree.JoinKeypaths(newParts, []byte("/"))

	var version *types.ID
	if vstr := r.Header.Get("Version"); vstr != "" {
		v, err := types.IDFromHex(vstr)
		if err != nil {
			http.Error(w, "bad Version header", http.StatusBadRequest)
			return
		}
		version = &v
	}

	var rng *tree.Range
	if rstr := r.Header.Get("Range"); rstr != "" {
		// Range: -10:-5
		// @@TODO: real json Range parsing
		parts := strings.SplitN(rstr, "=", 2)
		if len(parts) != 2 {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		} else if parts[0] != "json" {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		parts = strings.SplitN(parts[1], ":", 2)
		if len(parts) != 2 {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		end, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		rng = &tree.Range{start, end}
	}

	// Add the "Parents" header
	{
		leaves, err := t.controller.Leaves(stateURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusNotFound)
			return
		}
		var leaf types.ID
		for vid := range leaves {
			leaf = vid
			break
		}
		tx, err := t.controller.FetchTx(stateURI, leaf)
		if err != nil {
			http.Error(w, fmt.Sprintf("can't fetch tx %v: %+v", leaf, err.Error()), http.StatusNotFound)
			return
		}
		parents := tx.Parents

		var parentStrs []string
		for _, pid := range parents {
			parentStrs = append(parentStrs, pid.Hex())
		}
		w.Header().Add("Parents", strings.Join(parentStrs, ","))
	}

	indexName, indexArg := parseIndexParams(r)
	raw, err := parseRawParam(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusBadRequest)
		return
	}

	var state tree.Node
	var anyMissing bool

	if indexName != "" {
		// Index query

		// You can specify an index_arg of * in order to fetch the entire index
		var indexArgKeypath tree.Keypath
		if indexArg != "*" {
			indexArgKeypath = tree.Keypath(indexArg)
		}

		state, err = t.controller.QueryIndex(stateURI, version, keypath, tree.Keypath(indexName), indexArgKeypath, rng)
		if errors.Cause(err) == types.Err404 {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}

	} else {
		// State query
		state, err = t.controller.StateAtVersion(stateURI, version)
		if err != nil {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		}
		defer state.Close()

		if raw {
			state = state.NodeAt(keypath, rng)

		} else {
			var exists bool
			state, exists, err = nelson.Seek(state, keypath, t.controller)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			} else if !exists {
				http.Error(w, fmt.Sprintf("not found: %v", keypath), http.StatusNotFound)
				return
			}
			keypath = nil

			state, err = state.CopyToMemory(nil, rng)
			if errors.Cause(err) == types.Err404 {
				http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}

			state, anyMissing, err = nelson.Resolve(state, t.controller)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}

			indexHTMLExists, err := state.Exists(keypath.Push(tree.Keypath("index.html")))
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}
			if indexHTMLExists {
				keypath = keypath.Push(tree.Keypath("index.html"))
				state = state.NodeAt(tree.Keypath("index.html"), nil)
			}
		}
	}

	contentType, err := nelson.GetContentType(state)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentType == "application/octet-stream" {
		contentType = GuessContentTypeFromFilename(string(keypath.Part(-1)))
	}
	w.Header().Set("Content-Type", contentType)

	contentLength, err := nelson.GetContentLength(state)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	resourceLength, err := state.Length()
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Resource-Length", strconv.FormatUint(resourceLength, 10))

	var val interface{}
	var exists bool
	if !raw {
		val, exists, err = nelson.GetValueRecursive(state, nil, nil)
	} else {
		val, exists, err = state.Value(nil, nil)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusBadRequest)
		return
	} else if !exists {
		http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
		return
	}

	respBuf, ok := nelson.GetReadCloser(val)
	if !ok {
		contentType = "application/json"
		j, err := json.Marshal(val)
		if err != nil {
			panic(err)
		}
		respBuf = ioutil.NopCloser(bytes.NewBuffer(j))
	}
	defer respBuf.Close()

	// Right now, this is just to facilitate the Chrome extension
	allowSubscribe := map[string]bool{
		"application/json": true,
		"application/js":   true,
		"text/plain":       true,
	}
	if allowSubscribe[contentType] {
		w.Header().Set("Subscribe", "Allow")
	}

	if anyMissing {
		w.WriteHeader(http.StatusPartialContent)
	}

	_, err = io.Copy(w, respBuf)
	if err != nil {
		panic(err)
	}
}

func (t *httpTransport) serveAck(w http.ResponseWriter, r *http.Request, address types.Address) {
	defer r.Body.Close()

	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	var txID types.ID
	err = txID.UnmarshalText(bs)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	t.host.OnAckReceived(txID, t.makePeerWithAddress(w, nil, address))
}

func (t *httpTransport) servePostPrivateTx(w http.ResponseWriter, r *http.Request, address types.Address) {
	t.Infof(0, "incoming private tx")

	var encryptedTx EncryptedTx
	err := json.NewDecoder(r.Body).Decode(&encryptedTx)
	if err != nil {
		panic(err)
	}

	t.host.OnPrivateTxReceived(encryptedTx, t.makePeerWithAddress(w, nil, address))
}

func (t *httpTransport) servePostRef(w http.ResponseWriter, r *http.Request) {
	t.Infof(0, "incoming ref")

	err := r.ParseForm()
	if err != nil {
		panic(err)
	}

	file, _, err := r.FormFile("ref")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	sha1Hash, sha3Hash, err := t.refStore.StoreObject(file)
	if err != nil {
		t.Errorf("error storing ref: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, StoreRefResponse{SHA1: sha1Hash, SHA3: sha3Hash})
}

func (t *httpTransport) servePostTx(w http.ResponseWriter, r *http.Request, address types.Address) {
	t.Infof(0, "incoming tx")

	var err error

	var sig types.Signature
	sigHeaderStr := r.Header.Get("Signature")
	if sigHeaderStr == "" {
		http.Error(w, "missing Signature header", http.StatusBadRequest)
		return
	} else {
		sig, err = types.SignatureFromHex(sigHeaderStr)
		if err != nil {
			http.Error(w, "bad Signature header", http.StatusBadRequest)
			return
		}
	}

	var txID types.ID
	txIDStr := r.Header.Get("Version")
	if txIDStr == "" {
		txID = types.RandomID()
	} else {
		txID, err = types.IDFromHex(txIDStr)
		if err != nil {
			http.Error(w, "bad Version header", http.StatusBadRequest)
			return
		}
	}

	var parents []types.ID
	parentsStr := r.Header.Get("Parents")
	if parentsStr != "" {
		parentsStrs := strings.Split(parentsStr, ",")
		for _, pstr := range parentsStrs {
			parentID, err := types.IDFromHex(strings.TrimSpace(pstr))
			if err != nil {
				http.Error(w, "bad Parents header", http.StatusBadRequest)
				return
			}
			parents = append(parents, parentID)
		}
	}

	var checkpoint bool
	if checkpointStr := r.Header.Get("Checkpoint"); checkpointStr == "true" {
		checkpoint = true
	}

	var patches []Patch
	scanner := bufio.NewScanner(r.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		patch, err := ParsePatch(line)
		if err != nil {
			http.Error(w, fmt.Sprintf("bad patch string: %v", line), http.StatusBadRequest)
			return
		}
		patches = append(patches, patch)
	}
	if err := scanner.Err(); err != nil {
		http.Error(w, fmt.Sprintf("internal server error: %v", err), http.StatusInternalServerError)
		return
	}

	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		stateURI = t.defaultStateURI
	}

	tx := Tx{
		ID:         txID,
		Parents:    parents,
		Sig:        sig,
		Patches:    patches,
		StateURI:   stateURI,
		Checkpoint: checkpoint,
	}

	// @@TODO: remove .From entirely
	pubkey, err := RecoverSigningPubkey(tx.Hash(), sig)
	if err != nil {
		panic("bad sig")
	}
	tx.From = pubkey.Address()
	////////////////////////////////

	t.host.OnTxReceived(tx, t.makePeerWithAddress(w, nil, address))
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

func respondJSON(resp http.ResponseWriter, data interface{}) {
	resp.Header().Add("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(data)
	if err != nil {
		panic(err)
	}
}

func (t *httpTransport) SetHost(h Host) {
	t.host = h
}

func (t *httpTransport) trackSubscription(stateURI string, sub *httpSubscriptionIn) {
	t.subscriptionsInMu.Lock()
	defer t.subscriptionsInMu.Unlock()

	if _, exists := t.subscriptionsIn[stateURI]; !exists {
		t.subscriptionsIn[stateURI] = make(map[*httpSubscriptionIn]struct{})
	}
	t.subscriptionsIn[stateURI][sub] = struct{}{}
}

func (t *httpTransport) untrackSubscription(stateURI string, sub *httpSubscriptionIn) {
	t.subscriptionsInMu.Lock()
	defer t.subscriptionsInMu.Unlock()

	if _, exists := t.subscriptionsIn[stateURI]; !exists {
		return
	}
	delete(t.subscriptionsIn[stateURI], sub)
}

func (t *httpTransport) GetPeerByConnStrings(ctx context.Context, reachableAt StringSet) (Peer, error) {
	if len(reachableAt) != 1 {
		panic("weird")
	}
	return t.makePeer(nil, nil, reachableAt), nil
}

func (t *httpTransport) ForEachProviderOfStateURI(ctx context.Context, theURL string) (<-chan Peer, error) {
	u, err := url.Parse("http://" + theURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "providers")

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.Errorf("error GETting providers: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	var providers []string
	err = json.NewDecoder(resp.Body).Decode(&providers)
	if err != nil {
		return nil, err
	}

	ch := make(chan Peer)
	go func() {
		defer close(ch)
		for _, providerURL := range providers {
			if providerURL == t.ownURL {
				continue
			}

			select {
			case ch <- t.makePeer(nil, nil, NewStringSet([]string{providerURL})):
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) ForEachProviderOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *httpTransport) ForEachSubscriberToStateURI(ctx context.Context, stateURI string) (<-chan Peer, error) {
	ch := make(chan Peer)
	go func() {
		t.subscriptionsInMu.RLock()
		defer t.subscriptionsInMu.RUnlock()
		defer close(ch)

		if _, exists := t.subscriptionsIn[stateURI]; !exists {
			return
		}

		for sub := range t.subscriptionsIn[stateURI] {
			<-sub.chDoneCatchingUp
			select {
			case ch <- t.makePeer(sub.Writer, sub.Flusher, nil):
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *httpTransport) AnnounceRef(refID types.RefID) error {
	return types.ErrUnimplemented
}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *httpTransport) ensureSessionIDCookie(w http.ResponseWriter, r *http.Request) (types.ID, error) {
	sessionIDBytes, err := t.signedCookie(r, "sessionid")
	if err != nil {
		// t.Errorf("error reading signed sessionid cookie: %v", err)
		return t.setSessionIDCookie(w)
	}
	return types.IDFromBytes(sessionIDBytes), nil
}

func (t *httpTransport) setSessionIDCookie(w http.ResponseWriter) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookie(w, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *httpTransport) setSignedCookie(w http.ResponseWriter, name string, value []byte) error {
	w.Header().Del("Set-Cookie")

	sig, err := t.sigkeys.SignHash(types.HashBytes(append(value, t.cookieSecret[:]...)))
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   hex.EncodeToString(value) + ":" + hex.EncodeToString(sig),
		Expires: time.Now().AddDate(0, 0, 1),
		Path:    "/",
	})
	return nil
}

func (t *httpTransport) signedCookie(r *http.Request, name string) ([]byte, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' has %v parts", name, len(parts))
	}

	value, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' bad hex value: %v", name, err)
	}

	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' bad hex signature: %v", name, err)
	} else if !t.sigkeys.VerifySignature(types.HashBytes(append(value, t.cookieSecret[:]...)), sig) {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' has invalid signature (value: %0x)", name, value)
	}
	return value, nil
}

func (t *httpTransport) addressFromCookie(r *http.Request) types.Address {
	addressBytes, err := t.signedCookie(r, "address")
	if err != nil {
		return types.Address{}
	}
	return types.AddressFromBytes(addressBytes)
}

func (t *httpTransport) makePeer(writer io.Writer, flusher http.Flusher, reachableAt StringSet) *httpPeer {
	storedPeer := t.peerStore.PeerReachableAt("http", reachableAt)
	peer := &httpPeer{t: t, Writer: writer, Flusher: flusher}
	if storedPeer != nil {
		peer.storedPeer = *storedPeer
	} else {
		peer.storedPeer.reachableAt = reachableAt
	}
	return peer
}

func (t *httpTransport) makePeerWithAddress(writer io.Writer, flusher http.Flusher, address types.Address) *httpPeer {
	storedPeer := t.peerStore.PeersFromTransportWithAddress("http", address)
	peer := &httpPeer{t: t, Writer: writer, Flusher: flusher}
	if len(storedPeer) > 0 {
		peer.storedPeer = *storedPeer[0]
	} else {
		peer.storedPeer.address = address
	}
	return peer
}

type httpPeer struct {
	storedPeer

	t *httpTransport

	// stream
	sync.Mutex
	io.Writer
	io.ReadCloser
	http.Flusher

	state httpPeerState
}

type httpPeerState int

const (
	httpPeerState_Unknown = iota
	httpPeerState_ServingSubscription
	httpPeerState_VerifyingAddress
)

func (p *httpPeer) Transport() Transport {
	return p.t
}

func (p *httpPeer) ReachableAt() StringSet {
	return p.reachableAt
}

func (p *httpPeer) EnsureConnected(ctx context.Context) error {
	return nil
}

func (p *httpPeer) WriteMsg(msg Msg) error {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	switch msg.Type {
	case MsgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		var client http.Client
		req, err := http.NewRequest("GET", p.ReachableAt().Any(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("State-URI", stateURI)
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Subscribe", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			return err
		} else if resp.StatusCode != 200 {
			return errors.Errorf("error GETting peer: (%v) %v", resp.StatusCode, resp.Status)
		}

		p.state = httpPeerState_ServingSubscription
		p.ReadCloser = resp.Body

	case MsgType_Put:
		if p.Writer != nil {
			// This peer is subscribed, so we have a connection open already
			tx, is := msg.Payload.(Tx)
			if !is {
				panic("protocol error")
			}

			bs, err := json.Marshal(tx)
			if err != nil {
				return err
			}

			event := []byte("data: " + string(bs) + "\n\n")

			n, err := p.Write(event)
			if err != nil {
				return err
			} else if n < len(event) {
				return errors.New("didn't write enough")
			}

			if p.Flusher != nil {
				p.Flusher.Flush()
			}

		} else {
			// This peer is not subscribed, so we make a PUT
			bs, err := json.Marshal(msg)
			if err != nil {
				return err
			}

			client := http.Client{}
			req, err := http.NewRequest("PUT", p.ReachableAt().Any(), bytes.NewReader(bs))
			if err != nil {
				return err
			}

			resp, err := client.Do(req)
			if err != nil {
				return err
			} else if resp.StatusCode != 200 {
				return errors.Errorf("error PUTting to peer: (%v) %v", resp.StatusCode, resp.Status)
			}
			defer resp.Body.Close()
		}

	case MsgType_Ack:
		txID, ok := msg.Payload.(types.ID)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		txIDBytes, err := txID.MarshalText()
		if err != nil {
			return errors.WithStack(err)
		}

		client := http.Client{}
		req, err := http.NewRequest("ACK", p.ReachableAt().Any(), bytes.NewReader(txIDBytes))
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		} else if resp.StatusCode != 200 {
			return errors.Errorf("error ACKing to peer: (%v) %v", resp.StatusCode, resp.Status)
		}
		defer resp.Body.Close()

	case MsgType_VerifyAddress:
		challengeMsg, ok := msg.Payload.(types.ChallengeMsg)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		client := http.Client{}
		req, err := http.NewRequest("AUTHORIZE", p.ReachableAt().Any(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Challenge", hex.EncodeToString(challengeMsg))

		resp, err := client.Do(req)
		if err != nil {
			return err
		} else if resp.StatusCode != 200 {
			return errors.Errorf("error verifying peer address: (%v) %v", resp.StatusCode, resp.Status)
		}

		p.state = httpPeerState_VerifyingAddress
		p.ReadCloser = resp.Body

	case MsgType_VerifyAddressResponse:
		verifyAddressResponse, ok := msg.Payload.(VerifyAddressResponse)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		err := json.NewEncoder(p.Writer).Encode(verifyAddressResponse)
		if err != nil {
			http.Error(p.Writer.(http.ResponseWriter), err.Error(), http.StatusInternalServerError)
			panic(err)
			return err
		}

	case MsgType_Private:
		encPut, ok := msg.Payload.(EncryptedTx)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		encPutBytes, err := json.Marshal(encPut)
		if err != nil {
			return err
		}

		client := http.Client{}
		req, err := http.NewRequest("PUT", p.ReachableAt().Any(), bytes.NewReader(encPutBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Private", "true")

		resp, err := client.Do(req)
		if err != nil {
			return err
		} else if resp.StatusCode != 200 {
			return errors.Errorf("error sending private PUT: (%v) %v", resp.StatusCode, resp.Status)
		}
		defer resp.Body.Close()

	default:
		panic("unimplemented")
	}
	return nil
}

func (p *httpPeer) ReadMsg() (Msg, error) {
	switch p.state {
	case httpPeerState_VerifyingAddress:
		var verifyResp VerifyAddressResponse
		err := json.NewDecoder(p.ReadCloser).Decode(&verifyResp)
		if err != nil {
			return Msg{}, err
		}
		return Msg{Type: MsgType_VerifyAddressResponse, Payload: verifyResp}, nil

	case httpPeerState_ServingSubscription:
		var tx Tx
		r := bufio.NewReader(p.ReadCloser)
		bs, err := r.ReadBytes(byte('\n'))
		if err != nil {
			return Msg{}, err
		}
		bs = bytes.Trim(bs, "\n ")

		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return Msg{}, err
		}

		msg := Msg{Type: MsgType_Put, Payload: tx}

		return msg, err

	default:
		panic("bad")
	}
}

func (p *httpPeer) CloseConn() error {
	if p.ReadCloser != nil {
		return p.ReadCloser.Close()
	}
	return nil
}
