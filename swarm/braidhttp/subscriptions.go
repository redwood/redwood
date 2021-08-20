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
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type httpReadableSubscription struct {
	client  *http.Client
	stream  io.ReadCloser
	peer    *peerConn
	private bool
}

var _ prototree.ReadableSubscription = (*httpReadableSubscription)(nil)

func (s *httpReadableSubscription) Read() (_ *prototree.SubscriptionMsg, err error) {
	defer func() { s.peer.UpdateConnStats(err == nil) }()

	r := bufio.NewReader(s.stream)
	bs, err := r.ReadBytes(byte('\n'))
	if err != nil {
		return nil, err
	}
	bs = bytes.TrimPrefix(bs, []byte("data: "))
	bs = bytes.Trim(bs, "\n ")

	var msg prototree.SubscriptionMsg
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
			crypto.AsymEncPubkeyFromBytes(msg.EncryptedTx.SenderPublicKey),
			msg.EncryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, err
		}

		var tx tree.Tx
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
	process.Process
	log.Logger
	peerConn  *peerConn
	stateURI  string
	closeOnce sync.Once
}

var _ prototree.WritableSubscriptionImpl = (*httpWritableSubscription)(nil)

func newHTTPWritableSubscription(
	stateURI string,
	peerConn *peerConn,
	subscriptionType prototree.SubscriptionType,
) *httpWritableSubscription {
	return &httpWritableSubscription{
		Process:  *process.New("sub impl (http) " + peerConn.DialInfo().String() + " " + stateURI),
		Logger:   log.NewLogger(TransportName),
		stateURI: stateURI,
		peerConn: peerConn,
	}
}

func (sub *httpWritableSubscription) Start() error {
	err := sub.Process.Start()
	if err != nil {
		return err
	}
	defer sub.Process.Autoclose()

	// Listen to the closing of the http connection via the CloseNotifier
	notify := sub.peerConn.stream.Writer.(http.CloseNotifier).CloseNotify()
	sub.Process.Go("", func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-notify:
		}
	})
	return nil
}

func (sub *httpWritableSubscription) Close() (err error) {
	sub.Infof(0, "http writable subscription closed (%v)", sub.stateURI)
	return multierr.Append(
		sub.peerConn.Close(),
		sub.Process.Close(),
	)
}

func (sub *httpWritableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *httpWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	var msg *prototree.SubscriptionMsg

	if tx != nil && tx.IsPrivate() {
		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		peerAddrs := types.OverlappingAddresses(tx.Recipients, sub.peerConn.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}

		peerSigPubkey, peerEncPubkey := sub.peerConn.PublicKeys(peerAddrs[0])

		var ownIdentity identity.Identity
		for _, addr := range tx.Recipients {
			ownIdentity, err = sub.peerConn.t.keyStore.IdentityWithAddress(addr)
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

		encryptedTxBytes, err := sub.peerConn.t.keyStore.SealMessageFor(ownIdentity.Address(), peerEncPubkey, marshalledTx)
		if err != nil {
			return errors.WithStack(err)
		}

		etx := &prototree.EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  ownIdentity.AsymEncKeypair.AsymEncPubkey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}

		msg = &prototree.SubscriptionMsg{StateURI: stateURI, EncryptedTx: etx, Leaves: leaves}
	} else {
		msg = &prototree.SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves}
	}

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// This is encoded using HTTP's SSE format
	event := []byte("data: " + string(bs) + "\n\n")

	n, err := sub.peerConn.stream.Writer.Write(event)
	if err != nil {
		return err
	} else if n < len(event) {
		return errors.New("error writing message to http peer: didn't write enough")
	}
	sub.peerConn.stream.Flush()
	return nil
}

func (sub httpWritableSubscription) String() string {
	return sub.peerConn.DialInfo().TransportName + " " + sub.peerConn.DialInfo().DialAddr + " (" + sub.stateURI + ")"
}

const (
	wsWriteWait  = 10 * time.Second      // Time allowed to write a message to the peer.
	wsPongWait   = 10 * time.Second      // Time allowed to read the next pong message from the peer.
	wsPingPeriod = (wsPongWait * 9) / 10 // Send pings to peer with this period. Must be less than wsPongWait.
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
	peerConn  *peerConn
	stateURI  string
	wsConn    *websocket.Conn
	transport *transport
	messages  *utils.Mailbox
	startOnce sync.Once
	closeOnce sync.Once
}

var _ prototree.WritableSubscriptionImpl = (*wsWritableSubscription)(nil)

func newWSWritableSubscription(stateURI string, wsConn *websocket.Conn, peerConn *peerConn, transport *transport) *wsWritableSubscription {
	return &wsWritableSubscription{
		Process:   *process.New("sub impl (ws) " + peerConn.DialInfo().String() + " " + stateURI),
		Logger:    log.NewLogger(TransportName),
		stateURI:  stateURI,
		wsConn:    wsConn,
		peerConn:  peerConn,
		transport: transport,
		messages:  utils.NewMailbox(300), // @@TODO: configurable?
	}
}

