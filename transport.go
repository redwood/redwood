package redwood

import (
	"context"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Transport interface {
	Ctx() *ctx.Context
	Start() error
	Name() string

	SetHost(host Host)
	NewPeerConn(ctx context.Context, dialAddr string) (Peer, error)
	ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error)
	ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error)
	PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error)
	AnnounceRef(ctx context.Context, refID types.RefID) error
}

type Peer interface {
	PeerDetails

	Transport() Transport
	EnsureConnected(ctx context.Context) error
	CloseConn() error

	// Transactions
	Subscribe(ctx context.Context, stateURI string) (TxSubscriptionClient, error)
	Put(tx Tx, leaves []types.ID) error
	PutPrivate(tx Tx, leaves []types.ID) error
	Ack(stateURI string, txID types.ID) error

	// State subscriptions
	PutState(state tree.Node, leaves []types.ID) error

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

type ChallengeIdentityResponse struct {
	Signature           []byte `json:"signature"`
	EncryptingPublicKey []byte `json:"encryptingPublicKey"`
}

type FetchRefResponse struct {
	Header *FetchRefResponseHeader `json:"header,omitempty"`
	Body   *FetchRefResponseBody   `json:"body,omitempty"`
}

type FetchRefResponseHeader struct{}

type FetchRefResponseBody struct {
	Data []byte `json:"data"`
	End  bool   `json:"end"`
}

type EncryptedTx struct {
	TxID             types.ID `json:"txID"`
	EncryptedPayload []byte   `json:"encryptedPayload"`
	SenderPublicKey  []byte   `json:"senderPublicKey"`
}

type TxSubscriptionClient interface {
	Read() (*Tx, []types.ID, error)
	Close() error
}

type FetchHistoryHandler func(stateURI string, parents []types.ID, toVersion types.ID, peer Peer) error
type AckHandler func(txID types.ID, peer Peer)
type TxHandler func(tx Tx, peer Peer)
type PrivateTxHandler func(encryptedTx EncryptedTx, peer Peer)
type AuthorizeSelfHandler func(challengeMsg types.ChallengeMsg, peer Peer) error
type FetchRefHandler func(refHash types.RefID, peer Peer)

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
