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
)

type httpTransport struct {
	*ctx.Context

	address         Address
	controller      Controller
	ownURL          string
	port            uint
	sigkeys         *SigningKeypair
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	cookieJar       http.CookieJar

	pendingAuthorizations map[ID][]byte

	fetchHistoryHandler  FetchHistoryHandler
	ackHandler           AckHandler
	txHandler            TxHandler
	privateTxHandler     PrivateTxHandler
	verifyAddressHandler VerifyAddressHandler
	fetchRefHandler      FetchRefHandler

	subscriptionsIn   map[string][]*httpSubscriptionIn
	subscriptionsInMu sync.RWMutex

	refStore  RefStore
	peerStore PeerStore
}

func NewHTTPTransport(addr Address, port uint, controller Controller, refStore RefStore, peerStore PeerStore, sigkeys *SigningKeypair, cookieSecret [32]byte, tlsCertFilename, tlsKeyFilename string) (Transport, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	t := &httpTransport{
		Context:               &ctx.Context{},
		address:               addr,
		subscriptionsIn:       make(map[string][]*httpSubscriptionIn),
		controller:            controller,
		port:                  port,
		sigkeys:               sigkeys,
		cookieSecret:          cookieSecret,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		cookieJar:             jar,
		pendingAuthorizations: make(map[ID][]byte),
		ownURL:                fmt.Sprintf("localhost:%v", port),
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
			t.Infof(0, "opening http transport at :%v", t.port)

			if t.cookieSecret == [32]byte{} {
				_, err := rand.Read(t.cookieSecret[:])
				if err != nil {
					return err
				}
			}

			go func() {
				//caCert, err := ioutil.ReadFile("client.crt")
				//if err != nil {
				//    log.Fatal(err)
				//}
				//caCertPool := x509.NewCertPool()
				//caCertPool.AppendCertsFromPEM(caCert)
				cfg := &tls.Config{
					// ClientAuth: tls.RequireAndVerifyClientCert,
					// ClientCAs:  caCertPool,
				}

				srv := &http.Server{
					Addr:      fmt.Sprintf(":%v", t.port),
					Handler:   t,
					TLSConfig: cfg,
				}
				err := srv.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
				if err != nil {
					fmt.Printf("%+v\n", err.Error())
					panic("http transport failed to start")
				}
			}()

			t.peerStore.AddReachableAddresses(t.Name(), NewStringSet([]string{t.ownURL}))

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

	case "AUTHORIZE":
		//
		// Address verification requests (2 kinds)
		//
		if challengeMsgHex := r.Header.Get("Challenge"); challengeMsgHex != "" {
			// 1. Remote node is reaching out to us with a challenge
			challengeMsg, err := hex.DecodeString(challengeMsgHex)
			if err != nil {
				http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
				return
			}

			peer := &httpPeer{address: address, t: t, Writer: w}
			err = t.verifyAddressHandler([]byte(challengeMsg), peer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		} else if responseHex := r.Header.Get("Response"); responseHex != "" {
			// 2b. Remote node wanted a challenge, this is their response
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

			sigpubkey, err := RecoverSigningPubkey(HashBytes(challenge), sig)
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

		} else {
			// 2a. Remote node wants a challenge, so we send it
			challenge, err := GenerateChallengeMsg()
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

	case "GET":
		if r.Header.Get("Subscribe") != "" {
			//
			// Subscription request
			//
			t.Infof(0, "incoming subscription (address: %v)", address)

			// Make sure that the writer supports flushing.
			f, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}

			// Listen to the closing of the http connection via the CloseNotifier
			notify := w.(http.CloseNotifier).CloseNotify()
			go func() {
				<-notify
				t.Errorf("http connection closed")
			}()

			// Set the headers related to event streaming.
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Transfer-Encoding", "chunked")

			sub := &httpSubscriptionIn{w, f, make(chan struct{}), make(chan struct{})}

			urlToSubscribe := r.Header.Get("State-URI")

			t.subscriptionsInMu.Lock()
			t.subscriptionsIn[urlToSubscribe] = append(t.subscriptionsIn[urlToSubscribe], sub)
			t.subscriptionsInMu.Unlock()

			f.Flush()

			parentsHeader := r.Header.Get("Parents")
			if parentsHeader != "" {
				var parents []ID
				parentStrs := strings.Split(parentsHeader, "/")
				for _, parentStr := range parentStrs {
					parent, err := IDFromHex(parentStr)
					if err != nil {
						// @@TODO: what to do?
						parents = []ID{GenesisTxID}
						break
					}
					parents = append(parents, parent)
				}

				var toVersion ID
				if versionHeader := r.Header.Get("Version"); versionHeader != "" {
					toVersion, err = IDFromHex(versionHeader)
					if err != nil {
						// @@TODO: what to do?
					}
				}

				err := t.fetchHistoryHandler(parents, toVersion, &httpPeer{address: address, t: t, Writer: w, Flusher: f})
				if err != nil {
					t.Errorf("error fetching history: %v", err)
					// @@TODO: close subscription?
				}
			}

			close(sub.chDoneCatchingUp)

			// Block until the subscription is canceled so that net/http doesn't close the connection
			<-sub.chDone

		} else {
			//
			// Regular HTTP GET request (from browsers, etc.)
			//

			// @@TODO: this is hacky
			if r.URL.Path == "/braid.js" {
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

			keypath := filterEmptyStrings(strings.Split(r.URL.Path[1:], "/"))

			if r.Header.Get("Accept") == "application/json" {
				resolveRefs, err := parseResolveRefsParam(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				val, _, err := t.controller.State(keypath, resolveRefs)
				if err != nil {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}

				// @@TODO: hacky
				var resp struct {
					MostRecentTxID ID          `json:"mostRecentTxID"`
					Data           interface{} `json:"data"`
				}
				resp.MostRecentTxID = t.controller.MostRecentTxID()

				switch v := val.(type) {
				case string:
					resp.Data = v

				case []byte:
					resp.Data = string(v) // @@TODO: probably don't want this

				case map[string]interface{}, []interface{}:
					resp.Data = v

				default:
					http.Error(w, "not found", http.StatusNotFound)
				}

				j, err := json.Marshal(resp)
				if err != nil {
					panic(err)
				}

				_, err = io.Copy(w, bytes.NewBuffer(j))
				if err != nil {
					panic(err)
				}

			} else {
				// No "Accept" header, so we have to try to intelligently guess

				val, _, err := t.controller.State(keypath, false)
				if err != nil {
					t.Errorf("errrr %+v", err)
					http.Error(w, fmt.Sprintf("not found: %+v", err.Error()), http.StatusNotFound)
					return
				}

				contentType, _ := getString(val, []string{"Content-Type"})

				var allowSubscribe bool
				var anyMissing bool
				var respBuf io.Reader

				switch contentType {
				case "text/plain", "text/html", "application/js":
					val, anyMissing, err = t.controller.ResolveRefs(val)
					if err != nil {
						panic(err)
					}

					val, _ = getValue(val, []string{"src"})

					allowSubscribe = contentType != "text/html"

					if valStr, isStr := val.(string); isStr {
						respBuf = bytes.NewBuffer([]byte(valStr))
					} else if valBytes, isBytes := val.([]byte); isBytes {
						respBuf = bytes.NewBuffer(valBytes)
					} else {
						contentType = "application/json"
						j, err := json.Marshal(val)
						if err != nil {
							panic(err)
						}
						respBuf = bytes.NewBuffer(j)
					}

				case "image/png", "image/jpg", "image/jpeg":
					val, anyMissing, err = t.controller.ResolveRefs(val)
					if err != nil {
						panic(err)
					}

					val, _ = getValue(val, []string{"src"})

					if valBytes, isBytes := val.([]byte); isBytes {
						respBuf = bytes.NewBuffer(valBytes)
					} else {
						contentType = "application/json"
						j, err := json.Marshal(val)
						if err != nil {
							panic(err)
						}
						respBuf = bytes.NewBuffer(j)
					}

				case "application/json", "":
					resolveRefs, err := parseResolveRefsParam(r)
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					if resolveRefs {
						val, anyMissing, err = t.controller.ResolveRefs(val)
						if err != nil {
							panic(err)
						}
					}

					j, err := json.Marshal(val)
					if err != nil {
						panic(err)
					}
					respBuf = bytes.NewBuffer(j)
					allowSubscribe = true
					contentType = "application/json"

				default:
					http.Error(w, "not found", http.StatusNotFound)
					return
				}

				w.Header().Set("Content-Type", contentType)
				if allowSubscribe {
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
		}

	case "ACK":
		defer r.Body.Close()

		bs, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("error reading ACK body: %v", err)
			http.Error(w, "error reading body", http.StatusBadRequest)
			return
		}

		var txID ID
		err = txID.UnmarshalText(bs)
		if err != nil {
			t.Errorf("error reading ACK body: %v", err)
			http.Error(w, "error reading body", http.StatusBadRequest)
			return
		}

		t.ackHandler(txID, &httpPeer{address: address, t: t, Writer: w})

	case "PUT":
		defer r.Body.Close()

		if r.Header.Get("Private") == "true" {
			t.Infof(0, "incoming private tx")

			var encryptedTx EncryptedTx
			err := json.NewDecoder(r.Body).Decode(&encryptedTx)
			if err != nil {
				panic(err)
			}

			t.privateTxHandler(encryptedTx, &httpPeer{address: address, t: t, Writer: w})

		} else if r.Header.Get("Ref") == "true" {
			t.Infof(0, "incoming ref")

			file, header, err := r.FormFile("ref")
			if err != nil {
				panic(err)
			}
			defer file.Close()

			hash, err := t.refStore.StoreObject(file, header.Header.Get("Content-Type"))
			if err != nil {
				t.Errorf("error storing ref: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			respondJSON(w, struct {
				Hash Hash `json:"hash"`
			}{hash})

			return

		} else {
			t.Infof(0, "incoming tx")

			var err error

			var sig Signature
			sigHeaderStr := r.Header.Get("Signature")
			if sigHeaderStr == "" {
				http.Error(w, "missing Signature header", http.StatusBadRequest)
				return
			} else {
				sig, err = SignatureFromHex(sigHeaderStr)
				if err != nil {
					http.Error(w, "bad Signature header", http.StatusBadRequest)
					return
				}
			}

			var txID ID
			txIDStr := r.Header.Get("Version")
			if txIDStr == "" {
				txID = RandomID()
			} else {
				txID, err = IDFromHex(txIDStr)
				if err != nil {
					http.Error(w, "bad Version header", http.StatusBadRequest)
					return
				}
			}

			var parents []ID
			parentsStr := r.Header.Get("Parents")
			if parentsStr != "" {
				parentsStrs := strings.Split(parentsStr, ",")
				for _, pstr := range parentsStrs {
					parentID, err := IDFromHex(strings.TrimSpace(pstr))
					if err != nil {
						http.Error(w, "bad Parents header", http.StatusBadRequest)
						return
					}
					parents = append(parents, parentID)
				}
			}

			var patches []Patch
			scanner := bufio.NewScanner(r.Body)
			for scanner.Scan() {
				line := scanner.Text()
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

			tx := Tx{
				ID:      txID,
				Parents: parents,
				Sig:     sig,
				Patches: patches,
				URL:     r.Header.Get("State-URI"), // @@TODO: bad
			}

			// @@TODO: remove .From entirely
			pubkey, err := RecoverSigningPubkey(tx.Hash(), sig)
			if err != nil {
				panic("bad sig")
			}
			tx.From = pubkey.Address()
			////////////////////////////////

			t.txHandler(tx, &httpPeer{address: address, t: t, Writer: w})
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func parseResolveRefsParam(r *http.Request) (bool, error) {
	resolveRefsStr := r.URL.Query().Get("resolve_refs")
	if resolveRefsStr == "" {
		return false, nil
	}

	resolveRefs, err := strconv.ParseBool(resolveRefsStr)
	if err != nil {
		return false, errors.New("invalid resolve_refs param")
	}
	return resolveRefs, nil
}

func respondJSON(resp http.ResponseWriter, data interface{}) {
	resp.Header().Add("Content-Type", "application/json")

	err := json.NewEncoder(resp).Encode(data)
	if err != nil {
		panic(err)
	}
}

func (t *httpTransport) SetFetchHistoryHandler(handler FetchHistoryHandler) {
	t.fetchHistoryHandler = handler
}

func (t *httpTransport) SetTxHandler(handler TxHandler) {
	t.txHandler = handler
}

func (t *httpTransport) SetPrivateTxHandler(handler PrivateTxHandler) {
	t.privateTxHandler = handler
}

func (t *httpTransport) SetAckHandler(handler AckHandler) {
	t.ackHandler = handler
}

func (t *httpTransport) SetVerifyAddressHandler(handler VerifyAddressHandler) {
	t.verifyAddressHandler = handler
}

func (t *httpTransport) SetFetchRefHandler(handler FetchRefHandler) {
	t.fetchRefHandler = handler
}

func (t *httpTransport) GetPeerByConnStrings(ctx context.Context, reachableAt StringSet) (Peer, error) {
	if len(reachableAt) != 1 {
		panic("weird")
	}
	for ra := range reachableAt {
		return &httpPeer{t: t, reachableAt: ra}, nil
	}
	panic("unreachable")
}

func (t *httpTransport) ForEachProviderOfURL(ctx context.Context, theURL string) (<-chan Peer, error) {
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
			case ch <- &httpPeer{t: t, reachableAt: providerURL}:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) ForEachProviderOfRef(ctx context.Context, refHash Hash) (<-chan Peer, error) {
	return nil, errors.New("unimplemented")
}

func (t *httpTransport) ForEachSubscriberToURL(ctx context.Context, theURL string) (<-chan Peer, error) {
	u, err := url.Parse("http://" + theURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	domain := u.Host

	ch := make(chan Peer)
	go func() {
		t.subscriptionsInMu.RLock()
		defer t.subscriptionsInMu.RUnlock()
		defer close(ch)

		for _, sub := range t.subscriptionsIn[domain] {
			<-sub.chDoneCatchingUp
			select {
			case ch <- &httpPeer{t: t, Writer: sub.Writer, Flusher: sub.Flusher}:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) PeersClaimingAddress(ctx context.Context, address Address) (<-chan Peer, error) {
	return nil, errors.New("unimplemented")
}

func (t *httpTransport) AnnounceRef(refHash Hash) error {
	return errors.New("unimplemented")
}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *httpTransport) ensureSessionIDCookie(w http.ResponseWriter, r *http.Request) (ID, error) {
	sessionIDBytes, err := t.signedCookie(r, "sessionid")
	if err != nil {
		t.Errorf("error reading signed sessionid cookie: %v", err)
		return t.setSessionIDCookie(w)
	}
	return IDFromBytes(sessionIDBytes), nil
}

func (t *httpTransport) setSessionIDCookie(w http.ResponseWriter) (ID, error) {
	sessionID := RandomID()
	err := t.setSignedCookie(w, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *httpTransport) setSignedCookie(w http.ResponseWriter, name string, value []byte) error {
	w.Header().Del("Set-Cookie")

	sig, err := t.sigkeys.SignHash(HashBytes(append(value, t.cookieSecret[:]...)))
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
	} else if !t.sigkeys.VerifySignature(HashBytes(append(value, t.cookieSecret[:]...)), sig) {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' has invalid signature (value: %0x)", name, value)
	}
	return value, nil
}

func (t *httpTransport) addressFromCookie(r *http.Request) Address {
	addressBytes, err := t.signedCookie(r, "address")
	if err != nil {
		return Address{}
	}
	return AddressFromBytes(addressBytes)
}

type httpPeer struct {
	t       *httpTransport
	address Address

	// stream
	reachableAt string
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
	return NewStringSet([]string{p.reachableAt})
}

func (p *httpPeer) Address() Address {
	return p.address
}

func (p *httpPeer) SetAddress(addr Address) {
	p.address = addr
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

		client := http.Client{}
		req, err := http.NewRequest("GET", p.reachableAt, nil)
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
			req, err := http.NewRequest("PUT", p.reachableAt, bytes.NewReader(bs))
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
		txID, ok := msg.Payload.(ID)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		txIDBytes, err := txID.MarshalText()
		if err != nil {
			return errors.WithStack(err)
		}

		client := http.Client{}
		req, err := http.NewRequest("ACK", p.reachableAt, bytes.NewReader(txIDBytes))
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
		challengeMsg, ok := msg.Payload.(ChallengeMsg)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		client := http.Client{}
		req, err := http.NewRequest("AUTHORIZE", p.reachableAt, nil)
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
		req, err := http.NewRequest("PUT", p.reachableAt, bytes.NewReader(encPutBytes))
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