func (sub *wsWritableSubscription) Start() (err error) {
	sub.startOnce.Do(func() {
		fmt.Printf("START %v %v %p %+v\n", sub.peerConn.DialInfo().String(), sub.stateURI, sub, errors.New(""))
		err = sub.Process.Start()
		if err != nil {
			return
		}

		// sub.wsConn.SetPongHandler(func(string) error { sub.wsConn.SetReadDeadline(time.Now().Add(wsPongWait)); return nil })

		ticker := time.NewTicker(wsPingPeriod)

		sub.Process.Go("read", func(ctx context.Context) {
			defer ticker.Stop()
			defer sub.Close()

			for {
				select {
				case <-ctx.Done():
					return

				case <-sub.messages.Notify():
					err := sub.writePendingMessages(ctx)
					if err != nil {
						return
					}

				case <-ticker.C:
					sub.wsConn.SetWriteDeadline(time.Now().Add(wsWriteWait))
					sub.wsConn.SetReadDeadline(time.Now().Add(wsPongWait))

					err := sub.wsConn.WriteMessage(websocket.PingMessage, nil)
					sub.peerConn.UpdateConnStats(err == nil)
					if err != nil {
						sub.peerConn.t.Errorf("error pinging websocket client: %v", err)
						return
					}
				}
			}
		})

		sub.Process.Go("write", func(ctx context.Context) {
			defer sub.Close()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				_, bs, err := sub.wsConn.ReadMessage()
				if err != nil {
					sub.Errorf("error reading from websocket: %v", err)
					return
				}

				var addSubMsg struct {
					Params struct {
						StateURI         string `json:"stateURI"`
						Keypath          string `json:"keypath"`
						SubscriptionType string `json:"subscriptionType"`
						FromTxID         string `json:"fromTxID"`
					} `json:"params"`
				}
				err = json.Unmarshal(bs, &addSubMsg)
				if err != nil {
					sub.Errorf("got bad multiplexed subscription request: %v", err)
					continue
				}
				sub.Infof(0, "incoming websocket subscription (state uri: %v)", addSubMsg.Params.StateURI)

				var subscriptionType prototree.SubscriptionType
				if addSubMsg.Params.SubscriptionType != "" {
					err := subscriptionType.UnmarshalText([]byte(addSubMsg.Params.SubscriptionType))
					if err != nil {
						sub.Errorf("could not parse subscription type: %v", err)
						continue
					}
				}

				var fetchHistoryOpts prototree.FetchHistoryOpts
				if addSubMsg.Params.FromTxID != "" {
					fromTxID, err := types.IDFromHex(addSubMsg.Params.FromTxID)
					if err != nil {
						sub.Errorf("could not parse fromTxID: %v", err)
						continue
					}
					fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: fromTxID}
				}

				sub.transport.HandleWritableSubscriptionOpened(
					addSubMsg.Params.StateURI,
					state.Keypath(addSubMsg.Params.Keypath),
					subscriptionType,
					sub,
					&fetchHistoryOpts,
				)
			}
		})
	})
	return err
}

func (sub *wsWritableSubscription) writePendingMessages(ctx context.Context) error {
	for _, msgBytes := range sub.messages.RetrieveAll() {
		err := func() error {
			select {
			case <-ctx.Done():
				return context.Canceled
			default:
			}

			var err error
			defer func() { sub.peerConn.UpdateConnStats(err == nil) }()

			sub.wsConn.SetWriteDeadline(time.Now().Add(wsWriteWait))

			w, err := sub.wsConn.NextWriter(websocket.TextMessage)
			if err != nil {
				return errors.Wrapf(err, "while obtaining next writer")
			}
			defer w.Close()

			bs := append(msgBytes.([]byte), '\n')

			_, err = w.Write(bs)
			if err != nil {
				return errors.Wrapf(err, "while writing to websocket client")
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (sub *wsWritableSubscription) Close() error {
	sub.Infof(0, "ws writable subscription closed (%v)", sub.stateURI)

	sub.wsConn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = sub.wsConn.WriteMessage(websocket.CloseMessage, []byte{})

	return multierr.Combine(
		sub.wsConn.Close(),
		sub.peerConn.Close(),
		sub.Process.Close(),
	)
}

func (sub *wsWritableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *wsWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	bs, err := json.Marshal(prototree.SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
	if err != nil {
		sub.peerConn.t.Errorf("error marshaling message json: %v", err)
		return err
	}
	sub.messages.Deliver(bs)
	return nil
}

func (sub wsWritableSubscription) String() string {
	return sub.peerConn.DialInfo().TransportName + "-ws " + sub.peerConn.DialInfo().DialAddr + " (" + sub.stateURI + ")"
}
