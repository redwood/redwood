package prototree

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protohush"
	"redwood.dev/tree"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name TreeProtocol --output ./mocks/ --case=underscore

//go:generate mockery --name TreeProtocol --output ./mocks/ --case=underscore
type TreeProtocol interface {
	process.Interface
	ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan TreePeerConn
	Subscribe(ctx context.Context, stateURI string) error
	InProcessSubscription(ctx context.Context, stateURI string, subscriptionType SubscriptionType, keypath state.Keypath, fetchHistoryOpts *FetchHistoryOpts) (ReadableSubscription, error)
	Unsubscribe(stateURI string) error
	SubscribeStateURIs() (StateURISubscription, error)
	SendTx(ctx context.Context, tx tree.Tx) error

	Vaults() Set[string]
	AddVault(host string) error
	RemoveVault(host string) error
	SyncWithAllVaults() error
}

//go:generate mockery --name TreeTransport --output ./mocks/ --case=underscore
type TreeTransport interface {
	swarm.Transport
	ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan TreePeerConn, error)
	AnnounceStateURIs(ctx context.Context, stateURIs Set[string])
	OnTxReceived(handler TxReceivedCallback)
	OnEncryptedTxReceived(handler EncryptedTxReceivedCallback)
	OnAckReceived(handler AckReceivedCallback)
	OnWritableSubscriptionOpened(handler WritableSubscriptionOpenedCallback)
	OnP2PStateURIReceived(handler P2PStateURIReceivedCallback)
}

//go:generate mockery --name TreePeerConn --output ./mocks/ --case=underscore
type TreePeerConn interface {
	swarm.PeerConn
	Subscribe(ctx context.Context, stateURI string) (ReadableSubscription, error)
	SendTx(ctx context.Context, tx tree.Tx) error
	SendEncryptedTx(ctx context.Context, encryptedTx EncryptedTx) (err error)
	Ack(ctx context.Context, stateURI string, txID state.Version) error
	AnnounceP2PStateURI(ctx context.Context, stateURI string) error
}

type treeProtocol struct {
	swarm.BaseProtocol[TreeTransport, TreePeerConn]

	store Store

	controllerHub tree.ControllerHub
	txStore       tree.TxStore
	keyStore      identity.KeyStore
	peerStore     swarm.PeerStore
	vaultManager  *VaultManager

	hushProto protohush.HushProtocol

	acl ACL

	readableSubscriptions   map[string]*multiReaderSubscription // map[stateURI]
	readableSubscriptionsMu sync.RWMutex
	writableSubscriptions   map[string]map[WritableSubscription]struct{} // map[stateURI]
	writableSubscriptionsMu sync.RWMutex

	announceStateURIsTask    *announceStateURIsTask
	announceP2PStateURIsTask *announceP2PStateURIsTask
	poolWorker               process.PoolWorker
}

var (
	_ TreeProtocol = (*treeProtocol)(nil)
)

func NewTreeProtocol(
	transports []swarm.Transport,
	hushProto protohush.HushProtocol,
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

	acl := DefaultACL{ControllerHub: controllerHub}

	tp := &treeProtocol{
		BaseProtocol: swarm.BaseProtocol[TreeTransport, TreePeerConn]{
			Process:    *process.New(ProtocolName),
			Logger:     log.NewLogger(ProtocolName),
			Transports: transportsMap,
		},
		hushProto:     hushProto,
		store:         store,
		controllerHub: controllerHub,
		txStore:       txStore,
		keyStore:      keyStore,
		peerStore:     peerStore,
		vaultManager:  NewVaultManager(store, keyStore, controllerHub, acl, 1*time.Minute),

		acl: acl,

		readableSubscriptions: make(map[string]*multiReaderSubscription),
		writableSubscriptions: make(map[string]map[WritableSubscription]struct{}),
	}
	tp.announceStateURIsTask = NewAnnounceStateURIsTask(30*time.Second, tp)
	tp.controllerHub.OnNewStateURIWithData(func(stateURI string) {
		tp.announceStateURIsTask.ForceRerun()
	})
	tp.announceP2PStateURIsTask = NewAnnounceP2PStateURIsTask(10*time.Second, tp)
	tp.poolWorker = process.NewPoolWorker("pool worker", 8, process.NewStaticScheduler(5*time.Second, 10*time.Second))

	tp.controllerHub.OnNewState(tp.handleNewState)

	tp.hushProto.OnGroupMessageEncrypted(ProtocolName+":tx", tp.handlePrivateTxEncrypted)
	tp.hushProto.OnGroupMessageDecrypted(ProtocolName+":tx", tp.handlePrivateTxDecrypted)
	tp.hushProto.OnGroupMessageEncrypted(ProtocolName+":stateuri", tp.handlePrivateStateURIEncrypted)
	tp.hushProto.OnGroupMessageDecrypted(ProtocolName+":stateuri", tp.handlePrivateStateURIDecrypted)

	for _, tpt := range tp.Transports {
		tp.Infof("registering %v", tpt.Name())
		tpt.OnTxReceived(tp.handleTxReceived)
		tpt.OnAckReceived(tp.handleAckReceived)
		tpt.OnWritableSubscriptionOpened(tp.handleWritableSubscriptionOpened)
		tpt.OnP2PStateURIReceived(tp.handleP2PStateURIReceivedFromPeer)
	}

	return tp
}

const ProtocolName = "prototree"

func (tp *treeProtocol) Name() string {
	return ProtocolName
}

