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
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/markbates/pkger"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/net/publicsuffix"

	"redwood.dev/crypto"
	"redwood.dev/ctx"
	"redwood.dev/identity"
	"redwood.dev/nelson"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type httpTransport struct {
	*ctx.Context

	controllerHub   ControllerHub
	defaultStateURI string
	ownURL          string
	listenAddr      string
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	cookieJar       http.CookieJar
	devMode         bool

	pendingAuthorizations map[types.ID][]byte

	host      Host
	keyStore  identity.KeyStore
	refStore  RefStore
	peerStore PeerStore
}

func NewHTTPTransport(
	listenAddr string,
	reachableAt string,
	defaultStateURI string,
	controllerHub ControllerHub,
	keyStore identity.KeyStore,
	refStore RefStore,
	peerStore PeerStore,
	cookieSecret [32]byte,
	tlsCertFilename, tlsKeyFilename string,
	devMode bool,
) (Transport, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	var ownURL string
	if reachableAt != "" {
		ownURL = reachableAt
	} else {
		ownURL = "http://" + listenAddr
		if len(listenAddr) > 0 && listenAddr[0] == ':' {
			ownURL = "http://localhost" + listenAddr
		}
	}

	t := &httpTransport{
		Context:               &ctx.Context{},
		controllerHub:         controllerHub,
		listenAddr:            listenAddr,
		defaultStateURI:       defaultStateURI,
		cookieSecret:          cookieSecret,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		devMode:               devMode,
		cookieJar:             jar,
		pendingAuthorizations: make(map[types.ID][]byte),
		ownURL:                ownURL,
		keyStore:              keyStore,
		refStore:              refStore,
		peerStore:             peerStore,
	}
	return t, nil
}

func (t *httpTransport) Start() error {
	srv := &http.Server{}
	return t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel("http")
			t.Infof(0, "opening http transport at %v", t.listenAddr)

			if t.cookieSecret == [32]byte{} {
				_, err := rand.Read(t.cookieSecret[:])
				if err != nil {
					return err
				}
			}

			// Update our node's info in the peer store
			t.peerStore.AddDialInfos([]PeerDialInfo{{t.Name(), t.ownURL}})
			fmt.Println("Starting Server")
			go func() {
				if !t.devMode {
					srv = &http.Server{
						Addr:      t.listenAddr,
						Handler:   UnrestrictedCors(t),
						TLSConfig: &tls.Config{},
					}
					err := srv.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
					if err != nil {
						fmt.Printf("%+v\n", err.Error())
						panic("http transport failed to start")
					}
				} else {
					srv = &http.Server{
						Addr:    t.listenAddr,
						Handler: UnrestrictedCors(t),
					}
					err := srv.ListenAndServe()
					if err != nil {
						fmt.Sprintf("http transport failed to start: %+v", err.Error())
					}
				}
			}()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			err := srv.Shutdown(context.Background())

			if err != nil {
				fmt.Println(err)
			}
		},
	)
}

func (t *httpTransport) Name() string {
	return "http"
}

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

func (t *httpTransport) storeAltSvcHeaderPeers(h http.Header) {
	if altSvcHeader := h.Get("Alt-Svc"); altSvcHeader != "" {
		forEachAltSvcHeaderPeer(altSvcHeader, func(transportName, dialAddr string, metadata map[string]string) {
			t.peerStore.AddDialInfos([]PeerDialInfo{{transportName, dialAddr}})
		})
	}
}

