package protohush

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/status-im/doubleratchet"
	"github.com/status-im/status-go/protocol/encryption"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name HushProtocol --output ./mocks/ --case=underscore

//go:generate mockery --name HushProtocol --output ./mocks/ --case=underscore
type HushProtocol interface {
	process.Interface

	EncryptGroupMessage(messageType, messageID string, sender types.Address, recipients []types.Address, plaintext []byte) error
	ReencryptGroupMessage(messageType, messageID string, sender types.Address, newRecipients []types.Address) error
	DecryptGroupMessage(messageType, messageID string, msg GroupMessage) error
	OnGroupMessageEncrypted(messageType string, handler GroupMessageEncryptedCallback)
	OnGroupMessageReencrypted(messageType string, handler GroupMessageReencryptedCallback)
	OnGroupMessageDecrypted(messageType string, handler GroupMessageDecryptedCallback)

	Vaults() Set[string]
	AddVault(host string) error
	RemoveVault(host string) error
	SyncWithAllVaults() error

	PubkeyBundleAddresses() ([]types.Address, error)
}

//go:generate mockery --name HushTransport --output ./mocks/ --case=underscore
type HushTransport interface {
	swarm.Transport
	OnIncomingPubkeyBundles(handler IncomingPubkeyBundlesCallback)
}

//go:generate mockery --name HushPeerConn --output ./mocks/ --case=underscore
type HushPeerConn interface {
	swarm.PeerConn
	SendPubkeyBundles(ctx context.Context, bundles []PubkeyBundle) error
}

type hushProtocol struct {
	swarm.BaseProtocol[HushTransport, HushPeerConn]

	store        Store
	keyStore     identity.KeyStore
	peerStore    swarm.PeerStore
	vaultManager *VaultManager

	groupMessageEncryptedListeners     map[string][]GroupMessageEncryptedCallback
	groupMessageEncryptedListenersMu   sync.RWMutex
	groupMessageReencryptedListeners   map[string][]GroupMessageReencryptedCallback
	groupMessageReencryptedListenersMu sync.RWMutex
	groupMessageDecryptedListeners     map[string][]GroupMessageDecryptedCallback
	groupMessageDecryptedListenersMu   sync.RWMutex

	poolWorker                       process.PoolWorker
	exchangePubkeyBundlesTask        *exchangePubkeyBundlesTask
	decryptIncomingGroupMessagesTask *decryptIncomingGroupMessagesTask
}

const ProtocolName = "protohush"

type GroupMessageEncryptedCallback func(id string, msg GroupMessage)
type GroupMessageReencryptedCallback func(id string, msg GroupMessage)
type GroupMessageDecryptedCallback func(id string, plaintext []byte, msg GroupMessage)

func NewHushProtocol(
	transports []swarm.Transport,
	store Store,
	keyStore identity.KeyStore,
	peerStore swarm.PeerStore,
) *hushProtocol {
	transportsMap := make(map[string]HushTransport)
	for _, tpt := range transports {
		if tpt, is := tpt.(HushTransport); is {
			transportsMap[tpt.Name()] = tpt
		}
	}
	hp := &hushProtocol{
		BaseProtocol: swarm.BaseProtocol[HushTransport, HushPeerConn]{
			Process:    *process.New(ProtocolName),
			Logger:     log.NewLogger(ProtocolName),
			Transports: transportsMap,
		},
		store:                            store,
		keyStore:                         keyStore,
		peerStore:                        peerStore,
		vaultManager:                     NewVaultManager(store, keyStore, 1*time.Minute),
		groupMessageEncryptedListeners:   make(map[string][]GroupMessageEncryptedCallback),
		groupMessageReencryptedListeners: make(map[string][]GroupMessageReencryptedCallback),
		groupMessageDecryptedListeners:   make(map[string][]GroupMessageDecryptedCallback),
	}

	hp.poolWorker = process.NewPoolWorker("pool worker", 8, process.NewStaticScheduler(3*time.Second, 3*time.Second))
	hp.decryptIncomingGroupMessagesTask = NewDecryptIncomingGroupMessagesTask(10*time.Second, hp)
	hp.exchangePubkeyBundlesTask = NewExchangePubkeysBundleTask(10*time.Second, hp)

	for _, tpt := range hp.Transports {
		hp.Infof("registering %v", tpt.Name())
		tpt.OnIncomingPubkeyBundles(hp.handleIncomingPubkeyBundles)
	}

	hp.peerStore.OnNewVerifiedPeer(func(peer swarm.PeerDevice) {
		hp.poolWorker.Add(exchangePubkeyBundles{peer.DeviceUniqueID(), hp})
	})

	return hp
}

