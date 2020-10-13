package redwood

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/types"
)

type peerPool struct {
	chPeers       chan Peer
	chNeedNewPeer chan struct{}
	chProviders   <-chan Peer
	chStop        chan struct{}

	peerStates   map[types.Address]peerState
	peerStatesMu sync.RWMutex
}

type peerState int

const (
	peerState_Unknown peerState = iota
	peerState_Strike
	peerState_InUse
)

func newPeerPool(
	concurrentConns uint64,
	// host Host,
	fnGetPeers func(ctx context.Context) (<-chan Peer, error),
) *peerPool {
	chProviders := make(chan Peer)
	close(chProviders)

	p := &peerPool{
		chPeers:       make(chan Peer, concurrentConns),
		chNeedNewPeer: make(chan struct{}, concurrentConns),
		chProviders:   chProviders,
		chStop:        make(chan struct{}),
		peerStates:    make(map[types.Address]peerState),
	}

	// When a message is sent on the `needNewPeer` channel, this goroutine attempts
	// to take a peer from the `chProviders` channel, validate its identity, and add it to the pool.
	go func() {
		// defer close(p.chPeers)

		for {
			select {
			case <-p.chNeedNewPeer:
			case <-p.chStop:
				return
			}

			var peer Peer
		FindPeerLoop:
			for {
				select {
				case <-p.chStop:
					return
				case maybePeer, open := <-p.chProviders:
					if !open {
						var err error
						ctx, cancel := context.WithCancel(context.Background())
						p.chProviders, err = fnGetPeers(ctx)
						if err != nil {
							log.Warnf("[peer pool] error finding peers: %v", err)
							// @@TODO: exponential backoff
							cancel()
							continue FindPeerLoop
						}
						continue FindPeerLoop
					}

					address := maybePeer.Address()
					if address == (types.Address{}) {
						// _, _, err := host.ChallengePeerIdentity(context.TODO(), maybePeer)
						// if err != nil {
						// 	log.Warnf("error verifying peer: %v", err)
						// 	continue FindPeerLoop
						// }
						// address = maybePeer.Address()
						continue FindPeerLoop
					}

					p.peerStatesMu.Lock()
					if p.peerStates[address] != peerState_Unknown {
						p.peerStatesMu.Unlock()
						continue FindPeerLoop
					}

					p.peerStates[address] = peerState_InUse
					p.peerStatesMu.Unlock()
					peer = maybePeer
				}

				log.Debugf("[peer pool] found peer %v", peer.DialInfo())
				break
			}

			select {
			case p.chPeers <- peer:
			case <-p.chStop:
				return
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

func (p *peerPool) Close() {
	close(p.chStop)
}

func (p *peerPool) GetPeer() (Peer, error) {
	select {
	case peer, open := <-p.chPeers:
		if !open {
			return nil, errors.New("connection closed")
		}
		return peer, nil

	case <-p.chStop:
		return nil, nil
	}
}

func (p *peerPool) ReturnPeer(peer Peer, strike bool) {
	if strike {
		// Close the faulty connection
		peer.Close()

		p.peerStatesMu.Lock()
		p.peerStates[peer.Address()] = peerState_Strike
		p.peerStatesMu.Unlock()

		// Try to obtain a new peer
		select {
		case p.chNeedNewPeer <- struct{}{}:
		case <-p.chStop:
		}

	} else {
		// Return the peer to the pool
		select {
		case p.chPeers <- peer:
		case <-p.chStop:
		}
	}
}
