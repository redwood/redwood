package braidhttp

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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/markbates/pkger"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/net/publicsuffix"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type transport struct {
	log.Logger
	chStop chan struct{}

	controllerHub   tree.ControllerHub
	defaultStateURI string
	ownURL          string
	listenAddr      string
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	cookieJar       http.CookieJar
	devMode         bool

	srv        *http.Server
	httpClient *utils.HTTPClient

	pendingAuthorizations map[types.ID][]byte

	host      swarm.Host
	peerStore swarm.PeerStore
	keyStore  identity.KeyStore
	blobStore blob.Store
}

const (
	TransportName string = "braidhttp"
)

func NewTransport(
	listenAddr string,
	reachableAt string,
	defaultStateURI string,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
	tlsCertFilename, tlsKeyFilename string,
	devMode bool,
) (swarm.Transport, error) {
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

	t := &transport{
		Logger:                log.NewLogger("http"),
		chStop:                make(chan struct{}),
		controllerHub:         controllerHub,
		listenAddr:            listenAddr,
		defaultStateURI:       defaultStateURI,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		devMode:               devMode,
		httpClient:            utils.MakeHTTPClient(10*time.Second, 30*time.Second),
		cookieJar:             jar,
		pendingAuthorizations: make(map[types.ID][]byte),
		ownURL:                ownURL,
		keyStore:              keyStore,
		blobStore:             blobStore,
		peerStore:             peerStore,
	}
	keyStore.OnLoadUser(t.onLoadUser)
	keyStore.OnSaveUser(t.onSaveUser)
	return t, nil
}

func (t *transport) onLoadUser(user identity.User) error {
	maybeSecret, exists := user.ExtraData("http:cookiesecret")
	cookieSecret, isBytes := maybeSecret.([]byte)
	if !exists || !isBytes {
		cookieSecret := make([]byte, 32)
		_, err := rand.Read(cookieSecret)
		if err != nil {
			return err
		}
	}
	copy(t.cookieSecret[:], cookieSecret)
	return nil
}

func (t *transport) onSaveUser(user identity.User) error {
	user.SaveExtraData("http:cookiesecret", t.cookieSecret)
	return nil
}

func (t *transport) Start() error {
	t.Infof(0, "opening http transport at %v", t.listenAddr)

	if t.cookieSecret == [32]byte{} {
		_, err := rand.Read(t.cookieSecret[:])
		if err != nil {
			return err
		}
	}

	// Update our node's info in the peer store
	identities, err := t.keyStore.Identities()
	if err != nil {
		return err
	}
	for _, identity := range identities {
		t.peerStore.AddVerifiedCredentials(
			swarm.PeerDialInfo{TransportName: TransportName, DialAddr: t.ownURL},
			identity.Signing.SigningPublicKey.Address(),
			identity.Signing.SigningPublicKey,
			identity.Encrypting.EncryptingPublicKey,
		)
	}

	go func() {
		if !t.devMode {
			t.srv = &http.Server{
				Addr:      t.listenAddr,
				Handler:   utils.UnrestrictedCors(t),
				TLSConfig: &tls.Config{},
			}
			err := t.srv.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
			if err != nil {
				fmt.Printf("%+v\n", err.Error())
				panic("http transport failed to start")
			}
		} else {
			t.srv = &http.Server{
				Addr:    t.listenAddr,
				Handler: utils.UnrestrictedCors(t),
			}
			err := t.srv.ListenAndServe()
			if err != nil {
				fmt.Sprintf("http transport failed to start: %+v", err.Error())
			}
		}
	}()

	return nil
}

func (t *transport) Close() {
	close(t.chStop)

	// Non-graceful
	err := t.srv.Close()
	if err != nil {
		t.Errorf("error closing http server: %v", err)
	}
	// Graceful
	// err = t.srv.Shutdown(context.Background())
	// if err != nil {
	//  fmt.Println(err)
	// }

	t.httpClient.Close()
}

func (t *transport) Name() string {
	return TransportName
}

func (t *transport) storeAltSvcHeaderPeers(h http.Header) {
	if altSvcHeader := h.Get("Alt-Svc"); altSvcHeader != "" {
		forEachAltSvcHeaderPeer(altSvcHeader, func(transportName, dialAddr string, metadata map[string]string) {
			t.peerStore.AddDialInfos([]swarm.PeerDialInfo{{transportName, dialAddr}})
		})
	}
}

