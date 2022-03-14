package swarm

import (
	"context"
	"time"

	"redwood.dev/log"
	"redwood.dev/process"
)

type BaseProtocol[T Transport, P PeerConn] struct {
	process.Process
	log.Logger
	Transports map[string]T
}

func (t BaseProtocol[T, P]) TryPeerDevices(
	ctx context.Context,
	parent *process.Process,
	peerDevices []PeerDevice,
	fn func(ctx context.Context, peer P) error,
) (chDone <-chan struct{}) {
	ctx, cancel := context.WithCancel(ctx)

	child := parent.NewChild(ctx, "TryPeerDevices")
	defer child.AutocloseWithCleanup(cancel)

	for _, peer := range peerDevices {
		t.TryEndpoints(ctx, child, peer, func(ctx context.Context, peerConn P) error {
			return fn(ctx, peerConn)
		})
	}

	return child.Done()
}

// TryEndpoints accepts a list of peer endpoints for a single peer, attempts to
// establish connections to each of the endpoints concurrently, and runs the provided
// function on each one. As long as the function returns an error, it will continue
// attempting (while respecting the backoff for that endpoint). As soon as the
// function succeeds once, for a single endpoint, all connections are closed and
// TryEndpoints terminates. The returned channel closes when termination occurs.
func (t BaseProtocol[T, P]) TryEndpoints(
	ctx context.Context,
	parent *process.Process,
	peer PeerDevice,
	fn func(ctx context.Context, peerConn P) error,
) {
	ctx, cancel := context.WithCancel(ctx)

	child := parent.NewChild(ctx, "TryEndpoints")
	defer child.AutocloseWithCleanup(cancel)

	for _, endpoint := range peer.Endpoints() {
		if !endpoint.Dialable() {
			continue
		}

		dialInfo := endpoint.DialInfo()

		if _, exists := t.Transports[dialInfo.TransportName]; !exists {
			continue
		}

		child.Go(ctx, dialInfo.String(), func(ctx context.Context) {
			peerConn, err := t.Transports[dialInfo.TransportName].NewPeerConn(ctx, dialInfo.DialAddr)
			if err != nil {
				return
			}
			defer cancel()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if !peerConn.Ready() {
					wait(ctx, peerConn.RemainingBackoff())
					continue
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				err := do(ctx, peerConn.(P), fn)
				if err != nil {
					wait(ctx, peerConn.RemainingBackoff())
					continue
				}
				return
			}
		})
	}
}

func do[P PeerConn](ctx context.Context, peerConn P, fn func(ctx context.Context, peerConn P) error) error {
	err := peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	defer peerConn.Close()
	return fn(ctx, peerConn)
}

func wait(ctx context.Context, d time.Duration) {
	// timer := time.NewTimer(d)
	// defer timer.Stop()
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