func (tp *treeProtocol) Start() error {
	err := tp.Process.Start()
	if err != nil {
		return err
	}

	tp.Process.Go(nil, "initial subscribe", func(ctx context.Context) {
		for stateURI := range tp.store.SubscribedStateURIs() {
			err := tp.Subscribe(ctx, string(stateURI))
			if err != nil {
				tp.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
		}
	})

	err = tp.Process.SpawnChild(nil, tp.announceStateURIsTask)
	if err != nil {
		return err
	}
	tp.announceStateURIsTask.Enqueue()

	err = tp.Process.SpawnChild(nil, tp.announceP2PStateURIsTask)
	if err != nil {
		return err
	}
	tp.announceP2PStateURIsTask.Enqueue()

	err = tp.Process.SpawnChild(nil, tp.poolWorker)
	if err != nil {
		return err
	}

	err = tp.Process.SpawnChild(nil, tp.vaultManager)
	if err != nil {
		return err
	}

	go func() {
		time.Sleep(5 * time.Second)
		for messageID, encryptedTx := range tp.store.OpaqueEncryptedTxs() {
			err = tp.hushProto.DecryptGroupMessage(ProtocolName+":tx", messageID, encryptedTx)
			if err != nil {
				tp.Errorf("%v", err)
				continue
			}
		}
	}()

	tp.Process.Go(nil, "listen for encrypted messages from vault", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-tp.vaultManager.EncryptedStateURIs().Notify():
				items := tp.vaultManager.EncryptedStateURIs().RetrieveAll()
				for _, item := range items {
					tp.handleEncryptedStateURIReceivedFromVault(item)
				}

			case <-tp.vaultManager.EncryptedTxs().Notify():
				items := tp.vaultManager.EncryptedTxs().RetrieveAll()
				for _, item := range items {
					tp.handleEncryptedTxReceivedFromVault(item)
				}
			}
		}
	})

	return nil
}

func (tp *treeProtocol) Close() error {
	tp.Infof("tree protocol shutting down")
	return tp.Process.Close()
}

