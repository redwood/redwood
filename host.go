package redwood

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type Host interface {
	log.Logger
	Start() error
	Close()

	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	Subscribe(ctx context.Context, stateURI string, subscriptionType SubscriptionType, keypath tree.Keypath, fetchHistoryOpts *FetchHistoryOpts) (ReadableSubscription, error)
	Unsubscribe(stateURI string) error
	SubscribeStateURIs() StateURISubscription
	SendTx(ctx context.Context, tx Tx) error
	AddRef(reader io.ReadCloser) (types.Hash, types.Hash, error)
	FetchRef(ctx context.Context, ref types.RefID)
	AddPeer(dialInfo PeerDialInfo)
	Transport(name string) Transport
	Controllers() ControllerHub
	ChallengePeerIdentity(ctx context.Context, peer Peer) error

	Identities() ([]identity.Identity, error)
	NewIdentity(public bool) (identity.Identity, error)

	Peers() []*peerDetails
	ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan Peer
	ProvidersOfRef(ctx context.Context, refID types.RefID) <-chan Peer
	PeersClaimingAddress(ctx context.Context, address types.Address) <-chan Peer

	HandleFetchHistoryRequest(stateURI string, opts FetchHistoryOpts, writeSub WritableSubscription) error
	HandleWritableSubscriptionOpened(writeSub WritableSubscription, fetchHistoryOpts *FetchHistoryOpts)
	HandleWritableSubscriptionClosed(writeSub WritableSubscription)
	HandleReadableSubscriptionClosed(stateURI string)
	HandleTxReceived(tx Tx, peer Peer)
	HandleAckReceived(stateURI string, txID types.ID, peer Peer)
	HandleChallengeIdentity(challengeMsg types.ChallengeMsg, peer Peer) error
	HandleFetchRefReceived(refID types.RefID, peer Peer)
}

type host struct {
	log.Logger
	chStop chan struct{}
	chDone chan struct{}

	config *Config

	readableSubscriptions   map[string]*multiReaderSubscription // map[stateURI]
	readableSubscriptionsMu sync.RWMutex
	writableSubscriptions   map[string]map[WritableSubscription]struct{} // map[stateURI]
	writableSubscriptionsMu sync.RWMutex
	stateURISubscriptions   map[*stateURISubscription]struct{}
	stateURISubscriptionsMu sync.RWMutex

	peerSeenTxs   map[PeerDialInfo]map[string]map[types.ID]bool // map[PeerDialInfo]map[stateURI]map[tx.ID]
	peerSeenTxsMu sync.RWMutex

	processPeersTask *utils.PeriodicTask

	controllerHub ControllerHub
	transports    map[string]Transport
	peerStore     PeerStore
	refStore      RefStore
	keyStore      identity.KeyStore

	chRefsNeeded chan []types.RefID
}

var (
	ErrUnsignedTx = errors.New("unsigned tx")
	ErrProtocol   = errors.New("protocol error")
	ErrPeerIsSelf = errors.New("peer is self")
)

func NewHost(
	transports []Transport,
	controllerHub ControllerHub,
	keyStore identity.KeyStore,
	refStore RefStore,
	peerStore PeerStore,
	config *Config,
) (Host, error) {
	transportsMap := make(map[string]Transport)
	for _, tpt := range transports {
		transportsMap[tpt.Name()] = tpt
	}
	h := &host{
		Logger:                log.NewLogger("host"),
		chStop:                make(chan struct{}),
		chDone:                make(chan struct{}),
		transports:            transportsMap,
		controllerHub:         controllerHub,
		readableSubscriptions: make(map[string]*multiReaderSubscription),
		writableSubscriptions: make(map[string]map[WritableSubscription]struct{}),
		stateURISubscriptions: make(map[*stateURISubscription]struct{}),
		peerSeenTxs:           make(map[PeerDialInfo]map[string]map[types.ID]bool),
		peerStore:             peerStore,
		refStore:              refStore,
		keyStore:              keyStore,
		chRefsNeeded:          make(chan []types.RefID, 100),
		config:                config,
	}
	return h, nil
}

