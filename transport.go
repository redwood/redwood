package redwood

import (
	"context"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Transport interface {
	Ctx() *ctx.Context
	Start() error

	SetPutHandler(handler PutHandler)
	SetAckHandler(handler AckHandler)
	SetVerifyAddressHandler(handler VerifyAddressHandler)

	AddPeer(ctx context.Context, multiaddrString string) error
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

type AckHandler func(txHash Hash, peer Peer)
type PutHandler func(tx Tx, peer Peer)
type VerifyAddressHandler func(challengeMsg []byte) (VerifyAddressResponse, error)

type subscriptionOut struct {
	peer   Peer
	chDone chan struct{}
}

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
