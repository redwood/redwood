package swarm

import (
	"context"
	"time"
)

// TryEndpoints accepts a list of peer endpoints for a single peer, attempts to
// establish connections to each of the endpoints concurrently, and runs the provided
// function on each one. As long as the function returns an error, it will continue
// attempting (while respecting the backoff for that endpoint). As soon as the
// function succeeds once, for a single endpoint, all connections are closed and
// TryEndpoints terminates. The returned channel closes when termination occurs.
func TryEndpoints(
	ctx context.Context,
	transports map[string]Transport,
	endpoints map[PeerDialInfo]PeerEndpoint,
	fn func(ctx context.Context, peerConn PeerConn) error,
) (chDone <-chan struct{}) {

	var numDialable int
	for _, endpoint := range endpoints {
		if !endpoint.Dialable() {
			continue
		}
		numDialable++
	}
	if numDialable == 0 {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	ctx, cancel := context.WithCancel(ctx)

	for _, endpoint := range endpoints {
		dialInfo := endpoint.DialInfo()

		if _, exists := transports[dialInfo.TransportName]; !exists {
			continue
		}

		go func() {
			peerConn, err := transports[dialInfo.TransportName].NewPeerConn(ctx, dialInfo.DialAddr)
			if err != nil {
				return
			}
			defer cancel()

			for {
				if !peerConn.Ready() {
					wait(ctx, peerConn.RemainingBackoff())
				}

				select {
				case <-ctx.Done():
					return
				default:
				}

				err := do(ctx, peerConn, fn)
				if err != nil {
					wait(ctx, peerConn.RemainingBackoff())
					continue
				}
				return
			}
		}()
	}
	return ctx.Done()
}

func do(ctx context.Context, peerConn PeerConn, fn func(ctx context.Context, peerConn PeerConn) error) error {
	err := peerConn.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	defer peerConn.Close()
	return fn(ctx, peerConn)
}

func wait(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
