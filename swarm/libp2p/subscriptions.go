package libp2p

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/state"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type readableSubscription struct {
	*peerConn
}

var _ prototree.ReadableSubscription = (*readableSubscription)(nil)

func (sub *readableSubscription) Read() (_ *prototree.SubscriptionMsg, err error) {
	msg, err := sub.readMsg()
	if err != nil {
		return nil, errors.Errorf("error reading from subscription: %v", err)
	}

	switch msg.Type {
	case msgType_Tx:
		tx := msg.Payload.(tree.Tx)
		return &prototree.SubscriptionMsg{Tx: &tx}, nil

	case msgType_EncryptedTx:
		encryptedTx, ok := msg.Payload.(prototree.EncryptedTx)
		if !ok {
			return nil, errors.Errorf("encrypted tx: bad payload: (%T) %v", msg.Payload, msg.Payload)
		}
		bs, err := sub.t.keyStore.OpenMessageFrom(
			encryptedTx.RecipientAddress,
			crypto.EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, errors.Errorf("while decrypting tx: %v", err)
		}

		var tx tree.Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return nil, errors.Errorf("while unmarshaling encrypted tx: %v", err)
		} else if encryptedTx.TxID != tx.ID {
			return nil, errors.Errorf("encrypted tx ID does not match")
		}
		return &prototree.SubscriptionMsg{Tx: &tx, EncryptedTx: &encryptedTx}, nil

	default:
		return nil, errors.New("protocol error, expecting msgType_Tx or msgType_EncryptedTx")
	}
}

type writableSubscription struct {
	*peerConn
	stateURI string
	chClosed chan struct{}
}

func newWritableSubscription(peerConn *peerConn, stateURI string) *writableSubscription {
	return &writableSubscription{peerConn, stateURI, make(chan struct{})}
}

func (sub *writableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *writableSubscription) Close() error {
	close(sub.chClosed)
	return nil
}

func (sub *writableSubscription) Closed() <-chan struct{} {
	return sub.chClosed
}

func (sub *writableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	err = sub.peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	return sub.peerConn.Put(ctx, tx, state, leaves)
}