func (t *httpTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if dump, _ := strconv.ParseBool(os.Getenv("HTTP_DUMP_REQUESTS")); dump {
		reqBytes, err := httputil.DumpRequest(r, true)
		if err != nil {
			t.Errorf("error dumping request body: %v", err)
		} else {
			t.Debugf("incoming HTTP request:\n%v", string(reqBytes))
		}
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
		altSvcHeader := t.makeAltSvcHeader(t.peerStore.AllDialInfos())
		w.Header().Set("Alt-Svc", altSvcHeader)

		// Similarly, if other peers give us Alt-Svc headers, track them
		t.storeAltSvcHeaderPeers(r.Header)
	}

	switch r.Method {
	case "HEAD":
		// This is mainly used to poll for new peers

	case "OPTIONS":
		// w.Header().Set("Access-Control-Allow-Headers", "State-URI")

	case "AUTHORIZE":

		// Address verification requests (2 kinds)
		if challengeMsgHex := r.Header.Get("Challenge"); challengeMsgHex != "" {
			// 1. Remote node is reaching out to us with a challenge, and we respond
			t.serveChallengeIdentityResponse(w, r, address, challengeMsgHex)
		} else {
			responseHex := r.Header.Get("Response")
			if responseHex == "" {
				// 2a. Remote node wants a challenge, so we send it
				t.serveChallengeIdentity(w, r, sessionID)
			} else {
				// 2b. Remote node wanted a challenge, this is their response
				t.serveChallengeIdentityCheckResponse(w, r, sessionID, responseHex)
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

	case "POST":
		if r.Header.Get("Ref") == "true" {
			t.servePostRef(w, r)
		}

	case "ACK":
		t.serveAck(w, r, address)

	case "PUT":
		if r.Header.Get("Private") == "true" {
			t.servePostPrivateTx(w, r, address)
		} else {
			t.servePostTx(w, r, address)
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (t *httpTransport) serveChallengeIdentityResponse(w http.ResponseWriter, r *http.Request, address types.Address, challengeMsgHex string) {
	challengeMsg, err := hex.DecodeString(challengeMsgHex)
	if err != nil {
		http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
		return
	}

	peer := t.makePeerWithAddress(w, nil, address)
	err = t.host.HandleChallengeIdentity([]byte(challengeMsg), peer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Respond to a request from another node that wants us to issue them an identity
// challenge.  This is used with browser nodes which we cannot reach out to directly.
func (t *httpTransport) serveChallengeIdentity(w http.ResponseWriter, r *http.Request, sessionID types.ID) {
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

// Check the response from another node that requested an identity challenge from us.
// This is used with browser nodes which we cannot reach out to directly.
func (t *httpTransport) serveChallengeIdentityCheckResponse(w http.ResponseWriter, r *http.Request, sessionID types.ID, responseHex string) {
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

	sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challenge), sig)
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

	// @@TODO: make the request include the encrypting pubkey as well
	t.peerStore.AddVerifiedCredentials(PeerDialInfo{"http", ""}, addr, sigpubkey, nil)

	delete(t.pendingAuthorizations, sessionID) // @@TODO: expiration/garbage collection for failed auths
}

func (t *httpTransport) serveSubscription(w http.ResponseWriter, r *http.Request, address types.Address) {
	// @@TODO: ensure we actually have this stateURI
	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		stateURI = t.defaultStateURI
	}

	t.Infof(0, "incoming subscription (address: %v, state uri: %v)", address, stateURI)

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
	w.Header().Set("Transfer-Encoding", "chunked")

	keypath := r.Header.Get("Keypath")
	subscriptionTypeStr := r.Header.Get("Subscribe")

	var subscriptionType SubscriptionType
	err := subscriptionType.UnmarshalText([]byte(subscriptionTypeStr))
	if err != nil {
		http.Error(w, fmt.Sprintf("could not parse Subscribe header: %v", err), http.StatusBadRequest)
		return
	}

	httpWriteSub := &httpWritableSubscription{t.makePeerWithAddress(w, f, address)}

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		defer httpWriteSub.Close()
		<-notify
		t.Infof(0, "http subscription closed (%v)", stateURI)
	}()

	f.Flush()

	var fetchHistoryOpts *FetchHistoryOpts
	if fromTxHeader := r.Header.Get("From-Tx"); fromTxHeader != "" {
		fromTxID, err := types.IDFromHex(fromTxHeader)
		if err != nil {
			http.Error(w, "could not parse From-Tx header", http.StatusBadRequest)
			return
		}
		fetchHistoryOpts = &FetchHistoryOpts{FromTxID: fromTxID}
	}

	writeSub := newWritableSubscription(t.host, stateURI, tree.Keypath(keypath), subscriptionType, httpWriteSub)
	t.host.HandleWritableSubscriptionOpened(writeSub, fetchHistoryOpts)

	// Block until the subscription is canceled so that net/http doesn't close the connection
	<-writeSub.chDone
}

func (t *httpTransport) serveBraidJS(w http.ResponseWriter, r *http.Request) {
	f, err := pkger.Open("/braidjs/dist/braid.js")
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

	tx, err := t.controllerHub.FetchTx(stateURI, txID)
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
		leaves, err := t.controllerHub.Leaves(stateURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusNotFound)
			return
		}
		if len(leaves) > 0 {
			leaf := leaves[0]

			tx, err := t.controllerHub.FetchTx(stateURI, leaf)
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
		} else {
			w.Header().Add("Parents", "")
		}
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

		state, err = t.controllerHub.QueryIndex(stateURI, version, keypath, tree.Keypath(indexName), indexArgKeypath, rng)
		if errors.Cause(err) == types.Err404 {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}

	} else {
		// State query
		state, err = t.controllerHub.StateAtVersion(stateURI, version)
		if err != nil {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		}
		defer state.Close()

		if raw {
			state = state.NodeAt(keypath, rng)

		} else {
			var exists bool
			state, exists, err = nelson.Seek(state, keypath, t.controllerHub)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			} else if !exists {
				http.Error(w, fmt.Sprintf("not found: %v", keypath), http.StatusNotFound)
				return
			}
			keypath = nil

			// state.DebugPrint(t.Successf, false, 0)

			state, err = state.CopyToMemory(keypath, rng)
			if errors.Cause(err) == types.Err404 {
				http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}

			state, anyMissing, err = nelson.Resolve(state, t.controllerHub)
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
		http.Error(w, fmt.Sprintf("error getting content type: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentType == "application/octet-stream" {
		contentType = GuessContentTypeFromFilename(string(keypath.Part(-1)))
	}
	w.Header().Set("Content-Type", contentType)

	contentLength, err := nelson.GetContentLength(state)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting content length: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	resourceLength, err := state.Length()
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting length: %+v", err), http.StatusInternalServerError)
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

	stateURI := r.Header.Get("State-URI")

	t.host.HandleAckReceived(stateURI, txID, t.makePeerWithAddress(w, nil, address))
}

func (t *httpTransport) servePostPrivateTx(w http.ResponseWriter, r *http.Request, address types.Address) {
	t.Infof(0, "incoming private tx")

	var encryptedTx EncryptedTx
	err := json.NewDecoder(r.Body).Decode(&encryptedTx)
	if err != nil {
		panic(err)
	}

	bs, err := t.keyStore.OpenMessageFrom(
		encryptedTx.RecipientAddress,
		crypto.EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
		encryptedTx.EncryptedPayload,
	)
	if err != nil {
		t.Errorf("error decrypting tx: %v", err)
		return
	}

	var tx Tx
	err = json.Unmarshal(bs, &tx)
	if err != nil {
		t.Errorf("error decoding tx: %v", err)
		return
	} else if encryptedTx.TxID != tx.ID {
		t.Errorf("private tx id does not match")
		return
	}

	t.host.HandleTxReceived(tx, t.makePeerWithAddress(w, nil, address))
}

type StoreRefResponse struct {
	SHA1 types.Hash `json:"sha1"`
	SHA3 types.Hash `json:"sha3"`
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

	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		stateURI = t.defaultStateURI
	}

	var attachment []byte
	var patchReader io.Reader

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(10000000)
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}

		_, header, err := r.FormFile("attachment")
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}
		file, err := header.Open()
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}
		defer file.Close()

		attachment, err = ioutil.ReadAll(file)
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}

		patchesStrs := r.MultipartForm.Value["patches"]
		if len(patchesStrs) > 0 {
			patchReader = strings.NewReader(patchesStrs[0])
		}

	} else {
		patchReader = r.Body
	}

	var patches []Patch
	if patchReader != nil {
		scanner := bufio.NewScanner(patchReader)
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
	}

	tx := Tx{
		ID:         txID,
		Parents:    parents,
		Sig:        sig,
		Patches:    patches,
		Attachment: attachment,
		StateURI:   stateURI,
		Checkpoint: checkpoint,
	}

	// @@TODO: remove .From entirely
	pubkey, err := crypto.RecoverSigningPubkey(tx.Hash(), sig)
	if err != nil {
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	tx.From = pubkey.Address()
	////////////////////////////////

	peer := t.makePeerWithAddress(w, nil, address)
	go t.host.HandleTxReceived(tx, peer)
}

