package swarm

import (
	"context"
	"time"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils"
)

type EndpointPool struct {
	process.Process
	log.Logger
	peer       PeerInfo
	transports map[string]Transport
	ctxTimeout time.Duration
	pools      map[string]*process.Pool
	fn         func(ctx context.Context, peerConn PeerConn) error
}

func NewEndpointPool(
	name string,
	peer PeerInfo,
	transports map[string]Transport,
	ctxTimeout time.Duration,
	retryInterval time.Duration,
	fn func(ctx context.Context, peerConn PeerConn) error,
) *EndpointPool {
	pools := make(map[string]*process.Pool)
	for _, e := range peer.Endpoints() {
		tpt := e.DialInfo().TransportName

		if _, exists := transports[tpt]; !exists {
			continue
		}

		if _, exists := pools[tpt]; !exists {
			pools[tpt] = process.NewPool("process.Pool", 1, retryInterval)
		}
	}
	return &EndpointPool{
		Process:    *process.New(name),
		Logger:     log.NewLogger("endpool"),
		peer:       peer,
		transports: transports,
		ctxTimeout: ctxTimeout,
		pools:      pools,
		fn:         fn,
	}
}

func (p *EndpointPool) Start() error {
	err := p.Process.Start()
	if err != nil {
		return err
	}
	defer p.Process.Autoclose()

	for tpt := range p.pools {
		err = p.Process.SpawnChild(nil, p.pools[tpt])
		if err != nil {
			return err
		}
	}

	for _, e := range p.peer.Endpoints() {
		dialInfo := e.DialInfo()
		if !e.Dialable() {
			continue
		} else if _, exists := p.pools[dialInfo.TransportName]; !exists {
			continue
		}
		p.pools[dialInfo.TransportName].Add(poolEndpoint{e})
	}

	for tpt := range p.pools {
		pool := p.pools[tpt]

		p.Process.Go(nil, tpt, func(ctx context.Context) {
			defer pool.Close()

			for {
				if pool.NumItemsPending() == 0 {
					return
				}

				keepTrying := p.do(ctx, pool)
				if !keepTrying {
					return
				}
			}
		})
	}
	return nil
}

func (p *EndpointPool) do(ctx context.Context, pool *process.Pool) (keepTrying bool) {
	ctx, cancel := utils.CombinedContext(ctx, p.ctxTimeout)
	defer cancel()

	e, err := pool.Get(ctx)
	if err != nil {
		return false
	}
	endpoint := e.(poolEndpoint)

	if !endpoint.Ready() {
		pool.RetryLater(endpoint.ID(), time.Now().Add(endpoint.RemainingBackoff()))
		return true
	}

	dialInfo := endpoint.DialInfo()

	peerConn, err := p.transports[dialInfo.TransportName].NewPeerConn(ctx, dialInfo.DialAddr)
	if errors.Cause(err) == ErrPeerIsSelf {
		pool.Complete(endpoint.ID())
		return true
	} else if err != nil {
		p.Errorf("while creating new peer conn: %v", err)
		pool.RetryLater(endpoint.ID(), time.Now().Add(endpoint.RemainingBackoff()))
		return true
	}

	err = peerConn.EnsureConnected(ctx)
	if err != nil {
		p.Errorf("while connecting to peer: %v", err)
		pool.RetryLater(endpoint.ID(), time.Now().Add(endpoint.RemainingBackoff()))
		return true
	}
	defer peerConn.Close()

	err = p.fn(ctx, peerConn)
	if err != nil {
		pool.RetryLater(endpoint.ID(), time.Now().Add(endpoint.RemainingBackoff()))
		return true
	}

	pool.Complete(endpoint.ID())
	return false
}

type poolEndpoint struct {
	PeerEndpoint
}

func (e poolEndpoint) ID() process.PoolUniqueID {
	return e.DialInfo().String()
}