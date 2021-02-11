package redwood

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/tree"
	"redwood.dev/types"
)

type (
	ReadableSubscription interface {
		Read() (SubscriptionMsg, error)
		Close() error
	}

	WritableSubscription interface {
		StateURI() string
		Type() SubscriptionType
		Keypath() tree.Keypath

		// If an error is returned, the stream will be closed by the Host.
		Write(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error

		// If an error is returned, the stream will be closed by the Host.
		WritePrivate(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error

		Close() error
	}

	SubscriptionMsg struct {
		Tx          *Tx          `json:"tx,omitempty"`
		EncryptedTx *EncryptedTx `json:"encryptedTx,omitempty"`
		State       tree.Node    `json:"state,omitempty"`
		Leaves      []types.ID   `json:"leaves,omitempty"`
	}

	SubscriptionType uint8
)

const (
	SubscriptionType_Txs SubscriptionType = 1 << iota
	SubscriptionType_States
)

func (t SubscriptionType) Includes(x SubscriptionType) bool {
	return t&x == x
}

type inProcessSubscription struct {
	stateURI         string
	keypath          tree.Keypath
	subscriptionType SubscriptionType
	ch               chan SubscriptionMsg
	chStop           chan struct{}
}

var _ ReadableSubscription = inProcessSubscription{}
var _ WritableSubscription = inProcessSubscription{}

func (sub inProcessSubscription) StateURI() string {
	return sub.stateURI
}

func (sub inProcessSubscription) Type() SubscriptionType {
	return sub.subscriptionType
}

func (sub inProcessSubscription) Keypath() tree.Keypath {
	return sub.keypath
}

func (sub inProcessSubscription) Write(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error {
	select {
	case sub.ch <- SubscriptionMsg{Tx: tx, State: state, Leaves: leaves}:
	case <-sub.chStop:
		return errors.New("shutting down")
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (sub inProcessSubscription) WritePrivate(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error {
	select {
	case sub.ch <- SubscriptionMsg{Tx: tx, State: state, Leaves: leaves}:
	case <-sub.chStop:
		return errors.New("shutting down")
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (sub inProcessSubscription) Read() (SubscriptionMsg, error) {
	select {
	case msg := <-sub.ch:
		return msg, nil
	case <-sub.chStop:
		return SubscriptionMsg{}, errors.New("shutting down")
	}
}

func (sub inProcessSubscription) Close() error {
	close(sub.chStop)
	return nil
}

type multiReaderSubscription struct {
	stateURI string
	maxConns uint64
	host     Host
	conns    map[types.Address]Peer
	chStop   chan struct{}
	chDone   chan struct{}
	peerPool *peerPool
}

func newMultiReaderSubscription(stateURI string, maxConns uint64, host Host) *multiReaderSubscription {
	return &multiReaderSubscription{
		stateURI: stateURI,
		maxConns: maxConns,
		host:     host,
		conns:    make(map[types.Address]Peer),
		chStop:   make(chan struct{}),
		chDone:   make(chan struct{}),
	}
}

func (s *multiReaderSubscription) Start() {
	s.peerPool = newPeerPool(
		s.maxConns,
		func(ctx context.Context) (<-chan Peer, error) {
			return s.host.ProvidersOfStateURI(ctx, s.stateURI), nil
		},
	)
	defer s.peerPool.Close()

	var wgClose sync.WaitGroup
	wgClose.Add(1)
	defer func() {
		wgClose.Done()
		wgClose.Wait()
		close(s.chDone)
	}()

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

		wgClose.Add(1)
		go func() {
			defer wgClose.Done()
			defer s.peerPool.ReturnPeer(peer, false)
			defer peerSub.Close()

			for {
				select {
				case <-s.chStop:
					return
				default:
				}

				msg, err := peerSub.Read()
				if err != nil {
					s.host.Errorf("error reading: %v", err)
					return
				} else if msg.Tx == nil {
					s.host.Error("error: peer sent empty subscription message")
					return
				}

				s.host.HandleTxReceived(*msg.Tx, peer)
			}
		}()
	}
}

func (s *multiReaderSubscription) Close() error {
	close(s.chStop)
	<-s.chDone
	return nil
}
