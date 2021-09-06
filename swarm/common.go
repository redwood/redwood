package swarm

import (
	"context"

	"github.com/pkg/errors"

	"redwood.dev/process"
)

//go:generate mockery --name Transport --output ./mocks/ --case=underscore
type Transport interface {
	process.Interface
	NewPeerConn(ctx context.Context, dialAddr string) (PeerConn, error)
}

//go:generate mockery --name PeerConn --output ./mocks/ --case=underscore
type PeerConn interface {
	PeerDetails

	DeviceSpecificID() string
	Transport() Transport
	EnsureConnected(ctx context.Context) error
	Close() error

	AnnouncePeers(ctx context.Context, peerDialInfos []PeerDialInfo) error
}

var (
	ErrProtocol    = errors.New("protocol error")
	ErrPeerIsSelf  = errors.New("peer is self")
	ErrUnreachable = errors.New("peer unreachable")
)
