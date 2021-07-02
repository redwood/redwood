package swarm

import (
	"context"

	"github.com/pkg/errors"
)

//go:generate mockery --name Transport --output ./mocks/ --case=underscore
type Transport interface {
	Start() error
	Close()
	Name() string
	NewPeerConn(ctx context.Context, dialAddr string) (PeerConn, error)
}

//go:generate mockery --name Protocol --output ./mocks/ --case=underscore
type Protocol interface {
	Name() string
	Start()
	Close()
}

//go:generate mockery --name PeerConn --output ./mocks/ --case=underscore
type PeerConn interface {
	PeerDetails

	Transport() Transport
	EnsureConnected(ctx context.Context) error
	Close() error

	AnnouncePeers(ctx context.Context, peerDialInfos []PeerDialInfo) error
}

var (
	ErrProtocol   = errors.New("protocol error")
	ErrPeerIsSelf = errors.New("peer is self")
)
