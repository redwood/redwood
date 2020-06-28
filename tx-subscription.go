package redwood

import (
	"context"
	"time"

	"github.com/brynbellomy/redwood/types"
)

type txMultiSub struct {
	stateURI string
	maxConns uint64
	host     Host
	conns    map[types.Address]Peer
	chStop   chan struct{}
	peerPool *peerPool
}

func newTxMultiSub(
	stateURI string,
	maxConns uint64,
	host Host,
) *txMultiSub {
	return &txMultiSub{
		stateURI: stateURI,
		maxConns: maxConns,
		host:     host,
		conns:    make(map[types.Address]Peer),
		chStop:   make(chan struct{}),
	}
}

func (s *txMultiSub) Start() {
	s.peerPool = newPeerPool(
		s.maxConns,
		// s.host,
		func(ctx context.Context) (<-chan Peer, error) {
			return s.host.ProvidersOfStateURI(ctx, s.stateURI), nil
		},
	)
	defer s.peerPool.Close()

	for {
		select {
		case <-s.chStop:
			return
		default:
		}

		time.Sleep(1 * time.Second)
		peer, err := s.peerPool.GetPeer()
		if err != nil {
			log.Errorf("error getting peer from pool: %v", err)
			// @@TODO: exponential backoff
			continue
		}

		err = peer.EnsureConnected(context.TODO())
		if err != nil {
			log.Errorf("error connecting to peer: %v", err)
			s.peerPool.ReturnPeer(peer, false)
			continue
		}

		peerSub, err := peer.Subscribe(context.TODO(), s.stateURI)
		if err != nil {
			s.host.Errorf("error connecting to %v peer: %v", peer.Transport().Name(), err)
			s.peerPool.ReturnPeer(peer, false)
			continue
		}

		go func() {
			defer s.peerPool.ReturnPeer(peer, false)
			defer peerSub.Close()

			for {
				select {
				case <-s.chStop:
					return
				default:
				}

				tx, _, err := peerSub.Read()
				if err != nil {
					s.host.Errorf("error reading: %v", err)
					return
				}

				s.host.HandleTxReceived(*tx, peer)
			}
		}()
	}
}

func (s *txMultiSub) Stop() {
	close(s.chStop)
}