func (h *host) Start() error {
	h.SetLogLabel("host")

	// Set up the peer store
	h.peerStore.OnNewUnverifiedPeer(h.handleNewUnverifiedPeer)
	h.processPeersTask = utils.NewPeriodicTask(10*time.Second, h.processPeers)

	// Set up the controller Hub
	h.controllerHub.OnNewState(h.handleNewState)
	err := h.controllerHub.Start()
	if err != nil {
		return err
	}

	// Set up the ref store
	h.refStore.OnRefsNeeded(h.handleRefsNeeded)

	// Set up the transports
	for _, transport := range h.transports {
		transport.SetHost(h)
		err := transport.Start()
		if err != nil {
			return err
		}
	}

	go h.periodicallyFetchMissingRefs()

	return nil
}

func (h *host) Close() {
	close(h.chStop)

	h.processPeersTask.Close()

	var writableSubs []WritableSubscription
	func() {
		h.writableSubscriptionsMu.Lock()
		defer h.writableSubscriptionsMu.Unlock()
		for _, subs := range h.writableSubscriptions {
			for sub := range subs {
				writableSubs = append(writableSubs, sub)
			}
		}
	}()
	for _, sub := range writableSubs {
		err := sub.Close()
		if err != nil {
			h.Errorf("error closing writable subscription: %v", err)
		}
	}

	var readableSubs []*multiReaderSubscription
	func() {
		h.readableSubscriptionsMu.Lock()
		defer h.readableSubscriptionsMu.Unlock()
		for _, sub := range h.readableSubscriptions {
			readableSubs = append(readableSubs, sub)
		}
	}()
	for _, sub := range readableSubs {
		err := sub.Close()
		if err != nil {
			h.Errorf("error closing readable subscription: %v", err)
		}
	}

	h.controllerHub.Close()

	for _, tpt := range h.transports {
		tpt.Close()
	}
}

func (h *host) Peers() []*peerDetails {
	return h.peerStore.Peers()
}

func (h *host) Transport(name string) Transport {
	return h.transports[name]
}

func (h *host) Controllers() ControllerHub {
	return h.controllerHub
}

func (h *host) Identities() ([]identity.Identity, error) {
	return h.keyStore.Identities()
}

func (h *host) NewIdentity(public bool) (identity.Identity, error) {
	return h.keyStore.NewIdentity(public)
}

func (h *host) StateAtVersion(stateURI string, version *types.ID) (tree.Node, error) {
	return h.Controllers().StateAtVersion(stateURI, version)
}

