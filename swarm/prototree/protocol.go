package prototree

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/config"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name TreeProtocol --output ./mocks/ --case=underscore
type TreeProtocol interface {
	swarm.Protocol
	ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan TreePeerConn
	Subscribe(ctx context.Context, stateURI string, subscriptionType SubscriptionType, keypath state.Keypath, fetchHistoryOpts *FetchHistoryOpts) (ReadableSubscription, error)
	Unsubscribe(stateURI string) error
	SubscribeStateURIs() StateURISubscription
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
	log.Logger
	transports    map[string]TreeTransport
	controllerHub tree.ControllerHub
	txStore       tree.TxStore
	keyStore      identity.KeyStore
	peerStore     swarm.PeerStore
	config        *config.Config

	readableSubscriptions   map[string]*multiReaderSubscription // map[stateURI]
	readableSubscriptionsMu sync.RWMutex
	writableSubscriptions   map[string]map[WritableSubscriptionImpl]WritableSubscription // map[stateURI]
	writableSubscriptionsMu sync.RWMutex
	stateURISubscriptions   map[*stateURISubscription]struct{}
	stateURISubscriptionsMu sync.RWMutex

	peerSeenTxs   map[swarm.PeerDialInfo]map[string]map[types.ID]bool // map[swarm.PeerDialInfo]map[stateURI]map[tx.ID]
	peerSeenTxsMu sync.RWMutex

	chStop chan struct{}
	chDone chan struct{}
}

