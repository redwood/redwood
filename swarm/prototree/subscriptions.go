package prototree

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
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
		process.Interface
		StateURI() string
		Keypath() state.Keypath
		Type() SubscriptionType
		EnqueueWrite(stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID)
		String() string
	}
)

type writableSubscription struct {
	process.Process
	log.Logger

	stateURI         string
	keypath          state.Keypath
	subscriptionType SubscriptionType
	treeProtocol     *treeProtocol
	subImpl          WritableSubscriptionImpl
	messages         *utils.Mailbox
	stopOnce         sync.Once
}

//go:generate mockery --name WritableSubscriptionImpl --output ./mocks/ --case=underscore
type WritableSubscriptionImpl interface {
	process.Interface
	DialInfo() swarm.PeerDialInfo
	StateURI() string
	Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) error
	String() string
}

func newWritableSubscription(
	stateURI string,
	keypath state.Keypath,
	subscriptionType SubscriptionType,
	subImpl WritableSubscriptionImpl,
) *writableSubscription {
	return &writableSubscription{
		Process:          *process.New("WritableSubscription " + subImpl.String()),
		Logger:           log.NewLogger("tree proto"),
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		subImpl:          subImpl,
		messages:         utils.NewMailbox(10000),
	}
}

func (sub *writableSubscription) Start() error {
	err := sub.Process.Start()
	if err != nil {
		return err
	}
	defer sub.Process.Autoclose()

	err = sub.Process.SpawnChild(nil, sub.subImpl)
	if err != nil {
		return err
	}

	sub.Process.Go(nil, "runloop", func(ctx context.Context) {
		for {
			select {
			case <-sub.subImpl.Done():
				return

			case <-ctx.Done():
				return

			case <-sub.messages.Notify():
				err := sub.writeMessages(ctx)
				if errors.Cause(err) == context.Canceled {
					return
				} else if errors.Cause(err) == errors.ErrClosed {
					return
				} else if err != nil {
					sub.subImpl.Close()
					return
				}
			}
		}
	})
	return nil
}

func (sub *writableSubscription) writeMessages(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-sub.subImpl.Done():
			return errors.ErrClosed
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

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // @@TODO: make configurable?
		defer cancel()

		err = sub.subImpl.Put(ctx, msg.StateURI, tx, node, msg.Leaves)
		if err != nil {
			sub.Errorf("error writing to subscribed peer: %v", err)
			return err
		}
	}
}

func (sub *writableSubscription) StateURI() string       { return sub.stateURI }
func (sub *writableSubscription) Type() SubscriptionType { return sub.subscriptionType }
func (sub *writableSubscription) Keypath() state.Keypath { return sub.keypath }

func (sub *writableSubscription) EnqueueWrite(stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) {
	sub.messages.Deliver(&SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
}

func (sub *writableSubscription) String() string {
	return sub.subImpl.String()
}

type multiReaderSubscription struct {
	process.Process
	log.Logger
	stateURI       string
	maxConns       uint64
	onTxReceived   func(tx tree.Tx, peer TreePeerConn)
	searchForPeers func(ctx context.Context, stateURI string) <-chan TreePeerConn
	peerPool       swarm.PeerPool
}

func newMultiReaderSubscription(
	stateURI string,
	maxConns uint64,
	onTxReceived func(tx tree.Tx, peer TreePeerConn),
	searchForPeers func(ctx context.Context, stateURI string) <-chan TreePeerConn,
) *multiReaderSubscription {
	return &multiReaderSubscription{
		Process:        *process.New("MultiReaderSubscription " + stateURI),
		Logger:         log.NewLogger("tree proto"),
		stateURI:       stateURI,
		maxConns:       maxConns,
		onTxReceived:   onTxReceived,
		searchForPeers: searchForPeers,
	}
}

func (s *multiReaderSubscription) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}
	defer s.Process.Autoclose()

	var (
		restartSearchBackoff = utils.ExponentialBackoff{Min: 3 * time.Second, Max: 10 * time.Second}
		getPeerBackoff       = utils.ExponentialBackoff{Min: 1 * time.Second, Max: 10 * time.Second}
	)

	s.peerPool = swarm.NewPeerPool(
		s.maxConns,
		func(ctx context.Context) (<-chan swarm.PeerConn, error) {
			select {
			case <-ctx.Done():
				return nil, nil
			case <-s.Process.Done():
				return nil, nil
			case <-time.After(restartSearchBackoff.Next()):
			}
			chTreePeers := s.searchForPeers(ctx, s.stateURI)
			return convertTreePeerChan(ctx, chTreePeers), nil // Can't wait for generics
		},
	)

	err = s.Process.SpawnChild(nil, s.peerPool)
	if err != nil {
		return err
	}

	s.Process.Go(nil, "runloop", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
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

			s.Process.Go(nil, "readUntilErrorOrShutdown "+treePeer.DialInfo().String(), func(ctx context.Context) {
				defer s.peerPool.ReturnPeer(treePeer, false)
				s.readUntilErrorOrShutdown(ctx, treePeer)
			})
		}
	})
	return nil
}