func (t *httpTransport) makeAltSvcHeader(peerDialInfos []PeerDialInfo) string {
	var others []string
	for _, tuple := range peerDialInfos {
		others = append(others, fmt.Sprintf(`%s="%s"`, tuple.TransportName, tuple.DialAddr))
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

func (t *httpTransport) NewPeerConn(ctx context.Context, dialAddr string) (Peer, error) {
	if dialAddr == t.ownURL || strings.HasPrefix(dialAddr, "localhost") {
		return nil, errors.WithStack(ErrPeerIsSelf)
	}
	return t.makePeer(nil, nil, dialAddr), nil
}

func (t *httpTransport) ProvidersOfStateURI(ctx context.Context, stateURI string) (_ <-chan Peer, err error) {
	defer utils.WithStack(&err)

	providers, err := t.tryFetchProvidersFromAuthoritativeHost(stateURI)
	if err != nil {
		t.Warnf("could not fetch providers of state URI '%v' from authoritative host: %v", stateURI, err)
		return nil, err
	}

	u, err := url.Parse("http://" + stateURI)
	if err == nil {
		providers = append(providers, "http://"+u.Hostname())
	}

	ch := make(chan Peer)
	go func() {
		defer close(ch)
		for _, providerURL := range providers {
			if providerURL == t.ownURL {
				continue
			}

			select {
			case ch <- t.makePeer(nil, nil, providerURL):
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) tryFetchProvidersFromAuthoritativeHost(stateURI string) ([]string, error) {
	parts := strings.Split(stateURI, "/")

	u, err := url.Parse("http://" + parts[0] + ":80?state_uri=" + stateURI)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "providers")

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.Errorf("got status code %v %v: %v", resp.StatusCode, resp.Status, string(body))
	}

	var providers []string
	err = json.NewDecoder(resp.Body).Decode(&providers)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

func (t *httpTransport) ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *httpTransport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *httpTransport) AnnounceRef(ctx context.Context, refID types.RefID) error {
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

	publicIdentity, err := t.keyStore.DefaultPublicIdentity()
	if err != nil {
		return err
	}

	sig, err := t.keyStore.SignHash(publicIdentity.Address(), types.HashBytes(append(value, t.cookieSecret[:]...)))
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
	}

	publicIdentity, err := t.keyStore.DefaultPublicIdentity()
	if err != nil {
		return nil, err
	}

	valid, err := t.keyStore.VerifySignature(publicIdentity.Address(), types.HashBytes(append(value, t.cookieSecret[:]...)), sig)
	if err != nil {
		return nil, err
	} else if !valid {
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

func (t *httpTransport) makePeer(writer io.Writer, flusher http.Flusher, dialAddr string) *httpPeer {
	peer := &httpPeer{t: t}
	peer.stream.Writer = writer
	peer.stream.Flusher = flusher

	peerDetails := t.peerStore.PeerWithDialInfo(PeerDialInfo{t.Name(), dialAddr})
	if peerDetails != nil && peerDetails != PeerDetails(nil) {
		peer.PeerDetails = peerDetails
	} else {
		t.peerStore.AddDialInfos([]PeerDialInfo{{t.Name(), dialAddr}})
		peer.PeerDetails = t.peerStore.PeerWithDialInfo(PeerDialInfo{t.Name(), dialAddr})
	}
	return peer
}

func (t *httpTransport) makePeerWithAddress(writer io.Writer, flusher http.Flusher, address types.Address) *httpPeer {
	peer := &httpPeer{t: t}
	peer.stream.Writer = writer
	peer.stream.Flusher = flusher

	peerDetails := t.peerStore.PeersFromTransportWithAddress(t.Name(), address)
	if len(peerDetails) > 0 {
		peer.PeerDetails = peerDetails[0]
	} else {
		t.peerStore.AddVerifiedCredentials(PeerDialInfo{TransportName: t.Name()}, address, nil, nil)
		peerDetails := t.peerStore.PeersFromTransportWithAddress(t.Name(), address)
		peer.PeerDetails = peerDetails[0]
	}
	return peer
}

func (t *httpTransport) doRequest(req *http.Request) (*http.Response, error) {
	altSvcHeader := t.makeAltSvcHeader(t.peerStore.AllDialInfos())
	req.Header.Set("Alt-Svc", altSvcHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	t.storeAltSvcHeaderPeers(resp.Header)

	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		bs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = errors.Wrapf(err, "error reading response body")
			err2 := errors.Errorf("http request errored: (%v) %v", resp.StatusCode, resp.Status)
			return resp, multierr.Append(err, err2)
		}
		return resp, errors.Errorf("http request errored: (%v) %v: %v", resp.StatusCode, resp.Status, string(bs))
	}
	return resp, nil
}

type httpPeer struct {
	PeerDetails

	t *httpTransport
	sync.Mutex

	// stream
	stream struct {
		io.ReadCloser
		io.Writer
		http.Flusher
	}
}

func (p *httpPeer) Transport() Transport {
	return p.t
}

func (p *httpPeer) EnsureConnected(ctx context.Context) error {
	return nil
}

func (p *httpPeer) Subscribe(ctx context.Context, stateURI string) (_ ReadableSubscription, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		return nil, errors.New("peer has no DialAddr")
	}

	req, err := http.NewRequest("GET", p.DialInfo().DialAddr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("State-URI", stateURI)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	subTypeBytes, err := SubscriptionType_Txs.MarshalText()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Subscribe", string(subTypeBytes))

	var client http.Client
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error subscribing to peer (%v) (state URI: %v)", p.DialInfo().DialAddr, stateURI)
	} else if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "error subscribing to peer (%v) (state URI: %v)", p.DialInfo().DialAddr, stateURI)
	}

	p.t.storeAltSvcHeaderPeers(resp.Header)

	return &httpReadableSubscription{
		client:  &client,
		peer:    p,
		stream:  resp.Body,
		private: resp.Header.Get("Private") == "true",
	}, nil
}

