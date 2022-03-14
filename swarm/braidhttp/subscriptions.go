package braidhttp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/multierr"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm/prototree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type httpReadableSubscription struct {
	stream  io.ReadCloser
	peer    *peerConn
	private bool
}

var _ prototree.ReadableSubscription = (*httpReadableSubscription)(nil)

func (s *httpReadableSubscription) Read() (_ prototree.SubscriptionMsg, err error) {
	defer func() { s.peer.UpdateConnStats(err == nil) }()

	r := bufio.NewReader(s.stream)
	bs, err := r.ReadBytes(byte('\n'))
	if err != nil {
		return prototree.SubscriptionMsg{}, err
	}
	bs = bytes.TrimPrefix(bs, []byte("data: "))
	bs = bytes.Trim(bs, "\n ")

	var msg prototree.SubscriptionMsg
	err = json.Unmarshal(bs, &msg)
	if err != nil {
		return prototree.SubscriptionMsg{}, err
	}
	return msg, nil
}

func (c *httpReadableSubscription) Close() error {
	return c.peer.Close()
}

type httpWritableSubscription struct {
	process.Process
	log.Logger
	w         http.ResponseWriter
	r         *http.Request
	stateURI  string
	closeOnce sync.Once
}

var _ prototree.WritableSubscriptionImpl = (*httpWritableSubscription)(nil)

func newHTTPWritableSubscription(
	stateURI string,
	w http.ResponseWriter,
	r *http.Request,
) *httpWritableSubscription {
	return &httpWritableSubscription{
		Process:  *process.New("sub impl (" + TransportName + ") " + stateURI),
		Logger:   log.NewLogger(TransportName),
		stateURI: stateURI,
		w:        w,
		r:        r,
	}
}

func (sub *httpWritableSubscription) Start() error {
	err := sub.Process.Start()
	if err != nil {
		return err
	}
	defer sub.Process.Autoclose()

	// Set the headers related to event streaming
	sub.w.Header().Set("Content-Type", "text/event-stream")
	sub.w.Header().Set("Cache-Control", "no-cache")
	sub.w.Header().Set("Connection", "keep-alive")
	sub.w.Header().Set("Transfer-Encoding", "chunked")

	// Listen to the closing of the http connection via the CloseNotifier
	notify := sub.w.(http.CloseNotifier).CloseNotify()
	sub.Process.Go(nil, "", func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-notify:
		}
	})

	sub.w.(http.Flusher).Flush()

	return nil
}

func (sub *httpWritableSubscription) Close() (err error) {
	sub.Infof(0, "%v writable subscription closed (%v)", TransportName, sub.stateURI)
	return sub.Process.Close()
}

func (sub *httpWritableSubscription) Put(ctx context.Context, msg prototree.SubscriptionMsg) (err error) {
	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// This is encoded using HTTP's SSE format
	event := []byte("data: " + string(bs) + "\n\n")

	n, err := sub.w.Write(event)
	if err != nil {
		return err
	} else if n < len(event) {
		return errors.New("error writing message to http peer: didn't write enough")
	}
	sub.w.(http.Flusher).Flush()
	return nil
}

func (sub httpWritableSubscription) String() string {
	return TransportName + " (" + sub.stateURI + ")"
}

const (
	wsWriteWait  = 10 * time.Second // Time allowed to write a message to the peer.
	wsPongWait   = 10 * time.Second // Time allowed to read the next pong message from the peer.
	wsPingPeriod = 5 * time.Second  // Send pings to peer with this period. Must be less than wsPongWait.
)

var (
	newline    = []byte{'\n'}
	space      = []byte{' '}
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
)

type wsWritableSubscription struct {
	process.Process
	log.Logger
	wsConn                     *websocket.Conn
	addresses                  []types.Address
	writableSubscriptionOpener writableSubscriptionOpener

	stateURIs types.Set[string]
	messages  *utils.Mailbox[wsMessage]
	writeMu   sync.Mutex
	startOnce sync.Once
	closed    bool
	closeOnce sync.Once
}

type writableSubscriptionOpener interface {
	HandleWritableSubscriptionOpened(
		req prototree.SubscriptionRequest,
		writeSubImplFactory prototree.WritableSubscriptionImplFactory,
	) (<-chan struct{}, error)
}

type wsMessage struct {
	msgType int
	data    []byte
}

var _ prototree.WritableSubscriptionImpl = (*wsWritableSubscription)(nil)

func newWSWritableSubscription(
	stateURI string,
	wsConn *websocket.Conn,
	addresses []types.Address,
	writableSubscriptionOpener writableSubscriptionOpener,
) *wsWritableSubscription {
	return &wsWritableSubscription{
		Process:                    *process.New("sub impl (ws) " + stateURI),
		Logger:                     log.NewLogger(TransportName),
		stateURIs:                  types.NewSet[string]([]string{stateURI}),
		wsConn:                     wsConn,
		addresses:                  addresses,
		writableSubscriptionOpener: writableSubscriptionOpener,
		messages:                   utils.NewMailbox[wsMessage](300), // @@TODO: configurable?
	}
}