func NewTreeProtocol(
	transports []swarm.Transport,
	controllerHub tree.ControllerHub,
	txStore tree.TxStore,
	keyStore identity.KeyStore,
	peerStore swarm.PeerStore,
	config *config.Config,
) *treeProtocol {
	transportsMap := make(map[string]TreeTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(TreeTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	return &treeProtocol{
		Logger:                log.NewLogger("tree proto"),
		transports:            transportsMap,
		controllerHub:         controllerHub,
		txStore:               txStore,
		keyStore:              keyStore,
		peerStore:             peerStore,
		config:                config,
		readableSubscriptions: make(map[string]*multiReaderSubscription),
		writableSubscriptions: make(map[string]map[WritableSubscriptionImpl]WritableSubscription),
		stateURISubscriptions: make(map[*stateURISubscription]struct{}),
		peerSeenTxs:           make(map[swarm.PeerDialInfo]map[string]map[types.ID]bool),
		chStop:                make(chan struct{}),
		chDone:                make(chan struct{}),
	}
}

const ProtocolName = "tree"

func (tp *treeProtocol) Name() string {
	return ProtocolName
}

func (tp *treeProtocol) Start() {
	tp.controllerHub.OnNewState(tp.handleNewState)

	for _, tpt := range tp.transports {
		tp.Infof(0, "registering %v", tpt.Name())
		tpt.OnTxReceived(tp.handleTxReceived)
		tpt.OnAckReceived(tp.handleAckReceived)
		tpt.OnWritableSubscriptionOpened(tp.handleWritableSubscriptionOpened)
	}
}

func (tp *treeProtocol) Close() {
	close(tp.chStop)

	var writableSubs []WritableSubscription
	func() {
		tp.writableSubscriptionsMu.Lock()
		defer tp.writableSubscriptionsMu.Unlock()
		for _, subs := range tp.writableSubscriptions {
			for _, sub := range subs {
				writableSubs = append(writableSubs, sub)
			}
		}
	}()
	for _, sub := range writableSubs {
		_ = sub.Close()
	}

	func() {
		tp.readableSubscriptionsMu.Lock()
		defer tp.readableSubscriptionsMu.Unlock()
		for _, sub := range tp.readableSubscriptions {
			sub.Close()
		}
	}()

	<-tp.chDone
}

func (tp *treeProtocol) SendTx(ctx context.Context, tx tree.Tx) (err error) {
	tp.Infof(0, "adding tx (%v) %v", tx.StateURI, tx.ID.Pretty())

	defer func() {
		if err != nil {
			return
		}
		// If we send a tx to a state URI that we're not subscribed to yet, auto-subscribe.
		if !tp.config.Node.SubscribedStateURIs.Contains(tx.StateURI) {
			err := tp.config.Update(func() error {
				tp.config.Node.SubscribedStateURIs.Add(tx.StateURI)
				return nil
			})
			if err != nil {
				tp.Errorf("error adding %v to config.Node.SubscribedStateURIs: %v", tx.StateURI, err)
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
		var parents []types.ID
		parents, err = tp.controllerHub.Leaves(tx.StateURI)
		if err != nil {
			return err
		}
		tx.Parents = parents
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
	if utils.IsLocalStateURI(stateURI) {
		ch := make(chan TreePeerConn)
		close(ch)
		return ch
	}

	ctx, cancel := utils.CombinedContext(ctx, tp.chStop)

	var (
		ch          = make(chan TreePeerConn)
		wg          sync.WaitGroup
		alreadySent sync.Map
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
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
	}()

	for _, tpt := range tp.transports {
		innerCh, err := tpt.ProvidersOfStateURI(ctx, stateURI)
		if err != nil {
			tp.Warnf("error fetching providers of State-URI %v on transport %v: %v %+v", stateURI, tpt.Name(), err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	go func() {
		defer close(ch)
		defer cancel()
		wg.Wait()
	}()

	return ch
}

func (tp *treeProtocol) handleTxReceived(tx tree.Tx, peerConn TreePeerConn) {
	tp.Infof(0, "tx received: tx=%v peer=%v", tx.ID.Pretty(), peerConn.DialInfo())
	tp.markTxSeenByPeer(peerConn, tx.StateURI, tx.ID)

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

	err = peerConn.Ack(tx.StateURI, tx.ID)
	if err != nil {
		tp.Errorf("error ACKing peer: %v", err)
	}
}

func (tp *treeProtocol) handleAckReceived(stateURI string, txID types.ID, peerConn TreePeerConn) {
	tp.Infof(0, "ack received: tx=%v peer=%v", txID.Hex(), peerConn.DialInfo().DialAddr)
	tp.markTxSeenByPeer(peerConn, stateURI, txID)
}

func (tp *treeProtocol) markTxSeenByPeer(peerConn TreePeerConn, stateURI string, txID types.ID) {
	tp.peerSeenTxsMu.Lock()
	defer tp.peerSeenTxsMu.Unlock()

	dialInfo := peerConn.DialInfo()

	if tp.peerSeenTxs[dialInfo] == nil {
		tp.peerSeenTxs[dialInfo] = make(map[string]map[types.ID]bool)
	}
	if tp.peerSeenTxs[dialInfo][stateURI] == nil {
		tp.peerSeenTxs[dialInfo][stateURI] = make(map[types.ID]bool)
	}
	tp.peerSeenTxs[dialInfo][stateURI][txID] = true
}

func (tp *treeProtocol) txSeenByPeer(peerConn TreePeerConn, stateURI string, txID types.ID) bool {
	// @@TODO: convert to LRU cache

	if len(peerConn.Addresses()) == 0 {
		return false
	}

	tp.peerSeenTxsMu.Lock()
	defer tp.peerSeenTxsMu.Unlock()

	dialInfo := peerConn.DialInfo()

	if tp.peerSeenTxs[dialInfo] == nil {
		return false
	} else if tp.peerSeenTxs[dialInfo][stateURI] == nil {
		return false
	}
	return tp.peerSeenTxs[dialInfo][stateURI][txID]
}

type FetchHistoryOpts struct {
	FromTxID types.ID
	ToTxID   types.ID
}

func (tp *treeProtocol) handleFetchHistoryRequest(stateURI string, opts FetchHistoryOpts, writeSub WritableSubscription) error {
	// @@TODO: respect the `opts.ToTxID` param
	// @@TODO: if .FromTxID == 0, set it to GenesisTxID

	iter := tp.controllerHub.FetchTxs(stateURI, opts.FromTxID)
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
	writeSub := newWritableSubscription(tp, stateURI, keypath, subType, writeSubImpl)

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

	tp.writableSubscriptionsMu.Lock()
	defer tp.writableSubscriptionsMu.Unlock()

	if _, exists := tp.writableSubscriptions[stateURI]; !exists {
		tp.writableSubscriptions[stateURI] = make(map[WritableSubscriptionImpl]WritableSubscription)
	}
	tp.writableSubscriptions[stateURI][writeSubImpl] = writeSub
}

func (tp *treeProtocol) handleWritableSubscriptionClosed(writeSub WritableSubscriptionImpl) {
	func() {
		tp.writableSubscriptionsMu.Lock()
		defer tp.writableSubscriptionsMu.Unlock()
		delete(tp.writableSubscriptions[writeSub.StateURI()], writeSub)
	}()

	err := writeSub.Close()
	if err != nil {
		tp.Errorf("while closing subscription: %v", err)
	}
}

func (tp *treeProtocol) subscribe(ctx context.Context, stateURI string) error {
	err := tp.config.Update(func() error {
		tp.config.Node.SubscribedStateURIs.Add(stateURI)
		return nil
	})
	if err != nil {
		return err
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
					tp.config.Node.MaxPeersPerSubscription,
					tp.handleTxReceived,
					tp.ProvidersOfStateURI,
				)
				multiSub.Start()
				tp.readableSubscriptions[stateURI] = multiSub
			}
		}()
	}

	tp.stateURISubscriptionsMu.Lock()
	defer tp.stateURISubscriptionsMu.Unlock()
	for sub := range tp.stateURISubscriptions {
		sub.put(stateURI)
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

	err := tp.config.Update(func() error {
		tp.config.Node.SubscribedStateURIs.Remove(stateURI)
		return nil
	})
	return errors.Wrap(err, "while updating config")
}

func (tp *treeProtocol) SubscribeStateURIs() StateURISubscription {
	sub := &stateURISubscription{
		treeProtocol: tp,
		mailbox:      utils.NewMailbox(0),
		ch:           make(chan string),
		chStop:       make(chan struct{}),
		chDone:       make(chan struct{}),
	}

	tp.stateURISubscriptionsMu.Lock()
	defer tp.stateURISubscriptionsMu.Unlock()
	tp.stateURISubscriptions[sub] = struct{}{}

	go sub.start()
	return sub
}

func (tp *treeProtocol) handleStateURISubscriptionClosed(sub *stateURISubscription) {
	tp.stateURISubscriptionsMu.Lock()
	defer tp.stateURISubscriptionsMu.Unlock()
	delete(tp.stateURISubscriptions, sub)
}

func (tp *treeProtocol) handleNewState(tx *tree.Tx, node state.Node, leaves []types.ID) {
	node, err := node.CopyToMemory(nil, nil)
	if err != nil {
		tp.Errorf("handleNewState: couldn't copy state to memory: %v", err)
		node = state.NewMemoryNode() // give subscribers an empty state
	}

	// @@TODO: don't do this, this is stupid.  store ungossiped txs in the DB and create a
	// PeerManager that gossips them on a SleeperTask-like trigger.
	go func() {
		// If this is the genesis tx of a private state URI, ensure that we subscribe to that state URI
		// @@TODO: allow blacklisting of senders
		if tx.IsPrivate() && tx.ID == tree.GenesisTxID && !tp.config.Node.SubscribedStateURIs.Contains(tx.StateURI) {
			ctx, cancel := utils.CombinedContext(tp.chStop, 30*time.Second)
			defer cancel()

			sub, err := tp.Subscribe(ctx, tx.StateURI, 0, nil, nil)
			if err != nil {
				tp.Errorf("error subscribing to state URI %v: %v", tx.StateURI, err)
			}
			sub.Close() // We don't need the in-process subscription
		}

		// If this state URI isn't meant to be shared, don't broadcast
		// if utils.IsLocalStateURI(tx.StateURI) {
		//  return
		// }

		// Broadcast state and tx to others
		ctx, cancel := utils.CombinedContext(tp.chStop, 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var alreadySentPeers sync.Map

		wg.Add(3)
		go tp.broadcastToWritableSubscribers(ctx, tx, node, leaves, &alreadySentPeers, &wg)
		go tp.broadcastToStateURIProviders(ctx, tx, leaves, &alreadySentPeers, &wg)
		go tp.broadcastToPrivateRecipients(ctx, tx, leaves, &alreadySentPeers, &wg)

		wg.Wait()
	}()
}

func (tp *treeProtocol) broadcastToPrivateRecipients(
	ctx context.Context,
	tx *tree.Tx,
	leaves []types.ID,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

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
			} else if tp.txSeenByPeer(peer, tx.StateURI, tx.ID) {
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

			_, alreadySent := alreadySentPeers.LoadOrStore(peer.DialInfo(), struct{}{})
			if alreadySent {
				continue
			}

			wg.Add(1)
			go tp.broadcastToPeerConn(ctx, tx, nil, leaves, peer, alreadySentPeers, wg)
		}
	}
}

func (tp *treeProtocol) broadcastToStateURIProviders(
	ctx context.Context,
	tx *tree.Tx,
	leaves []types.ID,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	ch := make(chan TreePeerConn)
	go func() {
		defer close(ch)

		chProviders := tp.ProvidersOfStateURI(ctx, tx.StateURI)
		for {
			select {
			case peer, open := <-chProviders:
				if !open {
					return
				}
				select {
				case ch <- peer:
				case <-ctx.Done():
					return
				case <-tp.chStop:
					return
				}

			case <-ctx.Done():
				return
			case <-tp.chStop:
				return
			}
		}
	}()

Outer:
	for peer := range ch {
		if tp.txSeenByPeer(peer, tx.StateURI, tx.ID) {
			continue Outer
		}
		// @@TODO: do we always want to avoid broadcasting when `from == peer.address`?
		for _, addr := range peer.Addresses() {
			if tx.From == addr {
				continue Outer
			}
		}

		wg.Add(1)
		go tp.broadcastToPeerConn(ctx, tx, nil, leaves, peer, alreadySentPeers, wg)
	}
}

func (tp *treeProtocol) broadcastToPeerConn(
	ctx context.Context,
	tx *tree.Tx,
	state state.Node,
	leaves []types.ID,
	peerConn TreePeerConn,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	_, alreadySent := alreadySentPeers.LoadOrStore(peerConn.DialInfo(), struct{}{})
	if alreadySent {
		return
	}

	err := peerConn.EnsureConnected(ctx)
	if err != nil {
		return
	}
	defer peerConn.Close()

	err = peerConn.Put(ctx, tx, state, leaves)
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
	leaves []types.ID,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	tp.writableSubscriptionsMu.RLock()
	defer tp.writableSubscriptionsMu.RUnlock()

	for _, writeSub := range tp.writableSubscriptions[tx.StateURI] {
		if peer, isPeer := writeSub.(TreePeerConn); isPeer {
			// If the subscriber wants us to send states, we never skip sending
			if tp.txSeenByPeer(peer, tx.StateURI, tx.ID) && !writeSub.Type().Includes(SubscriptionType_States) {
				continue
			}
		}

		wg.Add(1)
		go tp.broadcastToWritableSubscriber(tx, state, leaves, writeSub, alreadySentPeers, wg)
	}
}

func (tp *treeProtocol) broadcastToWritableSubscriber(
	tx *tree.Tx,
	node state.Node,
	leaves []types.ID,
	writeSub WritableSubscription,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	_, alreadySent := alreadySentPeers.LoadOrStore(writeSub, struct{}{})
	if alreadySent {
		return
	}

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
