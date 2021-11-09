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

	activePeerDeviceIDs   map[string]struct{}
	activePeerDeviceIDsMu sync.RWMutex
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
		Process:             *process.New("PeerPool"),
		Logger:              log.NewLogger("peer pool"),
		concurrentConns:     concurrentConns,
		peersAvailable:      utils.NewMailbox(0),
		peersInTimeout:      utils.NewMailbox(0),
		chProviders:         chProviders,
		chPeers:             make(chan PeerConn),
		sem:                 semaphore.NewWeighted(int64(concurrentConns)),
		fnGetPeers:          fnGetPeers,
		peers:               make(map[PeerDialInfo]peersMapEntry),
		activePeerDeviceIDs: make(map[string]struct{}),
	}
}

func (p *peerPool) Start() error {
	p.Process.Start()
	p.Process.Go(nil, "fillPool", p.fillPool)
	p.Process.Go(nil, "deliverAvailablePeers", p.deliverAvailablePeers)
	p.Process.Go(nil, "handlePeersInTimeout", p.handlePeersInTimeout)
	return nil
}

// fillPool has two responsibilities:
//   - Adds peers to the `peers` map as they're received from the `fnGetPeers` channel
//   - If the transports stop searching before `.Close()` is called, the search is reinitiated
func (p *peerPool) fillPool(ctx context.Context) {
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
		case <-p.Process.Done():
			return
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
		case <-p.Process.Done():
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		for _, x := range p.peersInTimeout.RetrieveAll() {
			peer := x.(PeerConn)
			if p.peerIsEligible(peer) {
				p.peersAvailable.Deliver(peer)
			} else {
				p.peersInTimeout.Deliver(peer)
			}
		}
	}
}

func (p *peerPool) GetPeer(ctx context.Context) (_ PeerConn, err error) {
	ctx, cancel := utils.CombinedContext(ctx, p.Process.Ctx())
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
		case <-p.Process.Done():
			return nil, process.ErrClosed

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

func (p *peerPool) peerIsEligible(peerConn PeerConn) bool {
	return peerConn.Ready() && len(peerConn.Addresses()) > 0 && !p.deviceIDAlreadyActive(peerConn.DeviceUniqueID())
}

func (p *peerPool) deviceIDAlreadyActive(deviceID string) bool {
	p.activePeerDeviceIDsMu.RLock()
	defer p.activePeerDeviceIDsMu.RUnlock()
	_, exists := p.activePeerDeviceIDs[deviceID]
	return exists
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