func (hp *hushProtocol) Start() error {
	err := hp.Process.Start()
	if err != nil {
		return err
	}

	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return err
	}
	err = hp.ensureKeyBundlesAndSelfSession(identity)
	if err != nil {
		return err
	}

	// Start our workers
	err = hp.Process.SpawnChild(nil, hp.poolWorker)
	if err != nil {
		return err
	}
	err = hp.Process.SpawnChild(nil, hp.exchangePubkeyBundlesTask)
	if err != nil {
		return err
	}
	err = hp.Process.SpawnChild(nil, hp.decryptIncomingGroupMessagesTask)
	if err != nil {
		return err
	}
	err = hp.Process.SpawnChild(nil, hp.vaultManager)
	if err != nil {
		return err
	}

	hp.Process.Go(nil, "listen for pubkey bundles from vault", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hp.vaultManager.PubkeyBundles().Notify():
				items := hp.vaultManager.PubkeyBundles().RetrieveAll()
				for _, item := range items {
					hp.handleIncomingPubkeyBundleFromVault(item)
				}
			}
		}
	})

	// Add unsent outgoing group messages to the pool worker queue
	for _, id := range hp.store.OutgoingGroupMessageIDs() {
		hp.poolWorker.Add(encryptGroupMessage{id, hp})
	}

	return nil
}

func (hp *hushProtocol) Close() error {
	hp.Infof("hush protocol shutting down")
	return hp.Process.Close()
}

func (hp *hushProtocol) ensureKeyBundlesAndSelfSession(identity identity.Identity) error {
	keyBundles, err := hp.store.KeyBundles()
	if err != nil {
		return err
	}

	var keyBundle KeyBundle
	var pubkeyBundle PubkeyBundle

	keyBundles = Filter(keyBundles, func(bundle KeyBundle) bool { return !bundle.Revoked })
	if len(keyBundles) == 0 {
		keyBundle, _, err = hp.store.CreateKeyBundle(identity)
		if err != nil {
			return err
		}
		_, pubkeyBundle, err = hp.store.CreateKeyBundle(identity)
		if err != nil {
			return err
		}
	} else if len(keyBundles) == 1 {
		keyBundle = keyBundles[0]

		_, pubkeyBundle, err = hp.store.CreateKeyBundle(identity)
		if err != nil {
			return err
		}
	} else {
		keyBundle = keyBundles[0]
		pubkeyBundle, err = keyBundles[1].ToPubkeyBundle(identity)
		if err != nil {
			return err
		}
	}

	sharedKey, ephemeralPubkey, err := encryption.PerformActiveX3DH(
		gethcrypto.CompressPubkey((*ecdsa.PublicKey)(pubkeyBundle.IdentityPubkey)),
		gethcrypto.CompressPubkey((*ecdsa.PublicKey)(pubkeyBundle.SignedPreKeyPubkey)),
		(*ecdsa.PrivateKey)(keyBundle.IdentityKey),
	)

	_, _, err = hp.store.StartOutgoingSession(keyBundle, pubkeyBundle, (*X3DHPublicKey)(ephemeralPubkey), sharedKey)
	if err != nil {
		return err
	}
	return nil
}

