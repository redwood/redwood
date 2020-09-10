package redwood

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Host interface {
	ctx.Logger
	Ctx() *ctx.Context
	Start() error

	Subscribe(ctx context.Context, stateURI string) (TxSubscription, error)
	SubscribeStates(ctx context.Context, stateURI string) (StateSubscription, error)
	Unsubscribe(stateURI string) error
	SendTx(ctx context.Context, tx Tx) error
	AddRef(reader io.ReadCloser) (types.Hash, types.Hash, error)
	FetchRef(ctx context.Context, ref types.RefID)
	AddPeer(dialInfo PeerDialInfo)
	Transport(name string) Transport
	Controllers() ControllerHub
	Address() types.Address
	ChallengePeerIdentity(ctx context.Context, peer Peer) (SigningPublicKey, EncryptingPublicKey, error)

	ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan Peer
	ProvidersOfRef(ctx context.Context, refID types.RefID) <-chan Peer
	PeersClaimingAddress(ctx context.Context, address types.Address) <-chan Peer

	HandleFetchHistoryRequest(stateURI string, fromTxID types.ID, toVersion types.ID, peer Peer) error
	HandleIncomingTxSubscription(stateURI string, peer Peer)
	HandleIncomingTxSubscriptionClosed(stateURI string, peer Peer)
	HandleIncomingStateSubscription(stateURI string, keypath tree.Keypath, peer Peer)
	HandleIncomingStateSubscriptionClosed(stateURI string, keypath tree.Keypath, peer Peer)
	HandleTxReceived(tx Tx, peer Peer)
	HandleAckReceived(stateURI string, txID types.ID, peer Peer)
	HandleChallengeIdentity(challengeMsg types.ChallengeMsg, peer Peer) error
	HandleFetchRefReceived(refID types.RefID, peer Peer)
}

type host struct {
	*ctx.Context

	config *Config

	signingKeypair    *SigningKeypair
	encryptingKeypair *EncryptingKeypair

	subscriptionOutputs   map[string]map[subscriptionOutput]struct{}
	subscriptionOutputsMu sync.RWMutex

	txSubscriptionsOut     map[string]*txMultiSub // map[stateURI]
	txSubscriptionsOutMu   sync.RWMutex
	txSubscriptionsIn      map[string]map[Peer]struct{} // map[stateURI]
	txSubscriptionsInMu    sync.RWMutex
	stateSubscriptionsIn   map[string]map[string]map[Peer]struct{} // map[stateURI]map[keypath]
	stateSubscriptionsInMu sync.RWMutex
	peerSeenTxs            map[PeerDialInfo]map[string]map[types.ID]bool
	peerSeenTxsMu          sync.RWMutex

	verifyPeersWorker WorkQueue

	controllerHub ControllerHub
	transports    map[string]Transport
	peerStore     PeerStore
	refStore      RefStore

	chRefsNeeded chan []types.RefID
}

var (
	ErrUnsignedTx = errors.New("unsigned tx")
	ErrProtocol   = errors.New("protocol error")
	ErrPeerIsSelf = errors.New("peer is self")
)

func NewHost(
	signingKeypair *SigningKeypair,
	encryptingKeypair *EncryptingKeypair,
	transports []Transport,
	controllerHub ControllerHub,
	refStore RefStore,
	peerStore PeerStore,
	config *Config,
) (Host, error) {
	transportsMap := make(map[string]Transport)
	for _, tpt := range transports {
		transportsMap[tpt.Name()] = tpt
	}
	h := &host{
		Context:              &ctx.Context{},
		transports:           transportsMap,
		controllerHub:        controllerHub,
		signingKeypair:       signingKeypair,
		encryptingKeypair:    encryptingKeypair,
		subscriptionOutputs:  make(map[string]map[subscriptionOutput]struct{}),
		txSubscriptionsOut:   make(map[string]*txMultiSub),
		txSubscriptionsIn:    make(map[string]map[Peer]struct{}),
		stateSubscriptionsIn: make(map[string]map[string]map[Peer]struct{}),
		peerSeenTxs:          make(map[PeerDialInfo]map[string]map[types.ID]bool),
		peerStore:            peerStore,
		refStore:             refStore,
		chRefsNeeded:         make(chan []types.RefID, 100),
		config:               config,
	}
	return h, nil
}

