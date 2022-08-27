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
	. "redwood.dev/utils/generics"
)

type SubscriptionRequest struct {
	StateURI         string
	Keypath          state.Keypath
	Type             SubscriptionType
	FetchHistoryOpts *FetchHistoryOpts
	Addresses        Set[types.Address]
}

type ReadableSubscription interface {
	Read() (SubscriptionMsg, error)
	Close() error
}

type WritableSubscription interface {
	process.Interface
	StateURI() string
	Keypath() state.Keypath
	Type() SubscriptionType
	Addresses() Set[types.Address]
	EnqueueWrite(msg SubscriptionMsg)
	String() string
}

type writableSubscription struct {
	process.Process
	log.Logger

	stateURI         string
	keypath          state.Keypath
	subscriptionType SubscriptionType
	addresses        Set[types.Address]
	isPrivate        bool
	treeProtocol     *treeProtocol
	subImpl          WritableSubscriptionImpl
	messages         *utils.Mailbox[SubscriptionMsg]
	stopOnce         sync.Once
}

//go:generate mockery --name WritableSubscriptionImpl --output ./mocks/ --case=underscore
type WritableSubscriptionImpl interface {
	process.Interface
	Put(ctx context.Context, msg SubscriptionMsg) error
	String() string
}

func newWritableSubscription(
	stateURI string,
	keypath state.Keypath,
	subscriptionType SubscriptionType,
	isPrivate bool,
	addresses Set[types.Address],
	subImpl WritableSubscriptionImpl,
) *writableSubscription {
	return &writableSubscription{
		Process:          *process.New("WritableSubscription " + subImpl.String()),
		Logger:           log.NewLogger("tree proto"),
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		isPrivate:        isPrivate,
		addresses:        addresses,
		subImpl:          subImpl,
		messages:         utils.NewMailbox[SubscriptionMsg](10000),
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

func (sub *writableSubscription) Close() error {
	return sub.Process.Close()
}

func (sub *writableSubscription) writeMessages(ctx context.Context) (err error) {
	for _, msg := range sub.messages.RetrieveAll() {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-sub.subImpl.Done():
			return errors.ErrClosed
		default:
		}

		if !sub.subscriptionType.Includes(SubscriptionType_Txs) {
			msg.Tx = nil
			msg.EncryptedTx = nil
		}
		if !sub.subscriptionType.Includes(SubscriptionType_States) {
			msg.State = nil
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // @@TODO: make configurable?
		defer cancel()

		err = sub.subImpl.Put(ctx, msg)
		if err != nil {
			sub.Errorf("error writing to subscribed peer: %v", err)
			return err
		}
	}
	return nil
}

func (sub *writableSubscription) StateURI() string              { return sub.stateURI }
func (sub *writableSubscription) Type() SubscriptionType        { return sub.subscriptionType }
func (sub *writableSubscription) Keypath() state.Keypath        { return sub.keypath }
func (sub *writableSubscription) Addresses() Set[types.Address] { return sub.addresses }

func (sub *writableSubscription) EnqueueWrite(msg SubscriptionMsg) {
	sub.messages.Deliver(msg)
}

func (sub *writableSubscription) String() string {
	return sub.subImpl.String()
}

type multiReaderSubscription struct {
	process.Process
	log.Logger
	stateURI          string
	maxConns          uint64
	onMessageReceived func(msg SubscriptionMsg, peer TreePeerConn)
	searchForPeers    func(ctx context.Context, stateURI string) <-chan TreePeerConn
	peerPool          swarm.PeerPool[TreePeerConn]
}

func newMultiReaderSubscription(
	stateURI string,
	maxConns uint64,
	onMessageReceived func(msg SubscriptionMsg, peer TreePeerConn),
	searchForPeers func(ctx context.Context, stateURI string) <-chan TreePeerConn,
) *multiReaderSubscription {
	return &multiReaderSubscription{
		Process:           *process.New("MultiReaderSubscription " + stateURI),
		Logger:            log.NewLogger("tree proto"),
		stateURI:          stateURI,
		maxConns:          maxConns,
		onMessageReceived: onMessageReceived,
		searchForPeers:    searchForPeers,
	}
}

func (s *multiReaderSubscription) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}
	defer s.Process.Autoclose()

	s.peerPool = swarm.NewPeerPool[TreePeerConn](
		s.maxConns,
		func(ctx context.Context) (<-chan TreePeerConn, error) {
			ctx, _ = context.WithTimeout(ctx, 10*time.Second)
			return s.searchForPeers(ctx, s.stateURI), nil
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
				// time.Sleep(getPeerBackoff.Next())
				continue
			}
			// getPeerBackoff.Reset()

			s.Process.Go(ctx, "readUntilErrorOrShutdown "+treePeer.DialInfo().String(), func(ctx context.Context) {
				var strike bool
				defer func() { s.peerPool.ReturnPeer(treePeer, strike) }()
				strike = s.readUntilErrorOrShutdown(ctx, treePeer)
			})
		}
	})
	return nil
}

