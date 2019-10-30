package redwood

import (
	"context"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Transport interface {
	Ctx() *ctx.Context
	Start() error

	SetTxHandler(handler TxHandler)
	SetPrivateTxHandler(handler PrivateTxHandler)
	SetAckHandler(handler AckHandler)
	SetVerifyAddressHandler(handler VerifyAddressHandler)
	SetFetchRefHandler(handler FetchRefHandler)

	GetPeer(ctx context.Context, multiaddr string) (Peer, error)
	ForEachProviderOfURL(ctx context.Context, theURL string) (<-chan Peer, error)
	ForEachProviderOfRef(ctx context.Context, refHash Hash) (<-chan Peer, error)
	ForEachSubscriberToURL(ctx context.Context, theURL string) (<-chan Peer, error)
	PeersClaimingAddress(ctx context.Context, address Address) (<-chan Peer, error)
}

type Peer interface {
	ID() string
	EnsureConnected(ctx context.Context) error
	WriteMsg(msg Msg) error
	ReadMsg() (Msg, error)
	CloseConn() error
}

type AckHandler func(txHash Hash, peer Peer)
type TxHandler func(tx Tx, peer Peer)
type PrivateTxHandler func(encryptedTx EncryptedTx, peer Peer)
type VerifyAddressHandler func(challengeMsg []byte, peer Peer) error
type FetchRefHandler func(refHash Hash, peer Peer)

type subscriptionOut struct {
	peer   Peer
	chDone chan struct{}
}

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