func (p *httpPeer) Put(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	identity, err := p.t.keyStore.IdentityWithAddress(tx.From)
	if err != nil {
		return err
	}

	var (
		peerSigPubkey crypto.SigningPublicKey
		peerEncPubkey crypto.EncryptingPublicKey
	)
	if tx.IsPrivate() {
		peerAddrs := types.OverlappingAddresses(tx.Recipients, p.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}
		peerSigPubkey, peerEncPubkey = p.PublicKeys(peerAddrs[0])
	}

	req, err := PutRequestFromTx(ctx, tx, p.DialInfo().DialAddr, identity.Encrypting, peerSigPubkey.Address(), peerEncPubkey)
	if err != nil {
		return errors.WithStack(err)
	}

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error PUTting tx to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *httpPeer) Ack(stateURI string, txID types.ID) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	txIDBytes, err := txID.MarshalText()
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "ACK", p.DialInfo().DialAddr, bytes.NewReader(txIDBytes))
	if err != nil {
		return err
	}
	req.Header.Set("State-URI", stateURI)

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error ACKing to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *httpPeer) ChallengeIdentity(challengeMsg types.ChallengeMsg) (err error) {
	defer utils.WithStack(&err)
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "AUTHORIZE", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Challenge", hex.EncodeToString(challengeMsg))

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error verifying peer address (%v)", p.DialInfo().DialAddr)
	}
	p.stream.ReadCloser = resp.Body
	return nil
}

