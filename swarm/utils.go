package swarm

import (
	"context"
	"time"
)

func TryEndpoints(
	ctx context.Context,
	transports map[string]Transport,
	endpoints map[PeerDialInfo]PeerEndpoint,
	fn func(ctx context.Context, peerConn PeerConn) error,
) <-chan struct{} {
	ctx, cancel := context.WithCancel(ctx)

	var numDialable int
	for _, endpoint := range endpoints {
		if !endpoint.Dialable() {
			continue
		}

		endpoint := endpoint
		dialInfo := endpoint.DialInfo()

		if _, exists := transports[dialInfo.TransportName]; !exists {
			continue
		}
		numDialable++

		go func() {
			peerConn, err := transports[dialInfo.TransportName].NewPeerConn(ctx, dialInfo.DialAddr)
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

				err := do(ctx, peerConn, fn)
				if err != nil {
					wait(ctx, peerConn.RemainingBackoff())
					continue
				}
				return
			}
		}()
	}
	if numDialable == 0 {
		cancel()
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
