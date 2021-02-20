package redwood

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"

	"redwood.dev/ctx"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type peerPool struct {
	ctx.Logger

	chPeers         chan Peer
	chPeerAvailable chan struct{}
	chNeedNewPeer   chan struct{}
	chProviders     <-chan Peer
	chStop          chan struct{}
	sem             *semaphore.Weighted

	fnGetPeers func(ctx context.Context) (<-chan Peer, error)

	peers map[PeerDialInfo]struct {
		peer  Peer
		state peerState
	}
	peersMu sync.RWMutex
}

type peerState int

const (
	peerState_Unknown peerState = iota
	peerState_Strike
	peerState_InUse
)

func newPeerPool(concurrentConns uint64, fnGetPeers func(ctx context.Context) (<-chan Peer, error)) *peerPool {
	chProviders := make(chan Peer)
	close(chProviders)

	p := &peerPool{
		Logger:          ctx.NewLogger("peer pool"),
		chPeerAvailable: make(chan struct{}, concurrentConns),
		chPeers:         make(chan Peer, concurrentConns),
		chNeedNewPeer:   make(chan struct{}, concurrentConns),
		chProviders:     chProviders,
		chStop:          make(chan struct{}),
		sem:             semaphore.NewWeighted(int64(concurrentConns)),
		fnGetPeers:      fnGetPeers,
		peers: make(map[PeerDialInfo]struct {
			peer  Peer
			state peerState
		}),
	}

	// Searcher:
	//   - Adds peers to the `peers` map as they're received from the `fnGetPeers` channel
	//   - If the transports stop searching before `.Close()` is called, the search is reinitiated
	go func() {
		ctx, cancel := utils.ContextFromChan(p.chStop)
		defer cancel()

		for {
		FindPeerLoop:
			for {
				select {
				case <-p.chStop:
					return
				case peer, open := <-p.chProviders:
					if !open {
						func() {
							p.sem.Acquire(ctx, 1)
							defer p.sem.Release(1)
							p.restartSearch(ctx)
						}()
						continue FindPeerLoop
					}
					p.addPeerToPool(peer)
				}
			}
		}
	}()

	// Provider:
	//   - Fulfills requests for peers as peers become available
	go func() {
		defer close(p.chPeers)

		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-p.chStop:
				return
			case <-p.chNeedNewPeer:
			}

			var peer Peer
			for {
				select {
				case <-p.chStop:
					return
				case <-ticker.C:
				case <-p.chPeerAvailable:
				}

				peer = p.nextAvailablePeer()
				if peer != nil {
					break
				}
			}
			select {
			case <-p.chStop:
				return
			case p.chPeers <- peer:
			}
		}
	}()

	// This goroutine fills the peer pool with the initial peers.
	go func() {
		for i := uint64(0); i < concurrentConns; i++ {
			select {
			case <-p.chStop:
				return
			case p.chNeedNewPeer <- struct{}{}:
			}
		}
	}()

	return p
}

func (p *peerPool) addPeerToPool(peer Peer) {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()

	if _, exists := p.peers[peer.DialInfo()]; !exists {
		p.Debugf("[peer pool] found peer %v", peer.DialInfo())

		p.peers[peer.DialInfo()] = struct {
			peer  Peer
			state peerState
		}{peer, peerState_Unknown}

		select {
		case p.chPeerAvailable <- struct{}{}:
		case <-p.chStop:
			return
		}
	}
}

func (p *peerPool) nextAvailablePeer() Peer {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()

	for _, peer := range p.peers {
		if peer.state != peerState_Unknown {
			p.Debugf("skipping peer: not ready (%v, %v)", peer.peer.DialInfo(), peer.state)
			continue
		} else if !peer.peer.Ready() {
			p.Debugf("skipping peer: failures=%v lastFailure=%v", peer.peer.Failures(), time.Now().Sub(peer.peer.LastFailure()))
			continue
		} else if peer.peer.Address() == (types.Address{}) {
			p.Debugf("skipping peer: unverified (%v: %v)", peer.peer.DialInfo().TransportName, peer.peer.DialInfo().DialAddr)
			continue
		}
		return peer.peer
	}
	return nil
}

func (p *peerPool) restartSearch(ctx context.Context) {
	var err error
	p.chProviders, err = p.fnGetPeers(ctx)
	if err != nil {
		p.Warnf("[peer pool] error finding peers: %v", err)
		// @@TODO: exponential backoff
	}
}

func (p *peerPool) Close() {
	close(p.chStop)
}

func (p *peerPool) GetPeer() (Peer, error) {
	ctx, cancel := utils.ContextFromChan(p.chStop)
	defer cancel()

	p.sem.Acquire(ctx, 1)

	for {
		select {
		case peer, open := <-p.chPeers:
			if !open {
				return nil, errors.New("connection closed")
			}

			if !peer.Ready() {
				p.Warnf("skipping peer: failures=%v lastFailure=%v", peer.Failures(), time.Now().Sub(peer.LastFailure()))
				p.ReturnPeer(peer, false)
				continue
			}
			p.setPeerState(peer, peerState_InUse)
			return peer, nil

		case <-p.chStop:
			return nil, nil
		}
	}
}

func (p *peerPool) ReturnPeer(peer Peer, strike bool) {
	if strike {
		// Close the faulty connection
		peer.Close()

		p.setPeerState(peer, peerState_Strike)

		// Try to obtain a new peer
		select {
		case p.chNeedNewPeer <- struct{}{}:
		case <-p.chStop:
			return
		}

	} else {
		// Return the peer to the pool
		p.setPeerState(peer, peerState_Unknown)

		select {
		case p.chPeerAvailable <- struct{}{}:
		case <-p.chStop:
			return
		}
	}
	p.sem.Release(1)
}

func (p *peerPool) setPeerState(peer Peer, state peerState) {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()

	peerInfo := p.peers[peer.DialInfo()]
	peerInfo.state = state
	p.peers[peer.DialInfo()] = peerInfo
}