func (hp *hushProtocol) EncryptGroupMessage(messageType, messageID string, sender types.Address, recipients []types.Address, plaintext []byte) error {
	intent := OutgoingGroupMessage{
		MessageType: messageType,
		ID:          messageID,
		Sender:      sender,
		Recipients:  recipients,
		Plaintext:   plaintext,
	}
	err := hp.store.SaveOutgoingGroupMessage(intent)
	if err != nil {
		return errors.Wrapf(err, "while saving group message intent to database")
	}
	hp.poolWorker.Add(encryptGroupMessage{intent.ID, hp})
	return nil
}

func (hp *hushProtocol) ReencryptGroupMessage(messageType, messageID string, sender types.Address, newRecipients []types.Address) error {
	symEncKey, symEncMsg, err := hp.store.SymEncKeyAndMessage(messageType, messageID)
	if err != nil {
		return err
	}

	msg, _, err := hp.assembleGroupMessage(sender, newRecipients, symEncKey, symEncMsg)
	if err != nil {
		return err
	}

	hp.notifyGroupMessageReencryptedListeners(messageType, messageID, msg)
	return nil
}

func (hp *hushProtocol) DecryptGroupMessage(messageType, messageID string, msg GroupMessage) error {
	err := hp.store.SaveIncomingGroupMessage(IncomingGroupMessage{MessageType: messageType, ID: messageID, Message: msg})
	if err != nil {
		return errors.Wrapf(err, "while saving incoming group message")
	}
	hp.decryptIncomingGroupMessagesTask.Enqueue()
	return nil
}

func (hp *hushProtocol) Vaults() Set[string] {
	return hp.vaultManager.Vaults()
}

func (hp *hushProtocol) AddVault(host string) error {
	return hp.vaultManager.AddVault(host)
}

func (hp *hushProtocol) RemoveVault(host string) error {
	return hp.vaultManager.RemoveVault(host)
}

func (hp *hushProtocol) SyncWithAllVaults() error {
	return hp.vaultManager.SyncWithAllVaults(context.TODO())
}

func (hp *hushProtocol) PubkeyBundleAddresses() ([]types.Address, error) {
	return hp.store.PubkeyBundleAddresses()
}

func (hp *hushProtocol) assembleGroupMessage(sender types.Address, recipients []types.Address, symEncKey crypto.SymEncKey, symEncMsg crypto.SymEncMsg) (_ GroupMessage, retry bool, _ error) {
	identity, err := hp.keyStore.IdentityWithAddress(sender)
	if err != nil {
		return GroupMessage{}, true, errors.Wrap(err, "while loading identity")
	}

	var encryptionKeys []IndividualMessage
	for _, recipient := range recipients {
		// Ensure we have sessions with each user
		session, drsession, err := hp.store.LatestValidSessionWithUsers(identity.Address(), recipient)
		if errors.Cause(err) == errors.Err404 {
			hp.Debugf("no session for users %v %v", identity.Address(), recipient)
			hp.poolWorker.Add(ensureSessionOutgoing{myAddress: identity.Address(), remoteAddress: recipient, hushProto: hp})
			retry = true

		} else if err != nil {
			hp.Errorf("while fetching individual session: %+v", err)
			retry = true
		}
		if retry {
			continue
		}

		// Encrypt the encryption key for this recipient
		msg, err := drsession.RatchetEncrypt(symEncKey.Bytes(), nil)
		if err != nil {
			hp.Errorf("while encrypting outgoing group message (session=%v): %v", session.ID(), err)
			retry = true
			continue
		}
		encryptionKeys = append(encryptionKeys, IndividualMessage{
			SenderBundleID:    session.MyBundleID,
			RecipientBundleID: session.RemoteBundleID,
			EphemeralPubkey:   session.EphemeralPubkey,
			RatchetPubkey:     RatchetPublicKeyFromBytes([]byte(msg.Header.DH)),
			N:                 msg.Header.N,
			Pn:                msg.Header.PN,
			Ciphertext:        msg.Ciphertext,
		})
	}
	if retry {
		return
	}

	msg := GroupMessage{
		EncryptionKeys: encryptionKeys,
		Ciphertext:     symEncMsg.Bytes(),
	}
	// sig, err := identity.SignHash(types.HashBytes(intent.Plaintext))
	// if err != nil {
	//  t.hushProto.Errorf("while signing hash of outgoing group message: %v", err)
	//  return true
	// }
	// msg.Sig = sig
	return msg, retry, nil
}

