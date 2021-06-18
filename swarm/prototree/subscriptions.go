package prototree

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type (
	ReadableSubscription interface {
		Read() (*SubscriptionMsg, error)
		Close() error
	}

	WritableSubscription interface {
		StateURI() string
		Keypath() state.Keypath
		Type() SubscriptionType
		EnqueueWrite(stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID)
		Close() error
	}
)

type writableSubscription struct {
	log.Logger
	stateURI         string
	keypath          state.Keypath
	subscriptionType SubscriptionType
	treeProtocol     *treeProtocol
	subImpl          WritableSubscriptionImpl
	messages         *utils.Mailbox
	chStop           chan struct{}
	chDone           chan struct{}
	stopOnce         sync.Once
}

//go:generate mockery --name WritableSubscriptionImpl --output ./mocks/ --case=underscore
type WritableSubscriptionImpl interface {
	StateURI() string
	Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) error
	Close() error
	Closed() <-chan struct{}
}

func newWritableSubscription(
	treeProtocol *treeProtocol,
	stateURI string,
	keypath state.Keypath,
	subscriptionType SubscriptionType,
	subImpl WritableSubscriptionImpl,
) *writableSubscription {
	writeSub := &writableSubscription{
		Logger:           log.NewLogger("tree proto"),
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		treeProtocol:     treeProtocol,
		subImpl:          subImpl,
		messages:         utils.NewMailbox(10000),
		chStop:           make(chan struct{}),
		chDone:           make(chan struct{}),
	}

	go func() {
		defer func() {
			if perr := recover(); perr != nil {
				writeSub.Errorf("caught panic: %+v", perr)
			}
		}()
		defer writeSub.destroy()

		for {
			select {
			case <-writeSub.messages.Notify():
				err := writeSub.writeMessages()
				if err != nil {
					writeSub.subImpl.Close()
					return
				}
			case <-writeSub.subImpl.Closed():
				return
			case <-writeSub.chStop:
				writeSub.subImpl.Close()
				return
			}
		}
	}()

	return writeSub
}

func (sub *writableSubscription) writeMessages() (err error) {
	for {
		select {
		case <-sub.chStop:
			sub.subImpl.Close()
			return types.ErrClosed
		case <-sub.subImpl.Closed():
			return types.ErrClosed
		default:
		}

		x := sub.messages.Retrieve()
		if x == nil {
			return nil
		}
		msg := x.(*SubscriptionMsg)
		var tx *tree.Tx
		var node state.Node
		if sub.subscriptionType.Includes(SubscriptionType_Txs) {
			tx = msg.Tx
		}
		if sub.subscriptionType.Includes(SubscriptionType_States) {
			node = msg.State
		}

		ctx, cancel := utils.CombinedContext(sub.chStop, 10*time.Second) // @@TODO: make configurable?
		defer cancel()

		err = sub.subImpl.Put(ctx, msg.StateURI, tx, node, msg.Leaves)
		if err != nil {
			sub.Errorf("error writing to subscribed peer: %+v", err)
			return err
		}
	}
}

func (sub *writableSubscription) destroy() {
	defer close(sub.chDone)
	sub.treeProtocol.handleWritableSubscriptionClosed(sub.subImpl)
}

func (sub *writableSubscription) StateURI() string       { return sub.stateURI }
func (sub *writableSubscription) Type() SubscriptionType { return sub.subscriptionType }
func (sub *writableSubscription) Keypath() state.Keypath { return sub.keypath }