func (tp *treeProtocol) SendTx(ctx context.Context, tx tree.Tx) (err error) {
	tp.Infof("adding tx (%v) %v %v", tx.StateURI, tx.ID.Pretty(), utils.PrettyJSON(tx))

	defer func() {
		if err != nil {
			return
		}
		// If we send a tx to a state URI that we're not subscribed to yet, auto-subscribe.
		if !tp.store.SubscribedStateURIs().Contains(tree.StateURI(tx.StateURI)) {
			err := tp.Subscribe(context.TODO(), tx.StateURI)
			if err != nil {
				tp.Errorf("while subscribing to p2p state URI %v: %v", tx.StateURI, err)
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

	} else {
		exists, err := tp.keyStore.IdentityExists(tx.From)
		if err != nil {
			return errors.Wrapf(err, "while checking key store for address %v", tx.From.Hex())
		} else if !exists {
			return errors.Errorf("address %v is not controlled by this node", tx.From.Hex())
		}
	}

	if len(tx.Parents) == 0 && tx.ID != tree.GenesisTxID {
		var parents []state.Version
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

	switch tp.acl.TypeOf(tx.StateURI) {
	case StateURIType_Invalid:
		return errors.Errorf("invalid state URI: %v", tx.StateURI)

	case StateURIType_Public, StateURIType_Private, StateURIType_DeviceLocal:
		err = tp.controllerHub.AddTx(tx)
		if err != nil {
			return err
		}

	default:
		panic("invariant violation")
	}
	return nil
}

func (tp *treeProtocol) handleTxReceived(tx tree.Tx, peerConn TreePeerConn) {
	tp.Infof("tx received: tx=%v peer=%v", tx.ID.Pretty(), peerConn.DialInfo())

	tp.store.MarkTxSeenByPeer(peerConn.DeviceUniqueID(), tree.StateURI(tx.StateURI), tx.ID)

	exists, err := tp.txStore.TxExists(tx.StateURI, tx.ID)
	if err != nil {
		tp.Errorf("error fetching tx %v from store: %v", tx.ID.Pretty(), err)
		// @@TODO: does it make sense to return here?
		exists = false // Just to be clear
	}

	if !exists {
		// If this is a tx sent by ourselves, but via another node/client, let .SendTx() handle it and return
		myAddrs, err := tp.keyStore.Addresses()
		if err != nil {
			tp.Errorf("while fetching addresses from key store: %v", err)
			return
		}
		if myAddrs.Contains(tx.From) {
			err = tp.SendTx(nil, tx)
			if err != nil {
				tp.Errorf("while sending own tx: %v", err)
			}
			return
		}

		// Otherwise, go ahead and process it as a remote tx
		err = tp.controllerHub.AddTx(tx)
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
	err = peerConn2.(TreePeerConn).Ack(context.TODO(), tx.StateURI, tx.ID)
	if err != nil {
		tp.Errorf("error ACKing peer: %v", err)
	}
}

func (tp *treeProtocol) handleP2PStateURIReceivedFromPeer(stateURI string, peerConn TreePeerConn) {
	if peerConn != nil {
		peerConn.AddStateURI(stateURI) // @@TODO: dos vector
	}

	err := tp.Subscribe(context.TODO(), stateURI)
	if err != nil {
		tp.Errorf("while subscribing to p2p state URI %v: %v", stateURI, err)
	}
}

func (tp *treeProtocol) handleEncryptedStateURIReceivedFromVault(item vaultItem[protohush.GroupMessage]) {
	tp.Infow("private state uri received from vault", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "mtime", item.Mtime)

	hash, err := item.Item.Hash()
	if err != nil {
		tp.Errorw("failed to hash incoming encrypted state uri", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	err = tp.hushProto.DecryptGroupMessage(ProtocolName+":stateuri", hash.Hex(), item.Item)
	if err != nil {
		tp.Errorw("failed to submit private state uri for decryption", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	err = tp.store.MaybeSaveLatestMtimeForVaultAndCollection(item.Host, item.CollectionID, item.Mtime)
	if err != nil {
		tp.Errorw("failed to update latest mtime for vault/collection", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}
}

func (tp *treeProtocol) handlePrivateStateURIDecrypted(messageID string, plaintext []byte, encryptedTx protohush.GroupMessage) {
	tp.Debugf("private state uri decrypted: %v", string(plaintext))
	tp.handleP2PStateURIReceivedFromPeer(string(plaintext), nil)
}

func (tp *treeProtocol) handlePrivateStateURIEncrypted(messageID string, encryptedStateURI protohush.GroupMessage) {
	stateURI, err := parseHushMessageID_EncryptedStateURI(messageID)
	if err != nil {
		tp.Errorf("while parsing hush message id '%v': %v", messageID, err)
		return
	}

	err = tp.store.SaveEncryptedStateURI(stateURI, encryptedStateURI)
	if err != nil {
		tp.Errorw("failed to save encrypted state uri to store", "stateuri", stateURI, "err", err)
		return
	}

	members, err := tp.acl.MembersOf(string(stateURI))
	if err != nil {
		tp.Errorf("while fetching members of state uri %v", stateURI)
		return
	}

	// Send to vaults
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = tp.vaultManager.AddEncryptedStateURIForUsers(ctx, stateURI, members, encryptedStateURI)
	if err != nil {
		tp.Errorf("while adding encrypted state uri to vaults: %v", err)
		return
	}
}

func (tp *treeProtocol) handleEncryptedTxReceivedFromVault(item vaultItem[protohush.GroupMessage]) {
	tp.Infow("private tx received from vault", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "mtime", item.Mtime)

	hash, err := item.Item.Hash()
	if err != nil {
		tp.Errorw("failed to hash incoming encrypted tx", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	_, err = tp.store.OpaqueEncryptedTx(hash.Hex())
	if errors.Cause(err) == errors.Err404 {
		// no-op
	} else if err != nil {
		tp.Errorw("failed to fetch opaque encrypted tx from store", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
	} else {
		tp.Infow("ignoring incoming encrypted tx, already known", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	err = tp.store.SaveOpaqueEncryptedTx(hash.Hex(), item.Item)
	if err != nil {
		tp.Errorw("failed to save incoming encrypted tx", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	err = tp.store.MaybeSaveLatestMtimeForVaultAndCollection(item.Host, item.CollectionID, item.Mtime)
	if err != nil {
		tp.Errorw("failed to save latest mtime for vault/collection", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	err = tp.hushProto.DecryptGroupMessage(ProtocolName+":tx", hash.Hex(), item.Item)
	if err != nil {
		tp.Errorw("failed to submit encrypted tx for decryption", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}

	// @@TODO: ack?
}

func (tp *treeProtocol) handleEncryptedTxReceivedFromPeer(encryptedTx protohush.GroupMessage, peerConn TreePeerConn) {
	tp.Infow("private tx received", "peer", peerConn.DialInfo())

	hash, err := encryptedTx.Hash()
	if err != nil {
		tp.Errorf("while hashing incoming encrypted tx: %v", err)
		return
	}

	_, err = tp.store.OpaqueEncryptedTx(hash.Hex())
	if errors.Cause(err) == errors.Err404 {
		// no-op
	} else if err != nil {
		tp.Errorf("while fetching opaque encrypted tx from store: %v", err)
	} else {
		tp.Infof("ignoring incoming encrypted tx %v: already known", hash.Hex())
		return
	}

	err = tp.store.SaveOpaqueEncryptedTx(hash.Hex(), encryptedTx)
	if err != nil {
		tp.Errorf("while saving incoming encrypted tx: %v", err)
		return
	}

	err = tp.hushProto.DecryptGroupMessage(ProtocolName+":tx", hash.Hex(), encryptedTx)
	if err != nil {
		tp.Errorf("while submitting private tx for decryption: %v", err)
		return
	}

	// @@TODO: ack?
}

func (tp *treeProtocol) handlePrivateTxDecrypted(messageID string, plaintext []byte, encryptedTx protohush.GroupMessage) {
	var tx tree.Tx
	err := tx.Unmarshal(plaintext)
	if err != nil {
		tp.Errorf("while unmarshaling tx protobuf: %v", err)
		return
	}

	tp.Infof("private tx decrypted (stateURI=%v id=%v sender=%v)", tx.StateURI, tx.ID, tx.From)

	err = tp.store.SaveEncryptedTx(tree.StateURI(tx.StateURI), tx.ID, encryptedTx)
	if err != nil {
		tp.Errorf("while saving encrypted tx to database: %v", err)
		return
	}

	err = tp.store.DeleteOpaqueEncryptedTx(messageID)
	if err != nil {
		tp.Errorf("while deleting opaque encrypted tx to database: %v", err)
		return
	}

	err = tp.controllerHub.AddTx(tx)
	if err != nil {
		tp.Errorf("while adding private tx to controller: %v", err)
		return
	}

	// @@TODO: send to Vault
}

func (tp *treeProtocol) handlePrivateTxEncrypted(messageID string, encryptedTx protohush.GroupMessage) {
	stateURI, txID, err := parseHushMessageID_EncryptedTx(messageID)
	if err != nil {
		tp.Errorf("while parsing hush message id '%v': %v", messageID, err)
		return
	}

	tp.Infof("private tx encrypted (stateURI=%v id=%v)", stateURI, txID.Pretty())

	err = tp.store.SaveEncryptedTx(tree.StateURI(stateURI), txID, encryptedTx)
	if err != nil {
		tp.Errorf("while saving encrypted tx to database: %v", err)
		return
	}

	// Send to vaults
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = tp.vaultManager.AddEncryptedTx(ctx, stateURI, txID, encryptedTx)
	if err != nil {
		tp.Errorf("while adding encrypted tx to vaults: %v", err)
		return
	}

	// Broadcast
	tp.poolWorker.Add(broadcastPrivateTx{string(stateURI), txID, tp})
}

func (tp *treeProtocol) handleAckReceived(stateURI string, txID state.Version, peerConn TreePeerConn) {
	tp.Infof("ack received: tx=%v peer=%v", txID.Hex(), peerConn.DialInfo())
	tp.store.MarkTxSeenByPeer(peerConn.DeviceUniqueID(), tree.StateURI(stateURI), txID)
}

type FetchHistoryOpts struct {
	FromTxID state.Version
	ToTxID   state.Version
}

func (tp *treeProtocol) handleFetchHistoryRequest(stateURI string, opts FetchHistoryOpts, writeSub WritableSubscription) error {
	// @@TODO: respect the `opts.ToTxID` param
	// @@TODO: if .FromTxID == 0, set it to GenesisTxID

	allowed, err := tp.acl.HasReadAccess(stateURI, nil, writeSub.Addresses())
	if err != nil {
		return errors.Wrapf(err, "while querying ACL for read access (stateURI=%v)", stateURI)
	} else if !allowed {
		return errors.Err403
	}

	isPrivate := tp.acl.TypeOf(stateURI) == StateURIType_Private

	iter := tp.controllerHub.FetchValidTxsOrdered(stateURI, opts.FromTxID)

	for iter.Rewind(); iter.Valid(); iter.Next() {
		tx := iter.Tx()
		if tx == nil {
			break
		}

		var encryptedTx *EncryptedTx
		if isPrivate {
			encryptedTx2, err := tp.store.EncryptedTx(tree.StateURI(stateURI), tx.ID)
			if err != nil {
				return err
			}
			encryptedTx = &encryptedTx2
		}

		leaves, err := tp.controllerHub.Leaves(stateURI)
		if err != nil {
			return err
		}

		if isPrivate {
			tx = nil
		} else {
			encryptedTx = nil
		}
		msg := SubscriptionMsg{
			StateURI:    stateURI,
			Tx:          tx,
			EncryptedTx: encryptedTx,
			State:       nil,
			Leaves:      leaves,
		}
		writeSub.EnqueueWrite(msg)
	}
	if iter.Err() != nil {
		return iter.Err()
	}
	return nil
}

func (tp *treeProtocol) handleWritableSubscriptionOpened(
	req SubscriptionRequest,
	writeSubImplFactory WritableSubscriptionImplFactory,
) (<-chan struct{}, error) {
	req.Keypath = req.Keypath.Normalized()

	myAddrs, err := tp.keyStore.Addresses()
	if err != nil {
		return nil, errors.Wrapf(err, "while fetching default public identity from keystore")
	}

	if len(myAddrs.Intersection(req.Addresses)) == 0 {
		allowed, err := tp.acl.HasReadAccess(req.StateURI, req.Keypath, req.Addresses)
		if err != nil {
			return nil, errors.Wrapf(err, "while querying ACL for read access (stateURI=%v)", req.StateURI)
		} else if !allowed {
			tp.Debugf("blocked incoming subscription from %v", req.Addresses)
			return nil, errors.Err403
		}
	}

	isPrivate := tp.acl.TypeOf(req.StateURI) == StateURIType_Private

	writeSubImpl, err := writeSubImplFactory()
	if err != nil {
		return nil, err
	}

	writeSub := newWritableSubscription(req.StateURI, req.Keypath, req.Type, isPrivate, req.Addresses, writeSubImpl)
	err = tp.Process.SpawnChild(nil, writeSub)
	if err != nil {
		tp.Errorf("while spawning writable subscription: %v", err)
		return nil, err
	}

	func() {
		tp.writableSubscriptionsMu.Lock()
		defer tp.writableSubscriptionsMu.Unlock()

		if _, exists := tp.writableSubscriptions[req.StateURI]; !exists {
			tp.writableSubscriptions[req.StateURI] = make(map[WritableSubscription]struct{})
		}
		tp.writableSubscriptions[req.StateURI][writeSub] = struct{}{}
	}()

	tp.Process.Go(nil, "await close "+writeSub.String(), func(ctx context.Context) {
		select {
		case <-writeSub.Done():
		case <-ctx.Done():
		}
		tp.handleWritableSubscriptionClosed(writeSub)
	})

	if req.Type.Includes(SubscriptionType_Txs) && req.FetchHistoryOpts != nil {
		tp.handleFetchHistoryRequest(req.StateURI, *req.FetchHistoryOpts, writeSub)
	}

	if req.Type.Includes(SubscriptionType_States) {
		// Normalize empty keypaths
		if req.Keypath.Equals(state.KeypathSeparator) {
			req.Keypath = nil
		}

		// Immediately write the current state to the subscriber
		node, err := tp.controllerHub.StateAtVersion(req.StateURI, nil)
		if err != nil && errors.Cause(err) != errors.Err404 {
			tp.Errorf("error writing initial state to peer: %v", err)
			writeSub.Close()
			return nil, err

		} else if err == nil {
			defer node.Close()

			leaves, err := tp.controllerHub.Leaves(req.StateURI)
			if err != nil {
				tp.Errorf("error writing initial state to peer (%v): %v", req.StateURI, err)
			} else {
				node, err := node.CopyToMemory(req.Keypath, nil)
				if err != nil && errors.Cause(err) == errors.Err404 {
					// no-op
				} else if err != nil {
					tp.Errorf("error writing initial state to peer (%v): %v", req.StateURI, err)
				} else {
					writeSub.EnqueueWrite(SubscriptionMsg{
						StateURI:    req.StateURI,
						Tx:          nil,
						EncryptedTx: nil,
						State:       node,
						Leaves:      leaves,
					})
				}
			}
		}
	}
	return writeSub.Done(), nil
}

func (tp *treeProtocol) handleWritableSubscriptionClosed(sub WritableSubscription) {
	tp.writableSubscriptionsMu.Lock()
	defer tp.writableSubscriptionsMu.Unlock()
	delete(tp.writableSubscriptions[sub.StateURI()], sub)
}

func (tp *treeProtocol) openReadableSubscription(stateURI string) {
	tp.readableSubscriptionsMu.Lock()
	defer tp.readableSubscriptionsMu.Unlock()

	if _, exists := tp.readableSubscriptions[stateURI]; !exists {
		tp.Debugf("opening subscription to %v", stateURI)
		multiSub := newMultiReaderSubscription(
			stateURI,
			tp.store.MaxPeersPerSubscription(),
			func(msg SubscriptionMsg, peerConn TreePeerConn) {
				if msg.EncryptedTx != nil {
					tp.handleEncryptedTxReceivedFromPeer(*msg.EncryptedTx, peerConn)
				} else if msg.Tx != nil {
					tp.handleTxReceived(*msg.Tx, peerConn)
				} else {
					panic("wat")
				}
			},
			tp.ProvidersOfStateURI,
		)
		tp.Process.SpawnChild(nil, multiSub)
		tp.readableSubscriptions[stateURI] = multiSub
	}
}

func (tp *treeProtocol) Subscribe(ctx context.Context, stateURI string) error {
	if _, exists := tp.readableSubscriptions[stateURI]; exists {
		return nil
	}

	treeType := tp.acl.TypeOf(stateURI)
	if treeType == StateURIType_Invalid {
		return errors.Errorf("invalid state URI: %v", stateURI)
	}

	err := tp.store.AddSubscribedStateURI(tree.StateURI(stateURI))
	if err != nil {
		return errors.Wrap(err, "while updating config store")
	}

	switch treeType {
	case StateURIType_Invalid:
		return errors.Errorf("invalid state URI: %v", stateURI)
	case StateURIType_DeviceLocal:
		return nil
	case StateURIType_Private:
		tp.openReadableSubscription(stateURI)
	case StateURIType_Public:
		tp.openReadableSubscription(stateURI)
	}
	return nil
}

func (tp *treeProtocol) InProcessSubscription(
	ctx context.Context,
	stateURI string,
	subscriptionType SubscriptionType,
	keypath state.Keypath,
	fetchHistoryOpts *FetchHistoryOpts,
) (ReadableSubscription, error) {
	err := tp.Subscribe(ctx, stateURI)
	if err != nil {
		return nil, err
	}

	keypath = keypath.Normalized()

	// Open the subscription with the node's own credentials
	myAddrs, err := tp.keyStore.Addresses()
	if err != nil {
		return nil, err
	}

	sub := newInProcessSubscription(stateURI, keypath, subscriptionType, tp)
	req := SubscriptionRequest{
		StateURI:         stateURI,
		Keypath:          keypath,
		Type:             subscriptionType,
		FetchHistoryOpts: fetchHistoryOpts,
		Addresses:        myAddrs,
	}
	_, err = tp.handleWritableSubscriptionOpened(req, func() (WritableSubscriptionImpl, error) {
		return sub, nil
	})
	if err != nil {
		return nil, err
	}
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

	err := tp.store.RemoveSubscribedStateURI(tree.StateURI(stateURI))
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

// Returns peers discovered through any transport who claim to provide the
// stateURI in question.
func (tp *treeProtocol) ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan TreePeerConn {
	var (
		ch          = make(chan TreePeerConn)
		alreadySent = NewSyncSet[swarm.PeerDialInfo](nil)
	)

	switch tp.acl.TypeOf(stateURI) {
	case StateURIType_Invalid, StateURIType_DeviceLocal:
		close(ch)
		return ch

	case StateURIType_Private:
		tp.Process.Go(ctx, "ProvidersOfStateURI "+stateURI, func(ctx context.Context) {
			defer close(ch)
			defer time.Sleep(10 * time.Second)

			members, err := tp.acl.MembersOf(stateURI)
			if err != nil {
				tp.Errorf("while fetching members of state URI '%v': %v", stateURI, err)
				return
			}

			peerInfos := tp.peerStore.PeersServingStateURI(stateURI)

			for addr := range members {
				peerInfos = append(peerInfos, tp.peerStore.PeersWithAddress(addr)...)
			}

			peerConns := tp.PeerInfosToPeerConns(ctx, peerInfos)
			for _, peerConn := range peerConns {
				if exists := alreadySent.Add(peerConn.DialInfo()); exists {
					continue
				}
				peerConn.AddStateURI(stateURI)

				select {
				case <-ctx.Done():
					return
				case ch <- peerConn:
				}
			}
		})

	case StateURIType_Public:
		child := tp.Process.NewChild(ctx, "ProvidersOfStateURI "+stateURI)
		defer child.AutocloseWithCleanup(func() {
			close(ch)
		})

		child.Go(nil, "from PeerStore", func(ctx context.Context) {
			peerConns := tp.PeerInfosToPeerConns(ctx, tp.peerStore.PeersServingStateURI(stateURI))
			for _, peerConn := range peerConns {
				if exists := alreadySent.Add(peerConn.DialInfo()); exists {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case ch <- peerConn:
				}
			}
		})

		for _, tpt := range tp.Transports {
			tpt := tpt

			child.Go(nil, tpt.Name(), func(ctx context.Context) {
				ok := <-tpt.AwaitReady(ctx)
				if !ok {
					return
				}

				innerCh, err := tpt.ProvidersOfStateURI(ctx, stateURI)
				if err != nil {
					// tp.Warnf("error fetching providers of State-URI %v on transport %v: %v", stateURI, tpt.Name(), err)
					return
				}

				for {
					select {
					case <-ctx.Done():
						return
					case peer, open := <-innerCh:
						if !open {
							return
						}
						peer.AddStateURI(stateURI)

						if exists := alreadySent.Add(peer.DialInfo()); exists {
							continue
						}

						select {
						case <-ctx.Done():
							return
						case ch <- peer:
						}
					}
				}
			})
		}
	}

	return ch
}

func (tp *treeProtocol) handleNewState(tx tree.Tx, node state.Node, leaves []state.Version, diff *state.Diff) {
	switch tp.acl.TypeOf(tx.StateURI) {
	case StateURIType_Invalid:
		panic("invariant violation")

	case StateURIType_Private:
		if tp.acl.MembersAdded(diff).Length() > 0 || tp.acl.MembersRemoved(diff).Length() > 0 {
			err := tp.handlePrivateStateURIMembersChanged(tx)
			if err != nil {
				tp.Errorf("while handling member change for state URI %v: %v", tx.StateURI, err)
			}
		}

		tp.announceP2PStateURIsTask.Enqueue()

		// If this is the genesis tx of a private state URI, ensure that we subscribe to that state URI
		// @@TODO: allow blacklisting of senders
		if tx.ID == tree.GenesisTxID { //&& !tp.store.SubscribedStateURIs().Contains(tx.StateURI) {
			tp.Process.Go(nil, "auto-subscribe", func(ctx context.Context) {
				ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				err := tp.Subscribe(ctx, tx.StateURI)
				if err != nil {
					tp.Errorf("error subscribing to state URI %v: %v", tx.StateURI, err)
				}
			})
		}

		myAddrs, err := tp.keyStore.Addresses()
		if err != nil {
			tp.Errorf("while fetching own addresses from keystore: %v", err)
			return
		}

		if myAddrs.Contains(tx.From) {
			tp.poolWorker.Add(encryptOwnPrivateTx{tx.StateURI, tx.ID, tp})

		} else {
			members, err := tp.acl.MembersOf(tx.StateURI)
			if err != nil {
				tp.Errorf("while fetching members of state URI %v: %v", tx.StateURI, err)
				return
			}

			var peerDevices []swarm.PeerDevice
			for peerAddress := range members {
				peerDevices = append(peerDevices, tp.peerStore.PeersWithAddress(peerAddress)...)
			}
			tp.Process.Go(nil, "ack "+tx.StateURI+" "+tx.ID.Hex(), func(ctx context.Context) {
				tp.TryPeerDevices(ctx, &tp.Process, peerDevices, func(ctx context.Context, treePeerConn TreePeerConn) error {
					return treePeerConn.Ack(ctx, tx.StateURI, tx.ID)
				})
			})

			tp.poolWorker.Add(broadcastPrivateTx{tx.StateURI, tx.ID, tp})
		}

	case StateURIType_Public, StateURIType_DeviceLocal: // @@TODO: handle "device local" separately
		node, err := node.CopyToMemory(nil, nil)
		if err != nil {
			tp.Errorf("handleNewState: couldn't copy state to memory: %v", err)
			node = state.NewMemoryNode() // give subscribers an empty state
		}
		tp.broadcastToWritableSubscribers(context.TODO(), tx.StateURI, &tx, nil, node, leaves)
	}
}

func (tp *treeProtocol) handlePrivateStateURIMembersChanged(inTx tree.Tx) error {
	sender, err := tp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return err
	}
	members, err := tp.acl.MembersOf(inTx.StateURI)
	if err != nil {
		return err
	}

	tp.Debugw("state uri members changed", "stateuri", inTx.StateURI, "members", members.Slice())

	// Send encrypted state URI to members
	err = tp.hushProto.EncryptGroupMessage(ProtocolName+":stateuri", makeHushMessageID_EncryptedStateURI(tree.StateURI(inTx.StateURI)), sender.Address(), members.Slice(), []byte(inTx.StateURI))
	if err != nil {
		return err
	}

	// Send encrypted txs to members
	iter, err := tp.txStore.AllTxsForStateURI(inTx.StateURI, tree.GenesisTxID)
	if err != nil {
		return err
	}
	for iter.Rewind(); iter.Valid(); iter.Next() {
		tx := iter.Tx()
		if tx.ID == inTx.ID {
			continue
		}

		plaintext, err := tx.Marshal()
		if err != nil {
			return err
		}

		err = tp.hushProto.EncryptGroupMessage(ProtocolName+":tx", makeHushMessageID_EncryptedTx(tree.StateURI(tx.StateURI), tx.ID), sender.Address(), members.Slice(), plaintext)
		if err != nil {
			return err
		}
	}
	return iter.Err()
}

func (tp *treeProtocol) broadcastToWritableSubscribers(
	ctx context.Context,
	stateURI string,
	tx *tree.Tx,
	encryptedTx *EncryptedTx,
	state state.Node,
	leaves []state.Version,
) {
	tp.writableSubscriptionsMu.RLock()
	defer tp.writableSubscriptionsMu.RUnlock()

	isPrivate := tp.acl.TypeOf(stateURI) == StateURIType_Private

	for writeSub := range tp.writableSubscriptions[stateURI] {
		allowed, err := tp.acl.HasReadAccess(stateURI, nil, writeSub.Addresses())
		if err != nil {
			tp.Errorf("while checking ACL of state URI %v", stateURI)
			continue
		} else if !allowed {
			// @@TODO: close subscription
			continue
		}

		if peer, isPeer := writeSub.(TreePeerConn); isPeer {
			// If the subscriber wants us to send states, we never skip sending
			if tp.store.TxSeenByPeer(peer.DeviceUniqueID(), tree.StateURI(stateURI), tx.ID) && !writeSub.Type().Includes(SubscriptionType_States) {
				continue
			}
		}

		if state != nil {
			// Drill down to the part of the state that the subscriber is interested in
			state = state.NodeAt(writeSub.Keypath(), nil)
		}

		if isPrivate {
			tx = nil
		} else {
			encryptedTx = nil
		}
		writeSub.EnqueueWrite(SubscriptionMsg{
			StateURI:    stateURI,
			Tx:          tx,
			EncryptedTx: encryptedTx,
			State:       state,
			Leaves:      leaves,
		})
	}
}

func (tp *treeProtocol) Vaults() Set[string] {
	return tp.vaultManager.Vaults()
}

func (tp *treeProtocol) AddVault(host string) error {
	return tp.vaultManager.AddVault(host)
}

func (tp *treeProtocol) RemoveVault(host string) error {
	return tp.vaultManager.RemoveVault(host)
}

func (tp *treeProtocol) SyncWithAllVaults() error {
	return tp.vaultManager.SyncWithAllVaults(context.TODO())
}

type announceStateURIsTask struct {
	process.PeriodicTask
	log.Logger
	treeProto *treeProtocol
	interval  time.Duration
}

func NewAnnounceStateURIsTask(
	interval time.Duration,
	treeProto *treeProtocol,
) *announceStateURIsTask {
	t := &announceStateURIsTask{
		Logger:    log.NewLogger(ProtocolName),
		treeProto: treeProto,
		interval:  interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceStateURIsTask", utils.NewStaticTicker(interval), t.announceStateURIs)
	return t
}

func (t *announceStateURIsTask) announceStateURIs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announceStateURIs")
	defer child.AutocloseWithCleanup(cancel)

	stateURIs, err := t.treeProto.txStore.StateURIsWithData()
	if err != nil {
		t.Errorf("while fetching state URIs from tx store: %v", err)
		return
	}

	publicStateURIs := NewSet[string](nil)
	for stateURI := range stateURIs {
		if t.treeProto.acl.TypeOf(stateURI) != StateURIType_Public {
			continue
		}
		publicStateURIs.Add(stateURI)
	}

	for _, tpt := range t.treeProto.Transports {
		ok := <-tpt.AwaitReady(ctx)
		if !ok {
			continue
		}

		tpt.AnnounceStateURIs(ctx, publicStateURIs)
	}
}

type announceP2PStateURIsTask struct {
	process.PeriodicTask
	log.Logger
	treeProto *treeProtocol
	interval  time.Duration
}

func NewAnnounceP2PStateURIsTask(
	interval time.Duration,
	treeProto *treeProtocol,
) *announceP2PStateURIsTask {
	t := &announceP2PStateURIsTask{
		Logger:    log.NewLogger(ProtocolName),
		treeProto: treeProto,
		interval:  interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceP2PStateURIsTask", utils.NewStaticTicker(interval), t.announceP2PStateURIs)
	return t
}

func (t *announceP2PStateURIsTask) announceP2PStateURIs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announceP2PStateURIs")
	defer child.AutocloseWithCleanup(cancel)

	stateURIs, err := t.treeProto.txStore.StateURIsWithData()
	if err != nil {
		t.Errorf("while fetching state URIs from tx store: %v", err)
		return
	}

	for stateURI := range stateURIs {
		stateURI := stateURI

		if t.treeProto.acl.TypeOf(stateURI) != StateURIType_Private {
			continue
		}

		members, err := t.treeProto.acl.MembersOf(stateURI)
		if err != nil {
			t.Errorf("while fetching members of state URI %v: %v", stateURI, err)
			continue
		}

		var peerDevices []swarm.PeerDevice
		for peerAddress := range members {
			peerDevices = append(peerDevices, t.treeProto.peerStore.PeersWithAddress(peerAddress)...)
		}
		t.treeProto.TryPeerDevices(ctx, child, peerDevices, func(ctx context.Context, treePeerConn TreePeerConn) error {
			return treePeerConn.AnnounceP2PStateURI(ctx, stateURI)
		})
	}
}

type encryptOwnPrivateTx struct {
	stateURI  string
	txID      state.Version
	treeProto *treeProtocol
}

var _ process.PoolWorkerItem = encryptOwnPrivateTx{}

func (t encryptOwnPrivateTx) ID() process.PoolUniqueID { return t }

func (t encryptOwnPrivateTx) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.treeProto.Warnf("encrypt private tx %v %v: retrying later", t.stateURI, t.txID)
		} else {
			t.treeProto.Successf("encrypt private tx %v %v: done", t.stateURI, t.txID)
		}
	}()

	// _, err := t.treeProto.store.EncryptedTx(t.stateURI, t.txID)
	// if err == nil {
	// 	// Done
	// 	return false
	// }

	tx, err := t.treeProto.txStore.FetchTx(t.stateURI, t.txID)
	if errors.Cause(err) == errors.Err404 {
		t.treeProto.Warnf("encrypt private tx: retrying later (tx not found)")
		return true
	} else if err != nil {
		t.treeProto.Errorf("while fetching private tx %v %v: %v", t.stateURI, t.txID, err)
		return false
	}
	members, err := t.treeProto.acl.MembersOf(t.stateURI)
	if err != nil {
		t.treeProto.Errorf("while fetching members of %v: %v", t.stateURI, err)
		return true
	} else if len(members) == 0 && tx.Status != tree.TxStatusInvalid {
		t.treeProto.Warnf("while fetching members of %v: awaiting controller", t.stateURI)
		return true
	} else if tx.Status == tree.TxStatusInvalid {
		return false
	}
	txBytes, err := tx.Marshal()
	if err != nil {
		t.treeProto.Errorf("while marshaling tx %v %v: %v", t.stateURI, t.txID, err)
		return true
	}

	sender, err := t.treeProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.treeProto.Errorf("while fetching default identity from key store: %v", err)
		return
	}

	err = t.treeProto.hushProto.EncryptGroupMessage(ProtocolName+":tx", makeHushMessageID_EncryptedTx(tree.StateURI(tx.StateURI), tx.ID), sender.Address(), members.Slice(), txBytes)
	if err != nil {
		t.treeProto.Errorf("while enqueuing hush tx %v %v: %v", t.stateURI, t.txID, err)
		return true
	}
	return false
}

type broadcastPrivateTx struct {
	stateURI  string
	txID      state.Version
	treeProto *treeProtocol
}

var _ process.PoolWorkerItem = broadcastPrivateTx{}

func (t broadcastPrivateTx) ID() process.PoolUniqueID { return t }

func (t broadcastPrivateTx) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.treeProto.Warnf("broadcast private tx %v %v: retrying later", t.stateURI, t.txID.Pretty())
		} else {
			t.treeProto.Successf("broadcast private tx %v %v: done", t.stateURI, t.txID.Pretty())
		}
	}()

	tx, err := t.treeProto.txStore.FetchTx(t.stateURI, t.txID)
	if errors.Cause(err) == errors.Err404 {
		t.treeProto.Warnf("broadcast private tx %v %v: retrying later (tx not found)", t.stateURI, t.txID.Pretty())
		return true
	} else if err != nil {
		t.treeProto.Errorf("while fetching private tx %v %v: %v", t.stateURI, t.txID.Pretty(), err)
		return false
	}

	encryptedTx, err := t.treeProto.store.EncryptedTx(tree.StateURI(t.stateURI), t.txID)
	if errors.Cause(err) == errors.Err404 {
		t.treeProto.Warnf("broadcast private tx %v %v: retrying later (encrypted tx not found)", t.stateURI, t.txID.Pretty())
		return true
	} else if err != nil {
		t.treeProto.Errorf("while fetching private tx %v %v: %v", t.stateURI, t.txID.Pretty(), err)
		return false
	}

	leaves, err := t.treeProto.controllerHub.Leaves(t.stateURI)
	if err != nil {
		t.treeProto.Errorf("while fetching leaves of %v: %v", t.stateURI, err)
		return true
	}

	node, err := t.treeProto.controllerHub.StateAtVersion(t.stateURI, nil)
	if err != nil {
		t.treeProto.Errorf("while fetching state of %v: %v", t.stateURI, err)
		return true
	}
	defer node.Close()

	node, err = node.CopyToMemory(nil, nil)
	if err != nil {
		t.treeProto.Errorf("broadcastPrivateTx: couldn't copy state to memory: %v", err)
		node = state.NewMemoryNode() // give subscribers an empty state
	}

	t.treeProto.broadcastToWritableSubscribers(ctx, t.stateURI, &tx, &encryptedTx, node, leaves)
	return false
}

func makeHushMessageID_EncryptedStateURI(stateURI tree.StateURI) string {
	return url.QueryEscape(string(stateURI))
}

func parseHushMessageID_EncryptedStateURI(messageID string) (tree.StateURI, error) {
	stateURI, err := url.QueryUnescape(messageID)
	if err != nil {
		return "", err
	}
	return tree.StateURI(stateURI), nil
}

func makeHushMessageID_EncryptedTx(stateURI tree.StateURI, txID state.Version) string {
	return url.QueryEscape(string(stateURI)) + ":" + txID.Hex()
}

func parseHushMessageID_EncryptedTx(messageID string) (tree.StateURI, state.Version, error) {
	parts := strings.Split(messageID, ":")
	if len(parts) != 2 {
		return "", state.Version{}, errors.Errorf("expected 2 parts in message id, got %v", len(parts))
	}
	stateURI, err := url.QueryUnescape(parts[0])
	if err != nil {
		return "", state.Version{}, err
	}
	version, err := state.VersionFromHex(parts[1])
	if err != nil {
		return "", state.Version{}, err
	}
	return tree.StateURI(stateURI), version, nil
}
