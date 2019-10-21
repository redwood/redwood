package redwood

import (
	"context"

	"github.com/pkg/errors"
)

type Transport interface {
	AddPeer(ctx context.Context, multiaddrString string) error

	SetPutHandler(handler PutHandler)
	SetAckHandler(handler AckHandler)
	SetVerifyAddressHandler(handler VerifyAddressHandler)

	ForEachProviderOfURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error
	ForEachSubscriberToURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error
	PeersWithAddress(ctx context.Context, address Address) (<-chan Peer, error)
}

type Peer interface {
	ID() string
	EnsureConnected(ctx context.Context) error
	WriteMsg(msg Msg) error
	ReadMsg() (Msg, error)
	CloseConn() error
}

type AckHandler func(version ID, peer Peer)
type PutHandler func(tx Tx, peer Peer)
type VerifyAddressHandler func(challengeMsg []byte) ([]byte, error)

type subscriptionOut struct {
	peer   Peer
	chDone chan struct{}
}

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