func (hp *hushProtocol) handleIncomingPubkeyBundles(bundles []PubkeyBundle) {
	var valid []PubkeyBundle
	addrs := NewSet[types.Address](nil)
	for _, bundle := range bundles {
		addr, err := bundle.Address()
		if err != nil {
			hp.Errorf("while verifying pubkey bundle signature: %v", err)
			continue
		}
		addrs.Add(addr)
		valid = append(valid, bundle)
	}

	if len(valid) == 0 {
		return
	}

	err := hp.store.SavePubkeyBundles(valid)
	if err != nil {
		hp.Errorf("while saving DH pubkey bundles: %v", err)
		return
	}

	duIDs := NewSet(Map(hp.peerStore.VerifiedPeers(), func(pd swarm.PeerDevice) string { return pd.DeviceUniqueID() }))
	for duID := range duIDs {
		hp.poolWorker.Add(exchangePubkeyBundles{duID, hp})
	}

	for addr := range addrs {
		hp.resumeTasksForPeer(addr)
	}
}

func (hp *hushProtocol) handleIncomingPubkeyBundleFromVault(item vaultItem[PubkeyBundle]) {
	hp.Infow("pubkey bundle received from vault", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "mtime", item.Mtime)

	hp.handleIncomingPubkeyBundles([]PubkeyBundle{item.Item})

	err := hp.store.MaybeSaveLatestMtimeForVaultAndCollection(item.Host, item.CollectionID, item.Mtime)
	if err != nil {
		hp.Errorw("failed to save latest mtime for vault/collection", "host", item.Host, "collection", item.CollectionID, "item", item.ItemID, "err", err)
		return
	}
}

func (hp *hushProtocol) OnGroupMessageEncrypted(messageType string, handler GroupMessageEncryptedCallback) {
	hp.groupMessageEncryptedListenersMu.Lock()
	defer hp.groupMessageEncryptedListenersMu.Unlock()
	hp.groupMessageEncryptedListeners[messageType] = append(hp.groupMessageEncryptedListeners[messageType], handler)
}