// Returns peers discovered through any transport that have already been authenticated.
// It's not guaranteed that they actually provide the stateURI in question. These are
// simply peers that claim to.
func (h *host) ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan Peer {
	if utils.IsLocalStateURI(stateURI) {
		ch := make(chan Peer)
		close(ch)
		return ch
	}

	ctx, cancel := utils.CombinedContext(ctx, h.chStop)

	var (
		ch          = make(chan Peer)
		wg          = utils.NewWaitGroupChan(ctx)
		alreadySent sync.Map
	)

	wg.Add(1)
	defer wg.Done()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, peerDetails := range h.peerStore.PeersServingStateURI(stateURI) {
			dialInfo := peerDetails.DialInfo()
			tpt := h.Transport(dialInfo.TransportName)
			if tpt == nil {
				continue
			}

			if _, exists := alreadySent.LoadOrStore(dialInfo, struct{}{}); exists {
				continue
			}

			peer, err := tpt.NewPeerConn(ctx, dialInfo.DialAddr)
			if err != nil {
				h.Warnf("error creating new peer conn (transport: %v, dialAddr: %v)", dialInfo.TransportName, dialInfo.DialAddr)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- peer:
			}
		}
	}()

	for _, tpt := range h.transports {
		innerCh, err := tpt.ProvidersOfStateURI(ctx, stateURI)
		if err != nil {
			h.Warnf("error fetching providers of State-URI %v on transport %v: %v", stateURI, tpt.Name(), err)
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
		defer wg.Close()
		<-wg.Wait()
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
				case <-h.chStop:
					return
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-h.chStop:
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
				case <-h.chStop:
					return
				case <-ctx.Done():
					return
				case peer, open := <-innerCh:
					if !open {
						return
					}

					select {
					case <-h.chStop:
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
	// @@TODO: convert to LRU cache

	if len(peer.Addresses()) == 0 {
		return false
	}

	h.peerSeenTxsMu.Lock()
	defer h.peerSeenTxsMu.Unlock()

	dialInfo := peer.DialInfo()

	if h.peerSeenTxs[dialInfo] == nil {
		return false
	} else if h.peerSeenTxs[dialInfo][stateURI] == nil {
		return false
	}
	return h.peerSeenTxs[dialInfo][stateURI][txID]
}

func (h *host) AddPeer(dialInfo PeerDialInfo) {
	h.peerStore.AddDialInfos([]PeerDialInfo{dialInfo})
	h.processPeersTask.Enqueue()
}

func (h *host) handleNewUnverifiedPeer(dialInfo PeerDialInfo) {
	h.processPeersTask.Enqueue()
}

func (h *host) processPeers(ctx context.Context) {
	ctx, cancel := utils.CombinedContext(ctx, h.chStop)
	defer cancel()

	wg := utils.NewWaitGroupChan(ctx)
	defer wg.Close()

	wg.Add(1)

	// Announce peers
	{
		allDialInfos := h.peerStore.AllDialInfos()

		for _, tpt := range h.transports {
			for _, peerDetails := range h.peerStore.PeersFromTransport(tpt.Name()) {
				if peerDetails.DialInfo().TransportName != tpt.Name() {
					continue
				}

				tpt := tpt
				peerDetails := peerDetails

				wg.Add(1)
				go func() {
					defer wg.Done()

					peer, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
					if errors.Cause(err) == ErrPeerIsSelf {
						return
					} else if err != nil {
						h.Warnf("error creating new %v peer: %v", tpt.Name(), err)
						return
					}
					defer peer.Close()

					err = peer.EnsureConnected(ctx)
					if err != nil {
						h.Warnf("error connecting to %v peer (%v): %v", tpt.Name(), peerDetails.DialInfo().DialAddr, err)
						return
					}

					peer.AnnouncePeers(ctx, allDialInfos)
					if err != nil {
						// t.Errorf("error writing to peer: %+v", err)
					}
				}()
			}
		}
	}

	// Verify unverified peers
	{
		unverifiedPeers := h.peerStore.UnverifiedPeers()

		for _, unverifiedPeer := range unverifiedPeers {
			transport := h.Transport(unverifiedPeer.DialInfo().TransportName)
			if transport == nil {
				// Unsupported transport
				continue
			}

			peer, err := transport.NewPeerConn(ctx, unverifiedPeer.DialInfo().DialAddr)
			if errors.Cause(err) == ErrPeerIsSelf {
				continue
			} else if errors.Cause(err) == types.ErrConnection {
				continue
			} else if err != nil {
				h.Warnf("could not get peer at %v %v: %v", unverifiedPeer.DialInfo().TransportName, unverifiedPeer.DialInfo().DialAddr, err)
				continue
			} else if !peer.Ready() {
				h.Debugf("skipping peer %v: failures=%v lastFailure=%v", peer.DialInfo(), peer.Failures(), time.Now().Sub(peer.LastFailure()))
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer peer.Close()

				err := h.ChallengePeerIdentity(ctx, peer)
				if err != nil {
					h.Errorf("error verifying peer identity (%v): %v ", peer.DialInfo(), err)
					return
				}
			}()
		}
	}

	wg.Done()
	<-wg.Wait()
}

type FetchHistoryOpts struct {
	FromTxID types.ID
	ToTxID   types.ID
}

func (h *host) HandleFetchHistoryRequest(stateURI string, opts FetchHistoryOpts, writeSub WritableSubscription) error {
	// @@TODO: respect the `opts.ToTxID` param
	// @@TODO: if .FromTxID == 0, set it to GenesisTxID

	iter := h.controllerHub.FetchTxs(stateURI, opts.FromTxID)
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

		isPrivate, err := h.controllerHub.IsPrivate(tx.StateURI)
		if err != nil {
			return err
		}

		if isPrivate {
			var isAllowed bool
			if peer, isPeer := writeSub.(Peer); isPeer {
				for _, addr := range peer.Addresses() {
					isAllowed, err = h.controllerHub.IsMember(tx.StateURI, addr)
					if err != nil {
						h.Errorf("error determining if peer '%v' is a member of private state URI '%v': %v", addr, tx.StateURI, err)
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

func (h *host) HandleWritableSubscriptionOpened(writeSub WritableSubscription, fetchHistoryOpts *FetchHistoryOpts) {
	if writeSub.Type().Includes(SubscriptionType_Txs) && fetchHistoryOpts != nil {
		h.HandleFetchHistoryRequest(writeSub.StateURI(), *fetchHistoryOpts, writeSub)
	}

	if writeSub.Type().Includes(SubscriptionType_States) {
		// Normalize empty keypaths
		keypath := writeSub.Keypath()
		if keypath.Equals(tree.KeypathSeparator) {
			keypath = nil
		}

		// Immediately write the current state to the subscriber
		state, err := h.Controllers().StateAtVersion(writeSub.StateURI(), nil)
		if err != nil && errors.Cause(err) != ErrNoController {
			h.Errorf("error writing initial state to peer: %v", err)
			writeSub.Close()
			return
		} else if err == nil {
			defer state.Close()

			leaves, err := h.Controllers().Leaves(writeSub.StateURI())
			if err != nil {
				h.Errorf("error writing initial state to peer (%v): %v", writeSub.StateURI(), err)
			} else {
				node, err := state.CopyToMemory(keypath, nil)
				if err != nil && errors.Cause(err) == types.Err404 {
					// no-op
				} else if err != nil {
					h.Errorf("error writing initial state to peer (%v): %v", writeSub.StateURI(), err)
				} else {
					writeSub.EnqueueWrite(writeSub.StateURI(), nil, node, leaves)
				}
			}
		}
	}

	h.writableSubscriptionsMu.Lock()
	defer h.writableSubscriptionsMu.Unlock()

	if _, exists := h.writableSubscriptions[writeSub.StateURI()]; !exists {
		h.writableSubscriptions[writeSub.StateURI()] = make(map[WritableSubscription]struct{})
	}
	h.writableSubscriptions[writeSub.StateURI()][writeSub] = struct{}{}
}

func (h *host) HandleWritableSubscriptionClosed(writeSub WritableSubscription) {
	h.writableSubscriptionsMu.Lock()
	defer h.writableSubscriptionsMu.Unlock()

	if _, exists := h.writableSubscriptions[writeSub.StateURI()]; exists {
		delete(h.writableSubscriptions[writeSub.StateURI()], writeSub)
	}
}

func (h *host) HandleReadableSubscriptionClosed(stateURI string) {
	h.readableSubscriptionsMu.Lock()
	defer h.readableSubscriptionsMu.Unlock()
	delete(h.readableSubscriptions, stateURI)
}

func (h *host) subscribe(ctx context.Context, stateURI string) error {
	h.readableSubscriptionsMu.Lock()
	defer h.readableSubscriptionsMu.Unlock()

	err := h.config.Update(func() error {
		h.config.Node.SubscribedStateURIs.Add(stateURI)
		return nil
	})
	if err != nil {
		return err
	}

	_, err = h.Controllers().EnsureController(stateURI)
	if err != nil {
		return err
	}

	// If this state URI is not intended to be shared, don't bother subscribing to other nodes
	if utils.IsLocalStateURI(stateURI) {
		return nil
	}

	if _, exists := h.readableSubscriptions[stateURI]; !exists {
		multiSub := newMultiReaderSubscription(stateURI, h.config.Node.MaxPeersPerSubscription, h)
		go multiSub.Start()
		h.readableSubscriptions[stateURI] = multiSub
	}

	h.stateURISubscriptionsMu.Lock()
	defer h.stateURISubscriptionsMu.Unlock()
	for sub := range h.stateURISubscriptions {
		sub.put(stateURI)
	}

	return nil
}

func (h *host) Subscribe(
	ctx context.Context,
	stateURI string,
	subscriptionType SubscriptionType,
	stateKeypath tree.Keypath,
	fetchHistoryOpts *FetchHistoryOpts,
) (ReadableSubscription, error) {
	err := h.subscribe(ctx, stateURI)
	if err != nil {
		return nil, err
	}

	sub := newInProcessSubscription(stateURI, stateKeypath, subscriptionType, h)
	h.HandleWritableSubscriptionOpened(sub, fetchHistoryOpts)
	return sub, nil
}

func (h *host) Unsubscribe(stateURI string) error {
	// @@TODO: when the Host unsubscribes, it should close the subs of any peers reading from it
	h.readableSubscriptionsMu.Lock()
	defer h.readableSubscriptionsMu.Unlock()

	err := h.config.Update(func() error {
		h.config.Node.SubscribedStateURIs.Remove(stateURI)
		return nil
	})
	if err != nil {
		return err
	}

	h.readableSubscriptions[stateURI].Close()
	delete(h.readableSubscriptions, stateURI)
	return nil
}

func (h *host) SubscribeStateURIs() StateURISubscription {
	sub := &stateURISubscription{
		host:    h,
		mailbox: utils.NewMailbox(0),
		ch:      make(chan string),
		chStop:  make(chan struct{}),
		chDone:  make(chan struct{}),
	}

	sub.host.stateURISubscriptionsMu.Lock()
	defer sub.host.stateURISubscriptionsMu.Unlock()
	h.stateURISubscriptions[sub] = struct{}{}

	go func() {
		defer close(sub.chDone)
		for {
			select {
			case <-sub.chStop:
				return
			case <-sub.mailbox.Notify():
				for {
					x := sub.mailbox.Retrieve()
					if x == nil {
						break
					}
					stateURI := x.(string)
					select {
					case sub.ch <- stateURI:
					case <-sub.chStop:
						return
					}
				}
			}
		}
	}()

	return sub
}

type StateURISubscription interface {
	Read() (string, error)
	Close()
}

type stateURISubscription struct {
	host    *host
	mailbox *utils.Mailbox
	ch      chan string
	chStop  chan struct{}
	chDone  chan struct{}
}

func (sub *stateURISubscription) put(stateURI string) {
	sub.mailbox.Deliver(stateURI)
}

func (sub *stateURISubscription) Read() (string, error) {
	select {
	case <-sub.chStop:
		return "", errors.New("shutting down")
	case s := <-sub.ch:
		return s, nil
	}
}

func (sub *stateURISubscription) Close() {
	sub.host.stateURISubscriptionsMu.Lock()
	defer sub.host.stateURISubscriptionsMu.Unlock()
	delete(sub.host.stateURISubscriptions, sub)
	close(sub.chStop)
	<-sub.chDone
}

func (h *host) ChallengePeerIdentity(ctx context.Context, peer Peer) (err error) {
	defer utils.WithStack(&err)

	err = peer.EnsureConnected(ctx)
	if err != nil {
		return err
	}

	challengeMsg, err := types.GenerateChallengeMsg()
	if err != nil {
		return err
	}

	err = peer.ChallengeIdentity(challengeMsg)
	if err != nil {
		return err
	}

	resp, err := peer.ReceiveChallengeIdentityResponse()
	if err != nil {
		return err
	}

	for _, proof := range resp {
		sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challengeMsg), proof.Signature)
		if err != nil {
			return err
		}
		encpubkey := crypto.EncryptingPublicKeyFromBytes(proof.EncryptingPublicKey)

		h.peerStore.AddVerifiedCredentials(peer.DialInfo(), sigpubkey.Address(), sigpubkey, encpubkey)
	}

	return nil
}

func (h *host) HandleChallengeIdentity(challengeMsg types.ChallengeMsg, peer Peer) error {
	defer peer.Close()

	publicIdentities, err := h.keyStore.PublicIdentities()
	if err != nil {
		h.Errorf("error fetching public identities from key store: %v", err)
		return err
	}

	var responses []ChallengeIdentityResponse
	for _, identity := range publicIdentities {
		sig, err := h.keyStore.SignHash(identity.Address(), types.HashBytes(challengeMsg))
		if err != nil {
			h.Errorf("error signing hash: %v", err)
			return err
		}
		responses = append(responses, ChallengeIdentityResponse{
			Signature:           sig,
			EncryptingPublicKey: identity.Encrypting.EncryptingPublicKey.Bytes(),
		})
	}
	return peer.RespondChallengeIdentity(responses)
}

func (h *host) handleNewState(tx *Tx, state tree.Node, leaves []types.ID) {
	state, err := state.CopyToMemory(nil, nil)
	if err != nil {
		h.Errorf("handleNewState: couldn't copy state to memory: %v", err)
		state = tree.NewMemoryNode() // give subscribers an empty state
	}

	// @@TODO: don't do this, this is stupid.  store ungossiped txs in the DB and create a
	// PeerManager that gossips them on a SleeperTask-like trigger.
	go func() {
		// If this is the genesis tx of a private state URI, ensure that we subscribe to that state URI
		// @@TODO: allow blacklisting of senders
		if tx.IsPrivate() && tx.ID == GenesisTxID && !h.config.Node.SubscribedStateURIs.Contains(tx.StateURI) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sub, err := h.Subscribe(ctx, tx.StateURI, 0, nil, nil)
			if err != nil {
				h.Errorf("error subscribing to state URI %v: %v", tx.StateURI, err)
			}
			sub.Close() // We don't need the in-process subscription
		}

		// If this state URI isn't meant to be shared, don't broadcast
		// if utils.IsLocalStateURI(tx.StateURI) {
		// 	return
		// }

		// Broadcast state and tx to others
		ctx, cancel := utils.CombinedContext(h.chStop, 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var alreadySentPeers sync.Map

		wg.Add(3)
		go h.broadcastToWritableSubscribers(ctx, tx, state, leaves, &alreadySentPeers, &wg)
		go h.broadcastToStateURIProviders(ctx, tx, leaves, &alreadySentPeers, &wg)
		go h.broadcastToPrivateRecipients(ctx, tx, leaves, &alreadySentPeers, &wg)

		wg.Wait()
	}()
}

func (h *host) broadcastToPrivateRecipients(ctx context.Context, tx *Tx, leaves []types.ID, alreadySentPeers *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

	for _, address := range tx.Recipients {
		for _, peerDetails := range h.peerStore.PeersWithAddress(address) {
			tpt := h.Transport(peerDetails.DialInfo().TransportName)
			if tpt == nil {
				continue
			}

			peer, err := tpt.NewPeerConn(ctx, peerDetails.DialInfo().DialAddr)
			if err != nil {
				h.Errorf("error creating connection to peer %v: %v", peerDetails.DialInfo(), err)
				continue
			}

			if len(peer.Addresses()) == 0 {
				panic("impossible")
			} else if h.txSeenByPeer(peer, tx.StateURI, tx.ID) {
				continue
			}
			// @@TODO: do we always want to avoid broadcasting when `from == peer.address`?
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
			go func() {
				defer wg.Done()

				err := peer.EnsureConnected(ctx)
				if err != nil {
					h.Errorf("error connecting to peer: %v", err)
					return
				}
				defer peer.Close()

				err = peer.Put(ctx, tx, nil, leaves)
				if err != nil {
					h.Errorf("error writing tx to peer: %v", err)
					return
				}
			}()
		}
	}
}

func (h *host) broadcastToStateURIProviders(ctx context.Context, tx *Tx, leaves []types.ID, alreadySentPeers *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()

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
				select {
				case ch <- peer:
				case <-ctx.Done():
					return
				case <-h.chStop:
					return
				}

			case <-ctx.Done():
				return
			case <-h.chStop:
				return
			}
		}
	}()

Outer:
	for peer := range ch {
		if h.txSeenByPeer(peer, tx.StateURI, tx.ID) {
			continue Outer
		}
		// @@TODO: do we always want to avoid broadcasting when `from == peer.address`?
		for _, addr := range peer.Addresses() {
			if tx.From == addr {
				continue Outer
			}
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
				return
			}
			defer peer.Close()

			err = peer.Put(ctx, tx, nil, leaves)
			if err != nil {
				h.Errorf("error writing tx to peer: %v", err)
				return
			}
		}()
	}
}

func (h *host) broadcastToWritableSubscribers(
	ctx context.Context,
	tx *Tx,
	state tree.Node,
	leaves []types.ID,
	alreadySentPeers *sync.Map,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	h.writableSubscriptionsMu.RLock()
	defer h.writableSubscriptionsMu.RUnlock()

	for writeSub := range h.writableSubscriptions[tx.StateURI] {
		if peer, isPeer := writeSub.(Peer); isPeer {
			// If the subscriber wants us to send states, we never skip sending
			if h.txSeenByPeer(peer, tx.StateURI, tx.ID) && !writeSub.Type().Includes(SubscriptionType_States) {
				continue
			}
		}

		wg.Add(1)
		writeSub := writeSub
		go func() {
			defer wg.Done()

			_, alreadySent := alreadySentPeers.LoadOrStore(writeSub, struct{}{})
			if alreadySent {
				return
			}

			isPrivate, err := h.controllerHub.IsPrivate(tx.StateURI)
			if err != nil {
				h.Errorf("error determining if state URI '%v' is private: %v", tx.StateURI, err)
				return
			}

			// Drill down to the part of the state that the subscriber is interested in
			keypath := writeSub.Keypath()
			if keypath.Equals(tree.KeypathSeparator) {
				keypath = nil
			}
			state = state.NodeAt(keypath, nil)

			if isPrivate {
				var isAllowed bool
				if peer, isPeer := writeSub.(Peer); isPeer {
					for _, addr := range peer.Addresses() {
						isAllowed, err = h.controllerHub.IsMember(tx.StateURI, addr)
						if err != nil {
							h.Errorf("error determining if peer '%v' is a member of private state URI '%v': %v", addr, tx.StateURI, err)
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
					writeSub.EnqueueWrite(tx.StateURI, tx, state, leaves)
				}

			} else {
				writeSub.EnqueueWrite(tx.StateURI, tx, state, leaves)
			}
		}()
	}
}

func (h *host) SendTx(ctx context.Context, tx Tx) (err error) {
	h.Infof(0, "adding tx (%v) %v", tx.StateURI, tx.ID.Pretty())

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

	if tx.From.IsZero() {
		publicIdentities, err := h.keyStore.PublicIdentities()
		if err != nil {
			return err
		} else if len(publicIdentities) == 0 {
			return errors.New("keystore has no public identities")
		}
		tx.From = publicIdentities[0].Address()
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
		tx.Sig, err = h.keyStore.SignHash(tx.From, tx.Hash())
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

func (h *host) AddRef(reader io.ReadCloser) (types.Hash, types.Hash, error) {
	return h.refStore.StoreObject(reader)
}

func (h *host) handleRefsNeeded(refs []types.RefID) {
	select {
	case <-h.chStop:
		return
	case h.chRefsNeeded <- refs:
	}
}

func (h *host) periodicallyFetchMissingRefs() {
	tick := time.NewTicker(10 * time.Second) // @@TODO: make configurable
	defer tick.Stop()

	for {
		select {
		case <-h.chStop:
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
			h.FetchRef(context.Background(), refID)
		}()
	}
	wg.Wait()
}

func (h *host) FetchRef(ctx context.Context, refID types.RefID) {
	ctx, cancel := utils.CombinedContext(h.chStop)
	defer cancel()

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
	defer peer.Close()

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
