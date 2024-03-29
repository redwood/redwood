package braidhttp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	_ "embed"
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
	"strconv"
	"strings"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/net/publicsuffix"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/embed"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type Transport interface {
	process.Interface
	swarm.Transport
	protoauth.AuthTransport
	protoblob.BlobTransport
	prototree.TreeTransport
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type transport struct {
	process.Process
	log.Logger
	protoauth.BaseAuthTransport
	protoblob.BaseBlobTransport
	prototree.BaseTreeTransport

	defaultStateURI string
	ownURLs         types.Set[string]
	listenAddr      string
	listenAddrSSL   string
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	tlsCerts        []tls.Certificate
	cookieJar       http.CookieJar
	jwtSecret       []byte
	devMode         bool

	srv        *http.Server
	srvTLS     *http.Server
	httpClient *utils.HTTPClient

	pendingAuthorizations map[types.ID][]byte

	treeACL        prototree.ACL
	peerStore      swarm.PeerStore
	keyStore       identity.KeyStore
	blobStore      blob.Store
	controllerHub  tree.ControllerHub
	nelsonResolver nelson.Resolver
}

var _ Transport = (*transport)(nil)

const (
	TransportName string = "braidhttp"
)

func NewTransport(
	listenAddr string,
	listenAddrSSL string,
	ownURLs types.Set[string],
	defaultStateURI string,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
	tlsCertFilename, tlsKeyFilename string,
	tlsCerts []tls.Certificate,
	jwtSecret []byte,
	devMode bool,
) (*transport, error) {
	t := &transport{
		Process:               *process.New(TransportName),
		Logger:                log.NewLogger(TransportName),
		controllerHub:         controllerHub,
		listenAddr:            listenAddr,
		listenAddrSSL:         listenAddrSSL,
		defaultStateURI:       defaultStateURI,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		tlsCerts:              tlsCerts,
		jwtSecret:             jwtSecret,
		devMode:               devMode,
		pendingAuthorizations: make(map[types.ID][]byte),
		ownURLs:               ownURLs,
		keyStore:              keyStore,
		blobStore:             blobStore,
		peerStore:             peerStore,
		nelsonResolver:        nelson.NewResolver(controllerHub, blobStore, nil),
	}
	return t, nil
}

func (t *transport) Start() error {
	err := t.Process.Start()
	if err != nil {
		return err
	}

	t.Infof(0, "opening %v transport at %v", TransportName, t.listenAddr)
	t.Infof(0, "opening %v transport (ssl) at %v", TransportName, t.listenAddrSSL)

	t.cookieJar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}

	if len(t.listenAddrSSL) > 0 {
		if t.listenAddrSSL[0] == ':' {
			t.ownURLs.Add("https://localhost" + t.listenAddrSSL)
		} else {
			t.ownURLs.Add("https://" + t.listenAddrSSL)
		}
	}

	t.httpClient = utils.MakeHTTPClient(10*time.Second, 30*time.Second, t.cookieJar, t.tlsCerts)

	err = t.findOrCreateCookieSecret()
	if err != nil {
		return err
	}

	// Update our node's info in the peer store
	identities, err := t.keyStore.Identities()
	if err != nil {
		return err
	}
	for _, identity := range identities {
		for ownURL := range t.ownURLs {
			t.peerStore.AddVerifiedCredentials(
				swarm.PeerDialInfo{TransportName: TransportName, DialAddr: ownURL},
				"self",
				identity.SigKeypair.SigningPublicKey.Address(),
				identity.SigKeypair.SigningPublicKey,
				identity.AsymEncKeypair.AsymEncPubkey,
			)
		}
	}

	t.Process.Go(nil, "http server", func(ctx context.Context) {
		t.srvTLS = &http.Server{
			Addr:    t.listenAddrSSL,
			Handler: utils.UnrestrictedCors(t),
			TLSConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				MaxVersion:         tls.VersionTLS13,
				Certificates:       t.tlsCerts,
				ClientAuth:         tls.RequestClientCert,
				InsecureSkipVerify: true,
			},
		}
		go func() {
			t.Infof(0, "tls cert file: %v", t.tlsCertFilename)
			t.Infof(0, "tls key file: %v", t.tlsKeyFilename)
			err := t.srvTLS.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
			if err != nil {
				t.Errorf("while starting HTTPS server: %v", err)
			}
		}()

		t.srv = &http.Server{
			Addr:    t.listenAddr,
			Handler: utils.UnrestrictedCors(t),
		}
		go func() {
			err := t.srv.ListenAndServe()
			if err != nil {
				t.Errorf("while starting HTTP server: %v", err)
			}
		}()
	})

	return nil
}