func (sub *wsWritableSubscription) Start() (err error) {
	sub.startOnce.Do(func() {
		err = sub.Process.Start()
		if err != nil {
			return
		}
		defer sub.Process.Autoclose()

		// Say hello
		sub.write(websocket.PingMessage, nil)

		var (
			ticker        = time.NewTicker(wsPingPeriod)
			chWriteClosed = make(chan struct{})
			chReadClosed  = make(chan struct{})
		)

		sub.Process.Go(nil, "write", func(ctx context.Context) {
			defer close(chWriteClosed)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-chReadClosed:
					return

				case <-sub.messages.Notify():
					err := sub.writePendingMessages(ctx)
					if err != nil {
						return
					}

				case <-ticker.C:
					sub.messages.Deliver(wsMessage{websocket.PingMessage, nil})
				}
			}
		})

		sub.Process.Go(nil, "read", func(ctx context.Context) {
			defer close(chReadClosed)
			for {
				select {
				case <-ctx.Done():
					return
				case <-chWriteClosed:
					return
				default:
				}

				msg, err := sub.read()
				if err != nil {
					sub.Errorf("error reading from websocket: %v", err)
					return
				}

				if msg.msgType == websocket.CloseMessage {
					return
				} else if msg.msgType == websocket.PingMessage {
					sub.messages.Deliver(wsMessage{websocket.PongMessage, nil})
					continue
				} else if msg.msgType == websocket.PongMessage {
					continue
				} else if msg.msgType == websocket.BinaryMessage {
					sub.Errorf("websocket subscription received unexpected binary message")
					continue
				}

				var addSubMsg struct {
					Params struct {
						StateURI         string                     `json:"stateURI"`
						Keypath          state.Keypath              `json:"keypath"`
						SubscriptionType prototree.SubscriptionType `json:"subscriptionType"`
						FromTxID         string                     `json:"fromTxID"`
					} `json:"params"`
				}
				err = json.Unmarshal(msg.data, &addSubMsg)
				if err != nil {
					sub.Errorf("got bad multiplexed subscription request: %v", err)
					continue
				}
				sub.Infof(0, "incoming websocket subscription (state uri: %v)", addSubMsg.Params.StateURI)

				if sub.stateURIs.Contains(addSubMsg.Params.StateURI) {
					continue
				}
				sub.stateURIs.Add(addSubMsg.Params.StateURI)

				var fetchHistoryOpts prototree.FetchHistoryOpts
				if addSubMsg.Params.FromTxID != "" {
					fromTxID, err := state.VersionFromHex(addSubMsg.Params.FromTxID)
					if err != nil {
						sub.Errorf("could not parse fromTxID: %v", err)
						continue
					}
					fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: fromTxID}
				}

				_, err = sub.writableSubscriptionOpener.HandleWritableSubscriptionOpened(
					prototree.SubscriptionRequest{
						StateURI:         addSubMsg.Params.StateURI,
						Keypath:          addSubMsg.Params.Keypath,
						Type:             addSubMsg.Params.SubscriptionType,
						FetchHistoryOpts: &fetchHistoryOpts,
						Addresses:        types.NewSet[types.Address](sub.addresses),
					},
					func() (prototree.WritableSubscriptionImpl, error) { return sub, nil },
				)
				if err != nil {
					sub.Errorf("bad incoming websocket subscription request: %v", err)
					continue
				}
			}
		})
	})
	return err
}

var (
	pingMessage = []byte("ping")
	pongMessage = []byte("pong")
)

func (sub *wsWritableSubscription) read() (m wsMessage, err error) {
	sub.wsConn.SetReadDeadline(time.Now().Add(wsPongWait))

	msgType, bs, err := sub.wsConn.ReadMessage()
	if err == io.EOF {
		return wsMessage{websocket.CloseMessage, nil}, nil
	} else if _, is := err.(*websocket.CloseError); is {
		return wsMessage{websocket.CloseMessage, nil}, nil
	} else if err != nil {
		return wsMessage{}, err
	}

	switch msgType {
	case websocket.PingMessage, websocket.PongMessage, websocket.CloseMessage:
		return wsMessage{msgType, bs}, nil
	}

	bs = bytes.TrimSpace(bs)
	if bytes.Equal(bs, pingMessage) {
		return wsMessage{websocket.PingMessage, nil}, nil
	} else if bytes.Equal(bs, pongMessage) {
		return wsMessage{websocket.PongMessage, nil}, nil
	} else {
		return wsMessage{websocket.TextMessage, bs}, nil
	}
}

func (sub *wsWritableSubscription) writePendingMessages(ctx context.Context) error {
	for _, msg := range sub.messages.RetrieveAll() {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		err := sub.write(msg.msgType, msg.data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sub *wsWritableSubscription) write(messageType int, bytes []byte) error {
	sub.writeMu.Lock()
	defer sub.writeMu.Unlock()

	if sub.closed && messageType != websocket.CloseMessage {
		return nil
	}

	sub.wsConn.SetWriteDeadline(time.Now().Add(wsWriteWait))

	switch messageType {
	case websocket.TextMessage:
		bytes = append(bytes, '\n')
	case websocket.PingMessage:
		messageType = websocket.TextMessage
		bytes = []byte("ping\n")
	case websocket.PongMessage:
		messageType = websocket.TextMessage
		bytes = []byte("pong\n")
	}

	err := sub.wsConn.WriteMessage(messageType, bytes)
	if err != nil {
		return errors.Wrapf(err, "while writing to websocket client")
	}
	return nil
}

func (sub *wsWritableSubscription) Close() error {
	var err error
	sub.closeOnce.Do(func() {
		sub.writeMu.Lock()
		sub.closed = true
		sub.writeMu.Unlock()

		sub.Infof(0, "ws writable subscription closed")

		_ = sub.write(websocket.CloseMessage, []byte{})

		err = multierr.Combine(
			sub.wsConn.Close(),
			sub.Process.Close(),
		)
	})
	return err
}

func (sub *wsWritableSubscription) Put(ctx context.Context, msg prototree.SubscriptionMsg) (err error) {
	bs, err := json.Marshal(msg)
	if err != nil {
		sub.Errorf("error marshaling message json: %v", err)
		return err
	}
	sub.messages.Deliver(wsMessage{websocket.TextMessage, bs})
	return nil
}

func (sub wsWritableSubscription) String() string {
	return fmt.Sprintf("websocket %v", sub.stateURIs.Slice())
}