func (s *multiReaderSubscription) getPeer(ctx context.Context) (TreePeerConn, error) {
	for {
		peer, err := s.peerPool.GetPeer(ctx)
		if err != nil {
			return nil, err
		} else if peer == nil || reflect.ValueOf(peer).IsNil() {
			panic("peer is nil")
		}

		if !peer.Dialable() {
			s.peerPool.ReturnPeer(peer, true)
			continue
		} else if !peer.Ready() {
			s.peerPool.ReturnPeer(peer, false)
			continue
		}
		return peer, nil
	}
}

func (s *multiReaderSubscription) readUntilErrorOrShutdown(ctx context.Context, peerConn TreePeerConn) (strike bool) {
	ctxConnect, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
	defer cancelConnect()

	err := peerConn.EnsureConnected(ctxConnect)
	if errors.Cause(err) == errors.ErrConnection {
		s.Errorf("error connecting to %v peer (stateURI: %v): %v", peerConn.Transport().Name(), s.stateURI, err)
		return true
	} else if err != nil {
		s.Errorf("error connecting to %v peer (stateURI: %v): %v", peerConn.Transport().Name(), s.stateURI, err)
		return true
	}
	defer peerConn.Close()

	peerSub, err := peerConn.Subscribe(ctx, s.stateURI)
	if err != nil {
		s.Errorf("error subscribing to peer %v (stateURI: %v): %v", peerConn.DialInfo(), s.stateURI, err)
		return true
	}

	for {
		select {
		case <-ctx.Done():
			return false
		case <-s.Process.Done():
			return false
		default:
		}

		msg, err := peerSub.Read()
		if err != nil {
			now := time.Now()
			s.Errorw("while reading from peer subscription",
				"err", err,
				"peer", peerConn.DialInfo(),
				"lastcontact", now.Sub(peerConn.LastContact()),
				"lastfailure", now.Sub(peerConn.LastFailure()),
				"failures", peerConn.Failures(),
				"ready", peerConn.Ready(),
			)
			return false
		}

		s.onMessageReceived(msg, peerConn)
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

func (sub *inProcessSubscription) Put(ctx context.Context, msg SubscriptionMsg) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sub.Process.Done():
		return errors.ErrClosed
	case sub.chMessages <- msg:
		return nil
	}
}

func (sub *inProcessSubscription) Read() (SubscriptionMsg, error) {
	select {
	case <-sub.Process.Done():
		return SubscriptionMsg{}, errors.ErrClosed
	case msg := <-sub.chMessages:
		return msg, nil
	}
}

type StateURISubscription interface {
	Read(ctx context.Context) (tree.StateURI, error)
	Close() error
}

type stateURISubscription struct {
	process.Process
	store       Store
	mailbox     *utils.Mailbox[tree.StateURI]
	ch          chan tree.StateURI
	unsubscribe func()
}

func newStateURISubscription(store Store) *stateURISubscription {
	return &stateURISubscription{
		Process: *process.New("stateURI subscription"),
		store:   store,
		mailbox: utils.NewMailbox[tree.StateURI](0),
		ch:      make(chan tree.StateURI),
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
				for _, stateURI := range sub.mailbox.RetrieveAll() {
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

func (sub *stateURISubscription) put(stateURI tree.StateURI) {
	sub.mailbox.Deliver(stateURI)
}

func (sub *stateURISubscription) Read(ctx context.Context) (tree.StateURI, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-sub.Process.Done():
		return "", errors.ErrClosed
	case s := <-sub.ch:
		return s, nil
	}
}
