package prototree

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name TreeProtocol --output ./mocks/ --case=underscore
type TreeProtocol interface {
	process.Interface
	ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan TreePeerConn
	Subscribe(ctx context.Context, stateURI string, subscriptionType SubscriptionType, keypath state.Keypath, fetchHistoryOpts *FetchHistoryOpts) (ReadableSubscription, error)
	Unsubscribe(stateURI string) error
	SubscribeStateURIs() (StateURISubscription, error)
	SendTx(ctx context.Context, tx tree.Tx) error
}

//go:generate mockery --name TreeTransport --output ./mocks/ --case=underscore
type TreeTransport interface {
	swarm.Transport
	ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan TreePeerConn, error)
	OnTxReceived(handler TxReceivedCallback)
	OnAckReceived(handler AckReceivedCallback)
	OnWritableSubscriptionOpened(handler WritableSubscriptionOpenedCallback)
}

//go:generate mockery --name TreePeerConn --output ./mocks/ --case=underscore
type TreePeerConn interface {
	swarm.PeerConn
	Subscribe(ctx context.Context, stateURI string) (ReadableSubscription, error)
	Put(ctx context.Context, tx *tree.Tx, state state.Node, leaves []types.ID) error
	Ack(stateURI string, txID types.ID) error
}

type treeProtocol struct {
	process.Process
	log.Logger

	store Store

	transports    map[string]TreeTransport
	controllerHub tree.ControllerHub
	txStore       tree.TxStore
	keyStore      identity.KeyStore
	peerStore     swarm.PeerStore

	readableSubscriptions   map[string]*multiReaderSubscription // map[stateURI]
	readableSubscriptionsMu sync.RWMutex
	writableSubscriptions   map[string]map[WritableSubscription]struct{} // map[stateURI]
	writableSubscriptionsMu sync.RWMutex

	broadcastTxsToStateURIProvidersTask *broadcastTxsToStateURIProvidersTask
}

var (
	_ TreeProtocol      = (*treeProtocol)(nil)
	_ process.Interface = (*treeProtocol)(nil)
)

