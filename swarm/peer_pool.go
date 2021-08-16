package swarm

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils"
)

type PeerPool interface {
	process.Interface
	GetPeer(ctx context.Context) (_ PeerConn, err error)
	ReturnPeer(peer PeerConn, strike bool)
}

type peerPool struct {
	process.Process
	log.Logger

	concurrentConns uint64
	peersAvailable  *utils.Mailbox
	peersInTimeout  *utils.Mailbox
	chProviders     <-chan PeerConn
	chPeers         chan PeerConn
	sem             *semaphore.Weighted

	fnGetPeers func(ctx context.Context) (<-chan PeerConn, error)

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
	peer  PeerConn
	state peerState
}

func NewPeerPool(concurrentConns uint64, fnGetPeers func(ctx context.Context) (<-chan PeerConn, error)) *peerPool {
	chProviders := make(chan PeerConn)
	close(chProviders)

	return &peerPool{
		Process:         *process.New("PeerPool"),
		Logger:          log.NewLogger("peer pool"),
		concurrentConns: concurrentConns,
		peersAvailable:  utils.NewMailbox(0),
		peersInTimeout:  utils.NewMailbox(0),
		chProviders:     chProviders,
		chPeers:         make(chan PeerConn),
		sem:             semaphore.NewWeighted(int64(concurrentConns)),
		fnGetPeers:      fnGetPeers,
		peers:           make(map[PeerDialInfo]peersMapEntry),
	}
}

func (p *peerPool) Start() error {
	p.Process.Start()
	p.Process.Go("fillPool", p.fillPool)
	p.Process.Go("deliverAvailablePeers", p.deliverAvailablePeers)
	p.Process.Go("handlePeersInTimeout", p.handlePeersInTimeout)
	return nil
}

// fillPool has two responsibilities:
//   - Adds peers to the `peers` map as they're received from the `fnGetPeers` channel
//   - If the transports stop searching before `.Close()` is called, the search is reinitiated
func (p *peerPool) fillPool(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
					p.peers[peer.DialInfo()] = peersMapEntry{peer, peerState_Unknown}
					p.peersAvailable.Deliver(peer)
				}
			}()
		}
	}
}

func (p *peerPool) restartSearch(ctx context.Context) {
	var err error
	p.chProviders, err = p.fnGetPeers(ctx)
	if err != nil {
		p.Warnf("[peer pool] error finding peers: %v", err)
		// @@TODO: exponential backoff
	}
}

func (p *peerPool) deliverAvailablePeers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-p.peersAvailable.Notify():
			for _, x := range p.peersAvailable.RetrieveAll() {
				peer := x.(PeerConn)
				select {
				case <-ctx.Done():
					return
				case p.chPeers <- peer:
				}
			}
		}
	}
}

func (p *peerPool) handlePeersInTimeout(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		for _, x := range p.peersInTimeout.RetrieveAll() {
			peer := x.(PeerConn)
			if peer.Ready() && len(peer.Addresses()) > 0 {
				p.peersAvailable.Deliver(peer)
			} else {
				p.peersInTimeout.Deliver(peer)
			}
		}
	}
}

func (p *peerPool) GetPeer(ctx context.Context) (_ PeerConn, err error) {
	ctx, cancel := utils.CombinedContext(ctx, p.Ctx())
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
					p.peersInTimeout.Deliver(peer)
				} else if len(entry.peer.Addresses()) == 0 {
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

func (p *peerPool) ReturnPeer(peer PeerConn, strike bool) {
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

func (p *peerPool) setPeerState(peer PeerConn, state peerState) {
	peerInfo := p.peers[peer.DialInfo()]
	peerInfo.state = state
	p.peers[peer.DialInfo()] = peerInfo
}
