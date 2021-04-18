package libp2p

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type readableSubscription struct {
	*peerConn
}

func (sub *readableSubscription) Read() (_ *swarm.SubscriptionMsg, err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	msg, err := sub.readMsg()
	if err != nil {
		return nil, errors.Errorf("error reading from subscription: %v", err)
	}

	switch msg.Type {
	case MsgType_Put:
		tx := msg.Payload.(tree.Tx)
		return &swarm.SubscriptionMsg{Tx: &tx}, nil

	case MsgType_Private:
		encryptedTx, ok := msg.Payload.(swarm.EncryptedTx)
		if !ok {
			return nil, errors.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
		}
		bs, err := sub.t.keyStore.OpenMessageFrom(
			encryptedTx.RecipientAddress,
			crypto.EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, errors.Errorf("error decrypting tx: %v", err)
		}

		var tx tree.Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return nil, errors.Errorf("error decoding tx: %v", err)
		} else if encryptedTx.TxID != tx.ID {
			return nil, errors.Errorf("private tx id does not match")
		}
		return &swarm.SubscriptionMsg{Tx: &tx, EncryptedTx: &encryptedTx}, nil

	default:
		return nil, errors.New("protocol error, expecting MsgType_Put or MsgType_Private")
	}
}

type writableSubscription struct {
	*peerConn
}

func (sub *writableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	err = sub.peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	return sub.peerConn.Put(ctx, tx, state, leaves)
}
