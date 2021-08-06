package braidhttp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/identity"
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
	*peerConn
	stateURI  string
	typ       prototree.SubscriptionType
	transport *transport
	closeOnce sync.Once
	chClosed  chan struct{}
}

var _ prototree.WritableSubscriptionImpl = (*httpWritableSubscription)(nil)

func newHTTPWritableSubscription(
	stateURI string,
	peerConn *peerConn,
	subscriptionType prototree.SubscriptionType,
	transport *transport,
) *httpWritableSubscription {
	sub := &httpWritableSubscription{
		stateURI:  stateURI,
		peerConn:  peerConn,
		typ:       subscriptionType,
		transport: transport,
		chClosed:  make(chan struct{}),
	}

	// Listen to the closing of the http connection via the CloseNotifier
	notify := peerConn.stream.Writer.(http.CloseNotifier).CloseNotify()
	go func() {
		defer sub.Close()
		<-notify
		transport.Infof(0, "http subscription closed (%v)", stateURI)
	}()

	return sub
}

func (sub *httpWritableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *httpWritableSubscription) Close() error {
	sub.closeOnce.Do(func() {
		sub.peerConn.Close()
		close(sub.chClosed)
	})
	return nil
}

func (sub *httpWritableSubscription) Closed() <-chan struct{} {
	return sub.chClosed
}

func (sub *httpWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	var msg *prototree.SubscriptionMsg

	if tx != nil && tx.IsPrivate() {
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

	n, err := sub.stream.Writer.Write(event)
	if err != nil {
		sub.Close()
		return err
	} else if n < len(event) {
		sub.Close()
		return errors.New("error writing message to http peer: didn't write enough")
	}
	sub.stream.Flush()
	return nil
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
	*peerConn
	stateURI         string
	conn             *websocket.Conn
	transport        *transport
	messages         *utils.Mailbox
	chStop           chan struct{}
	wgReadingWriting sync.WaitGroup
	chClosed         chan struct{}
	closeOnce        sync.Once
}

var _ prototree.WritableSubscriptionImpl = (*wsWritableSubscription)(nil)

func newWSWritableSubscription(stateURI string, conn *websocket.Conn, peerConn *peerConn, transport *transport) *wsWritableSubscription {
	return &wsWritableSubscription{
		stateURI:  stateURI,
		conn:      conn,
		peerConn:  peerConn,
		transport: transport,
		messages:  utils.NewMailbox(300), // @@TODO: configurable?
		chStop:    make(chan struct{}),
		chClosed:  make(chan struct{}),
	}
}

func (sub *wsWritableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *wsWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	bs, err := json.Marshal(prototree.SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
	if err != nil {
		sub.t.Errorf("error marshaling message json: %v", err)
		return err
	}
	sub.messages.Deliver(bs)
	return nil
}

func (sub *wsWritableSubscription) Close() error {
	sub.closeOnce.Do(func() {
		defer close(sub.chClosed)

		close(sub.chStop)
		sub.wgReadingWriting.Wait()

		sub.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		err := sub.conn.WriteMessage(websocket.CloseMessage, []byte{})
		sub.UpdateConnStats(err == nil)
		sub.conn.Close()
	})
	return nil
}

func (sub *wsWritableSubscription) Closed() <-chan struct{} {
	return sub.chClosed
}

func (sub *wsWritableSubscription) start() {
	// sub.conn.SetPongHandler(func(string) error { sub.conn.SetReadDeadline(time.Now().Add(wsPongWait)); return nil })

	ticker := time.NewTicker(wsPingPeriod)

	sub.wgReadingWriting.Add(2)

	go func() {
		defer ticker.Stop()
		defer sub.Close()
		defer sub.wgReadingWriting.Done()

		for {
			select {
			case <-sub.chStop:
				return

			case <-sub.messages.Notify():
				err := sub.writePendingMessages()
				if err != nil {
					return
				}

			case <-ticker.C:
				sub.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				sub.conn.SetReadDeadline(time.Now().Add(wsPongWait))

				err := sub.conn.WriteMessage(websocket.PingMessage, nil)
				sub.UpdateConnStats(err == nil)
				if err != nil {
					sub.t.Errorf("error pinging websocket client: %v", err)
					return
				}
			}
		}
	}()

	go func() {
		defer sub.Close()
		defer sub.wgReadingWriting.Done()

		for {
			select {
			case <-sub.chStop:
				return
			default:
			}

			_, bs, err := sub.conn.ReadMessage()
			if err != nil {
				sub.t.Errorf("error reading from websocket: %v", err)
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
				sub.t.Errorf("got bad multiplexed subscription request: %v", err)
				continue
			}
			sub.t.Infof(0, "incoming websocket subscription (state uri: %v)", addSubMsg.Params.StateURI)

			var subscriptionType prototree.SubscriptionType
			if addSubMsg.Params.SubscriptionType != "" {
				err := subscriptionType.UnmarshalText([]byte(addSubMsg.Params.SubscriptionType))
				if err != nil {
					sub.t.Errorf("could not parse subscription type: %v", err)
					continue
				}
			}

			var fetchHistoryOpts *prototree.FetchHistoryOpts
			if addSubMsg.Params.FromTxID != "" {
				fromTxID, err := types.IDFromHex(addSubMsg.Params.FromTxID)
				if err != nil {
					sub.t.Errorf("could not parse fromTxID: %v", err)
					continue
				}
				fetchHistoryOpts = &prototree.FetchHistoryOpts{FromTxID: fromTxID}
			}

			sub.transport.HandleWritableSubscriptionOpened(
				addSubMsg.Params.StateURI,
				state.Keypath(addSubMsg.Params.Keypath),
				subscriptionType,
				sub,
				fetchHistoryOpts,
			)
		}
	}()
}

func (sub *wsWritableSubscription) writePendingMessages() error {
	for _, msgBytes := range sub.messages.RetrieveAll() {
		err := func() error {
			select {
			case <-sub.chStop:
				return types.ErrClosed
			default:
			}

			var err error
			defer func() { sub.UpdateConnStats(err == nil) }()

			sub.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))

			w, err := sub.conn.NextWriter(websocket.TextMessage)
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
