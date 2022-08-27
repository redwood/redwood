package libp2p

import (
	"context"

	"go.uber.org/multierr"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm/libp2p/pb"
	"redwood.dev/swarm/prototree"
	"redwood.dev/utils"
)

type readableSubscription struct {
	*peerConn
}

var _ prototree.ReadableSubscription = (*readableSubscription)(nil)

func (sub *readableSubscription) Read() (prototree.SubscriptionMsg, error) {
	var msg pb.TreeMessage
	err := sub.readProtobuf(nil, &msg) // Nil context so that we don't time out while waiting for new txs
	if err != nil {
		return prototree.SubscriptionMsg{}, errors.Errorf("error reading from subscription: %v", err)
	}

	if tx := msg.GetTx(); tx != nil {
		return prototree.SubscriptionMsg{Tx: tx}, nil
	} else if etx := msg.GetEncryptedTx(); etx != nil {
		return prototree.SubscriptionMsg{EncryptedTx: &etx.EncryptedTx}, nil
	}
	return prototree.SubscriptionMsg{}, errors.Errorf("protocol error, expecting Tx or EncryptedTx: %v", utils.PrettyJSON(msg))
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

func (sub *writableSubscription) Start() error {
	return sub.Process.Start()
}

func (sub *writableSubscription) Close() error {
	sub.Infof("libp2p writable subscription closed (%v)", sub.stateURI)
	return multierr.Append(
		sub.peerConn.Close(),
		sub.Process.Close(),
	)
}

func (sub *writableSubscription) Put(ctx context.Context, msg prototree.SubscriptionMsg) (err error) {
	// err = sub.peerConn.EnsureConnected(ctx)
	// if err != nil {
	// 	return err
	// }
	if msg.EncryptedTx != nil {
		return sub.peerConn.SendEncryptedTx(ctx, *msg.EncryptedTx)
	} else if msg.Tx != nil {
		return sub.peerConn.SendTx(ctx, *msg.Tx)
	} else {
		panic("invariant violation")
	}
}

func (sub writableSubscription) String() string {
	return sub.peerConn.DialInfo().String() + " (" + sub.stateURI + ")"
}