func NewTreeProtocol(
	transports []swarm.Transport,
	controllerHub tree.ControllerHub,
	txStore tree.TxStore,
	keyStore identity.KeyStore,
	peerStore swarm.PeerStore,
	store Store,
) *treeProtocol {
	transportsMap := make(map[string]TreeTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(TreeTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	tp := &treeProtocol{
		Process:               *process.New("TreeProtocol"),
		Logger:                log.NewLogger("tree proto"),
		store:                 store,
		transports:            transportsMap,
		controllerHub:         controllerHub,
		txStore:               txStore,
		keyStore:              keyStore,
		peerStore:             peerStore,
		readableSubscriptions: make(map[string]*multiReaderSubscription),
		writableSubscriptions: make(map[string]map[WritableSubscription]struct{}),
	}
	tp.broadcastTxsToStateURIProvidersTask = NewBroadcastTxsToStateURIProvidersTask(10*time.Second, store, peerStore, transportsMap)
	return tp
}

const ProtocolName = "tree"

func (tp *treeProtocol) Name() string {
	return ProtocolName
}

func (tp *treeProtocol) Start() error {
	err := tp.Process.Start()
	if err != nil {
		return err
	}

	tp.controllerHub.OnNewState(tp.handleNewState)

	for _, tpt := range tp.transports {
		tp.Infof(0, "registering %v", tpt.Name())
		tpt.OnTxReceived(tp.handleTxReceived)
		tpt.OnAckReceived(tp.handleAckReceived)
		tpt.OnWritableSubscriptionOpened(tp.handleWritableSubscriptionOpened)
	}

	tp.Process.Go(nil, "initial subscribe", func(ctx context.Context) {
		for _, stateURI := range tp.store.SubscribedStateURIs().Slice() {
			tp.Infof(0, "subscribing to %v", stateURI)
			sub, err := tp.Subscribe(ctx, stateURI, SubscriptionType_Txs, nil, nil)
			if err != nil {
				tp.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
		}
	})

	err = tp.Process.SpawnChild(nil, tp.broadcastTxsToStateURIProvidersTask)
	if err != nil {
		return err
	}
	return nil
}

func (tp *treeProtocol) SendTx(ctx context.Context, tx tree.Tx) (err error) {
	tp.Infof(0, "adding tx (%v) %v", tx.StateURI, tx.ID.Pretty())

	defer func() {
		if err != nil {
			return
		}
		// If we send a tx to a state URI that we're not subscribed to yet, auto-subscribe.
		if !tp.store.SubscribedStateURIs().Contains(tx.StateURI) {
			err := tp.store.AddSubscribedStateURI(tx.StateURI)
			if err != nil {
				tp.Errorf("error adding %v to config store SubscribedStateURIs: %v", tx.StateURI, err)
			}
		}
	}()

	if tx.From.IsZero() {
		publicIdentities, err := tp.keyStore.PublicIdentities()
		if err != nil {
			return err
		} else if len(publicIdentities) == 0 {
			return errors.New("keystore has no public identities")
		}
		tx.From = publicIdentities[0].Address()
	}

	if len(tx.Parents) == 0 && tx.ID != tree.GenesisTxID {
		parents, err := tp.controllerHub.Leaves(tx.StateURI)
		if err != nil {
			return err
		}
		tx.Parents = parents.Slice()
	}

	if len(tx.Sig) == 0 {
		tx.Sig, err = tp.keyStore.SignHash(tx.From, tx.Hash())
		if err != nil {
			return err
		}
	}

	err = tp.controllerHub.AddTx(&tx)
	if err != nil {
		return err
	}
	return nil
}

// Returns peers discovered through any transport who claim to provide the
// stateURI in question.
func (tp *treeProtocol) ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan TreePeerConn {
	ch := make(chan TreePeerConn)

	if utils.IsLocalStateURI(stateURI) {
		close(ch)
		return ch
	}

	child := tp.Process.NewChild(ctx, "ProvidersOfStateURI "+stateURI)
	defer child.AutocloseWithCleanup(func() {
		close(ch)
	})

	var alreadySent sync.Map

	child.Go(nil, "from PeerStore", func(ctx context.Context) {
		for _, peerDetails := range tp.peerStore.PeersServingStateURI(stateURI) {
			dialInfo := peerDetails.DialInfo()
			tpt, exists := tp.transports[dialInfo.TransportName]
			if !exists {
				continue
			}

			if _, exists := alreadySent.LoadOrStore(dialInfo, struct{}{}); exists {
				continue
			}

			peerConn, err := tpt.NewPeerConn(ctx, dialInfo.DialAddr)
			if err != nil {
				tp.Warnf("error creating new peer conn (transport: %v, dialAddr: %v)", dialInfo.TransportName, dialInfo.DialAddr)
				continue
			}

			treePeerConn, is := peerConn.(TreePeerConn)
			if !is {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- treePeerConn:
			}
		}
	})

	for _, tpt := range tp.transports {
		innerCh, err := tpt.ProvidersOfStateURI(ctx, stateURI)
		if err != nil {
			tp.Warnf("error fetching providers of State-URI %v on transport %v: %v %+v", stateURI, tpt.Name(), err)
			continue
		}

		child.Go(nil, tpt.Name(), func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					if _, exists := alreadySent.LoadOrStore(peer.DialInfo(), struct{}{}); exists {
						continue
					}

					peer.AddStateURI(stateURI)

					select {
					case <-ctx.Done():
						return
					case ch <- peer:
					}
				}
			}
		})
	}

	return ch
}

func (tp *treeProtocol) handleTxReceived(tx tree.Tx, peerConn TreePeerConn) {
	tp.Infof(0, "tx received: tx=%v peer=%v", tx.ID.Pretty(), peerConn.DialInfo())
	tp.store.MarkTxSeenByPeer(peerConn.DeviceSpecificID(), tx.StateURI, tx.ID)

	exists, err := tp.txStore.TxExists(tx.StateURI, tx.ID)
	if err != nil {
		tp.Errorf("error fetching tx %v from store: %v", tx.ID.Pretty(), err)
		// @@TODO: does it make sense to return here?
		return
	}

	if !exists {
		err := tp.controllerHub.AddTx(&tx)
		if err != nil {
			tp.Errorf("error adding tx to controllerHub: %v", err)
		}
	}

	// The ACK happens in a separate stream
	peerConn2, err := peerConn.Transport().NewPeerConn(context.TODO(), peerConn.DialInfo().DialAddr)
	if err != nil {
		tp.Errorf("error ACKing peer: %v", err)
	}
	defer peerConn2.Close()
	err = peerConn2.(TreePeerConn).Ack(tx.StateURI, tx.ID)
	if err != nil {
		tp.Errorf("error ACKing peer: %v", err)
	}
}

