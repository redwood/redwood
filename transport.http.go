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
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

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

	pendingAuthorizations map[ID][]byte

	fetchHistoryHandler  FetchHistoryHandler
	ackHandler           AckHandler
	txHandler            TxHandler
	privateTxHandler     PrivateTxHandler
	verifyAddressHandler VerifyAddressHandler
	fetchRefHandler      FetchRefHandler

	subscriptionsIn   map[string][]*httpSubscriptionIn
	subscriptionsInMu sync.RWMutex

	refStore RefStore

	otherKnownPeers   map[string]struct{}
	otherKnownPeersMu sync.Mutex
}

func NewHTTPTransport(addr Address, port uint, controller Controller, refStore RefStore, sigkeys *SigningKeypair, cookieSecret [32]byte, tlsCertFilename, tlsKeyFilename string) (Transport, error) {
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
		pendingAuthorizations: make(map[ID][]byte),
		ownURL:                fmt.Sprintf("localhost:%v", port),
		refStore:              refStore,
		otherKnownPeers:       make(map[string]struct{}),
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

func (t *httpTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	peerID, err := t.ensurePeerIDCookie(w, r)
	if err != nil {
		t.Errorf("error reading peerID cookie: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	address := t.addressFromCookie(r)

	t.otherKnownPeersMu.Lock()
	// On every incoming request, advertise other peers via the Alt-Svc header
	var others []string
	for peer := range t.otherKnownPeers {
		others = append(others, peer)
	}
	w.Header().Set("Alt-Svc", strings.Join(others, ", "))

	// Similarly, if other peers give us Alt-Svc headers, track them
	if altSvcHeader := r.Header.Get("Alt-Svc"); altSvcHeader != "" {
		parts := strings.Split(altSvcHeader, ";")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		t.otherKnownPeers[parts[0]] = struct{}{}
	}
	t.otherKnownPeersMu.Unlock()

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

			peer := &httpPeer{peerID: peerID, address: address, t: t, Writer: w}
			err = t.verifyAddressHandler([]byte(challengeMsg), peer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		} else if responseHex := r.Header.Get("Response"); responseHex != "" {
			// 2b. Remote node wanted a challenge, this is their response
			challenge, exists := t.pendingAuthorizations[peerID]
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
			delete(t.pendingAuthorizations, peerID) // @@TODO: expiration/garbage collection for failed auths

		} else {
			// 2a. Remote node wants a challenge, so we send it
			challenge, err := GenerateChallengeMsg()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			t.pendingAuthorizations[peerID] = challenge

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
			t.Infof(0, "incoming subscription (peerID: %v, address: %v)", peerID, address)

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

				err := t.fetchHistoryHandler(parents, toVersion, &httpPeer{peerID: peerID, address: address, t: t, Writer: w, Flusher: f})
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

		var txHash Hash
		err = txHash.UnmarshalText(bs)
		if err != nil {
			t.Errorf("error reading ACK body: %v", err)
			http.Error(w, "error reading body", http.StatusBadRequest)
			return
		}

		url := strings.Replace(r.RemoteAddr, "http://", "", -1)
		t.ackHandler(txHash, &httpPeer{peerID: peerID, address: address, t: t, url: url, Writer: w})

	case "PUT":
		defer r.Body.Close()

		if r.Header.Get("Private") == "true" {
			t.Infof(0, "incoming private tx")

			var encryptedTx EncryptedTx
			err := json.NewDecoder(r.Body).Decode(&encryptedTx)
			if err != nil {
				panic(err)
			}

			url := strings.Replace(r.RemoteAddr, "http://", "", -1)
			t.privateTxHandler(encryptedTx, &httpPeer{peerID: peerID, address: address, t: t, url: url, Writer: w})

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

			url := strings.Replace(r.RemoteAddr, "http://", "", -1)
			t.txHandler(tx, &httpPeer{peerID: peerID, address: address, t: t, url: url, Writer: w})
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

func (t *httpTransport) GetPeer(ctx context.Context, peerIDStr string) (Peer, error) {
	panic("unimplemented, requires WebRTC")
}

func (t *httpTransport) GetPeerByConnString(ctx context.Context, addrString string) (Peer, error) {
	return &httpPeer{t: t, url: addrString}, nil
}

func (t *httpTransport) ForEachProviderOfURL(ctx context.Context, theURL string) (<-chan Peer, error) {
	// theURL = braidURLToHTTP(theURL)

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
			case ch <- &httpPeer{t: t, url: providerURL}:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *httpTransport) ForEachProviderOfRef(ctx context.Context, refHash Hash) (<-chan Peer, error) {
	panic("unimplemented")
}

func (t *httpTransport) ForEachSubscriberToURL(ctx context.Context, theURL string) (<-chan Peer, error) {
	// theURL = braidURLToHTTP(theURL)

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
	panic("unimplemented")
}

func (t *httpTransport) AnnounceRef(refHash Hash) error {
	panic("unimplemented")
}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *httpTransport) ensurePeerIDCookie(w http.ResponseWriter, r *http.Request) (ID, error) {
	peerIDBytes, err := t.signedCookie(r, "peerid")
	if err != nil {
		t.Errorf("error reading signed peerID cookie: %v", err)
		return t.setPeerIDCookie(w)
	}
	return IDFromBytes(peerIDBytes), nil
}

func (t *httpTransport) setPeerIDCookie(w http.ResponseWriter) (ID, error) {
	peerID := RandomID()
	err := t.setSignedCookie(w, "peerid", peerID[:])
	return peerID, err
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
	peerID  ID
	address Address

	// stream
	url string
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

func (p *httpPeer) ID() string {
	return p.peerID.String()
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
		urlToSubscribe, ok := msg.Payload.(string)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		// url = braidURLToHTTP(url)

		client := http.Client{}
		req, err := http.NewRequest("GET", p.url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("State-URI", urlToSubscribe)
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
			req, err := http.NewRequest("PUT", p.url, bytes.NewReader(bs))
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
		txHash, ok := msg.Payload.(Hash)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		vidBytes, err := txHash.MarshalText()
		if err != nil {
			return errors.WithStack(err)
		}

		client := http.Client{}
		req, err := http.NewRequest("ACK", p.url, bytes.NewReader(vidBytes))
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
		req, err := http.NewRequest("AUTHORIZE", p.url, nil)
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
		req, err := http.NewRequest("PUT", p.url, bytes.NewReader(encPutBytes))
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