func (h *host) Ctx() *ctx.Context {
	return h.Context
}

func (h *host) Start() error {
	return h.CtxStart(
		// on startup
		func() error {
			h.SetLogLabel("host")

			// Set up the peer store
			h.peerStore.OnNewUnverifiedPeer(h.handleNewUnverifiedPeer)
			h.verifyPeersWorker = NewWorkQueue(1, h.verifyPeers)

			// Set up the controller Hub
			h.controllerHub.OnNewState(h.handleNewState)

			h.CtxAddChild(h.controllerHub.Ctx(), nil)
			err := h.controllerHub.Start()
			if err != nil {
				return err
			}

			// Set up the ref store
			h.refStore.OnRefsNeeded(h.handleRefsNeeded)

			// Set up the transports
			for _, transport := range h.transports {
				transport.SetHost(h)
				h.CtxAddChild(transport.Ctx(), nil)
				err := transport.Start()
				if err != nil {
					return err
				}
			}

			go h.periodicallyFetchMissingRefs()
			go h.periodicallyVerifyPeers()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
}

func (h *host) Transport(name string) Transport {
	return h.transports[name]
}

func (h *host) Controllers() ControllerHub {
	return h.controllerHub
}

func (h *host) Address() types.Address {
	return h.signingKeypair.Address()
}

// Returns peers discovered through any transport that have already been authenticated.
// It's not guaranteed that they actually provide
func (h *host) ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan Peer {
	var wg sync.WaitGroup
	ch := make(chan Peer)
	for _, tpt := range h.transports {
		innerCh, err := tpt.ProvidersOfStateURI(ctx, stateURI)
		if err != nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-h.Ctx().Done():
					return
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-h.Ctx().Done():
						return
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
		wg.Wait()
	}()

	return ch
}

func (h *host) ProvidersOfRef(ctx context.Context, refID types.RefID) <-chan Peer {
	var wg sync.WaitGroup
	ch := make(chan Peer)
	for _, tpt := range h.transports {
		innerCh, err := tpt.ProvidersOfRef(ctx, refID)
		if err != nil {
			h.Warnf("transport %v could not fetch providers of ref %v", tpt.Name(), refID)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-h.Ctx().Done():
					return
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-h.Ctx().Done():
						return
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
		wg.Wait()
	}()

	return ch
}

func (h *host) PeersClaimingAddress(ctx context.Context, address types.Address) <-chan Peer {
	var wg sync.WaitGroup
	ch := make(chan Peer)
	for _, tpt := range h.transports {
		innerCh, err := tpt.PeersClaimingAddress(ctx, address)
		if err != nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-h.Ctx().Done():
					return
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-h.Ctx().Done():
						return
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
		wg.Wait()
	}()

	return ch
}

func (h *host) HandleTxReceived(tx Tx, peer Peer) {
	h.Infof(0, "tx received: tx=%v peer=%v", tx.ID.Pretty(), peer.DialInfo())
	h.markTxSeenByPeer(peer, tx.StateURI, tx.ID)

	have, err := h.controllerHub.HaveTx(tx.StateURI, tx.ID)
	if err != nil {
		h.Errorf("error fetching tx %v from store: %v", tx.ID.Pretty(), err)
		// @@TODO: does it make sense to return here?
		return
	}

	if !have {
		err := h.controllerHub.AddTx(&tx, false)
		if err != nil {
			h.Errorf("error adding tx to controllerHub: %v", err)
		}
	}

	err = peer.Ack(tx.StateURI, tx.ID)
	if err != nil {
		h.Errorf("error ACKing peer: %v", err)
	}
}

func (h *host) HandleAckReceived(stateURI string, txID types.ID, peer Peer) {
	h.Infof(0, "ack received: tx=%v peer=%v", txID.Hex(), peer.DialInfo().DialAddr)
	h.markTxSeenByPeer(peer, stateURI, txID)
}

func (h *host) markTxSeenByPeer(peer Peer, stateURI string, txID types.ID) {
	h.peerSeenTxsMu.Lock()
	defer h.peerSeenTxsMu.Unlock()

	dialInfo := peer.DialInfo()

	if h.peerSeenTxs[dialInfo] == nil {
		h.peerSeenTxs[dialInfo] = make(map[string]map[types.ID]bool)
	}
	if h.peerSeenTxs[dialInfo][stateURI] == nil {
		h.peerSeenTxs[dialInfo][stateURI] = make(map[types.ID]bool)
	}
	h.peerSeenTxs[dialInfo][stateURI][txID] = true
}

func (h *host) txSeenByPeer(peer Peer, stateURI string, txID types.ID) bool {
	if peer.Address() == (types.Address{}) {
		return false
	}

	// @@TODO: convert to LRU cache
	h.peerSeenTxsMu.Lock()
	defer h.peerSeenTxsMu.Unlock()

	dialInfo := peer.DialInfo()

	if h.peerSeenTxs[dialInfo] == nil {
		return false
	}
	if h.peerSeenTxs[dialInfo][stateURI] == nil {
		return false
	}
	return h.peerSeenTxs[dialInfo][stateURI][txID]
}

func (h *host) AddPeer(dialInfo PeerDialInfo) {
	h.peerStore.AddDialInfos([]PeerDialInfo{dialInfo})
	h.verifyPeersWorker.Enqueue()
}

func (h *host) periodicallyVerifyPeers() {
	for {
		h.verifyPeersWorker.Enqueue()
		time.Sleep(10 * time.Second)
	}
}

func (h *host) handleNewUnverifiedPeer(dialInfo PeerDialInfo) {
	h.verifyPeersWorker.Enqueue()
}

func (h *host) verifyPeers() {
	unverifiedPeers := h.peerStore.UnverifiedPeers()
	var wg sync.WaitGroup
	wg.Add(len(unverifiedPeers))
	for _, unverifiedPeer := range unverifiedPeers {
		unverifiedPeer := unverifiedPeer
		go func() {
			defer wg.Done()

			transport := h.Transport(unverifiedPeer.DialInfo().TransportName)
			if transport == nil {
				// Unsupported transport
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			peer, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
			if errors.Cause(err) == ErrPeerIsSelf {
				return
			} else if errors.Cause(err) == types.ErrConnection {
				return
			} else if err != nil {
				h.Warnf("could not get peer at %v %v: %v", unverifiedPeer.DialInfo().TransportName, unverifiedPeer.DialInfo().DialAddr, err)
				return
			}

			_, _, err = h.ChallengePeerIdentity(ctx, peer)
			if err != nil {
				h.Errorf("error verifying peer identity: %v ", err)
				return
			}
		}()
	}
	wg.Wait()
}

func (h *host) HandleFetchHistoryRequest(stateURI string, fromTxID types.ID, toVersion types.ID, peer Peer) error {
	// @@TODO: respect the input params

	iter := h.controllerHub.FetchTxs(stateURI, fromTxID)
	defer iter.Cancel()

	for {
		tx := iter.Next()
		if iter.Error() != nil {
			return iter.Error()
		} else if tx == nil {
			return nil
		}

		leaves, err := h.controllerHub.Leaves(stateURI)
		if err != nil {
			return err
		}

		err = peer.Put(*tx, leaves)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *host) HandleIncomingTxSubscription(stateURI string, peer Peer) {
	h.txSubscriptionsInMu.Lock()
	defer h.txSubscriptionsInMu.Unlock()

	if _, exists := h.txSubscriptionsIn[stateURI]; !exists {
		h.txSubscriptionsIn[stateURI] = make(map[Peer]struct{})
	}

	h.txSubscriptionsIn[stateURI][peer] = struct{}{}
}

func (h *host) HandleIncomingTxSubscriptionClosed(stateURI string, peer Peer) {
	h.txSubscriptionsInMu.Lock()
	defer h.txSubscriptionsInMu.Unlock()

	if _, exists := h.txSubscriptionsIn[stateURI]; exists {
		delete(h.txSubscriptionsIn[stateURI], peer)
	}
}

func (h *host) HandleIncomingStateSubscription(stateURI string, keypath tree.Keypath, peer Peer) {
	h.stateSubscriptionsInMu.Lock()
	defer h.stateSubscriptionsInMu.Unlock()

	if keypath.Equals(tree.KeypathSeparator) {
		keypath = nil
	}

	if _, exists := h.stateSubscriptionsIn[stateURI]; !exists {
		h.stateSubscriptionsIn[stateURI] = make(map[string]map[Peer]struct{})
	}
	if _, exists := h.stateSubscriptionsIn[stateURI][string(keypath)]; !exists {
		h.stateSubscriptionsIn[stateURI][string(keypath)] = make(map[Peer]struct{})
	}

	h.stateSubscriptionsIn[stateURI][string(keypath)][peer] = struct{}{}
}

func (h *host) HandleIncomingStateSubscriptionClosed(stateURI string, keypath tree.Keypath, peer Peer) {
	h.stateSubscriptionsInMu.Lock()
	defer h.stateSubscriptionsInMu.Unlock()

	if _, exists := h.stateSubscriptionsIn[stateURI]; exists {
		if _, exists := h.stateSubscriptionsIn[stateURI][string(keypath)]; exists {
			delete(h.stateSubscriptionsIn[stateURI][string(keypath)], peer)
		}
	}
}

func (h *host) subscribe(ctx context.Context, stateURI string) error {
	h.txSubscriptionsOutMu.Lock()
	defer h.txSubscriptionsOutMu.Unlock()

	err := h.config.Update(func() error {
		h.config.Node.SubscribedStateURIs.Add(stateURI)
		return nil
	})
	if err != nil {
		return err
	}

	if _, exists := h.txSubscriptionsOut[stateURI]; !exists {
		multiSub := newTxMultiSub(stateURI, h.config.Node.MaxPeersPerSubscription, h)
		multiSub.Start()
		h.txSubscriptionsOut[stateURI] = multiSub

		go func() {
			select {
			case <-h.Ctx().Done():
			case <-multiSub.chStop:
			}

			h.txSubscriptionsOutMu.Lock()
			defer h.txSubscriptionsOutMu.Unlock()
			delete(h.txSubscriptionsOut, stateURI)
		}()
	}
	return nil
}

type subscriptionOutput struct {
	stateURI string
	chTx     chan TxSubscriptionMsg
	chState  chan StateSubscriptionMsg
	chStop   chan struct{}
}

func (so subscriptionOutput) Txs() <-chan TxSubscriptionMsg {
	return so.chTx
}

func (so subscriptionOutput) States() <-chan StateSubscriptionMsg {
	return so.chState
}

func (so subscriptionOutput) Close() {
	close(so.chStop)
}

type TxSubscription interface {
	Txs() <-chan TxSubscriptionMsg
	Close()
}

type TxSubscriptionMsg struct {
	Tx     *Tx
	Leaves []types.ID
}

func (h *host) Subscribe(ctx context.Context, stateURI string) (TxSubscription, error) {
	err := h.subscribe(ctx, stateURI)
	if err != nil {
		return nil, err
	}

	h.subscriptionOutputsMu.Lock()
	defer h.subscriptionOutputsMu.Unlock()

	if _, exists := h.subscriptionOutputs[stateURI]; !exists {
		h.subscriptionOutputs[stateURI] = make(map[subscriptionOutput]struct{})
	}

	so := subscriptionOutput{stateURI: stateURI, chTx: make(chan TxSubscriptionMsg), chStop: make(chan struct{})}
	h.subscriptionOutputs[stateURI][so] = struct{}{}

	go func() {
		defer close(so.chTx)

		select {
		case <-h.Ctx().Done():
		case <-so.chStop:
		}

		h.subscriptionOutputsMu.Lock()
		defer h.subscriptionOutputsMu.Unlock()
		delete(h.subscriptionOutputs[stateURI], so)
	}()

	return so, nil
}

type StateSubscription interface {
	States() <-chan StateSubscriptionMsg
	Close()
}

type StateSubscriptionMsg struct {
	State  tree.Node
	Leaves []types.ID
}

func (h *host) SubscribeStates(ctx context.Context, stateURI string) (StateSubscription, error) {
	err := h.subscribe(ctx, stateURI)
	if err != nil {
		return nil, err
	}

	h.subscriptionOutputsMu.Lock()
	defer h.subscriptionOutputsMu.Unlock()

	if _, exists := h.subscriptionOutputs[stateURI]; !exists {
		h.subscriptionOutputs[stateURI] = make(map[subscriptionOutput]struct{})
	}

	so := subscriptionOutput{stateURI: stateURI, chState: make(chan StateSubscriptionMsg), chStop: make(chan struct{})}
	h.subscriptionOutputs[stateURI][so] = struct{}{}

	go func() {
		defer close(so.chState)

		select {
		case <-h.Ctx().Done():
		case <-so.chStop:
		}

		h.subscriptionOutputsMu.Lock()
		defer h.subscriptionOutputsMu.Unlock()
		delete(h.subscriptionOutputs[stateURI], so)
	}()

	return so, nil
}

func (h *host) Unsubscribe(stateURI string) error {
	h.txSubscriptionsOutMu.Lock()
	defer h.txSubscriptionsOutMu.Unlock()

	err := h.config.Update(func() error {
		h.config.Node.SubscribedStateURIs.Remove(stateURI)
		return nil
	})
	if err != nil {
		return err
	}

	h.txSubscriptionsOut[stateURI].Stop()
	delete(h.txSubscriptionsOut, stateURI)
	return nil
}

func (h *host) ChallengePeerIdentity(ctx context.Context, peer Peer) (_ SigningPublicKey, _ EncryptingPublicKey, err error) {
	defer withStack(&err)

	err = peer.EnsureConnected(ctx)
	if err != nil {
		return nil, nil, err
	}

	challengeMsg, err := types.GenerateChallengeMsg()
	if err != nil {
		return nil, nil, err
	}

	err = peer.ChallengeIdentity(types.ChallengeMsg(challengeMsg))
	if err != nil {
		return nil, nil, err
	}

	resp, err := peer.ReceiveChallengeIdentityResponse()
	if err != nil {
		return nil, nil, err
	}

	sigpubkey, err := RecoverSigningPubkey(types.HashBytes(challengeMsg), resp.Signature)
	if err != nil {
		return nil, nil, err
	}
	encpubkey := EncryptingPublicKeyFromBytes(resp.EncryptingPublicKey)

	h.peerStore.AddVerifiedCredentials(peer.DialInfo(), sigpubkey.Address(), sigpubkey, encpubkey)

	return sigpubkey, encpubkey, nil
}

func (h *host) HandleChallengeIdentity(challengeMsg types.ChallengeMsg, peer Peer) error {
	defer peer.CloseConn()

	sig, err := h.signingKeypair.SignHash(types.HashBytes(challengeMsg))
	if err != nil {
		return err
	}
	return peer.RespondChallengeIdentity(ChallengeIdentityResponse{
		Signature:           sig,
		EncryptingPublicKey: h.encryptingKeypair.EncryptingPublicKey.Bytes(),
	})
}

func (h *host) handleNewState(tx *Tx, state tree.Node, leaves []types.ID) {
	// @@TODO: don't do this, this is stupid.  store ungossiped txs in the DB and create a
	// PeerManager that gossips them on a SleeperTask-like trigger.
	go func() {
		// If this is the genesis tx of a private state URI, ensure that we subscribe to that state URI
		// @@TODO: allow blacklisting of senders
		if tx.IsPrivate() && tx.ID == GenesisTxID && !h.config.Node.SubscribedStateURIs.Contains(tx.StateURI) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sub, err := h.Subscribe(ctx, tx.StateURI)
			if err != nil {
				h.Errorf("error subscribing to state URI %v: %v", tx.StateURI, err)
			}
			sub.Close() // We don't need the in-process subscription
		}

		// Broadcast state and tx to others
		ctx, cancel := context.WithTimeout(h.Ctx(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var alreadySentPeers sync.Map

		wg.Add(4)
		go h.broadcastTxToSubscribedPeers(ctx, tx, leaves, &alreadySentPeers, &wg)
		go h.broadcastTxToStateURIProviders(ctx, tx, leaves, &alreadySentPeers, &wg)
		go h.broadcastStateToSubscribedPeers(ctx, tx, state, leaves, &wg)
		go h.broadcastToInProcessSubscribers(ctx, tx, state, leaves, &wg)
	}()
}

func (h *host) broadcastToInProcessSubscribers(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID, wg *sync.WaitGroup) {
	defer wg.Done()

	h.subscriptionOutputsMu.RLock()
	defer h.subscriptionOutputsMu.RUnlock()

	for so := range h.subscriptionOutputs[tx.StateURI] {
		wg.Add(1)

		so := so
		go func() {
			defer wg.Done()

			select {
			case so.chTx <- TxSubscriptionMsg{Tx: tx, Leaves: leaves}:
			case so.chState <- StateSubscriptionMsg{State: state, Leaves: leaves}:
			case <-ctx.Done():
			case <-h.Ctx().Done():
			}
		}()
	}
}

func (h *host) broadcastStateToSubscribedPeers(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID, wg *sync.WaitGroup) {
	defer wg.Done()

	h.stateSubscriptionsInMu.RLock()
	defer h.stateSubscriptionsInMu.RUnlock()

	for keypathStr := range h.stateSubscriptionsIn[tx.StateURI] {
		for peer := range h.stateSubscriptionsIn[tx.StateURI][keypathStr] {
			peer := peer
			keypathStr := keypathStr

			wg.Add(1)
			go func() {
				defer wg.Done()

				err := peer.EnsureConnected(ctx)
				if err != nil {
					h.Errorf("error connecting to peer: %v", err)
					h.HandleIncomingStateSubscriptionClosed(tx.StateURI, tree.Keypath(keypathStr), peer)
					return
				}

				state := state.NodeAt(tree.Keypath(keypathStr), nil)

				err = peer.PutState(state, leaves)
				if err != nil {
					h.Errorf("error writing tx to peer: %v", err)
					h.HandleIncomingStateSubscriptionClosed(tx.StateURI, tree.Keypath(keypathStr), peer)
					return
				}
			}()
		}
	}
}

func (h *host) broadcastTxToStateURIProviders(ctx context.Context, tx *Tx, leaves []types.ID, alreadySentPeers *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // @@TODO: make configurable
	defer cancel()

	ch := make(chan Peer)
	go func() {
		defer close(ch)

		chProviders := h.ProvidersOfStateURI(ctx, tx.StateURI)
		for {
			select {
			case peer, open := <-chProviders:
				if !open {
					return
				}
				ch <- peer
			case <-ctx.Done():
				return
			case <-h.Ctx().Done():
				return
			}
		}
	}()

	for peer := range ch {
		if h.txSeenByPeer(peer, tx.StateURI, tx.ID) || tx.From == peer.Address() { // @@TODO: do we always want to avoid broadcasting when `from == peer.address`?
			continue
		}
		h.Debugf("rebroadcasting %v to %v", tx.ID.Pretty(), peer.DialInfo().DialAddr)

		wg.Add(1)
		peer := peer
		go func() {
			defer wg.Done()

			_, alreadySent := alreadySentPeers.LoadOrStore(peer.DialInfo(), struct{}{})
			if alreadySent {
				return
			}

			err := peer.EnsureConnected(ctx)
			if err != nil {
				h.Errorf("error connecting to peer: %v", err)
				return
			}
			defer peer.CloseConn()

			err = peer.Put(*tx, leaves)
			if err != nil {
				h.Errorf("error writing tx to peer: %v", err)
				return
			}
		}()
	}
}

func (h *host) broadcastTxToSubscribedPeers(ctx context.Context, tx *Tx, leaves []types.ID, alreadySentPeers *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

	h.txSubscriptionsInMu.RLock()
	defer h.txSubscriptionsInMu.RUnlock()

	for peer := range h.txSubscriptionsIn[tx.StateURI] {
		if h.txSeenByPeer(peer, tx.StateURI, tx.ID) {
			continue
		}

		wg.Add(1)
		peer := peer
		go func() {
			defer wg.Done()

			_, alreadySent := alreadySentPeers.LoadOrStore(peer.DialInfo(), struct{}{})
			if alreadySent {
				return
			}

			err := peer.EnsureConnected(ctx)
			if err != nil {
				h.Errorf("error connecting to peer: %v", err)
				h.HandleIncomingTxSubscriptionClosed(tx.StateURI, peer)
				return
			}

			isPrivate, err := h.controllerHub.IsPrivate(tx.StateURI)
			if err != nil {
				h.Errorf("error determining if state URI '%v' is private: %v", tx.StateURI, err)
				return
			}

			if isPrivate {
				isMember, err := h.controllerHub.IsMember(tx.StateURI, peer.Address())
				if err != nil {
					h.Errorf("error determining if peer '%v' is a member of state URI '%v': %v", peer.Address(), tx.StateURI, err)
					return
				}

				if isMember {
					err = peer.PutPrivate(*tx, leaves)
					if err != nil {
						h.Errorf("error writing tx to peer: %v", err)
						h.HandleIncomingTxSubscriptionClosed(tx.StateURI, peer)
						return
					}
				}

			} else {
				err = peer.Put(*tx, leaves)
				if err != nil {
					h.Errorf("error writing tx to peer: %v", err)
					h.HandleIncomingTxSubscriptionClosed(tx.StateURI, peer)
					return
				}
			}
		}()
	}
}

func (h *host) SendTx(ctx context.Context, tx Tx) (err error) {
	h.Info(0, "adding tx ", tx.ID.Pretty())

	defer func() {
		if err != nil {
			return
		}
		// If we send a tx to a state URI that we're not subscribed to yet, auto-subscribe.
		if !h.config.Node.SubscribedStateURIs.Contains(tx.StateURI) {
			err := h.config.Update(func() error {
				h.config.Node.SubscribedStateURIs.Add(tx.StateURI)
				return nil
			})
			if err != nil {
				h.Errorf("error adding %v to config.Node.SubscribedStateURIs: %v", tx.StateURI, err)
			}
		}
	}()

	if tx.From == (types.Address{}) {
		tx.From = h.signingKeypair.Address()
	}

	if len(tx.Parents) == 0 && tx.ID != GenesisTxID {
		var parents []types.ID
		parents, err = h.controllerHub.Leaves(tx.StateURI)
		if err != nil {
			return err
		}

		tx.Parents = parents
	}

	if len(tx.Sig) == 0 {
		err = h.SignTx(&tx)
		if err != nil {
			return err
		}
	}

	err = h.controllerHub.AddTx(&tx, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *host) SignTx(tx *Tx) error {
	var err error
	tx.Sig, err = h.signingKeypair.SignHash(tx.Hash())
	return err
}

func (h *host) AddRef(reader io.ReadCloser) (types.Hash, types.Hash, error) {
	return h.refStore.StoreObject(reader)
}

func (h *host) handleRefsNeeded(refs []types.RefID) {
	select {
	case <-h.Ctx().Done():
		return
	case h.chRefsNeeded <- refs:
	}
}

func (h *host) periodicallyFetchMissingRefs() {
	tick := time.NewTicker(10 * time.Second) // @@TODO: make configurable
	defer tick.Stop()

	for {
		select {
		case <-h.Ctx().Done():
			return

		case refs := <-h.chRefsNeeded:
			h.fetchMissingRefs(refs)

		case <-tick.C:
			refs, err := h.refStore.RefsNeeded()
			if err != nil {
				h.Errorf("error fetching list of needed refs: %v", err)
				continue
			}

			if len(refs) > 0 {
				h.fetchMissingRefs(refs)
			}
		}
	}
}

func (h *host) fetchMissingRefs(refs []types.RefID) {
	var wg sync.WaitGroup
	for _, refID := range refs {
		wg.Add(1)
		refID := refID
		go func() {
			defer wg.Done()
			h.FetchRef(h.Ctx(), refID)
		}()
	}
	wg.Wait()
}

func (h *host) FetchRef(ctx context.Context, refID types.RefID) {
	for peer := range h.ProvidersOfRef(ctx, refID) {
		err := peer.EnsureConnected(ctx)
		if err != nil {
			h.Errorf("error connecting to peer: %v", err)
			continue
		}

		err = peer.FetchRef(refID)
		if err != nil {
			h.Errorf("error writing to peer: %v", err)
			continue
		}

		// Not currently used
		_, err = peer.ReceiveRefHeader()
		if err != nil {
			h.Errorf("error reading from peer: %v", err)
			continue
		}

		pr, pw := io.Pipe()
		go func() {
			var err error
			defer func() { pw.CloseWithError(err) }()

			for {
				select {
				case <-ctx.Done():
					err = ctx.Err()
					return
				default:
				}

				pkt, err := peer.ReceiveRefPacket()
				if err != nil {
					h.Errorf("error receiving ref from peer: %v", err)
					return
				} else if pkt.End {
					return
				}

				var n int
				n, err = pw.Write(pkt.Data)
				if err != nil {
					h.Errorf("error receiving ref from peer: %v", err)
					return
				} else if n < len(pkt.Data) {
					err = io.ErrUnexpectedEOF
					return
				}
			}
		}()

		sha1Hash, sha3Hash, err := h.refStore.StoreObject(pr)
		if err != nil {
			h.Errorf("could not store ref: %v", err)
			continue
		}
		// @@TODO: check stored refHash against the one we requested

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		h.announceRefs(ctx, []types.RefID{
			{HashAlg: types.SHA1, Hash: sha1Hash},
			{HashAlg: types.SHA3, Hash: sha3Hash},
		})
		return
	}
}

func (h *host) announceRefs(ctx context.Context, refIDs []types.RefID) {
	var wg sync.WaitGroup
	wg.Add(len(refIDs) * len(h.transports))

	for _, transport := range h.transports {
		for _, refID := range refIDs {
			transport := transport
			refID := refID

			go func() {
				defer wg.Done()

				err := transport.AnnounceRef(ctx, refID)
				if errors.Cause(err) == types.ErrUnimplemented {
					return
				} else if err != nil {
					h.Warnf("error announcing ref %v over transport %v: %v", refID, transport.Name(), err)
				}
			}()
		}
	}
	wg.Wait()
}

const (
	REF_CHUNK_SIZE = 1024 // @@TODO: tunable buffer size?
)

func (h *host) HandleFetchRefReceived(refID types.RefID, peer Peer) {
	defer peer.CloseConn()

	objectReader, _, err := h.refStore.Object(refID)
	// @@TODO: handle the case where we don't have the ref more gracefully
	if err != nil {
		panic(err)
	}

	err = peer.SendRefHeader()
	if err != nil {
		h.Errorf("[ref server] %+v", errors.WithStack(err))
		return
	}

	buf := make([]byte, REF_CHUNK_SIZE)
	for {
		n, err := io.ReadFull(objectReader, buf)
		if err == io.EOF {
			break
		} else if err == io.ErrUnexpectedEOF {
			buf = buf[:n]
		} else if err != nil {
			h.Errorf("[ref server] %+v", err)
			return
		}

		err = peer.SendRefPacket(buf, false)
		if err != nil {
			h.Errorf("[ref server] %+v", errors.WithStack(err))
			return
		}
	}

	err = peer.SendRefPacket(nil, true)
	if err != nil {
		h.Errorf("[ref server] %+v", errors.WithStack(err))
		return
	}
}