func (tp *treeProtocol) handleAckReceived(stateURI string, txID types.ID, peerConn TreePeerConn) {
	tp.Infof(0, "ack received: tx=%v peer=%v", txID.Hex(), peerConn.DialInfo().DialAddr)
	tp.store.MarkTxSeenByPeer(peerConn.DeviceSpecificID(), stateURI, txID)
}

type FetchHistoryOpts struct {
	FromTxIDs types.IDSet
	ToTxID    types.ID
}

func (tp *treeProtocol) handleFetchHistoryRequest(stateURI string, opts FetchHistoryOpts, writeSub WritableSubscription) error {
	// @@TODO: respect the `opts.ToTxID` param
	// @@TODO: if .FromTxID == 0, set it to GenesisTxID

	iter := tp.controllerHub.FetchTxs(stateURI, opts.FromTxIDs)
	defer iter.Cancel()

	for {
		tx := iter.Next()
		if iter.Error() != nil {
			return iter.Error()
		} else if tx == nil {
			return nil
		}

		leaves, err := tp.controllerHub.Leaves(stateURI)
		if err != nil {
			return err
		}

		isPrivate, err := tp.controllerHub.IsPrivate(tx.StateURI)
		if err != nil {
			return err
		}

		if isPrivate {
			var isAllowed bool
			if peerConn, isTreePeerConn := writeSub.(TreePeerConn); isTreePeerConn {
				for _, addr := range peerConn.Addresses() {
					isAllowed, err = tp.controllerHub.IsMember(tx.StateURI, addr)
					if err != nil {
						tp.Errorf("error determining if peer '%v' is a member of private state URI '%v': %v", addr, tx.StateURI, err)
						return err
					}
					if isAllowed {
						break
					}
				}
			} else {
				// In-process subscriptions are trusted
				isAllowed = true
			}

			if isAllowed {
				writeSub.EnqueueWrite(tx.StateURI, tx, nil, leaves)
			}

		} else {
			writeSub.EnqueueWrite(tx.StateURI, tx, nil, leaves)
		}
	}
	return nil
}

func (tp *treeProtocol) handleWritableSubscriptionOpened(
	stateURI string,
	keypath state.Keypath,
	subType SubscriptionType,
	writeSubImpl WritableSubscriptionImpl,
	fetchHistoryOpts *FetchHistoryOpts,
) {
	tp.Debugf("write sub opened (%v %v)", stateURI, writeSubImpl.DialInfo())

	writeSub := newWritableSubscription(stateURI, keypath, subType, writeSubImpl)
	err := tp.Process.SpawnChild(nil, writeSub)
	if err != nil {
		tp.Errorf("while spawning writable subscription: %v", err)
		return
	}

	func() {
		tp.writableSubscriptionsMu.Lock()
		defer tp.writableSubscriptionsMu.Unlock()

		if _, exists := tp.writableSubscriptions[stateURI]; !exists {
			tp.writableSubscriptions[stateURI] = make(map[WritableSubscription]struct{})
		}
		tp.writableSubscriptions[stateURI][writeSub] = struct{}{}
	}()

	tp.Process.Go(nil, "await close "+writeSub.String(), func(ctx context.Context) {
		select {
		case <-writeSub.Done():
		case <-ctx.Done():
		}
		tp.handleWritableSubscriptionClosed(writeSub)
	})

	if subType.Includes(SubscriptionType_Txs) && fetchHistoryOpts != nil {
		tp.handleFetchHistoryRequest(stateURI, *fetchHistoryOpts, writeSub)
	}

	if subType.Includes(SubscriptionType_States) {
		// Normalize empty keypaths
		if keypath.Equals(state.KeypathSeparator) {
			keypath = nil
		}

		// Immediately write the current state to the subscriber
		node, err := tp.controllerHub.StateAtVersion(stateURI, nil)
		if err != nil && errors.Cause(err) != tree.ErrNoController {
			tp.Errorf("error writing initial state to peer: %v", err)
			writeSub.Close()
			return
		} else if err == nil {
			defer node.Close()

			leaves, err := tp.controllerHub.Leaves(stateURI)
			if err != nil {
				tp.Errorf("error writing initial state to peer (%v): %v", stateURI, err)
			} else {
				node, err := node.CopyToMemory(keypath, nil)
				if err != nil && errors.Cause(err) == types.Err404 {
					// no-op
				} else if err != nil {
					tp.Errorf("error writing initial state to peer (%v): %v", stateURI, err)
				} else {
					writeSub.EnqueueWrite(stateURI, nil, node, leaves)
				}
			}
		}
	}
}