func (p *httpPeer) ReceiveChallengeIdentityResponse() (_ []ChallengeIdentityResponse, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	var verifyResp []ChallengeIdentityResponse
	err = json.NewDecoder(p.stream.ReadCloser).Decode(&verifyResp)
	if err != nil {
		return nil, err
	}
	return verifyResp, nil
}

func (p *httpPeer) RespondChallengeIdentity(verifyAddressResponse []ChallengeIdentityResponse) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	err = json.NewEncoder(p.stream.Writer).Encode(verifyAddressResponse)
	if err != nil {
		http.Error(p.stream.Writer.(http.ResponseWriter), err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}

func (p *httpPeer) FetchRef(refID types.RefID) error {
	return types.ErrUnimplemented
}

func (p *httpPeer) SendRefHeader() error {
	return types.ErrUnimplemented
}

func (p *httpPeer) SendRefPacket(data []byte, end bool) error {
	return types.ErrUnimplemented
}

func (p *httpPeer) ReceiveRefHeader() (FetchRefResponseHeader, error) {
	return FetchRefResponseHeader{}, types.ErrUnimplemented
}

func (p *httpPeer) ReceiveRefPacket() (FetchRefResponseBody, error) {
	return FetchRefResponseBody{}, types.ErrUnimplemented
}

func (p *httpPeer) AnnouncePeers(ctx context.Context, peerDialInfos []PeerDialInfo) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	if p.DialInfo().DialAddr == "" {
		p.t.Warn("peer has no DialAddr")
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", p.DialInfo().DialAddr, nil)
	if err != nil {
		return err
	}

	resp, err := p.t.doRequest(req)
	if err != nil {
		return errors.Wrapf(err, "error announcing peers to peer (%v)", p.DialInfo().DialAddr)
	}
	defer resp.Body.Close()
	return nil
}

func (p *httpPeer) Close() error {
	if p.stream.ReadCloser != nil {
		return p.stream.ReadCloser.Close()
	}
	return nil
}

type httpReadableSubscription struct {
	client  *http.Client
	stream  io.ReadCloser
	peer    *httpPeer
	private bool
}

func (s *httpReadableSubscription) Read() (_ *SubscriptionMsg, err error) {
	defer func() { s.peer.UpdateConnStats(err == nil) }()

	r := bufio.NewReader(s.stream)
	bs, err := r.ReadBytes(byte('\n'))
	if err != nil {
		return nil, err
	}
	bs = bytes.TrimPrefix(bs, []byte("data: "))
	bs = bytes.Trim(bs, "\n ")

	var msg SubscriptionMsg
	err = json.Unmarshal(bs, &msg)
	if err != nil {
		return nil, err
	}

	if s.private {
		if msg.EncryptedTx == nil {
			return nil, errors.New("no encrypted tx sent by http peer")
		}

		bs, err = s.peer.t.keyStore.OpenMessageFrom(
			msg.EncryptedTx.RecipientAddress,
			crypto.EncryptingPublicKeyFromBytes(msg.EncryptedTx.SenderPublicKey),
			msg.EncryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, err
		}

		var tx Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return nil, err
		}
		msg.Tx = &tx
		return &msg, nil

	} else {
		if msg.Tx == nil {
			return nil, errors.New("no tx sent by http peer")
		}
		return &msg, nil
	}
}