func convertTreePeerChan(ctx context.Context, ch <-chan TreePeerConn) <-chan swarm.PeerConn {
	chPeer := make(chan swarm.PeerConn)
	go func() {
		defer close(chPeer)
		for {
			select {
			case <-ctx.Done():
				return

			case peer, open := <-ch:
				if !open {
					return
				}

				select {
				case <-ctx.Done():
					return
				case chPeer <- peer:
				}
			}
		}
	}()
	return chPeer
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
		return treePeer, nil
	}
}

func (s *multiReaderSubscription) readUntilErrorOrShutdown(ctx context.Context, peer TreePeerConn) {
	err := peer.EnsureConnected(ctx)
	if errors.Cause(err) == errors.ErrConnection {
		return
	} else if err != nil {
		s.Errorf("error connecting to %v peer (stateURI: %v): %v", peer.Transport().Name(), s.stateURI, err)
		return
	}
	defer peer.Close()

	peerSub, err := peer.Subscribe(ctx, s.stateURI)
	if err != nil {
		s.Errorf("error subscribing to peer %v (stateURI: %v): %v", peer.DialInfo(), s.stateURI, err)
		return
	}

	for {
		select {
		case <-s.Process.Done():
			return
		default:
		}

		msg, err := peerSub.Read()
		if err != nil {
			s.Errorf("while reading from peer subscription (%v): %v", peer.DialInfo(), err)
			return
		} else if msg.Tx == nil {
			s.Error("peer sent empty subscription message")
			return
		}

		s.onTxReceived(*msg.Tx, peer)
	}
}

type inProcessSubscription struct {
	process.Process
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
		Process:          *process.New("in process sub " + stateURI),
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		treeProtocol:     treeProtocol,
		chMessages:       make(chan SubscriptionMsg),
		chClosed:         make(chan struct{}),
	}
}

func (sub *inProcessSubscription) DialInfo() swarm.PeerDialInfo {
	return swarm.PeerDialInfo{"in process", fmt.Sprintf("%p", sub)}
}

func (sub *inProcessSubscription) String() string {
	return "in process " + fmt.Sprintf("%p", sub) + " (" + sub.stateURI + "/" + sub.keypath.String() + ")"
}

func (sub *inProcessSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *inProcessSubscription) Type() SubscriptionType {
	return sub.subscriptionType
}

func (sub *inProcessSubscription) Put(ctx context.Context, stateURI string, tx *tree.Tx, state state.Node, leaves []types.ID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sub.Process.Done():
		return types.ErrClosed
	case sub.chMessages <- SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves}:
		return nil
	}
}

func (sub *inProcessSubscription) Read() (*SubscriptionMsg, error) {
	select {
	case <-sub.Process.Done():
		return SubscriptionMsg{}, errors.ErrClosed
	case msg := <-sub.chMessages:
		return &msg, nil
	}
}

type StateURISubscription interface {
	Read(ctx context.Context) (string, error)
	Close() error
}

type stateURISubscription struct {
	process.Process
	store       Store
	mailbox     *utils.Mailbox
	ch          chan string
	unsubscribe func()
}

func newStateURISubscription(store Store) *stateURISubscription {
	return &stateURISubscription{
		Process: *process.New("stateURI subscription"),
		store:   store,
		mailbox: utils.NewMailbox(0),
		ch:      make(chan string),
	}
}

func (sub *stateURISubscription) Start() error {
	err := sub.Process.Start()
	if err != nil {
		return err
	}

	sub.unsubscribe = sub.store.OnNewSubscribedStateURI(sub.put)
	for stateURI := range sub.store.SubscribedStateURIs() {
		sub.mailbox.Deliver(stateURI)
	}

	sub.Process.Go(nil, "runloop", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sub.mailbox.Notify():
				for _, x := range sub.mailbox.RetrieveAll() {
					stateURI := x.(string)
					select {
					case sub.ch <- stateURI:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	})
	return nil
}

func (sub *stateURISubscription) Close() error {
	sub.unsubscribe()
	return sub.Process.Close()
}

func (sub *stateURISubscription) put(stateURI string) {
	sub.mailbox.Deliver(stateURI)
}

func (sub *stateURISubscription) Read(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-sub.Process.Done():
		return "", errors.ErrClosed
	case s := <-sub.ch:
		return s, nil
	}
}