func (sub *writableSubscription) EnqueueWrite(stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) {
	sub.messages.Deliver(&SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
}

func (sub *writableSubscription) Close() error {
	sub.stopOnce.Do(func() {
		close(sub.chStop)
		<-sub.chDone
	})
	return nil
}

type multiReaderSubscription struct {
	log.Logger
	stateURI       string
	maxConns       uint64
	onTxReceived   func(tx tree.Tx, peer TreePeerConn)
	searchForPeers func(ctx context.Context, stateURI string) <-chan TreePeerConn

	conns    sync.Map
	chStop   chan struct{}
	wgDone   sync.WaitGroup
	peerPool swarm.PeerPool
}

func newMultiReaderSubscription(
	stateURI string,
	maxConns uint64,
	onTxReceived func(tx tree.Tx, peer TreePeerConn),
	searchForPeers func(ctx context.Context, stateURI string) <-chan TreePeerConn,
) *multiReaderSubscription {
	return &multiReaderSubscription{
		Logger:         log.NewLogger("tree proto"),
		stateURI:       stateURI,
		maxConns:       maxConns,
		onTxReceived:   onTxReceived,
		searchForPeers: searchForPeers,
		chStop:         make(chan struct{}),
	}
}

func convertTreePeerChan(ctx context.Context, ch <-chan TreePeerConn) <-chan swarm.PeerConn {
	chPeer := make(chan swarm.PeerConn)
	go func() {
		defer close(chPeer)
		for {
			select {
			case peer, open := <-ch:
				if !open {
					return
				}

				select {
				case chPeer <- peer:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return chPeer
}

func (s *multiReaderSubscription) Start() {
	var (
		restartSearchBackoff = utils.ExponentialBackoff{Min: 5 * time.Second, Max: 30 * time.Second}
		getPeerBackoff       = utils.ExponentialBackoff{Min: 5 * time.Second, Max: 30 * time.Second}
	)

	s.peerPool = swarm.NewPeerPool(
		s.maxConns,
		func(ctx context.Context) (<-chan swarm.PeerConn, error) {
			time.Sleep(restartSearchBackoff.Next())
			chTreePeers := s.searchForPeers(ctx, s.stateURI)
			return convertTreePeerChan(ctx, chTreePeers), nil // Can't wait for generics
		},
	)

	s.wgDone.Add(1)
	go func() {
		defer s.wgDone.Done()

		ctx, cancel := utils.ContextFromChan(s.chStop)
		defer cancel()

		for {
			select {
			case <-s.chStop:
				return
			default:
			}

			treePeer, err := s.getPeer(ctx)
			if err != nil {
				s.Errorf("error getting peer from pool: %v", err)
				time.Sleep(getPeerBackoff.Next())
				continue
			}
			getPeerBackoff.Reset()

			s.wgDone.Add(1)
			go s.readUntilErrorOrShutdown(ctx, treePeer)
		}
	}()
}

func (s *multiReaderSubscription) getPeer(ctx context.Context) (TreePeerConn, error) {
	for {
		peer, err := s.peerPool.GetPeer(ctx)
		if err != nil {
			return nil, err
		} else if peer == nil || reflect.ValueOf(peer).IsNil() {
			panic("peer is nil")
		}

		// Ensure the peer supports the tree protocol
		treePeer, is := peer.(TreePeerConn)
		if !is {
			// If not, strike it so the pool doesn't return it again
			s.peerPool.ReturnPeer(peer, true)
			continue
		}

		s.conns.Store(treePeer, struct{}{})
		return treePeer, nil
	}
}

func (s *multiReaderSubscription) readUntilErrorOrShutdown(ctx context.Context, peer TreePeerConn) {
	defer s.wgDone.Done()
	defer func() {
		s.peerPool.ReturnPeer(peer, false)
		s.conns.Delete(peer)
	}()

	err := peer.EnsureConnected(ctx)
	if errors.Cause(err) == types.ErrConnection {
		return
	} else if err != nil {
		s.Errorf("error connecting to %v peer (stateURI: %v): %v", peer.Transport().Name(), s.stateURI, err)
		return
	}

	// err = s.treeProtocol.ChallengePeerIdentity(ctx, peer)
	// if err != nil {
	//  s.Errorf("error connecting to %v peer (stateURI: %v): %v", peer.Transport().Name(), s.stateURI, err)
	//  return
	// }

	peerSub, err := peer.Subscribe(ctx, s.stateURI)
	if err != nil {
		s.Errorf("error subscribing to peer %v (stateURI: %v): %v", peer.DialInfo(), s.stateURI, err)
		return
	}
	defer peerSub.Close()

	for {
		select {
		case <-s.chStop:
			return
		default:
		}

		msg, err := peerSub.Read()
		if err != nil {
			s.Errorf("while reading from peer subscription: %v", err)
			return
		} else if msg.Tx == nil {
			s.Error("peer sent empty subscription message")
			return
		}

		s.onTxReceived(*msg.Tx, peer)
	}
}

func (s *multiReaderSubscription) Close() {
	close(s.chStop)

	s.peerPool.Close()
	s.conns.Range(func(peer, val interface{}) bool {
		peer.(TreePeerConn).Close()
		return true
	})

	s.wgDone.Wait()
}

type inProcessSubscription struct {
	stateURI         string
	keypath          state.Keypath
	subscriptionType SubscriptionType
	treeProtocol     *treeProtocol
	chMessages       chan SubscriptionMsg
	stopOnce         sync.Once
	chClosed         chan struct{}
}

var _ ReadableSubscription = (*inProcessSubscription)(nil)
var _ WritableSubscriptionImpl = (*inProcessSubscription)(nil)

func newInProcessSubscription(
	stateURI string,
	keypath state.Keypath,
	subscriptionType SubscriptionType,
	treeProtocol *treeProtocol,
) *inProcessSubscription {
	return &inProcessSubscription{
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		treeProtocol:     treeProtocol,
		chMessages:       make(chan SubscriptionMsg),
		chClosed:         make(chan struct{}),
	}
}

func (sub *inProcessSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *inProcessSubscription) Type() SubscriptionType {
	return sub.subscriptionType
}

func (sub *inProcessSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) error {
	select {
	case sub.chMessages <- SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-sub.chClosed:
		return types.ErrClosed
	}
}

func (sub *inProcessSubscription) Read() (*SubscriptionMsg, error) {
	select {
	case <-sub.chClosed:
		return nil, types.ErrClosed
	case msg := <-sub.chMessages:
		return &msg, nil
	}
}

func (sub *inProcessSubscription) Close() error {
	sub.stopOnce.Do(func() {
		close(sub.chClosed)
	})
	return nil
}

func (sub *inProcessSubscription) Closed() <-chan struct{} {
	return sub.chClosed
}

type StateURISubscription interface {
	Read(ctx context.Context) (string, error)
	Close()
}

type stateURISubscription struct {
	treeProtocol *treeProtocol
	mailbox      *utils.Mailbox
	ch           chan string
	chStop       chan struct{}
	chDone       chan struct{}
}

func (sub *stateURISubscription) start() {
	defer close(sub.chDone)
	defer sub.treeProtocol.handleStateURISubscriptionClosed(sub)

	for {
		select {
		case <-sub.chStop:
			return
		case <-sub.mailbox.Notify():
			for _, x := range sub.mailbox.RetrieveAll() {
				stateURI := x.(string)
				select {
				case sub.ch <- stateURI:
				case <-sub.chStop:
					return
				}
			}
		}
	}
}

func (sub *stateURISubscription) put(stateURI string) {
	sub.mailbox.Deliver(stateURI)
}

func (sub *stateURISubscription) Read(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-sub.chStop:
		return "", types.ErrClosed
	case s := <-sub.ch:
		return s, nil
	}
}

func (sub *stateURISubscription) Close() {
	close(sub.chStop)
	<-sub.chDone
}