func (tp *treeProtocol) handleWritableSubscriptionClosed(sub WritableSubscription) {
	tp.writableSubscriptionsMu.Lock()
	defer tp.writableSubscriptionsMu.Unlock()
	delete(tp.writableSubscriptions[sub.StateURI()], sub)
}

func (tp *treeProtocol) subscribe(ctx context.Context, stateURI string) error {
	err := tp.store.AddSubscribedStateURI(stateURI)
	if err != nil {
		return errors.Wrap(err, "while updating config store")
	}

	_, err = tp.controllerHub.EnsureController(stateURI)
	if err != nil {
		return err
	}

	// If this state URI is not intended to be shared, don't bother subscribing to other nodes
	if !utils.IsLocalStateURI(stateURI) {
		func() {
			tp.readableSubscriptionsMu.Lock()
			defer tp.readableSubscriptionsMu.Unlock()

			if _, exists := tp.readableSubscriptions[stateURI]; !exists {
				multiSub := newMultiReaderSubscription(
					stateURI,
					tp.store.MaxPeersPerSubscription(),
					tp.handleTxReceived,
					tp.ProvidersOfStateURI,
				)
				tp.Process.SpawnChild(nil, multiSub)
				tp.readableSubscriptions[stateURI] = multiSub
			}
		}()
	}
	return nil
}

func (tp *treeProtocol) Subscribe(
	ctx context.Context,
	stateURI string,
	subscriptionType SubscriptionType,
	keypath state.Keypath,
	fetchHistoryOpts *FetchHistoryOpts,
) (ReadableSubscription, error) {
	err := tp.subscribe(ctx, stateURI)
	if err != nil {
		return nil, err
	}

	sub := newInProcessSubscription(stateURI, keypath, subscriptionType, tp)
	tp.handleWritableSubscriptionOpened(stateURI, keypath, subscriptionType, sub, fetchHistoryOpts)
	return sub, nil
}

func (tp *treeProtocol) Unsubscribe(stateURI string) error {
	// @@TODO: when we unsubscribe, we should close the subs of any peers reading from us
	func() {
		tp.readableSubscriptionsMu.Lock()
		defer tp.readableSubscriptionsMu.Unlock()

		if sub, exists := tp.readableSubscriptions[stateURI]; exists {
			sub.Close()
			delete(tp.readableSubscriptions, stateURI)
		}
	}()

	err := tp.store.RemoveSubscribedStateURI(stateURI)
	if err != nil {
		return errors.Wrap(err, "while updating config")
	}
	return nil
}

func (tp *treeProtocol) SubscribeStateURIs() (StateURISubscription, error) {
	sub := newStateURISubscription(tp.store)
	err := tp.SpawnChild(nil, sub)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (tp *treeProtocol) handleNewState(tx *tree.Tx, node state.Node, leaves types.IDSet) {
	node, err := node.CopyToMemory(nil, nil)
	if err != nil {
		tp.Errorf("handleNewState: couldn't copy state to memory: %v", err)
		node = state.NewMemoryNode() // give subscribers an empty state
	}

	// @@TODO: don't do this, this is stupid.  store ungossiped txs in the DB and create a
	// PeerManager that gossips them on a SleeperTask-like trigger.

	// If this is the genesis tx of a private state URI, ensure that we subscribe to that state URI
	// @@TODO: allow blacklisting of senders
	if tx.IsPrivate() && tx.ID == tree.GenesisTxID && !tp.store.SubscribedStateURIs().Contains(tx.StateURI) {
		tp.Process.Go(nil, "auto-subscribe", func(ctx context.Context) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			sub, err := tp.Subscribe(ctx, tx.StateURI, 0, nil, nil)
			if err != nil {
				tp.Errorf("error subscribing to state URI %v: %v", tx.StateURI, err)
			}
			sub.Close() // We don't need the in-process subscription
		})
	}

	// If this state URI isn't meant to be shared, don't broadcast
	// if utils.IsLocalStateURI(tx.StateURI) {
	//  return
	// }

	// Broadcast state and tx to others
	ctx, cancel := context.WithTimeout(tp.Process.Ctx(), 10*time.Second)

	child := tp.Process.NewChild(ctx, "handleNewState")
	defer child.AutocloseWithCleanup(cancel)

	var alreadySentPeers sync.Map

	child.Go(nil, "broadcastToWritableSubscribers", func(ctx context.Context) {
		tp.broadcastToWritableSubscribers(ctx, tx, node, leaves, &alreadySentPeers, child)
	})
	child.Go(nil, "broadcastToPrivateRecipients", func(ctx context.Context) {
		tp.broadcastToPrivateRecipients(ctx, tx, leaves, &alreadySentPeers, child)
	})

	tp.broadcastTxsToStateURIProvidersTask.addTx(tx)
}

