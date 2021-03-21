package redwood

import (
	"context"

	"github.com/pkg/errors"

	"redwood.dev/tree"
	"redwood.dev/types"
)

type Transport interface {
	Start() error
	Close()
	Name() string

	SetHost(host Host)
	NewPeerConn(ctx context.Context, dialAddr string) (Peer, error)
	ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error)
	ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error)
	PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error)
	AnnounceRef(ctx context.Context, refID types.RefID) error
}

//go:generate mockery --name Peer --output ./mocks/ --case=underscore
type Peer interface {
	PeerDetails

	Transport() Transport
	EnsureConnected(ctx context.Context) error
	Close() error

	AnnouncePeers(ctx context.Context, peerDialInfos []PeerDialInfo) error

	// Transactions
	Subscribe(ctx context.Context, stateURI string) (ReadableSubscription, error)
	Put(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error
	Ack(stateURI string, txID types.ID) error

	// Identity/authentication
	ChallengeIdentity(challengeMsg types.ChallengeMsg) error
	RespondChallengeIdentity(verifyAddressResponse []ChallengeIdentityResponse) error
	ReceiveChallengeIdentityResponse() ([]ChallengeIdentityResponse, error)

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
	TxID             types.ID      `json:"txID"`
	EncryptedPayload []byte        `json:"encryptedPayload"`
	SenderPublicKey  []byte        `json:"senderPublicKey"`
	RecipientAddress types.Address `json:"recipientAddress"`
}

type FetchHistoryHandler func(stateURI string, parents []types.ID, toVersion types.ID, peer Peer) error
type AckHandler func(txID types.ID, peer Peer)
type TxHandler func(tx Tx, peer Peer)
type PrivateTxHandler func(encryptedTx EncryptedTx, peer Peer)
type AuthorizeSelfHandler func(challengeMsg types.ChallengeMsg, peer Peer) error
type FetchRefHandler func(refHash types.RefID, peer Peer)

var ErrNoPeersForURL = errors.New("no known peers for the provided url")