func (t *transport) Close() error {
	t.Infof(0, "braidhttp transport shutting down")
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
	return t.Process.Close()
}

func (t *transport) Name() string {
	return TransportName
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

	sessionID, err := t.ensureSessionIDCookieOnResponse(w, r)
	if err != nil {
		t.Errorf("error reading sessionID cookie: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	address, err := t.addressFromRequest(r)
	if err != nil {
		t.Errorf("err: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	duID := sessionID.Hex()
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		duID = utils.DeviceIDFromX509Pubkey(r.TLS.PeerCertificates[0].PublicKey)
	}
	peerConn := t.makePeerConn(w, nil, "", sessionID, duID, address)

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
		// Without a path, this method is used to poll for new peers
		// Otherwise...
		if strings.HasPrefix(r.URL.Path, "/__blob/") {
			t.serveGetBlobMetadata(w, r)
		} else {
			t.serveHeadRequest(w, r)
		}

	case "OPTIONS":
		// w.Header().Set("Access-Control-Allow-Headers", "State-URI")

	case "AUTHORIZE":
		// Address verification requests (2 kinds)
		if challengeMsgHex := r.Header.Get("Challenge"); challengeMsgHex != "" {
			// 1. Remote node is reaching out to us with a challenge, and we respond
			t.serveChallengeIdentityResponse(w, r, peerConn, challengeMsgHex)
		} else {
			responseHex := r.Header.Get("Response")
			if responseHex == "" {
				// 2a. Remote node wants a challenge, so we send it
				t.serveChallengeIdentity(w, r, peerConn)
			} else {
				// 2b. Remote node wanted a challenge, this is their response
				t.serveChallengeIdentityCheckResponse(w, r, peerConn, responseHex)
			}
		}

	case "GET":
		if r.URL.Path == "/ws" {
			t.serveWSSubscription(w, r, sessionID, address)
		} else if r.Header.Get("Subscribe") != "" {
			t.serveHTTPSubscription(w, r, sessionID, address)
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
		if r.Header.Get("Blob") == "true" {
			t.servePostBlob(w, r)
		}

	case "ACK":
		t.serveAck(w, r, peerConn)

	case "PUT":
		if r.Header.Get("Private") == "true" {
			t.servePostPrivateTx(w, r, peerConn)
		} else {
			t.servePostTx(w, r, peerConn)
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (t *transport) findOrCreateCookieSecret() error {
	maybeSecret, exists, err := t.keyStore.ExtraUserData("http:cookiesecret")
	if err != nil {
		return err
	}

	cookieSecret, isBytes := maybeSecret.([]byte)
	if !exists || !isBytes {
		cookieSecret := make([]byte, 32)
		_, err := rand.Read(cookieSecret)
		if err != nil {
			return err
		}
	}
	copy(t.cookieSecret[:], cookieSecret)
	return t.keyStore.SaveExtraUserData("http:cookiesecret", t.cookieSecret)
}

func (t *transport) storeAltSvcHeaderPeers(h http.Header) {
	if altSvcHeader := h.Get("Alt-Svc"); altSvcHeader != "" {
		forEachAltSvcHeaderPeer(altSvcHeader, func(transportName, dialAddr string, metadata map[string]string) {
			t.peerStore.AddDialInfo(swarm.PeerDialInfo{transportName, dialAddr}, "")
		})
	}
}

func (t *transport) addressFromRequest(r *http.Request) (types.Address, error) {
	type request struct {
		JWT string `header:"Authorization" query:"ucan"`
	}
	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		return types.Address{}, err
	}

	claims, exists, err := utils.ParseJWT(req.JWT, t.jwtSecret)
	if err != nil {
		return types.Address{}, err
	} else if !exists {
		claims, exists, err = utils.ParseJWT(req.JWT, t.jwtSecret)
		if err != nil {
			return types.Address{}, err
		}
	}
	if exists {
		addrHex, ok := claims["address"].(string)
		if !ok {
			return types.Address{}, errors.Errorf("jwt does not contain 'address' claim")
		}
		addr, err := types.AddressFromHex(addrHex)
		if err != nil {
			return types.Address{}, errors.Wrapf(err, "jwt 'address' claim contains invalid data")
		}
		return addr, nil
	}

	addressBytes, err := t.signedCookie(r, "address")
	if errors.Cause(err) == errors.Err404 {
		return types.Address{}, nil
	} else if err != nil {
		return types.Address{}, err
	}
	return types.AddressFromBytes(addressBytes), nil
}

func (t *transport) serveHeadRequest(w http.ResponseWriter, r *http.Request) {
	type request struct {
		StateURI string        `header:"State-URI" query:"state_uri"`
		Keypath  state.Keypath `header:"Keypath"   query:"keypath"   path:""`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}
	if req.StateURI == "" {
		// Maybe the client doesn't care about state data (they just want the Alt-Svc header)
		return
	}

	isKnown, err := t.controllerHub.IsStateURIWithData(req.StateURI)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !isKnown {
		http.Error(w, "state URI not found", http.StatusNotFound)
		return
	}

	err = t.addParentsHeader(req.StateURI, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	node, err := t.controllerHub.StateAtVersion(req.StateURI, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
		return
	}
	defer node.Close()

	// @@TODO
}

// Respond to a request from another node challenging our identity.
func (t *transport) serveChallengeIdentityResponse(w http.ResponseWriter, r *http.Request, peerConn *peerConn, challengeMsgHex string) {
	challengeMsg, err := hex.DecodeString(challengeMsgHex)
	if err != nil {
		http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
		return
	}

	err = t.HandleChallengeIdentity([]byte(challengeMsg), peerConn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Respond to a request from another node that wants us to issue them an identity
// challenge.  This is used with browser nodes which we cannot reach out to directly.
func (t *transport) serveChallengeIdentity(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	challenge, err := protoauth.GenerateChallengeMsg()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.pendingAuthorizations[peerConn.sessionID] = challenge

	challengeHex := hex.EncodeToString(challenge)
	_, err = w.Write([]byte(challengeHex))
	if err != nil {
		t.Errorf("error writing challenge message: %v", err)
		return
	}
}

// Check the response from another node that requested an identity challenge from us.
// This is used with browser nodes which we cannot reach out to directly.
func (t *transport) serveChallengeIdentityCheckResponse(w http.ResponseWriter, r *http.Request, peerConn *peerConn, responseHex string) {
	challenge, exists := t.pendingAuthorizations[peerConn.sessionID]
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
	err = t.setSignedCookieOnResponse(w, "address", addr[:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// @@TODO: make the request include the encrypting pubkey as well
	t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName, ""}, peerConn.DeviceUniqueID(), addr, sigpubkey, nil)

	delete(t.pendingAuthorizations, peerConn.sessionID) // @@TODO: expiration/garbage collection for failed auths
}

func (t *transport) serveHTTPSubscription(w http.ResponseWriter, r *http.Request, sessionID types.ID, address types.Address) {
	type request struct {
		StateURI string                     `header:"State-URI" query:"state_uri"         required:"true"`
		Keypath  state.Keypath              `header:"Keypath"   query:"keypath"`
		SubType  prototree.SubscriptionType `header:"Subscribe" query:"subscription_type" required:"true"`
		FromTxID *state.Version             `header:"From-Tx"   query:"from_tx"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// @@TODO: ensure we actually have this stateURI?
	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	t.Infof(0, "incoming http subscription (address: %v, state uri: %v)", address, req.StateURI)

	var fetchHistoryOpts prototree.FetchHistoryOpts
	if req.FromTxID != nil {
		fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: *req.FromTxID}
	}

	subRequest := prototree.SubscriptionRequest{
		StateURI:         req.StateURI,
		Keypath:          state.Keypath(req.Keypath),
		Type:             req.SubType,
		FetchHistoryOpts: &fetchHistoryOpts,
		Addresses:        types.NewSet[types.Address]([]types.Address{address}),
	}
	chSubClosed, err := t.HandleWritableSubscriptionOpened(subRequest, func() (prototree.WritableSubscriptionImpl, error) {
		return newHTTPWritableSubscription(req.StateURI, w, r), nil
	})
	if errors.Cause(err) == errors.Err403 {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	} else if errors.Cause(err) == errors.Err404 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Block until the subscription is canceled so that net/http doesn't close the connection
	select {
	case <-chSubClosed:
	case <-t.Process.Done():
	}
}

func (t *transport) serveWSSubscription(w http.ResponseWriter, r *http.Request, sessionID types.ID, address types.Address) {
	type request struct {
		StateURI string                     `header:"State-URI" query:"state_uri"         required:"true"`
		Keypath  state.Keypath              `header:"Keypath"   query:"keypath"`
		SubType  prototree.SubscriptionType `header:"Subscribe" query:"subscription_type" required:"true"`
		FromTxID *state.Version             `header:"From-Tx"   query:"from_tx"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	var fetchHistoryOpts prototree.FetchHistoryOpts
	if req.FromTxID != nil {
		fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: *req.FromTxID}
	}

	subRequest := prototree.SubscriptionRequest{
		StateURI:         req.StateURI,
		Keypath:          req.Keypath,
		Type:             req.SubType,
		FetchHistoryOpts: &fetchHistoryOpts,
		Addresses:        types.NewSet[types.Address]([]types.Address{address}),
	}
	chSubClosed, err := t.HandleWritableSubscriptionOpened(subRequest, func() (prototree.WritableSubscriptionImpl, error) {
		wsConn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return nil, err
		}
		return newWSWritableSubscription(req.StateURI, wsConn, []types.Address{address}, t), nil
	})
	if errors.Cause(err) == errors.Err403 {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	} else if errors.Cause(err) == errors.Err404 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Block until the subscription is canceled so that net/http doesn't close the connection
	select {
	case <-chSubClosed:
	case <-t.Process.Done():
	}
}

func (t *transport) serveRedwoodJS(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(w, r, "./redwood.js", time.Now(), bytes.NewReader(embed.BrowserSrc))
}

func (t *transport) serveGetTx(w http.ResponseWriter, r *http.Request) {
	type request struct {
		StateURI string `header:"State-URI" query:"state_uri" required:"true"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parts := strings.Split(r.URL.Path[1:], "/")
	txIDStr := parts[1]
	txID, err := state.VersionFromHex(txIDStr)
	if err != nil {
		http.Error(w, "bad tx id", http.StatusBadRequest)
		return
	}

	tx, err := t.controllerHub.FetchTx(req.StateURI, txID)
	if err != nil {
		http.Error(w, fmt.Sprintf("not found: %v", err), http.StatusNotFound)
		return
	}

	utils.RespondJSON(w, tx)
}

func (t *transport) serveGetState(w http.ResponseWriter, r *http.Request) {
	var req resourceRequest
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	var rangeReq *RangeRequest
	if req.RangeHeader != nil {
		rangeReq = req.RangeHeader
	} else {
		rangeReq = req.KeypathAndRange.RangeRequest
	}

	// @@TODO
	// var shouldSendBody bool
	// if req.Method == "GET" {
	//     shouldSendBody = true
	// } else if req.Method == "HEAD" {
	//     shouldSendBody = false
	// } else {
	//     panic("no")
	// }

	rootNode, err := t.controllerHub.StateAtVersion(req.StateURI, req.Version)
	if err != nil {
		http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
		return
	}
	defer rootNode.Close()

	node, exists, err := t.nelsonResolver.Seek(rootNode, req.KeypathAndRange.Keypath)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
		return
	} else if !exists {
		http.Error(w, fmt.Sprintf("not found: %v", req.KeypathAndRange.Keypath), http.StatusNotFound)
		return
	}

	if !req.Raw {
		indexHTMLExists, err := node.Exists(state.Keypath("index.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}
		if indexHTMLExists {
			node, exists, err = t.nelsonResolver.Seek(node, state.Keypath("index.html"))
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			} else if !exists {
				http.Error(w, fmt.Sprintf("not found: index.html"), http.StatusNotFound)
				return
			}
		}
	}

	var response resourceResponse

	if req.Raw {
		readCloser, contentLength, err := jsonReadCloser(node.NodeAt(nil, nil), rangeReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}
		response.Body = readCloser
		response.ContentType = "application/json"
		response.ContentLength = contentLength
		response.StatusCode = http.StatusOK

	} else {
		node, anyMissing, err := t.nelsonResolver.Resolve(node)
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}

		switch node := node.(type) {
		case nelson.Node:
			readCloser, contentLength, err := node.BytesReader(rangeReq.BytesRange())
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}
			response.Body = readCloser
			response.ContentType = node.ContentType()
			if response.ContentType == "" {
				newReadCloser, contentType, err := utils.SniffContentType(string(node.Keypath().Part(-1)), readCloser)
				if err != nil {
					http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
					return
				}
				response.Body = newReadCloser
				response.ContentType = contentType
			}
			response.ContentLength = contentLength

		case state.Node:
			readCloser, contentLength, err := jsonReadCloser(node.NodeAt(nil, nil), rangeReq)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}
			response.Body = readCloser
			response.ContentType = "application/json"
			response.ContentLength = contentLength

		default:
			panic("wat")
		}

		if anyMissing {
			response.StatusCode = http.StatusPartialContent
		} else {
			response.StatusCode = http.StatusOK
		}
	}
	defer response.Body.Close()

	resourceLength, err := node.Length()
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
		return
	}
	response.ResourceLength = resourceLength

	// Add the "Parents" header
	{
		leaves, err := t.controllerHub.Leaves(req.StateURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}
		var leavesStrs []string
		for _, leafID := range leaves {
			leavesStrs = append(leavesStrs, leafID.Hex())
		}
		w.Header().Add("Parents", strings.Join(leavesStrs, ","))
	}

	// Add resource headers
	{
		w.Header().Set("State-URI", req.StateURI)
		w.Header().Set("Content-Type", response.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))
		w.Header().Set("Resource-Length", strconv.FormatUint(response.ResourceLength, 10))
		w.WriteHeader(response.StatusCode)
	}

	_, err = io.Copy(w, response.Body)
	if err != nil {
		http.Error(w, "could not copy response", http.StatusInternalServerError)
	}
}

func (t *transport) serveAck(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	defer r.Body.Close()

	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	var txID state.Version
	err = txID.UnmarshalText(bs)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	stateURI := r.Header.Get("State-URI")

	t.HandleAckReceived(stateURI, txID, peerConn)
}

func (t *transport) serveGetBlobMetadata(w http.ResponseWriter, r *http.Request) {
	blobIDStr := r.URL.Path[len("/__blob/"):]
	var blobID blob.ID
	err := blobID.UnmarshalText([]byte(blobIDStr))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	have, err := t.blobStore.HaveBlob(blobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if !have {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (t *transport) servePostBlob(w http.ResponseWriter, r *http.Request) {
	t.Infof(0, "incoming blob")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("blob")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	sha1Hash, sha3Hash, err := t.blobStore.StoreBlob(file)
	if err != nil {
		t.Errorf("error storing blob: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, StoreBlobResponse{SHA1: sha1Hash, SHA3: sha3Hash})
}

func (t *transport) servePostTx(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	type request struct {
		StateURI   string          `header:"State-URI"`
		Version    state.Version   `header:"Version"`
		Parents    ParentsHeader   `header:"Parents"`
		Checkpoint bool            `header:"Checkpoint"`
		Signature  types.Signature `header:"Signature"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t.Infof(0, "incoming tx %v", utils.PrettyJSON(req))

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}
	if req.Version == (state.Version{}) {
		req.Version = state.RandomVersion()
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
			var patch tree.Patch
			err := patch.UnmarshalText([]byte(line))
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
		ID:         req.Version,
		Parents:    req.Parents.Slice(),
		Patches:    patches,
		Attachment: attachment,
		StateURI:   req.StateURI,
		Checkpoint: req.Checkpoint,
	}

	// If no signature was provided, we're going through the UCAN
	// mechanism. Let prototree sign the tx for the requester.
	if len(req.Signature) == 0 {
		myAddrs, err := t.keyStore.Addresses()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if myAddrs.Contains(peerConn.addressFromRequest) {
			tx.From = peerConn.addressFromRequest
			t.HandleTxReceived(tx, peerConn)
			return

		} else {
			http.Error(w, fmt.Sprintf("bad signature, no local address (%v) matches UCAN %v", myAddrs, peerConn.addressFromRequest.Hex()), http.StatusBadRequest)
			return
		}
	}

	// @@TODO: remove .From entirely?
	pubkey, err := crypto.RecoverSigningPubkey(tx.Hash(), req.Signature)
	if err != nil {
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	tx.From = pubkey.Address()
	tx.Sig = req.Signature
	////////////////////////////////

	t.Process.Go(nil, "HandleTxReceived", func(ctx context.Context) {
		t.HandleTxReceived(tx, peerConn)
	})
}

func (t *transport) servePostPrivateTx(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	t.Infof(0, "incoming private tx")

	var encryptedTx prototree.EncryptedTx
	err := json.NewDecoder(r.Body).Decode(&encryptedTx)
	if err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	t.HandlePrivateTxReceived(encryptedTx, peerConn)
}

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	if t.ownURLs.Contains(dialAddr) || strings.HasPrefix(dialAddr, "localhost") {
		return nil, errors.WithStack(swarm.ErrPeerIsSelf)
	}
	return t.makePeerConn(nil, nil, dialAddr, types.ID{}, "", types.Address{}), nil
}

func (t *transport) AnnounceStateURIs(ctx context.Context, stateURIs types.Set[string]) {}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (_ <-chan prototree.TreePeerConn, err error) {
	defer errors.AddStack(&err)

	providers, err := t.tryFetchProvidersFromAuthoritativeHost(stateURI)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse("http://" + stateURI)
	if err == nil {
		providers = append(providers, "http://"+u.Hostname())
	}

	ch := make(chan prototree.TreePeerConn)
	t.Process.Go(nil, "ProvidersOfStateURI", func(ctx context.Context) {
		defer close(ch)
		for _, providerURL := range providers {
			if t.ownURLs.Contains(providerURL) {
				continue
			}

			select {
			case ch <- t.makePeerConn(nil, nil, providerURL, types.ID{}, "", types.Address{}):
			case <-ctx.Done():
			}
		}
	})
	return ch, nil
}

func (t *transport) tryFetchProvidersFromAuthoritativeHost(stateURI string) ([]string, error) {
	parts := strings.Split(stateURI, "/")

	u, err := url.Parse("http://" + parts[0] + ":80?state_uri=" + stateURI)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "providers")

	ctx, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
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

func (t *transport) ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	return nil, errors.ErrUnimplemented
}

func (t *transport) AnnounceBlobs(ctx context.Context, blobIDs types.Set[blob.ID]) {}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *transport) ensureSessionIDCookieOnResponse(w http.ResponseWriter, r *http.Request) (types.ID, error) {
	sessionIDBytes, err := t.signedCookie(r, "sessionid")
	if err != nil {
		// t.Errorf("error reading signed sessionid cookie: %v", err)
		return t.setSessionIDCookieOnResponse(w)
	}
	return types.IDFromBytes(sessionIDBytes), nil
}

func (t *transport) setSessionIDCookieOnResponse(w http.ResponseWriter) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookieOnResponse(w, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *transport) setSessionIDCookieOnRequest(r *http.Request) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookieOnRequest(r, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *transport) setSignedCookieOnResponse(w http.ResponseWriter, name string, value []byte) error {
	w.Header().Del("Set-Cookie")

	cookie, err := t.makeSignedCookie(name, value)
	if err != nil {
		return err
	}
	http.SetCookie(w, cookie)
	return nil
}

func (t *transport) setSignedCookieOnRequest(r *http.Request, name string, value []byte) error {
	cookie, err := t.makeSignedCookie(name, value)
	if err != nil {
		return err
	}
	r.AddCookie(cookie)
	return nil
}

func (t *transport) makeSignedCookie(name string, value []byte) (*http.Cookie, error) {
	publicIdentity, err := t.keyStore.DefaultPublicIdentity()
	if err != nil {
		return nil, err
	}

	sig, err := t.keyStore.SignHash(publicIdentity.Address(), types.HashBytes(append(value, t.cookieSecret[:]...)))
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:    name,
		Value:   hex.EncodeToString(value) + ":" + hex.EncodeToString(sig),
		Expires: time.Now().AddDate(0, 0, 1),
		Path:    "/",
	}, nil
}

func (t *transport) signedCookie(r *http.Request, name string) ([]byte, error) {
	cookie, err := r.Cookie(name)
	if errors.Cause(err) == http.ErrNoCookie {
		return nil, errors.Err404
	} else if err != nil {
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

func (t *transport) makePeerConn(writer io.Writer, flusher http.Flusher, dialAddr string, sessionID types.ID, deviceUniqueID string, address types.Address) *peerConn {
	peer := &peerConn{t: t, sessionID: sessionID, addressFromRequest: address}
	peer.stream.Writer = writer
	peer.stream.Flusher = flusher

	if !address.IsZero() {
		peer.PeerEndpoint = t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName, dialAddr}, deviceUniqueID, address, nil, nil)
	}
	peer.PeerEndpoint = t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, dialAddr}, deviceUniqueID)
	return peer
}

func (t *transport) addParentsHeader(stateURI string, w http.ResponseWriter) error {
	leaves, err := t.controllerHub.Leaves(stateURI)
	if err != nil {
		return err
	}
	var leavesStrs []string
	for _, leafID := range leaves {
		leavesStrs = append(leavesStrs, leafID.Hex())
	}
	w.Header().Add("Parents", strings.Join(leavesStrs, ","))
	return nil
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
