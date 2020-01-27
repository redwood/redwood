package redwood

import (
	"context"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type Transport interface {
	Ctx() *ctx.Context
	Start() error
	Name() string

	SetFetchHistoryHandler(handler FetchHistoryHandler)
	SetTxHandler(handler TxHandler)
	SetPrivateTxHandler(handler PrivateTxHandler)
	SetAckHandler(handler AckHandler)
	SetVerifyAddressHandler(handler VerifyAddressHandler)
	SetFetchRefHandler(handler FetchRefHandler)

	GetPeerByConnStrings(ctx context.Context, reachableAt StringSet) (Peer, error)
	ForEachProviderOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error)
	ForEachSubscriberToStateURI(ctx context.Context, stateURI string) (<-chan Peer, error)
	ForEachProviderOfRef(ctx context.Context, refHash types.Hash) (<-chan Peer, error)
	PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error)
	AnnounceRef(refHash types.Hash) error
}

type Peer interface {
	Transport() Transport
	ReachableAt() StringSet
	Address() types.Address
	SetAddress(addr types.Address)
	EnsureConnected(ctx context.Context) error
	WriteMsg(msg Msg) error
	ReadMsg() (Msg, error)
	CloseConn() error
}

type FetchHistoryHandler func(stateURI string, parents []types.ID, toVersion types.ID, peer Peer) error
type AckHandler func(txID types.ID, peer Peer)
type TxHandler func(tx Tx, peer Peer)
type PrivateTxHandler func(encryptedTx EncryptedTx, peer Peer)
type VerifyAddressHandler func(challengeMsg types.ChallengeMsg, peer Peer) error
type FetchRefHandler func(refHash types.Hash, peer Peer)

type subscriptionOut struct {
	peer   Peer
	chDone chan struct{}
}

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
