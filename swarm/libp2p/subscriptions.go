package libp2p

import (
	"context"

	"go.uber.org/multierr"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
)

type readableSubscription struct {
	*peerConn
}

var _ prototree.ReadableSubscription = (*readableSubscription)(nil)

func (sub *readableSubscription) Read() (_ prototree.SubscriptionMsg, err error) {
	msg, err := sub.readMsg()
	if err != nil {
		return prototree.SubscriptionMsg{}, errors.Errorf("error reading from subscription: %v", err)
	}

	switch msg.Type {
	case msgType_Tx:
		tx := msg.Payload.(tree.Tx)
		return prototree.SubscriptionMsg{Tx: &tx}, nil

	case msgType_EncryptedTx:
		encryptedTx := msg.Payload.(prototree.EncryptedTx)
		return prototree.SubscriptionMsg{EncryptedTx: &encryptedTx}, nil

	default:
		return prototree.SubscriptionMsg{}, errors.New("protocol error, expecting msgType_Tx or msgType_EncryptedTx")
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

func (sub *writableSubscription) Close() error {
	sub.Infof(0, "libp2p writable subscription closed (%v)", sub.stateURI)
	return multierr.Append(
		sub.peerConn.Close(),
		sub.Process.Close(),
	)
}

func (sub *writableSubscription) Put(ctx context.Context, msg prototree.SubscriptionMsg) (err error) {
	err = sub.peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	if msg.EncryptedTx != nil {
		return sub.peerConn.SendPrivateTx(ctx, *msg.EncryptedTx)
	} else if msg.Tx != nil {
		return sub.peerConn.SendTx(ctx, *msg.Tx)
	} else {
		panic("invariant violation")
	}
}

func (sub writableSubscription) String() string {
	return sub.peerConn.DialInfo().String() + " (" + sub.stateURI + ")"
}