func (tp *treeProtocol) broadcastToPrivateRecipients(
	ctx context.Context,
	tx *tree.Tx,
	leaves types.IDSet,
	alreadySentPeers *sync.Map,
	child *process.Process,
) {
	for _, address := range tx.Recipients {
		for _, peerDetails := range tp.peerStore.PeersWithAddress(address) {
			tpt := tp.transports[peerDetails.DialInfo().TransportName]
			if tpt == nil {
				continue
			}

			maybePeer, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
			if err != nil {
				tp.Errorf("error creating connection to peer %v: %v", peerDetails.DialInfo(), err)
				continue
			}
			peer, is := maybePeer.(TreePeerConn)
			if !is {
				continue
			}

			if len(peer.Addresses()) == 0 {
				panic("impossible")
			} else if tp.store.TxSeenByPeer(peer.DeviceSpecificID(), tx.StateURI, tx.ID) {
				continue
			}
			// @@TODO: do we always want to avoid broadcasting when `from == peer.address`?
			// What if multiple devices/users are sharing an address? What if you want your
			// own devices to sync automatically?
			for _, addr := range peer.Addresses() {
				if tx.From == addr {
					continue
				}
			}

			_, alreadySent := alreadySentPeers.LoadOrStore(peer.DeviceSpecificID(), struct{}{})
			if alreadySent {
				continue
			}

			child.Go(nil, "broadcastToPeerConn "+peer.DialInfo().String(), func(ctx context.Context) {
				tp.broadcastToPeerConn(ctx, tx, nil, leaves, peer, alreadySentPeers)
			})
		}
	}
}

func (tp *treeProtocol) broadcastToPeerConn(
	ctx context.Context,
	tx *tree.Tx,
	state state.Node,
	leaves types.IDSet,
	peerConn TreePeerConn,
	alreadySentPeers *sync.Map,
) {
	_, alreadySent := alreadySentPeers.LoadOrStore(peerConn.DeviceSpecificID(), struct{}{})
	if alreadySent {
		return
	}

	err := peerConn.EnsureConnected(ctx)
	if err != nil {
		return
	}
	defer peerConn.Close()

	err = peerConn.Put(ctx, tx, state, leaves.Slice())
	if errors.Cause(err) == types.ErrConnection {
		return
	} else if err != nil {
		tp.Errorf("error writing tx to peer: %v", err)
		return
	}
}

func (tp *treeProtocol) broadcastToWritableSubscribers(
	ctx context.Context,
	tx *tree.Tx,
	state state.Node,
	leaves types.IDSet,
	alreadySentPeers *sync.Map,
	child *process.Process,
) {
	tp.writableSubscriptionsMu.RLock()
	defer tp.writableSubscriptionsMu.RUnlock()

	for writeSub := range tp.writableSubscriptions[tx.StateURI] {
		if peer, isPeer := writeSub.(TreePeerConn); isPeer {
			// If the subscriber wants us to send states, we never skip sending
			if tp.store.TxSeenByPeer(peer.DeviceSpecificID(), tx.StateURI, tx.ID) && !writeSub.Type().Includes(SubscriptionType_States) {
				continue
			}
			_, alreadySent := alreadySentPeers.LoadOrStore(peer.DeviceSpecificID(), struct{}{})
			if alreadySent {
				continue
			}
		}

		writeSub := writeSub

		child.Go(nil, "broadcastToWritableSubscriber "+writeSub.String(), func(ctx context.Context) {
			tp.broadcastToWritableSubscriber(tx, state, leaves, writeSub)
		})
	}
}