func (c *httpReadableSubscription) Close() error {
	c.client.CloseIdleConnections()
	return c.peer.Close()
}

type httpWritableSubscription struct {
	*httpPeer
}

var _ WritableSubscriptionImpl = (*httpWritableSubscription)(nil)

func (sub *httpWritableSubscription) Put(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) (err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	var msg *SubscriptionMsg
	if tx.IsPrivate() {
		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return errors.WithStack(err)
		}

		peerAddrs := types.OverlappingAddresses(tx.Recipients, sub.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}

		peerSigPubkey, peerEncPubkey := sub.PublicKeys(peerAddrs[0])

		var ownIdentity identity.Identity
		for _, addr := range tx.Recipients {
			ownIdentity, err = sub.t.keyStore.IdentityWithAddress(addr)
			if err != nil {
				return err
			}
			if ownIdentity != (identity.Identity{}) {
				break
			}
		}
		if ownIdentity == (identity.Identity{}) {
			return errors.New("private tx Recipients field must contain own address")
		}

		encryptedTxBytes, err := sub.t.keyStore.SealMessageFor(ownIdentity.Address(), peerEncPubkey, marshalledTx)
		if err != nil {
			return errors.WithStack(err)
		}

		etx := &EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  ownIdentity.Encrypting.EncryptingPublicKey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}

		msg = &SubscriptionMsg{EncryptedTx: etx, Leaves: leaves}
	} else {
		msg = &SubscriptionMsg{Tx: tx, State: state, Leaves: leaves}
	}

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// This is encoded using HTTP's SSE format
	event := []byte("data: " + string(bs) + "\n\n")

	n, err := sub.stream.Writer.Write(event)
	if err != nil {
		return err
	} else if n < len(event) {
		return errors.New("error writing message to http peer: didn't write enough")
	}
	return nil
}