func (hp *hushProtocol) notifyGroupMessageEncryptedListeners(messageType, messageID string, msg GroupMessage) {
	hp.groupMessageEncryptedListenersMu.RLock()
	defer hp.groupMessageEncryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify group message encrypted listeners")
	for _, listener := range hp.groupMessageEncryptedListeners[messageType] {
		listener := listener
		child.Go(context.TODO(), "notify group message encrypted listener", func(ctx context.Context) {
			listener(messageID, msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (hp *hushProtocol) OnGroupMessageReencrypted(messageType string, handler GroupMessageReencryptedCallback) {
	hp.groupMessageReencryptedListenersMu.Lock()
	defer hp.groupMessageReencryptedListenersMu.Unlock()
	hp.groupMessageReencryptedListeners[messageType] = append(hp.groupMessageReencryptedListeners[messageType], handler)
}

func (hp *hushProtocol) notifyGroupMessageReencryptedListeners(messageType, messageID string, msg GroupMessage) {
	hp.groupMessageReencryptedListenersMu.RLock()
	defer hp.groupMessageReencryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify group message reencrypted listeners")
	for _, listener := range hp.groupMessageReencryptedListeners[messageType] {
		listener := listener
		child.Go(context.TODO(), "notify group message reencrypted listener", func(ctx context.Context) {
			listener(messageID, msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (hp *hushProtocol) OnGroupMessageDecrypted(messageType string, handler GroupMessageDecryptedCallback) {
	hp.groupMessageDecryptedListenersMu.Lock()
	defer hp.groupMessageDecryptedListenersMu.Unlock()
	hp.groupMessageDecryptedListeners[messageType] = append(hp.groupMessageDecryptedListeners[messageType], handler)
}

func (hp *hushProtocol) notifyGroupMessageDecryptedListeners(messageType, messageID string, plaintext []byte, msg GroupMessage) {
	hp.groupMessageDecryptedListenersMu.RLock()
	defer hp.groupMessageDecryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify group message decrypted listeners")
	for _, listener := range hp.groupMessageDecryptedListeners[messageType] {
		listener := listener
		child.Go(context.TODO(), "notify group message decrypted listener", func(ctx context.Context) {
			listener(messageID, plaintext, msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (hp *hushProtocol) resumeTasksForPeer(peerAddr types.Address) {
	// Outgoing group messages
	for _, id := range hp.store.OutgoingGroupMessageIDs() {
		// @@TODO: fix pool
		hp.poolWorker.Add(encryptGroupMessage{id, hp})
		hp.poolWorker.ForceRetry(encryptGroupMessage{id, hp})
	}

	// Incoming individual + group messages
	hp.decryptIncomingGroupMessagesTask.Enqueue()
}

type exchangePubkeyBundlesTask struct {
	process.PeriodicTask
	log.Logger
	hushProto *hushProtocol
}

func NewExchangePubkeysBundleTask(
	interval time.Duration,
	hushProto *hushProtocol,
) *exchangePubkeyBundlesTask {
	t := &exchangePubkeyBundlesTask{
		Logger:    log.NewLogger(ProtocolName),
		hushProto: hushProto,
	}
	t.PeriodicTask = *process.NewPeriodicTask("ExchangePubkeysBundleTask", utils.NewStaticTicker(interval), t.exchangePubkeyBundles)
	return t
}

func (t *exchangePubkeyBundlesTask) exchangePubkeyBundles(ctx context.Context) {
	for _, peer := range t.hushProto.peerStore.VerifiedPeers() {
		t.hushProto.poolWorker.Add(exchangePubkeyBundles{peer.DeviceUniqueID(), t.hushProto})
	}
}

type exchangePubkeyBundles struct {
	deviceUniqueID string
	hushProto      *hushProtocol
}

func (t exchangePubkeyBundles) ID() process.PoolUniqueID { return t }

func (t exchangePubkeyBundles) Work(ctx context.Context) (retry bool) {
	bundles, err := t.hushProto.store.PubkeyBundles()
	if err != nil {
		t.hushProto.Errorf("while fetching DH pubkey bundles: %v", err)
		return true
	}

	peer, exists := t.hushProto.peerStore.PeerWithDeviceUniqueID(t.deviceUniqueID)
	if !exists {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	retry = true
	chDone := t.hushProto.TryPeerDevices(ctx, &t.hushProto.Process, []swarm.PeerDevice{peer}, func(ctx context.Context, peerConn HushPeerConn) error {
		err = peerConn.SendPubkeyBundles(ctx, bundles)
		if err != nil {
			t.hushProto.Errorf("while exchanging DH pubkey: %v", err)
		} else {
			retry = false
		}
		return err
	})
	<-chDone
	return retry
}

type ensureSessionOutgoing struct {
	myAddress     types.Address
	remoteAddress types.Address
	hushProto     *hushProtocol
}

func (t ensureSessionOutgoing) ID() process.PoolUniqueID { return t }

func (t ensureSessionOutgoing) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("ensure session outgoing (%v %v): retrying later", t.myAddress, t.remoteAddress)
		} else {
			t.hushProto.Successf("ensure session outgoing (%v %v): done", t.myAddress, t.remoteAddress)
		}
	}()

	session, drsession, err := t.hushProto.store.LatestValidSessionWithUsers(t.myAddress, t.remoteAddress)
	if err == nil {
		t.hushProto.Successf("started outgoing session: %v", utils.PrettyJSON(session))
		t.hushProto.Successf("started outgoing dr session: %v", utils.PrettyJSON(drsession))
		// Done
		return false
	} else if err != nil && errors.Cause(err) != errors.Err404 {
		t.hushProto.Errorf("while fetching latest session with users: %v", err)
		return true
	}

	myBundle, err := t.hushProto.store.LatestValidKeyBundleFor(t.myAddress)
	if err != nil {
		t.hushProto.Errorf("while fetching latest valid key bundle: %v", err)
		return true
	}

	remoteBundle, err := t.hushProto.store.LatestValidPubkeyBundleFor(t.remoteAddress)
	if errors.Cause(err) == errors.Err404 {
		t.hushProto.vaultManager.FetchPubkeyBundlesForAddress(ctx, t.remoteAddress)
		return true

	} else if err != nil {
		t.hushProto.Errorf("while fetching latest valid pubkey bundle for %v: %v", t.remoteAddress, err)
		return true
	}

	sharedKey, ephemeralPubkey, err := encryption.PerformActiveX3DH(
		gethcrypto.CompressPubkey((*ecdsa.PublicKey)(remoteBundle.IdentityPubkey)),
		gethcrypto.CompressPubkey((*ecdsa.PublicKey)(remoteBundle.SignedPreKeyPubkey)),
		(*ecdsa.PrivateKey)(myBundle.IdentityKey),
	)
	if err != nil {
		return true
	}

	session, drsession, err = t.hushProto.store.StartOutgoingSession(myBundle, remoteBundle, (*X3DHPublicKey)(ephemeralPubkey), sharedKey)
	if err != nil {
		t.hushProto.Errorf("while saving outgoing session: %+v", err)
		return true
	}
	t.hushProto.Successf("started outgoing session: %v", utils.PrettyJSON(session))
	t.hushProto.Successf("started outgoing dr session: %v", utils.PrettyJSON(drsession))

	t.hushProto.resumeTasksForPeer(t.remoteAddress)
	return false
}

type ensureSessionIncoming struct {
	myBundleID      types.Hash
	remoteBundleID  types.Hash
	ephemeralPubkey *X3DHPublicKey
	hushProto       *hushProtocol
}

func (t ensureSessionIncoming) ID() process.PoolUniqueID { return t }

func (t ensureSessionIncoming) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("ensure session incoming (%v %v): retrying later", t.myBundleID, t.remoteBundleID)
		} else {
			t.hushProto.Successf("ensure session incoming (%v %v): done", t.myBundleID, t.remoteBundleID)
		}
	}()

	session, drsession, err := t.hushProto.store.Session(SessionID(t.myBundleID, t.remoteBundleID, t.ephemeralPubkey))
	if err == nil {
		t.hushProto.Successf("started incoming session: %v", utils.PrettyJSON(session))
		t.hushProto.Successf("started incoming dr session: %v", utils.PrettyJSON(drsession))
		// Done
		return false
	} else if err != nil && errors.Cause(err) != errors.Err404 {
		t.hushProto.Errorf("while fetching session: %v", err)
		return true
	}

	myBundle, err := t.hushProto.store.KeyBundle(t.myBundleID)
	if err != nil {
		t.hushProto.Errorf("while fetching key bundle: %v", err)
		return true
	}

	remoteBundle, err := t.hushProto.store.PubkeyBundle(t.remoteBundleID)
	if errors.Cause(err) == errors.Err404 {
		t.hushProto.vaultManager.FetchPubkeyBundleByID(ctx, t.remoteBundleID)
		return true

	} else if err != nil {
		t.hushProto.Errorf("while fetching latest valid pubkey bundle: %v", err)
		return true
	}

	sharedKey, err := encryption.PerformPassiveX3DH(
		remoteBundle.IdentityPubkey.ECDSA(),
		myBundle.SignedPreKey.ECDSA(),
		t.ephemeralPubkey.ECDSA(),
		myBundle.IdentityKey.ECDSA(),
	)
	if err != nil {
		t.hushProto.Errorf("while performing passive x3dh handshake: %v", err)
		return true
	}

	session, drsession, err = t.hushProto.store.StartIncomingSession(myBundle, remoteBundle, t.ephemeralPubkey, sharedKey)
	if err != nil {
		t.hushProto.Errorf("while saving incoming session: %v", err)
		return true
	}

	remoteAddress, err := remoteBundle.Address()
	if err != nil {
		t.hushProto.Errorf("while verifying remote bundle signature: %v", err)
		return true
	}
	t.hushProto.Successf("started incoming session: %v", utils.PrettyJSON(session))
	t.hushProto.Successf("started incoming dr session: %v", utils.PrettyJSON(drsession))

	t.hushProto.resumeTasksForPeer(remoteAddress)
	return false
}

type encryptGroupMessage struct {
	id        string
	hushProto *hushProtocol
}

var _ process.PoolWorkerItem = encryptGroupMessage{}

func (t encryptGroupMessage) ID() process.PoolUniqueID { return t }

func (t encryptGroupMessage) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("encrypt group message %v: retrying later", t.id)
		} else {
			t.hushProto.Successf("encrypt group message %v: done", t.id)
		}
	}()

	intent, err := t.hushProto.store.OutgoingGroupMessage(t.id)
	if errors.Cause(err) == errors.Err404 {
		// Done
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching outgoing group message intent: %v", err)
		return true
	}

	// Create the symmetric encryption key and encrypt the plaintext with it
	symEncKey, err := crypto.NewSymEncKey()
	if err != nil {
		t.hushProto.Errorf("while generating symmetric encryption key for group message: %v", err)
		return true
	}
	symEncMsg, err := symEncKey.Encrypt(intent.Plaintext)
	if err != nil {
		t.hushProto.Errorf("while encrypting group message: %v", err)
		return true
	}

	msg, retry, err := t.hushProto.assembleGroupMessage(intent.Sender, intent.Recipients, symEncKey, symEncMsg)
	if err != nil {
		t.hushProto.Errorw(fmt.Sprintf("while encrypting: %v", err), "sender", intent.Sender, "recipients", intent.Recipients)
	}
	if retry {
		return true
	}

	err = t.hushProto.store.SaveSymEncKeyAndMessage(intent.MessageType, intent.ID, symEncKey, symEncMsg)
	if err != nil {
		return true
	}

	t.hushProto.notifyGroupMessageEncryptedListeners(intent.MessageType, intent.ID, msg)

	err = t.hushProto.store.DeleteOutgoingGroupMessage(intent.ID)
	if err != nil {
		t.hushProto.Errorf("while deleting outgoing group message: %v", err)
		return true
	}
	return false
}

type decryptIncomingGroupMessagesTask struct {
	process.PeriodicTask
	log.Logger
	hushProto *hushProtocol
}

func NewDecryptIncomingGroupMessagesTask(
	interval time.Duration,
	hushProto *hushProtocol,
) *decryptIncomingGroupMessagesTask {
	t := &decryptIncomingGroupMessagesTask{
		Logger:    log.NewLogger(ProtocolName),
		hushProto: hushProto,
	}
	t.PeriodicTask = *process.NewPeriodicTask("DecryptIncomingGroupMessagesTask", utils.NewStaticTicker(interval), t.decryptIncomingGroupMessages)
	return t
}

func (t *decryptIncomingGroupMessagesTask) decryptIncomingGroupMessages(ctx context.Context) {
	messages, err := t.hushProto.store.IncomingGroupMessages()
	if err != nil {
		t.Errorf("while fetching incoming messages from database: %v", err)
		return
	}

	for _, msg := range messages {
		msg := msg
		t.Process.Go(ctx, "decrypt incoming message", func(ctx context.Context) {
			t.decryptIncomingGroupMessage(ctx, msg)
		})
	}
}

func (t *decryptIncomingGroupMessagesTask) decryptIncomingGroupMessage(ctx context.Context, msg IncomingGroupMessage) {
	encEncKey, ok := First(msg.Message.EncryptionKeys, func(key IndividualMessage) (IndividualMessage, bool) {
		_, err := t.hushProto.store.KeyBundle(key.RecipientBundleID)
		if errors.Cause(err) == errors.Err404 {
			return IndividualMessage{}, false
		} else if err != nil {
			t.hushProto.Errorf("while looking up session with ID: %v", err)
			return IndividualMessage{}, false
		}
		return key, true
	})
	if !ok {
		t.hushProto.Errorf("message %v not meant for us", msg.ID)
		// This message was apparently not meant for us
		return
	}

	session, drsession, err := t.hushProto.store.Session(SessionID(encEncKey.SenderBundleID, encEncKey.RecipientBundleID, encEncKey.EphemeralPubkey))
	if errors.Cause(err) == errors.Err404 {
		t.hushProto.Debugw("cannot decrypt message yet, no session", "sender_bundle", encEncKey.SenderBundleID, "recipient_bundle", encEncKey.RecipientBundleID)
		t.hushProto.poolWorker.Add(ensureSessionIncoming{encEncKey.RecipientBundleID, encEncKey.SenderBundleID, encEncKey.EphemeralPubkey, t.hushProto})
		return
	} else if err != nil {
		t.hushProto.Errorf("while looking up session with ID: %v", err)
		return
	}

	sessionID := session.ID()

	var shouldDelete bool
	defer func() {
		if shouldDelete {
			err := t.hushProto.store.DeleteIncomingGroupMessage(msg.ID)
			if err != nil {
				t.Errorf("while deleting incoming group message (session=%v): %v", sessionID, err)
				return
			}
		}
	}()

	t.hushProto.Debugf("decrypting incoming group message (session=%v)", sessionID)

	drmsg := doubleratchet.Message{
		Header: doubleratchet.MessageHeader{
			DH: doubleratchet.Key(encEncKey.RatchetPubkey.Bytes()),
			N:  encEncKey.N,
			PN: encEncKey.Pn,
		},
		Ciphertext: encEncKey.Ciphertext,
	}
	encKeyBytes, err := drsession.RatchetDecrypt(drmsg, nil)
	if err != nil {
		t.hushProto.Errorf("while decrypting incoming group message (session=%v): %v", sessionID, err)
		shouldDelete = true
		return
	}

	symEncKey := crypto.SymEncKeyFromBytes(encKeyBytes)
	symEncMsg := crypto.SymEncMsgFromBytes(msg.Message.Ciphertext)

	err = t.hushProto.store.SaveSymEncKeyAndMessage(msg.MessageType, msg.ID, symEncKey, symEncMsg)
	if err != nil {
		t.hushProto.Errorf("while saving symmetric encryption key and message to database (session=%v): %v", sessionID, err)
		return
	}

	plaintext, err := symEncKey.Decrypt(symEncMsg)
	if err != nil {
		t.hushProto.Errorf("while decrypting incoming group message (session=%v): %v", sessionID, err)
		shouldDelete = true
		return
	}

	// pubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(plaintext), msg.Message.Sig)
	// if err != nil {
	// 	shouldDelete = true
	// 	t.hushProto.Errorf("while recovering address from incoming group message hash: %v", err)
	// 	return
	// } else if pubkey.Address() != session.RemoteAddress {
	// 	shouldDelete = true
	// 	t.hushProto.Errorf("address recovered from signature (%v) does not match other party in individual session (%v)", pubkey.Address(), session.RemoteAddress)
	// 	return
	// }

	t.hushProto.notifyGroupMessageDecryptedListeners(msg.MessageType, msg.ID, plaintext, msg.Message)

	t.Successf("decrypted incoming group message (session=%v message=%v)", sessionID, msg.ID)
	shouldDelete = true
}
