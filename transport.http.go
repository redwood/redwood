package redwood

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type httpTransport struct {
	*ctx.Context

	address    Address
	controller Controller
	ownURL     string
	port       uint

	ackHandler           AckHandler
	txHandler            TxHandler
	privateTxHandler     PrivateTxHandler
	verifyAddressHandler VerifyAddressHandler

	subscriptionsIn   map[string][]*httpSubscriptionIn
	subscriptionsInMu sync.RWMutex
}

func NewHTTPTransport(addr Address, port uint, controller Controller) (Transport, error) {
	t := &httpTransport{
		Context:         &ctx.Context{},
		address:         addr,
		subscriptionsIn: make(map[string][]*httpSubscriptionIn),
		controller:      controller,
		port:            port,
		ownURL:          fmt.Sprintf("localhost:%v", port),
	}
	return t, nil
}

func (t *httpTransport) Start() error {
	return t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel(t.address.Pretty() + " transport")
			t.Infof(0, "opening http transport at :%v", t.port)
			go func() {
				err := http.ListenAndServe(fmt.Sprintf(":%v", t.port), t)
				if err != nil {
					panic(err.Error())
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
	chDone chan struct{}
}

func (s *httpSubscriptionIn) Close() error {
	close(s.chDone)
	return nil
}

func (t *httpTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	switch r.Method {
	case "GET":
		if challengeMsgHex := r.Header.Get("Verify-Credentials"); challengeMsgHex != "" {
			//
			// Address verification request
			//
			challengeMsg, err := hex.DecodeString(challengeMsgHex)
			if err != nil {
				http.Error(w, "Verify-Credentials header: bad challenge message", http.StatusBadRequest)
				return
			}

			peer := &httpPeer{t: t, Writer: w}
			err = t.verifyAddressHandler([]byte(challengeMsg), peer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		} else if r.Header.Get("Subscribe") != "" {
			//
			// Subscription request
			//
			t.Infof(0, "incoming subscription")

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
				t.Info(0, "http connection closed")
			}()

			// Set the headers related to event streaming.
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			// w.Header().Set("Transfer-Encoding", "chunked")

			sub := &httpSubscriptionIn{w, f, make(chan struct{})}

			urlToSubscribe := r.Header.Get("Subscribe")

			t.subscriptionsInMu.Lock()
			t.subscriptionsIn[urlToSubscribe] = append(t.subscriptionsIn[urlToSubscribe], sub)
			t.subscriptionsInMu.Unlock()

			f.Flush()

			// Block until the subscription is canceled
			<-sub.chDone

		} else {
			//
			// Regular HTTP GET request (from browsers, etc.)
			//

			// @@TODO: this is hacky
			if r.URL.Path == "/braid.js" {
				var filename string
				if fileExists("./braidjs/braid-dist.js") {
					filename = "./braidjs/braid-dist.js"
				} else if fileExists("../braidjs/braid-dist.js") {
					filename = "../braidjs/braid-dist.js"
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
			stateMap, isMap := t.controller.State().(map[string]interface{})
			if !isMap {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			val, exists := M(stateMap).GetValue(keypath...)
			if !exists {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			if r.Header.Get("Accept") == "application/json" {
				var resp struct {
					MostRecentTxHash Hash        `json:"mostRecentTxHash"`
					Data             interface{} `json:"data"`
				}
				resp.MostRecentTxHash = t.controller.MostRecentTxHash() // @@TODO: hacky

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
				switch v := val.(type) {
				case string:
					_, err := io.Copy(w, bytes.NewBuffer([]byte(v)))
					if err != nil {
						panic(err)
					}

				case []byte:
					_, err := io.Copy(w, bytes.NewBuffer(v))
					if err != nil {
						panic(err)
					}

				case map[string]interface{}, []interface{}:
					j, err := json.Marshal(v)
					if err != nil {
						panic(err)
					}
					_, err = io.Copy(w, bytes.NewBuffer(j))
					if err != nil {
						panic(err)
					}

				default:
					http.Error(w, "not found", http.StatusNotFound)
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
		t.ackHandler(txHash, &httpPeer{t: t, url: url, Writer: w})

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
			t.privateTxHandler(encryptedTx, &httpPeer{t: t, url: url, Writer: w})

		} else {
			t.Infof(0, "incoming tx")

			var tx Tx
			err := json.NewDecoder(r.Body).Decode(&tx)
			if err != nil {
				panic(err)
			}

			url := strings.Replace(r.RemoteAddr, "http://", "", -1)
			t.txHandler(tx, &httpPeer{t: t, url: url, Writer: w})
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
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

func (t *httpTransport) GetPeer(ctx context.Context, addrString string) (Peer, error) {
	return &httpPeer{t: t, url: addrString}, nil
}

func (t *httpTransport) ForEachProviderOfURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error {
	// theURL = braidURLToHTTP(theURL)

	u, err := url.Parse("http://" + theURL)
	if err != nil {
		return err
	}

	u.Path = path.Join(u.Path, "providers")

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return errors.Errorf("error GETting providers: (%v) %v", resp.StatusCode, resp.Status)
	}
	defer resp.Body.Close()

	var providers []string
	err = json.NewDecoder(resp.Body).Decode(&providers)
	if err != nil {
		return err
	}

	for _, providerURL := range providers {
		if providerURL == t.ownURL {
			continue
		}

		keepGoing, err := fn(&httpPeer{t: t, url: providerURL})
		if err != nil {
			return errors.WithStack(err)
		} else if !keepGoing {
			break
		}
	}
	return nil
}

func (t *httpTransport) ForEachSubscriberToURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error {
	// theURL = braidURLToHTTP(theURL)

	u, err := url.Parse("http://" + theURL)
	if err != nil {
		return errors.WithStack(err)
	}

	domain := u.Host

	t.subscriptionsInMu.RLock()
	defer t.subscriptionsInMu.RUnlock()

	for _, sub := range t.subscriptionsIn[domain] {
		keepGoing, err := fn(&httpPeer{t: t, Writer: sub.Writer, Flusher: sub.Flusher})
		if err != nil {
			return errors.WithStack(err)
		} else if !keepGoing {
			break
		}
	}
	return nil
}

func (t *httpTransport) PeersClaimingAddress(ctx context.Context, address Address) (<-chan Peer, error) {
	panic("unimplemented")
}

type httpPeer struct {
	t *httpTransport

	// stream
	url string
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
	return p.url
}

func (p *httpPeer) EnsureConnected(ctx context.Context) error {
	return nil
}

func (p *httpPeer) WriteMsg(msg Msg) error {
	switch msg.Type {
	case MsgType_Subscribe:
		urlToSubscribe, ok := msg.Payload.(string)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		// url = braidURLToHTTP(url)

		client := http.Client{}
		req, err := http.NewRequest("GET", "http://"+p.url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Subscribe", urlToSubscribe)

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
			err := WriteMsg(p, msg)
			if err != nil {
				return err
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
			req, err := http.NewRequest("PUT", "http://"+p.url, bytes.NewReader(bs))
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
		req, err := http.NewRequest("ACK", "http://"+p.url, bytes.NewReader(vidBytes))
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
		challengeMsg, ok := msg.Payload.([]byte)
		if !ok {
			return errors.WithStack(ErrProtocol)
		}

		client := http.Client{}
		req, err := http.NewRequest("GET", "http://"+p.url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Verify-Credentials", hex.EncodeToString(challengeMsg))

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
		req, err := http.NewRequest("PUT", "http://"+p.url, bytes.NewReader(encPutBytes))
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
		var msg Msg
		err := ReadMsg(p.ReadCloser, &msg)
		if err != nil {
			return msg, err
		} else if msg.Type != MsgType_Put {
			panic("bad")
		} else if _, is := msg.Payload.(Tx); !is {
			panic("bad")
		}
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
