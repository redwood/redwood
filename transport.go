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

	SetHost(host Host)
	GetPeerByConnStrings(ctx context.Context, reachableAt StringSet) (Peer, error)
	ForEachProviderOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error)
	ForEachProviderOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error)
	PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error)
	AnnounceRef(ctx context.Context, refID types.RefID) error
}

type Peer interface {
	Transport() Transport
	ReachableAt() StringSet
	Address() types.Address
	SetAddress(addr types.Address)
	PublicKeypairs() (SigningPublicKey, EncryptingPublicKey)
	DialInfos() []PeerDialInfo
	EnsureConnected(ctx context.Context) error
	CloseConn() error

	// Transactions
	Subscribe(ctx context.Context, stateURI string) (TxSubscription, error)
	Put(tx Tx) error
	PutPrivate(tx EncryptedTx) error
	Ack(txID types.ID) error

	// Identity/authentication
	ChallengeIdentity(challengeMsg types.ChallengeMsg) error
	RespondChallengeIdentity(verifyAddressResponse ChallengeIdentityResponse) error
	ReceiveChallengeIdentityResponse() (ChallengeIdentityResponse, error)

	// Refs (referenced extrinsics)
	FetchRef(refID types.RefID) error
	SendRefHeader() error
	SendRefPacket(data []byte, end bool) error
	ReceiveRefHeader() (FetchRefResponseHeader, error)
	ReceiveRefPacket() (FetchRefResponseBody, error)
}

type TxSubscription interface {
	Read() (*Tx, error)
	Close() error
}

type FetchHistoryHandler func(stateURI string, parents []types.ID, toVersion types.ID, peer Peer) error
type AckHandler func(txID types.ID, peer Peer)
type TxHandler func(tx Tx, peer Peer)
type PrivateTxHandler func(encryptedTx EncryptedTx, peer Peer)
type AuthorizeSelfHandler func(challengeMsg types.ChallengeMsg, peer Peer) error
type FetchRefHandler func(refHash types.RefID, peer Peer)

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