func (t *transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		altSvcHeader := makeAltSvcHeader(t.peerStore.AllDialInfos())
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
		if r.Header.Get("Subscribe") != "" || r.URL.Path == "/ws" {
			t.serveSubscription(w, r, address)
		} else {
			if r.URL.Path == "/redwood.js" {
				// @@TODO: this is hacky
				t.serveRedwoodJS(w, r)
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

// Respond to a request from another node challenging our identity.
func (t *transport) serveChallengeIdentityResponse(w http.ResponseWriter, r *http.Request, address types.Address, challengeMsgHex string) {
	challengeMsg, err := hex.DecodeString(challengeMsgHex)
	if err != nil {
		http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
		return
	}

	peer := t.makePeerConn(w, nil, "", address)
	err = t.host.HandleChallengeIdentity([]byte(challengeMsg), peer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Respond to a request from another node that wants us to issue them an identity
// challenge.  This is used with browser nodes which we cannot reach out to directly.
func (t *transport) serveChallengeIdentity(w http.ResponseWriter, r *http.Request, sessionID types.ID) {
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
func (t *transport) serveChallengeIdentityCheckResponse(w http.ResponseWriter, r *http.Request, sessionID types.ID, responseHex string) {
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
	t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName, ""}, addr, sigpubkey, nil)

	delete(t.pendingAuthorizations, sessionID) // @@TODO: expiration/garbage collection for failed auths
}

func (t *transport) serveSubscription(w http.ResponseWriter, r *http.Request, address types.Address) {
	var (
		stateURI         string
		keypath          string
		subscriptionType swarm.SubscriptionType
		fetchHistoryOpts *swarm.FetchHistoryOpts
		innerWriteSub    swarm.WritableSubscriptionImpl
	)
	if r.URL.Path == "/ws" {
		stateURI = r.URL.Query().Get("state_uri")
		keypath = r.URL.Query().Get("keypath")
		subscriptionTypeStr := r.URL.Query().Get("subscription_type")
		fromTx := r.URL.Query().Get("from_tx")

		if stateURI == "" {
			stateURI = t.defaultStateURI
		}

		err := subscriptionType.UnmarshalText([]byte(subscriptionTypeStr))
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse Subscribe header: %v", err), http.StatusBadRequest)
			return
		}

		if fromTx != "" {
			fromTxID, err := types.IDFromHex(fromTx)
			if err != nil {
				http.Error(w, "could not parse From-Tx header", http.StatusBadRequest)
				return
			}
			fetchHistoryOpts = &swarm.FetchHistoryOpts{FromTxID: fromTxID}
		}

		t.Infof(0, "incoming websocket subscription (address: %v, state uri: %v)", address, stateURI)

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		wsWriteSub := &wsWritableSubscription{
			messages: utils.NewMailbox(300), // @@TODO: configurable?
			peerConn: t.makePeerConn(nil, nil, "", address),
			conn:     conn,
		}
		wsWriteSub.onAddMultiplexedSubscription =
			func(stateURI string, keypath state.Keypath, subscriptionType swarm.SubscriptionType, fetchHistoryOpts *swarm.FetchHistoryOpts) {
				writeSub := swarm.NewWritableSubscription(t.host, stateURI, keypath, subscriptionType, wsWriteSub)
				t.host.HandleWritableSubscriptionOpened(writeSub, fetchHistoryOpts)
			}
		innerWriteSub = wsWriteSub

		wsWriteSub.start()
		defer wsWriteSub.close()

	} else {
		// Make sure that the writer supports flushing
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// @@TODO: ensure we actually have this stateURI?
		stateURI = r.Header.Get("State-URI")
		if stateURI == "" {
			stateURI = t.defaultStateURI
		}

		t.Infof(0, "incoming subscription (address: %v, state uri: %v)", address, stateURI)

		keypath = r.Header.Get("Keypath")
		subscriptionTypeStr := r.Header.Get("Subscribe")

		err := subscriptionType.UnmarshalText([]byte(subscriptionTypeStr))
		if err != nil {
			http.Error(w, fmt.Sprintf("could not parse Subscribe header: %v", err), http.StatusBadRequest)
			return
		}

		if fromTxHeader := r.Header.Get("From-Tx"); fromTxHeader != "" {
			fromTxID, err := types.IDFromHex(fromTxHeader)
			if err != nil {
				http.Error(w, "could not parse From-Tx header", http.StatusBadRequest)
				return
			}
			fetchHistoryOpts = &swarm.FetchHistoryOpts{FromTxID: fromTxID}
		}

		// Set the headers related to event streaming
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		httpWriteSub := &httpWritableSubscription{t.makePeerConn(w, f, "", address), subscriptionType}
		innerWriteSub = httpWriteSub

		// Listen to the closing of the http connection via the CloseNotifier
		notify := w.(http.CloseNotifier).CloseNotify()
		go func() {
			defer httpWriteSub.Close()
			<-notify
			t.Infof(0, "http subscription closed (%v)", stateURI)
		}()

		f.Flush()
	}

	writeSub := swarm.NewWritableSubscription(t.host, stateURI, state.Keypath(keypath), subscriptionType, innerWriteSub)
	t.host.HandleWritableSubscriptionOpened(writeSub, fetchHistoryOpts)

	// Block until the subscription is canceled so that net/http doesn't close the connection
	select {
	case <-writeSub.Done():
	case <-t.chStop:
	}
}

func (t *transport) serveRedwoodJS(w http.ResponseWriter, r *http.Request) {
	f, err := pkger.Open("/redwood.js/dist/browser.js")
	if err != nil {
		http.Error(w, "can't find redwood.js", http.StatusNotFound)
		return
	}
	defer f.Close()
	http.ServeContent(w, r, "./redwood.js", time.Now(), f)
	return
}

func (t *transport) serveGetTx(w http.ResponseWriter, r *http.Request) {
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

	utils.RespondJSON(w, tx)
}

func (t *transport) serveGetState(w http.ResponseWriter, r *http.Request) {
	keypathStrs := utils.FilterEmptyStrings(strings.Split(r.URL.Path[1:], "/"))

	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		queryStateURI := r.URL.Query().Get("state_uri")
		if queryStateURI != "" {
			stateURI = queryStateURI
		} else {
			stateURI = t.defaultStateURI
		}
	}

	keypathStr := strings.Join(keypathStrs, string(state.KeypathSeparator))
	keypath := state.Keypath(keypathStr)

	parts := keypath.Parts()
	newParts := make([]state.Keypath, 0, len(parts))
	for _, part := range parts {
		if idx := part.IndexByte('['); idx > -1 {
			newParts = append(newParts, part[:idx])
			x, err := strconv.ParseInt(string(part[idx+1:len(part)-1]), 10, 64)
			if err != nil {
				http.Error(w, "bad slice index", http.StatusBadRequest)
				return
			}
			newParts = append(newParts, state.EncodeSliceIndex(uint64(x)))
		} else {
			newParts = append(newParts, part)
		}
	}
	keypath = state.JoinKeypaths(newParts, []byte("/"))

	var version *types.ID
	if vstr := r.Header.Get("Version"); vstr != "" {
		v, err := types.IDFromHex(vstr)
		if err != nil {
			http.Error(w, "bad Version header", http.StatusBadRequest)
			return
		}
		version = &v
	}

	var rng *state.Range
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
		rng = &state.Range{start, end}
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

	var node state.Node
	var anyMissing bool

	if indexName != "" {
		// Index query

		// You can specify an index_arg of * in order to fetch the entire index
		var indexArgKeypath state.Keypath
		if indexArg != "*" {
			indexArgKeypath = state.Keypath(indexArg)
		}

		node, err = t.controllerHub.QueryIndex(stateURI, version, keypath, state.Keypath(indexName), indexArgKeypath, rng)
		if errors.Cause(err) == types.Err404 {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}

	} else {
		// State query
		node, err = t.controllerHub.StateAtVersion(stateURI, version)
		if err != nil {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		}
		defer node.Close()

		if raw {
			node = node.NodeAt(keypath, rng)

		} else {
			var exists bool
			node, exists, err = nelson.Seek(node, keypath, t.controllerHub)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			} else if !exists {
				http.Error(w, fmt.Sprintf("not found: %v", keypath), http.StatusNotFound)
				return
			}
			keypath = nil

			node, err = node.CopyToMemory(keypath, rng)
			if errors.Cause(err) == types.Err404 {
				http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}

			node, anyMissing, err = nelson.Resolve(node, t.controllerHub)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}

			indexHTMLExists, err := node.Exists(keypath.Push(state.Keypath("index.html")))
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}
			if indexHTMLExists {
				keypath = keypath.Push(state.Keypath("index.html"))
				node = node.NodeAt(state.Keypath("index.html"), nil)
			}
		}
	}

	contentType, err := nelson.GetContentType(node)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting content type: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentType == "application/octet-stream" {
		contentType = utils.GuessContentTypeFromFilename(string(keypath.Part(-1)))
	}
	w.Header().Set("Content-Type", contentType)

	contentLength, err := nelson.GetContentLength(node)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting content length: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	resourceLength, err := node.Length()
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting length: %+v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Resource-Length", strconv.FormatUint(resourceLength, 10))

	var val interface{}
	var exists bool
	if !raw {
		val, exists, err = nelson.GetValueRecursive(node, nil, nil)
	} else {
		val, exists, err = node.Value(nil, nil)
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

func (t *transport) serveAck(w http.ResponseWriter, r *http.Request, address types.Address) {
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

	t.host.HandleAckReceived(stateURI, txID, t.makePeerConn(w, nil, "", address))
}

func (t *transport) servePostPrivateTx(w http.ResponseWriter, r *http.Request, address types.Address) {
	t.Infof(0, "incoming private tx")

	var encryptedTx swarm.EncryptedTx
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

	var tx tree.Tx
	err = json.Unmarshal(bs, &tx)
	if err != nil {
		t.Errorf("error decoding tx: %v", err)
		return
	} else if encryptedTx.TxID != tx.ID {
		t.Errorf("private tx id does not match")
		return
	}

	t.host.HandleTxReceived(tx, t.makePeerConn(w, nil, "", address))
}

func (t *transport) servePostRef(w http.ResponseWriter, r *http.Request) {
	t.Infof(0, "incoming ref")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("ref")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	sha1Hash, sha3Hash, err := t.blobStore.StoreObject(file)
	if err != nil {
		t.Errorf("error storing ref: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, swarm.StoreRefResponse{SHA1: sha1Hash, SHA3: sha3Hash})
}

func (t *transport) servePostTx(w http.ResponseWriter, r *http.Request, address types.Address) {
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

	var patches []tree.Patch
	if patchReader != nil {
		scanner := bufio.NewScanner(patchReader)
		for scanner.Scan() {
			line := scanner.Bytes()
			patch, err := tree.ParsePatch(line)
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

	tx := tree.Tx{
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

	peer := t.makePeerConn(w, nil, "", address)
	go t.host.HandleTxReceived(tx, peer)
}

func (t *transport) SetHost(h swarm.Host) {
	t.host = h
}

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.Peer, error) {
	if dialAddr == t.ownURL || strings.HasPrefix(dialAddr, "localhost") {
		return nil, errors.WithStack(swarm.ErrPeerIsSelf)
	}
	return t.makePeerConn(nil, nil, dialAddr, types.Address{}), nil
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (_ <-chan swarm.Peer, err error) {
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

	ch := make(chan swarm.Peer)
	go func() {
		defer close(ch)
		for _, providerURL := range providers {
			if providerURL == t.ownURL {
				continue
			}

			select {
			case ch <- t.makePeerConn(nil, nil, providerURL, types.Address{}):
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *transport) tryFetchProvidersFromAuthoritativeHost(stateURI string) ([]string, error) {
	parts := strings.Split(stateURI, "/")

	u, err := url.Parse("http://" + parts[0] + ":80?state_uri=" + stateURI)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "providers")

	ctx, cancel := utils.CombinedContext(10*time.Second, t.chStop)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.doRequest(req)
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

func (t *transport) ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan swarm.Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *transport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan swarm.Peer, error) {
	return nil, types.ErrUnimplemented
}

func (t *transport) AnnounceRef(ctx context.Context, refID types.RefID) error {
	return types.ErrUnimplemented
}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *transport) ensureSessionIDCookie(w http.ResponseWriter, r *http.Request) (types.ID, error) {
	sessionIDBytes, err := t.signedCookie(r, "sessionid")
	if err != nil {
		// t.Errorf("error reading signed sessionid cookie: %v", err)
		return t.setSessionIDCookie(w)
	}
	return types.IDFromBytes(sessionIDBytes), nil
}

func (t *transport) setSessionIDCookie(w http.ResponseWriter) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookie(w, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *transport) setSignedCookie(w http.ResponseWriter, name string, value []byte) error {
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

func (t *transport) signedCookie(r *http.Request, name string) ([]byte, error) {
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

func (t *transport) addressFromCookie(r *http.Request) types.Address {
	addressBytes, err := t.signedCookie(r, "address")
	if err != nil {
		return types.Address{}
	}
	return types.AddressFromBytes(addressBytes)
}

func (t *transport) makePeerConn(writer io.Writer, flusher http.Flusher, dialAddr string, address types.Address) *peerConn {
	peer := &peerConn{t: t}
	peer.stream.Writer = writer
	peer.stream.Flusher = flusher

	if !address.IsZero() {
		t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr}, address, nil, nil)
	} else if dialAddr != "" {
		t.peerStore.AddDialInfos([]swarm.PeerDialInfo{{TransportName, dialAddr}})
	}

	var pd swarm.PeerDetails
	if address.IsZero() && dialAddr != "" {
		pd = t.peerStore.PeerWithDialInfo(swarm.PeerDialInfo{TransportName, dialAddr})

	} else if !address.IsZero() {
		peerDetails := t.peerStore.PeersFromTransportWithAddress(TransportName, address)
		// @@TODO: choose the one we prefer intelligently?
		if len(peerDetails) > 0 {
			pd = peerDetails[0]
		}
	}
	if pd == nil || reflect.ValueOf(pd).IsNil() {
		peer.PeerDetails = swarm.NewEphemeralPeerDetails(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr})
	} else {
		peer.PeerDetails = pd
	}
	return peer
}

func (t *transport) doRequest(req *http.Request) (*http.Response, error) {
	altSvcHeader := makeAltSvcHeader(t.peerStore.AllDialInfos())
	req.Header.Set("Alt-Svc", altSvcHeader)

	resp, err := t.httpClient.Do(req)
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
