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

type PeerPool[P PeerConn] interface {
	process.Interface
	GetPeer(ctx context.Context) (_ P, err error)
	ReturnPeer(peer P, strike bool)
}

type peerPool[P PeerConn] struct {
	process.Process
	log.Logger

	concurrentConns uint64
	peersAvailable  *utils.Mailbox[P]
	peersInTimeout  *utils.Mailbox[P]
	chProviders     <-chan P
	chPeers         chan P
	sem             *semaphore.Weighted

	fnGetPeers func(ctx context.Context) (<-chan P, error)

	peers   map[PeerDialInfo]peersMapEntry[P]
	peersMu sync.RWMutex

	activePeerDeviceIDs   map[string]struct{}
	activePeerDeviceIDsMu sync.RWMutex
}

type peerState int

const (
	peerState_Unknown peerState = iota
	peerState_Strike
	peerState_InUse
)

type peersMapEntry[P PeerConn] struct {
	peer  P
	state peerState
}

func NewPeerPool[P PeerConn](concurrentConns uint64, fnGetPeers func(ctx context.Context) (<-chan P, error)) *peerPool[P] {
	chProviders := make(chan P)
	close(chProviders)

	return &peerPool[P]{
		Process:             *process.New("PeerPool"),
		Logger:              log.NewLogger("peer pool"),
		concurrentConns:     concurrentConns,
		peersAvailable:      utils.NewMailbox[P](0),
		peersInTimeout:      utils.NewMailbox[P](0),
		chProviders:         chProviders,
		chPeers:             make(chan P),
		sem:                 semaphore.NewWeighted(int64(concurrentConns)),
		fnGetPeers:          fnGetPeers,
		peers:               make(map[PeerDialInfo]peersMapEntry[P]),
		activePeerDeviceIDs: make(map[string]struct{}),
	}
}

func (p *peerPool[P]) Start() error {
	p.Process.Start()
	p.Process.Go(nil, "fillPool", p.fillPool)
	p.Process.Go(nil, "deliverAvailablePeers", p.deliverAvailablePeers)
	p.Process.Go(nil, "handlePeersInTimeout", p.handlePeersInTimeout)
	return nil
}

// fillPool has two responsibilities:
//   - Adds peers to the `peers` map as they're received from the `fnGetPeers` channel
//   - If the transports stop searching before `.Close()` is called, the search is reinitiated
func (p *peerPool[P]) fillPool(ctx context.Context) {
	for {
		select {
		case <-p.Process.Done():
			return
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
					p.peers[peer.DialInfo()] = peersMapEntry[P]{peer, peerState_Unknown}
					p.peersAvailable.Deliver(peer)
				}
			}()
		}
	}
}

func (p *peerPool[P]) restartSearch(ctx context.Context) {
	var err error
	p.chProviders, err = p.fnGetPeers(ctx)
	if err != nil {
		p.Warnf("[peer pool] error finding peers: %v", err)
	}
}

func (p *peerPool[P]) deliverAvailablePeers(ctx context.Context) {
	for {
		select {
		case <-p.Process.Done():
			return
		case <-ctx.Done():
			return
		case <-p.peersAvailable.Notify():
			for _, peer := range p.peersAvailable.RetrieveAll() {
				select {
				case <-ctx.Done():
					return
				case p.chPeers <- peer:
				}
			}
		}
	}
}

func (p *peerPool[P]) handlePeersInTimeout(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.Process.Done():
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		for _, peer := range p.peersInTimeout.RetrieveAll() {
			if p.peerIsEligible(peer) {
				p.peersAvailable.Deliver(peer)
			} else {
				p.peersInTimeout.Deliver(peer)
			}
		}
	}
}

func (p *peerPool[P]) GetPeer(ctx context.Context) (_ P, err error) {
	ctx, cancel := utils.CombinedContext(ctx, p.Process.Ctx())
	defer cancel()

	err = p.sem.Acquire(ctx, 1)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			p.sem.Release(1)
		}
	}()

	for {
		select {
		case <-p.Process.Done():
			var p P
			return p, process.ErrClosed

		case <-ctx.Done():
			var p P
			return p, ctx.Err()

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
				} else if !p.peerIsEligible(peer) {
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

func (p *peerPool[P]) peerIsEligible(peerConn P) bool {
	return peerConn.Ready() && len(peerConn.Addresses()) > 0 // && !p.deviceIDAlreadyActive(peerConn.DeviceUniqueID())
}

func (p *peerPool[P]) deviceIDAlreadyActive(deviceID string) bool {
	p.activePeerDeviceIDsMu.RLock()
	defer p.activePeerDeviceIDsMu.RUnlock()
	_, exists := p.activePeerDeviceIDs[deviceID]
	return exists
}

func (p *peerPool[P]) countActivePeers() int {
	var i int
	for _, entry := range p.peers {
		if entry.state == peerState_InUse {
			i++
		}
	}
	return i
}

func (p *peerPool[P]) ReturnPeer(peer P, strike bool) {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	defer p.sem.Release(1)

	if strike {
		p.setPeerState(peer, peerState_Strike)

	} else {
		// Return the peer to the pool
		p.setPeerState(peer, peerState_Unknown)
		p.peersAvailable.Deliver(peer)
	}
}

func (p *peerPool[P]) setPeerState(peer P, state peerState) {
	peerInfo := p.peers[peer.DialInfo()]
	peerInfo.state = state
	p.peers[peer.DialInfo()] = peerInfo

	if state == peerState_InUse {
		func() {
			p.activePeerDeviceIDsMu.Lock()
			defer p.activePeerDeviceIDsMu.Unlock()
			p.activePeerDeviceIDs[peer.DeviceUniqueID()] = struct{}{}
		}()
	} else {
		func() {
			p.activePeerDeviceIDsMu.Lock()
			defer p.activePeerDeviceIDsMu.Unlock()
			delete(p.activePeerDeviceIDs, peer.DeviceUniqueID())
		}()
	}
}
