package swarm

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"redwood.dev/log"
	"redwood.dev/utils"
)

type PeerPool interface {
	GetPeer(ctx context.Context) (_ Peer, err error)
	ReturnPeer(peer Peer, strike bool)
	Close()
}

type peerPool struct {
	log.Logger

	concurrentConns uint64
	peersAvailable  *utils.Mailbox
	peersInTimeout  *utils.Mailbox
	chProviders     <-chan Peer
	chPeers         chan Peer
	sem             *semaphore.Weighted
	chStop          chan struct{}
	wgDone          sync.WaitGroup

	fnGetPeers func(ctx context.Context) (<-chan Peer, error)

	peers   map[PeerDialInfo]peersMapEntry
	peersMu sync.RWMutex
}

type peerState int

const (
	peerState_Unknown peerState = iota
	peerState_Strike
	peerState_InUse
)

type peersMapEntry struct {
	peer  Peer
	state peerState
}

func NewPeerPool(concurrentConns uint64, fnGetPeers func(ctx context.Context) (<-chan Peer, error)) *peerPool {
	chProviders := make(chan Peer)
	close(chProviders)

	p := &peerPool{
		Logger:          log.NewLogger("peer pool"),
		concurrentConns: concurrentConns,
		peersAvailable:  utils.NewMailbox(0),
		peersInTimeout:  utils.NewMailbox(0),
		chProviders:     chProviders,
		chPeers:         make(chan Peer),
		chStop:          make(chan struct{}),
		sem:             semaphore.NewWeighted(int64(concurrentConns)),
		fnGetPeers:      fnGetPeers,
		peers:           make(map[PeerDialInfo]peersMapEntry),
	}

	go p.fillPool()
	go p.deliverAvailablePeers()
	go p.handlePeersInTimeout()

	return p
}

func (p *peerPool) Close() {
	close(p.chStop)
}

// fillPool has two responsibilities:
//   - Adds peers to the `peers` map as they're received from the `fnGetPeers` channel
//   - If the transports stop searching before `.Close()` is called, the search is reinitiated
func (p *peerPool) fillPool() {
	ctx, cancel := utils.ContextFromChan(p.chStop)
	defer cancel()

	for {
		select {
		case <-p.chStop:
			return
		case peer, open := <-p.chProviders:
			if !open {
				p.restartSearch(ctx)
				continue
			}

			func() {
				p.peersMu.Lock()
				defer p.peersMu.Unlock()

				if _, exists := p.peers[peer.DialInfo()]; !exists {
					p.Debugf("[peer pool] found peer %v", peer.DialInfo())
					p.peers[peer.DialInfo()] = peersMapEntry{peer, peerState_Unknown}
					p.peersAvailable.Deliver(peer)
				}
			}()
		}
	}
}

func (p *peerPool) restartSearch(ctx context.Context) {
	ctx, _ = utils.CombinedContext(ctx, p.chStop)

	var err error
	p.chProviders, err = p.fnGetPeers(ctx)
	if err != nil {
		p.Warnf("[peer pool] error finding peers: %v", err)
		// @@TODO: exponential backoff
	}
}

func (p *peerPool) deliverAvailablePeers() {
	for {
		select {
		case <-p.chStop:
			return

		case <-p.peersAvailable.Notify():
			for _, x := range p.peersAvailable.RetrieveAll() {
				peer := x.(Peer)
				select {
				case <-p.chStop:
					return
				case p.chPeers <- peer:
				}
			}
		}
	}
}

func (p *peerPool) handlePeersInTimeout() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-p.chStop:
			return
		case <-ticker.C:
		}

		for _, x := range p.peersInTimeout.RetrieveAll() {
			peer := x.(Peer)
			if peer.Ready() && len(peer.Addresses()) > 0 {
				p.peersAvailable.Deliver(peer)
			} else {
				p.peersInTimeout.Deliver(peer)
			}
		}
	}
}

func (p *peerPool) GetPeer(ctx context.Context) (_ Peer, err error) {
	ctx, cancel := utils.CombinedContext(ctx, p.chStop)
	defer cancel()

	err = p.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.sem.Release(1)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case peer := <-p.chPeers:
			var valid bool
			func() {
				p.peersMu.Lock()
				defer p.peersMu.Unlock()

				entry, exists := p.peers[peer.DialInfo()]
				if !exists {
					panic("!exists")
				} else if entry.state != peerState_Unknown {
					panic("!available")
				} else if !entry.peer.Ready() {
					p.Debugf("skipping peer: failures=%v lastFailure=%v", entry.peer.Failures(), time.Now().Sub(entry.peer.LastFailure()))
					p.peersInTimeout.Deliver(peer)
				} else if len(entry.peer.Addresses()) == 0 {
					p.Debugf("skipping peer: unverified (%v: %v)", entry.peer.DialInfo().TransportName, entry.peer.DialInfo().DialAddr)
					p.peersInTimeout.Deliver(peer)
				} else {
					p.setPeerState(peer, peerState_InUse)
					if p.countActivePeers() > int(p.concurrentConns) {
						panic("invariant violation")
					}
					valid = true
				}
			}()
			if !valid {
				continue
			}
			return peer, nil

		}
	}
}

func (p *peerPool) countActivePeers() int {
	var i int
	for _, entry := range p.peers {
		if entry.state == peerState_InUse {
			i++
		}
	}
	return i
}

func (p *peerPool) ReturnPeer(peer Peer, strike bool) {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	defer p.sem.Release(1)

	if strike {
		p.setPeerState(peer, peerState_Strike)
		peer.Close() // Close the faulty connection

	} else {
		// Return the peer to the pool
		p.setPeerState(peer, peerState_Unknown)
		p.peersAvailable.Deliver(peer)
	}
}

func (p *peerPool) setPeerState(peer Peer, state peerState) {
	peerInfo := p.peers[peer.DialInfo()]
	peerInfo.state = state
	p.peers[peer.DialInfo()] = peerInfo
}