func (tp *treeProtocol) broadcastToWritableSubscriber(
	tx *tree.Tx,
	node state.Node,
	leaves types.IDSet,
	writeSub WritableSubscription,
) {
	isPrivate, err := tp.controllerHub.IsPrivate(tx.StateURI)
	if err != nil {
		tp.Errorf("error determining if state URI '%v' is private: %v", tx.StateURI, err)
		return
	}

	// Drill down to the part of the state that the subscriber is interested in
	keypath := writeSub.Keypath()
	if keypath.Equals(state.KeypathSeparator) {
		keypath = nil
	}
	node = node.NodeAt(keypath, nil)

	if isPrivate {
		var isAllowed bool
		if peer, isPeer := writeSub.(TreePeerConn); isPeer {
			for _, addr := range peer.Addresses() {
				isAllowed, err = tp.controllerHub.IsMember(tx.StateURI, addr)
				if err != nil {
					tp.Errorf("error determining if peer '%v' is a member of private state URI '%v': %v", addr, tx.StateURI, err)
					return
				}
				if isAllowed {
					break
				}
			}
		} else {
			// In-process subscriptions are trusted
			isAllowed = true
		}

		if isAllowed {
			writeSub.EnqueueWrite(tx.StateURI, tx, node, leaves)
		}

	} else {
		writeSub.EnqueueWrite(tx.StateURI, tx, node, leaves)
	}
}

type broadcastTxsToStateURIProvidersTask struct {
	process.PeriodicTask
	log.Logger
	treeProto                 TreeProtocol
	treeStore                 Store
	peerStore                 swarm.PeerStore
	transports                map[string]TreeTransport
	txsForStateURIProviders   map[string][]*tree.Tx
	txsForStateURIProvidersMu sync.Mutex
}

func NewBroadcastTxsToStateURIProvidersTask(
	interval time.Duration,
	treeStore Store,
	peerStore swarm.PeerStore,
	transports map[string]TreeTransport,
) *broadcastTxsToStateURIProvidersTask {
	t := &broadcastTxsToStateURIProvidersTask{
		Logger:                  log.NewLogger("tree proto"),
		treeStore:               treeStore,
		peerStore:               peerStore,
		transports:              transports,
		txsForStateURIProviders: make(map[string][]*tree.Tx),
	}
	t.PeriodicTask = *process.NewPeriodicTask("BroadcastTxsToStateURIProvidersTask", interval, t.broadcastTxsToStateURIProviders)
	return t
}

func (t *broadcastTxsToStateURIProvidersTask) addTx(tx *tree.Tx) {
	t.txsForStateURIProvidersMu.Lock()
	defer t.txsForStateURIProvidersMu.Unlock()
	t.txsForStateURIProviders[tx.StateURI] = append(t.txsForStateURIProviders[tx.StateURI], tx.Copy())
}

func (t *broadcastTxsToStateURIProvidersTask) takeTxs() map[string][]*tree.Tx {
	t.txsForStateURIProvidersMu.Lock()
	defer t.txsForStateURIProvidersMu.Unlock()
	txs := t.txsForStateURIProviders
	t.txsForStateURIProviders = make(map[string][]*tree.Tx)
	return txs
}

func (t *broadcastTxsToStateURIProvidersTask) broadcastTxsToStateURIProviders(ctx context.Context) {
	txs := t.takeTxs()
	if len(txs) == 0 {
		return
	}

	t.Debugf("broadcasting txs to state URI providers")

	for stateURI, txs := range txs {
		for _, peerDetails := range t.peerStore.PeersServingStateURI(stateURI) {
			txs := txs
			peerDetails := peerDetails

			t.Process.Go(nil, peerDetails.DialInfo().String(), func(ctx context.Context) {
				tpt, exists := t.transports[peerDetails.DialInfo().TransportName]
				if !exists {
					return
				}
				peerConn, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
				if err != nil {
					t.Errorf("while creating NewPeerConn: %v", err)
					return
				}
				treePeer, is := peerConn.(TreePeerConn)
				if !is {
					t.Errorf("peer is not TreePeerConn, should be impossible")
					return
				}
				err = treePeer.EnsureConnected(ctx)
				if err != nil {
					return
				}
				defer treePeer.Close()

				for _, tx := range txs {
					if t.treeStore.TxSeenByPeer(treePeer.DeviceSpecificID(), stateURI, tx.ID) {
						continue
					}
					err = treePeer.Put(ctx, tx, nil, nil)
					if err != nil {
						t.Errorf("while sending tx to state URI provider: %v", err)
						continue
					}
				}
			})
		}
	}
}
