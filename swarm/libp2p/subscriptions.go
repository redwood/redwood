package libp2p

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"redwood.dev/crypto"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm"
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
			crypto.AsymEncPubkeyFromBytes(encryptedTx.SenderPublicKey),
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
	process.Process
	log.Logger
	peerConn *peerConn
	stateURI string
}

func newWritableSubscription(peerConn *peerConn, stateURI string) *writableSubscription {
	return &writableSubscription{
		Process:  *process.New("sub impl " + peerConn.DialInfo().String() + " " + stateURI),
		Logger:   log.NewLogger(TransportName),
		peerConn: peerConn,
		stateURI: stateURI,
	}
}

func (sub *writableSubscription) DialInfo() swarm.PeerDialInfo {
	return sub.peerConn.DialInfo()
}

func (sub *writableSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *writableSubscription) Close() error {
	sub.Infof(0, "libp2p writable subscription closed (%v)", sub.stateURI)
	return multierr.Append(
		sub.peerConn.Close(),
		sub.Process.Close(),
	)
}

func (sub *writableSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) (err error) {
	err = sub.peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	return sub.peerConn.Put(ctx, tx, state, leaves)
}

func (sub writableSubscription) String() string {
	return sub.peerConn.DialInfo().String() + " (" + sub.stateURI + ")"
}
