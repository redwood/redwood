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
	"redwood.dev/swarm"
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

func (s *httpReadableSubscription) Read() (_ *swarm.SubscriptionMsg, err error) {
	defer func() { s.peer.UpdateConnStats(err == nil) }()

	r := bufio.NewReader(s.stream)
	bs, err := r.ReadBytes(byte('\n'))
	if err != nil {
		return nil, err
	}
	bs = bytes.TrimPrefix(bs, []byte("data: "))
	bs = bytes.Trim(bs, "\n ")

	var msg swarm.SubscriptionMsg
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
	typ swarm.SubscriptionType
}

var _ swarm.WritableSubscriptionImpl = (*httpWritableSubscription)(nil)

func (sub *httpWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	var msg *swarm.SubscriptionMsg
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

		etx := &swarm.EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  ownIdentity.Encrypting.EncryptingPublicKey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}

		msg = &swarm.SubscriptionMsg{StateURI: stateURI, EncryptedTx: etx, Leaves: leaves}
	} else {
		msg = &swarm.SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves}
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
	conn                         *websocket.Conn
	onAddMultiplexedSubscription func(stateURI string, keypath state.Keypath, subscriptionType swarm.SubscriptionType, fetchHistoryOpts *swarm.FetchHistoryOpts)
	messages                     *utils.Mailbox
	chStop                       chan struct{}
	chDone                       chan struct{}
	closeOnce                    sync.Once
}

var _ swarm.WritableSubscriptionImpl = (*wsWritableSubscription)(nil)

func (sub *wsWritableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	bs, err := json.Marshal(swarm.SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
	if err != nil {
		sub.t.Errorf("error marshaling message json: %v", err)
		return err
	}
	sub.messages.Deliver(bs)
	return nil
}

func (sub *wsWritableSubscription) writePendingMessages() {
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
		if errors.Cause(err) == types.ErrClosed {
			return
		} else if err != nil {
			sub.t.Errorf("error: %v", err)
		}
	}
}

func (sub *wsWritableSubscription) close() {
	sub.closeOnce.Do(func() {
		close(sub.chStop)
		<-sub.chDone
		sub.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
		err := sub.conn.WriteMessage(websocket.CloseMessage, []byte{})
		sub.UpdateConnStats(err == nil)
		sub.conn.Close()
	})
}

func (sub *wsWritableSubscription) start() {
	sub.chStop = make(chan struct{})
	sub.chDone = make(chan struct{})

	ticker := time.NewTicker(wsPingPeriod)

	go func() {
		defer close(sub.chDone)
		defer ticker.Stop()

		for {
			select {
			case <-sub.chStop:
				return

			case <-sub.messages.Notify():
				sub.writePendingMessages()

			case <-ticker.C:
				sub.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))

				err := sub.conn.WriteMessage(websocket.PingMessage, nil)
				sub.UpdateConnStats(err == nil)
				if err != nil {
					sub.t.Errorf("error pinging websocket client: %v", err)
					sub.close()
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-sub.chStop:
				return
			default:
			}

			_, bs, err := sub.conn.ReadMessage()
			if err != nil {
				sub.t.Errorf("error reading from websocket: %v", err)
				sub.close()
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

			var subscriptionType swarm.SubscriptionType
			if addSubMsg.Params.SubscriptionType != "" {
				err := subscriptionType.UnmarshalText([]byte(addSubMsg.Params.SubscriptionType))
				if err != nil {
					sub.t.Errorf("could not parse subscription type: %v", err)
					continue
				}
			}

			var fetchHistoryOpts *swarm.FetchHistoryOpts
			if addSubMsg.Params.FromTxID != "" {
				fromTxID, err := types.IDFromHex(addSubMsg.Params.FromTxID)
				if err != nil {
					sub.t.Errorf("could not parse fromTxID: %v", err)
					continue
				}
				fetchHistoryOpts = &swarm.FetchHistoryOpts{FromTxID: fromTxID}
			}

			sub.onAddMultiplexedSubscription(addSubMsg.Params.StateURI, state.Keypath(addSubMsg.Params.Keypath), subscriptionType, fetchHistoryOpts)
		}
	}()
}
