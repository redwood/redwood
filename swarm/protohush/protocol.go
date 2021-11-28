package protohush

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/status-im/doubleratchet"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//
// @@TODO:
//   - move store.EnsureDHPair() to hushProtocol and make it save the attestation
//   - handleIncomingIndividualSessionTask#validateProposal: ensure that epoch > latest epoch. If not, return proposedSessionExists = true
//   - OnIndividualSessionProposed(sessionType string, handler func(proposal IndividualSessionProposal) bool)
//

//go:generate mockery --name HushProtocol --output ./mocks/ --case=underscore
type HushProtocol interface {
	process.Interface

	ProposeIndividualSession(ctx context.Context, sessionType string, recipient types.Address, epoch uint64) error
	ProposeNextIndividualSession(ctx context.Context, sessionType string, recipient types.Address) error

	EncryptIndividualMessage(sessionType string, recipient types.Address, plaintext []byte) error
	OnIndividualMessageDecrypted(sessionType string, handler IndividualMessageDecryptedCallback)

	EncryptGroupMessage(sessionType, messageID string, recipients []types.Address, plaintext []byte) error
	DecryptGroupMessage(msg GroupMessage) error
	OnGroupMessageEncrypted(sessionType string, handler GroupMessageEncryptedCallback)
	OnGroupMessageDecrypted(sessionType string, handler GroupMessageDecryptedCallback)
}

//go:generate mockery --name HushTransport --output ./mocks/ --case=underscore
type HushTransport interface {
	swarm.Transport
	OnIncomingDHPubkeyAttestations(handler IncomingDHPubkeyAttestationsCallback)
	OnIncomingIndividualSessionProposal(handler IncomingIndividualSessionProposalCallback)
	OnIncomingIndividualSessionApproval(handler IncomingIndividualSessionApprovalCallback)
	OnIncomingIndividualMessage(handler IncomingIndividualMessageCallback)
	OnIncomingGroupMessage(handler IncomingGroupMessageCallback)
}

//go:generate mockery --name HushPeerConn --output ./mocks/ --case=underscore
type HushPeerConn interface {
	swarm.PeerConn
	SendDHPubkeyAttestations(ctx context.Context, attestations []DHPubkeyAttestation) error
	ProposeIndividualSession(ctx context.Context, encryptedProposal []byte) error
	ApproveIndividualSession(ctx context.Context, approval IndividualSessionApproval) error
	SendHushIndividualMessage(ctx context.Context, msg IndividualMessage) error
	SendHushGroupMessage(ctx context.Context, msg GroupMessage) error
}

type hushProtocol struct {
	process.Process
	log.Logger

	store      Store
	keyStore   identity.KeyStore
	peerStore  swarm.PeerStore
	transports map[string]HushTransport

	groupMessageEncryptedListeners        map[string][]GroupMessageEncryptedCallback
	groupMessageEncryptedListenersMu      sync.RWMutex
	individualMessageDecryptedListeners   map[string][]IndividualMessageDecryptedCallback
	individualMessageDecryptedListenersMu sync.RWMutex
	groupMessageDecryptedListeners        map[string][]GroupMessageDecryptedCallback
	groupMessageDecryptedListenersMu      sync.RWMutex

	poolWorker                            process.PoolWorker
	exchangeDHPubkeysTask                 *exchangeDHPubkeysTask
	decryptIncomingIndividualMessagesTask *decryptIncomingIndividualMessagesTask
	decryptIncomingGroupMessagesTask      *decryptIncomingGroupMessagesTask
}

const ProtocolName = "protohush"

type GroupMessageEncryptedCallback func(msg GroupMessage)
type IndividualMessageDecryptedCallback func(sender types.Address, plaintext []byte, msg IndividualMessage)
type GroupMessageDecryptedCallback func(sender types.Address, plaintext []byte, msg GroupMessage)

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
	return &hushProtocol{
		Process:                             *process.New(ProtocolName),
		Logger:                              log.NewLogger(ProtocolName),
		store:                               store,
		keyStore:                            keyStore,
		peerStore:                           peerStore,
		transports:                          transportsMap,
		groupMessageEncryptedListeners:      make(map[string][]GroupMessageEncryptedCallback),
		individualMessageDecryptedListeners: make(map[string][]IndividualMessageDecryptedCallback),
		groupMessageDecryptedListeners:      make(map[string][]GroupMessageDecryptedCallback),
	}
}

func (hp *hushProtocol) Start() error {
	err := hp.Process.Start()
	if err != nil {
		return err
	}

	// Ensure we have a DH keypair and a signed attestation of that keypair in the DB
	{
		identity, err := hp.keyStore.DefaultPublicIdentity()
		if err != nil {
			return err
		}
		dhPair, err := hp.store.EnsureDHPair()
		if err != nil {
			return err
		}
		sig, err := hp.keyStore.SignHash(identity.Address(), dhPair.AttestationHash())
		if err != nil {
			return err
		}
		err = hp.store.SaveDHPubkeyAttestations([]DHPubkeyAttestation{{Pubkey: dhPair.Public, Epoch: dhPair.Epoch, Sig: sig}})
		if err != nil {
			return err
		}
	}

	// Start our workers
	hp.exchangeDHPubkeysTask = NewExchangeDHPubkeysTask(10*time.Second, hp)
	err = hp.Process.SpawnChild(nil, hp.exchangeDHPubkeysTask)
	if err != nil {
		return err
	}

	hp.poolWorker = process.NewPoolWorker("pool worker", 8, process.NewStaticScheduler(3*time.Second, 3*time.Second))
	err = hp.Process.SpawnChild(nil, hp.poolWorker)
	if err != nil {
		return err
	}

	hp.decryptIncomingIndividualMessagesTask = NewDecryptIncomingIndividualMessagesTask(10*time.Second, hp)
	err = hp.Process.SpawnChild(nil, hp.decryptIncomingIndividualMessagesTask)
	if err != nil {
		return err
	}

	hp.decryptIncomingGroupMessagesTask = NewDecryptIncomingGroupMessagesTask(10*time.Second, hp)
	err = hp.Process.SpawnChild(nil, hp.decryptIncomingGroupMessagesTask)
	if err != nil {
		return err
	}

	for _, tpt := range hp.transports {
		hp.Infof(0, "registering %v", tpt.Name())
		tpt.OnIncomingDHPubkeyAttestations(hp.handleIncomingDHPubkeyAttestations)
		tpt.OnIncomingIndividualSessionProposal(hp.handleIncomingIndividualSessionProposal)
		tpt.OnIncomingIndividualSessionApproval(hp.handleIncomingIndividualSessionApproval)
		tpt.OnIncomingIndividualMessage(hp.handleIncomingIndividualMessage)
		tpt.OnIncomingGroupMessage(hp.handleIncomingGroupMessage)
	}

	hp.peerStore.OnNewVerifiedPeer(func(peer swarm.PeerInfo) {
		hp.poolWorker.Add(exchangeDHPubkeys{peer.DeviceUniqueID(), hp})
	})

	// Transports start before protocols, which means that we may not receive
	// the `OnNewVerifiedPeer` notification if some transports authenticate
	// quickly enough that protohush has not started up yet. Therefore, we must
	// manually exchange DH pubkeys with verified peers on startup.
	for _, peer := range hp.peerStore.VerifiedPeers() {
		hp.poolWorker.Add(exchangeDHPubkeys{peer.DeviceUniqueID(), hp})
	}

	// Add unfulfilled outgoing individual session proposals to the pool worker queue
	proposalHashes, err := hp.store.OutgoingIndividualSessionProposalHashes()
	if err != nil {
		return err
	}
	for proposalHash := range proposalHashes {
		hp.poolWorker.Add(proposeIndividualSession{proposalHash, hp})
	}

	approvals, err := hp.store.OutgoingIndividualSessionApprovals()
	if err != nil {
		return err
	}
	for addr, x := range approvals {
		for proposalHash := range x {
			hp.poolWorker.Add(approveIndividualSession{addr, proposalHash, hp})
		}
	}

	// Add unfulfilled incoming individual session proposals to the pool worker queue
	proposals, err := hp.store.IncomingIndividualSessionProposals()
	if err != nil {
		return err
	}
	for _, proposal := range proposals {
		hp.poolWorker.Add(handleIncomingIndividualSession{proposal.Hash(), hp})
	}

	// Add unsent outgoing individual messages to the pool worker queue
	messageIntents, err := hp.store.OutgoingIndividualMessageIntents()
	if err != nil {
		return err
	}
	for _, intent := range messageIntents {
		hp.poolWorker.Add(sendIndividualMessage{intent.SessionType, intent.Recipient, intent.ID, hp})
	}

	// Add unsent outgoing group messages to the pool worker queue
	sessionTypes, err := hp.store.OutgoingGroupMessageSessionTypes()
	if err != nil {
		return err
	}
	for sessionType := range sessionTypes {
		ids, err := hp.store.OutgoingGroupMessageIntentIDsForSessionType(sessionType)
		if err != nil {
			return err
		}
		for id := range ids {
			hp.poolWorker.Add(encryptGroupMessage{sessionType, id, hp})
		}
	}

	return nil
}

func (hp *hushProtocol) ensureSessionWithSelf(sessionType string) error {
	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return err
	}

	_, err = hp.store.IndividualSessionByID(IndividualSessionID{
		SessionType: sessionType + ":self:send",
		AliceAddr:   identity.Address(),
		BobAddr:     identity.Address(),
		Epoch:       0,
	})
	if err != nil && errors.Cause(err) != errors.Err404 {
		return err
	} else if err == nil {
		return nil
	}

	dhPair, err := hp.store.EnsureDHPair()
	if err != nil {
		return err
	}

	sk, err := GenerateSharedKey()
	if err != nil {
		return errors.Wrapf(err, "while generating shared key for session with self")
	}
	sessionSend := IndividualSessionProposal{
		SessionID: IndividualSessionID{
			SessionType: sessionType + ":self:send",
			AliceAddr:   identity.Address(),
			BobAddr:     identity.Address(),
			Epoch:       0,
		},
		SharedKey:      sk,
		AliceSig:       nil,
		RemoteDHPubkey: dhPair.Public,
	}
	sessionRecv := IndividualSessionProposal{
		SessionID: IndividualSessionID{
			SessionType: sessionType + ":self:recv",
			AliceAddr:   identity.Address(),
			BobAddr:     identity.Address(),
			Epoch:       0,
		},
		SharedKey:      sk,
		AliceSig:       nil,
		RemoteDHPubkey: dhPair.Public,
	}

	sessionHash, err := sessionSend.Hash()
	if err != nil {
		return err
	}

	sig, err := hp.keyStore.SignHash(identity.Address(), sessionHash)
	if err != nil {
		return err
	}
	sessionSend.AliceSig = sig

	_, err = doubleratchet.NewWithRemoteKey(
		sessionSend.SessionID.Bytes(),
		sk[:],
		dhPair.Public,
		hp.store.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(hp.store.RatchetKeyStore()),
	)
	if err != nil {
		return errors.Wrapf(err, "while initializing double ratchet session")
	}

	_, err = doubleratchet.New(
		sessionRecv.SessionID.Bytes(),
		sk[:],
		dhPair,
		hp.store.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(hp.store.RatchetKeyStore()),
	)
	if err != nil {
		return errors.Wrapf(err, "while initializing double ratchet session")
	}

	err = hp.store.SaveApprovedIndividualSession(sessionSend)
	if err != nil {
		return errors.Wrapf(err, "while saving approved individual session")
	}
	err = hp.store.SaveApprovedIndividualSession(sessionRecv)
	if err != nil {
		return errors.Wrapf(err, "while saving approved individual session")
	}
	return nil
}

func (hp *hushProtocol) ProposeNextIndividualSession(ctx context.Context, sessionType string, recipient types.Address) error {
	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return errors.Wrapf(err, "while fetching default public identity")
	}

	session, err := hp.store.LatestIndividualSessionWithUsers(sessionType, identity.Address(), recipient)
	if errors.Cause(err) == errors.Err404 {
		return hp.ProposeIndividualSession(ctx, sessionType, recipient, 0)
	} else if err != nil {
		return errors.Wrapf(err, "while fetching latest individual session ID with %v", recipient)
	}
	return hp.ProposeIndividualSession(ctx, sessionType, recipient, session.SessionID.Epoch+1)
}

func (hp *hushProtocol) ProposeIndividualSession(ctx context.Context, sessionType string, recipient types.Address, epoch uint64) error {
	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return errors.Wrapf(err, "while fetching default public identity")
	} else if recipient == identity.Address() {
		return ErrRecipientIsSelf
	}

	sk, err := GenerateSharedKey()
	if err != nil {
		return errors.Wrapf(err, "while generating shared key for session %v-%v-%v", sessionType, recipient.Hex(), epoch)
	}

	sessionID := IndividualSessionID{
		SessionType: sessionType,
		AliceAddr:   identity.Address(),
		BobAddr:     recipient,
		Epoch:       epoch,
	}

	proposal := IndividualSessionProposal{
		SessionID: sessionID,
		SharedKey: sk,
	}

	err = hp.store.SaveOutgoingIndividualSessionProposal(proposal)
	if err != nil {
		return err
	}

	proposalHash, err := proposal.Hash()
	if err != nil {
		return err
	}

	hp.Debugf("Add proposeIndividualSession")
	hp.poolWorker.Add(proposeIndividualSession{proposalHash, hp})
	return nil
}

func (hp *hushProtocol) EncryptIndividualMessage(sessionType string, recipient types.Address, plaintext []byte) error {
	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return errors.Wrapf(err, "while fetching default public identity")
	} else if recipient == identity.Address() {
		return ErrRecipientIsSelf
	}

	intent := IndividualMessageIntent{
		ID:          types.RandomID(),
		SessionType: sessionType,
		Recipient:   recipient,
		Plaintext:   plaintext,
	}
	err = hp.store.SaveOutgoingIndividualMessageIntent(intent)
	if err != nil {
		return errors.Wrapf(err, "while saving individual message intent to database")
	}
	hp.poolWorker.Add(sendIndividualMessage{sessionType, recipient, intent.ID, hp})
	return nil
}

func (hp *hushProtocol) EncryptGroupMessage(sessionType, messageID string, recipients []types.Address, plaintext []byte) error {
	intent := GroupMessageIntent{
		SessionType: sessionType,
		ID:          messageID,
		Recipients:  recipients,
		Plaintext:   plaintext,
	}
	err := hp.store.SaveOutgoingGroupMessageIntent(intent)
	if err != nil {
		return errors.Wrapf(err, "while saving group message intent to database")
	}
	hp.poolWorker.Add(encryptGroupMessage{intent.SessionType, intent.ID, hp})
	return nil
}

func (hp *hushProtocol) DecryptGroupMessage(msg GroupMessage) error {
	msgHash, err := msg.Hash()
	if err != nil {
		return errors.Wrapf(err, "while hashing incoming group message")
	}
	pubkey, err := crypto.RecoverSigningPubkey(msgHash, msg.Sig)
	if err != nil {
		return errors.Wrapf(err, "while recovering address from incoming group message hash")
	}
	sender := pubkey.Address()

	err = hp.store.SaveIncomingGroupMessage(sender, msg)
	if err != nil {
		return errors.Wrapf(err, "while saving incoming group message")
	}
	hp.decryptIncomingGroupMessagesTask.Enqueue()
	return nil
}

func (hp *hushProtocol) handleIncomingDHPubkeyAttestations(ctx context.Context, attestations []DHPubkeyAttestation, peer HushPeerConn) {
	var valid []DHPubkeyAttestation
	addrs := types.NewAddressSet(nil)
	for _, a := range attestations {
		addr, err := a.Address()
		if err != nil {
			continue
		}
		addrs.Add(addr)
		valid = append(valid, a)
	}

	err := hp.store.SaveDHPubkeyAttestations(valid)
	if err != nil {
		hp.Errorf("while saving DH pubkey attestations: %v", err)
		return
	}

	duIDs := types.NewStringSet(nil)
	for _, p := range hp.peerStore.VerifiedPeers() {
		duIDs.Add(p.DeviceUniqueID())
	}
	for duID := range duIDs {
		hp.poolWorker.Add(exchangeDHPubkeys{duID, hp})
	}

	// Kick off workers to propose outgoing indvidual sessions that depend on the DH pubkeys we received
	{
		identity, err := hp.keyStore.DefaultPublicIdentity()
		if err != nil {
			hp.Errorf("while fetching default public identity: %v", err)
			return
		}
		myAddr := identity.Address()

		for addr := range addrs {
			proposals, err := hp.store.OutgoingIndividualSessionProposalsForUsers(myAddr, addr)
			if err != nil {
				hp.Errorf("while fetching outgoing individual session proposals for %v and %v: %v", myAddr, addr, err)
				continue
			}
			for _, p := range proposals {
				proposalHash, err := p.Hash()
				if err != nil {
					hp.Errorf("while hashing outgoing individual session proposal for %v and %v: %v", myAddr, addr, err)
					continue
				}
				hp.Successf("retrying outgoing session proposal %v", proposalHash)
				hp.poolWorker.ForceRetry(proposeIndividualSession{proposalHash, hp})
			}

			approvals, err := hp.store.OutgoingIndividualSessionApprovalsForUser(addr)
			if err != nil {
				hp.Errorf("while fetching outgoing individual session approvals for %v and %v: %v", myAddr, addr, err)
				continue
			}
			for _, a := range approvals {
				hp.Successf("retrying outgoing session approval %v", a.ProposalHash)
				hp.poolWorker.ForceRetry(approveIndividualSession{addr, a.ProposalHash, hp})
			}
		}
	}
}

func (hp *hushProtocol) handleIncomingIndividualSessionProposal(ctx context.Context, encryptedProposal []byte, alice HushPeerConn) {
	aliceAddrs := alice.Addresses()
	if len(aliceAddrs) == 0 {
		hp.Errorf("while handling incoming individual session proposal: not authenticated")
		return
	}
	aliceAddr := aliceAddrs[0]
	proposal := EncryptedIndividualSessionProposal{aliceAddr, encryptedProposal}

	err := hp.store.SaveIncomingIndividualSessionProposal(proposal)
	if err != nil {
		hp.Errorf("while saving incoming individual session proposal: %v", err)
		return
	}
	hp.Debugf("Add handleIncomingIndividualSession")
	hp.poolWorker.Add(handleIncomingIndividualSession{proposal.Hash(), hp})
}

func (hp *hushProtocol) handleIncomingIndividualSessionApproval(ctx context.Context, approval IndividualSessionApproval, bob HushPeerConn) {
	hp.Warnf("incoming individual session approval")

	proposal, err := hp.store.OutgoingIndividualSessionProposalByHash(approval.ProposalHash)
	if errors.Cause(err) == errors.Err404 {
		hp.Errorf("proposal hash does not match any known proposal (hash: %v)", approval.ProposalHash)
		return
	} else if err != nil {
		hp.Errorf("while fetching outgoing shared key proposal: %v", err)
		return
	}

	// This saves the session to the DB
	_, err = doubleratchet.NewWithRemoteKey(
		proposal.SessionID.Bytes(),
		proposal.SharedKey[:],
		proposal.RemoteDHPubkey,
		hp.store.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(hp.store.RatchetKeyStore()),
	)
	if err != nil {
		hp.Errorf("while initializing double ratchet session: %v", err)
		return
	}

	err = hp.store.SaveApprovedIndividualSession(proposal)
	if err != nil {
		hp.Errorf("while saving approved individual session: %v", err)
		return
	}

	hp.onSessionOpened(approval.ProposalHash, proposal.SessionID.BobAddr)

	err = hp.store.DeleteOutgoingIndividualSessionProposal(approval.ProposalHash)
	if err != nil {
		hp.Errorf("while deleting incoming individual session approval: %v", err)
		return
	}
}

func (hp *hushProtocol) handleIncomingIndividualMessage(ctx context.Context, msg IndividualMessage, peerConn HushPeerConn) {
	peerAddrs := peerConn.Addresses()
	if len(peerAddrs) == 0 {
		panic("impossible") // @@TODO
	}
	sender := peerAddrs[0]

	hp.Debugf("received encrypted individual message from %v", sender)

	err := hp.store.SaveIncomingIndividualMessage(sender, msg)
	if err != nil {
		hp.Errorf("while saving incoming individual message: %v", err)
		return
	}
	hp.decryptIncomingIndividualMessagesTask.Enqueue()
}

func (hp *hushProtocol) handleIncomingGroupMessage(ctx context.Context, msg GroupMessage, peerConn HushPeerConn) {
	err := hp.DecryptGroupMessage(msg)
	if err != nil {
		hp.Errorf("while handling incoming group message: %v", err)
		return
	}
}

func (hp *hushProtocol) OnGroupMessageEncrypted(sessionType string, handler GroupMessageEncryptedCallback) {
	hp.groupMessageEncryptedListenersMu.Lock()
	defer hp.groupMessageEncryptedListenersMu.Unlock()
	hp.groupMessageEncryptedListeners[sessionType] = append(hp.groupMessageEncryptedListeners[sessionType], handler)
}

func (hp *hushProtocol) notifyGroupMessageEncryptedListeners(msg GroupMessage) {
	hp.groupMessageEncryptedListenersMu.RLock()
	defer hp.groupMessageEncryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify group message encrypted listeners")
	for _, listener := range hp.groupMessageEncryptedListeners[msg.SessionType] {
		listener := listener
		child.Go(context.TODO(), "notify group message encrypted listener", func(ctx context.Context) {
			listener(msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (hp *hushProtocol) OnIndividualMessageDecrypted(sessionType string, handler IndividualMessageDecryptedCallback) {
	hp.individualMessageDecryptedListenersMu.Lock()
	defer hp.individualMessageDecryptedListenersMu.Unlock()
	hp.individualMessageDecryptedListeners[sessionType] = append(hp.individualMessageDecryptedListeners[sessionType], handler)
}

func (hp *hushProtocol) notifyIndividualMessageDecryptedListeners(sessionType string, sender types.Address, plaintext []byte, msg IndividualMessage) {
	hp.individualMessageDecryptedListenersMu.RLock()
	defer hp.individualMessageDecryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify message decrypted listeners")
	for _, listener := range hp.individualMessageDecryptedListeners[sessionType] {
		listener := listener
		child.Go(context.TODO(), "notify message decrypted listener", func(ctx context.Context) {
			listener(sender, plaintext, msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

func (hp *hushProtocol) OnGroupMessageDecrypted(sessionType string, handler GroupMessageDecryptedCallback) {
	hp.groupMessageDecryptedListenersMu.Lock()
	defer hp.groupMessageDecryptedListenersMu.Unlock()
	hp.groupMessageDecryptedListeners[sessionType] = append(hp.groupMessageDecryptedListeners[sessionType], handler)
}

func (hp *hushProtocol) notifyGroupMessageDecryptedListeners(sender types.Address, plaintext []byte, msg GroupMessage) {
	hp.groupMessageDecryptedListenersMu.RLock()
	defer hp.groupMessageDecryptedListenersMu.RUnlock()

	child := hp.Process.NewChild(context.TODO(), "notify message decrypted listeners")
	for _, listener := range hp.groupMessageDecryptedListeners[msg.SessionType] {
		listener := listener
		child.Go(context.TODO(), "notify message decrypted listener", func(ctx context.Context) {
			listener(sender, plaintext, msg)
		})
	}
	child.Autoclose()
	<-child.Done()
}

type exchangeDHPubkeysTask struct {
	process.PeriodicTask
	log.Logger
	hushProto *hushProtocol
	x         map[string]time.Time
}

func NewExchangeDHPubkeysTask(
	interval time.Duration,
	hushProto *hushProtocol,
) *exchangeDHPubkeysTask {
	t := &exchangeDHPubkeysTask{
		Logger:    log.NewLogger(ProtocolName),
		hushProto: hushProto,
	}
	t.PeriodicTask = *process.NewPeriodicTask("ExchangeDHPubkeysTask", utils.NewStaticTicker(interval), t.exchangeDHPubkeys)
	return t
}

func (t *exchangeDHPubkeysTask) exchangeDHPubkeys(ctx context.Context) {
	duIDs := types.NewStringSet(nil)
	for _, peer := range t.hushProto.peerStore.VerifiedPeers() {
		duIDs.Add(peer.DeviceUniqueID())
	}
	for duID := range duIDs {
		t.hushProto.poolWorker.Add(exchangeDHPubkeys{duID, t.hushProto})
	}
}

type exchangeDHPubkeys struct {
	deviceUniqueID string
	hushProto      *hushProtocol
}

func (t exchangeDHPubkeys) ID() process.PoolUniqueID { return t }

func (t exchangeDHPubkeys) Work(ctx context.Context) (retry bool) {
	attestations, err := t.hushProto.store.DHPubkeyAttestations()
	if err != nil {
		t.hushProto.Errorf("while fetching DH pubkey attestations: %v", err)
		return true
	}

	peer, exists := t.hushProto.peerStore.PeerWithDeviceUniqueID(t.deviceUniqueID)
	if !exists {
		return true
	}

	t.hushProto.withPeers(ctx, 3*time.Second, []swarm.PeerInfo{peer}, func(ctx context.Context, peerConn HushPeerConn) error {
		err = peerConn.SendDHPubkeyAttestations(ctx, attestations)
		if err != nil {
			t.hushProto.Errorf("while exchanging DH pubkey: %v", err)
			retry = true
		}
		return err
	})
	return retry
}

type proposeIndividualSession struct {
	proposalHash types.Hash
	hushProto    *hushProtocol
}

var _ process.PoolWorkerItem = proposeIndividualSession{}

func (t proposeIndividualSession) ID() process.PoolUniqueID { return t }

func (t proposeIndividualSession) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("propose individual session %v: retrying later", t.proposalHash)
		} else {
			t.hushProto.Successf("propose individual session %v: done", t.proposalHash)
		}
	}()

	session, err := t.hushProto.store.OutgoingIndividualSessionProposalByHash(t.proposalHash)
	if errors.Cause(err) == errors.Err404 {
		t.hushProto.Errorf("while proposing individual session %v: %v", t.proposalHash, err)
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching outgoing individual session proposal: %v", err)
		return true
	}

	bobAddr := session.SessionID.BobAddr

	if len(t.hushProto.peerStore.PeersWithAddress(bobAddr)) == 0 {
		t.hushProto.Errorf("while proposing individual session %v: no peers with address %v", t.proposalHash, bobAddr)
		return true
	}

	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.hushProto.Errorf("while fetching default public identity from keystore: %v", err)
		return true
	}

	// Assign Bob's latest DH pubkey
	remoteDHPubkey, err := t.hushProto.store.LatestDHPubkeyFor(bobAddr)
	if errors.Cause(err) == errors.Err404 {
		t.hushProto.Debugf("waiting to propose individual session: still awaiting DH pubkey for %v", bobAddr)
		return true
	} else if err != nil {
		t.hushProto.Errorf("while fetching latest DH pubkey for %v: %v", bobAddr, err)
		return true
	}
	session.RemoteDHPubkey = remoteDHPubkey.Pubkey

	// Sign the proposal
	hash, err := session.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing outgoing individual session proposal: %v", err)
		return true
	}
	sig, err := t.hushProto.keyStore.SignHash(identity.Address(), hash)
	if err != nil {
		t.hushProto.Errorf("while signing hash of outgoing individual session proposal: %v", err)
		return true
	}
	session.AliceSig = sig

	err = t.hushProto.store.SaveOutgoingIndividualSessionProposal(session)
	if err != nil {
		t.hushProto.Errorf("while updating outgoing individual session proposal: %v", err)
		return true
	}

	sessionBytes, err := session.Marshal()
	if err != nil {
		t.hushProto.Errorf("while marshaling outgoing individual session proposal: %v", err)
		return true
	}

	t.hushProto.withPeers(ctx, 3*time.Second, t.hushProto.peerStore.PeersWithAddress(bobAddr), func(ctx context.Context, peerConn HushPeerConn) error {
		t.hushProto.Debugf("proposeIndividualSession WITH PEERS %v", peerConn.DialInfo())
		_, bobAsymEncPubkey := peerConn.PublicKeys(session.SessionID.BobAddr)

		encryptedProposalBytes, err := t.hushProto.keyStore.SealMessageFor(identity.Address(), bobAsymEncPubkey, sessionBytes)
		if err != nil {
			return err
		}
		err = peerConn.ProposeIndividualSession(ctx, encryptedProposalBytes)
		if err != nil {
			return err
		}
		t.hushProto.Infof(0, "proposed individual session %v", session.SessionID)
		return nil
	})

	// Continue retrying until we get an approval
	return true
}

type handleIncomingIndividualSession struct {
	proposalHash types.Hash
	hushProto    *hushProtocol
}

var _ process.PoolWorkerItem = handleIncomingIndividualSession{}

func (t handleIncomingIndividualSession) ID() process.PoolUniqueID { return t }

func (t handleIncomingIndividualSession) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("handle incoming individual session %v: retrying later", t.proposalHash)
		} else {
			t.hushProto.Successf("handle incoming individual session %v: done", t.proposalHash)
		}
	}()

	proposal, err := t.hushProto.store.IncomingIndividualSessionProposal(t.proposalHash)
	if errors.Cause(err) == errors.Err404 {
		// Done
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching incoming individual session proposal from database: %v", err)
		return true
	}

	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.hushProto.Errorf("while fetching identity from database: %v", err)
		return true
	}

	proposedSession, err := t.decryptProposal(proposal)
	if err != nil {
		t.hushProto.Errorf("while decrypting incoming individual session proposal: %v", err)
		return true
	}
	t.hushProto.Debugf("incoming individual session proposal: %v", proposedSession.SessionID)

	err = t.validateProposal(proposal.AliceAddr, proposedSession)
	switch errors.Cause(err) {
	case ErrInvalidSignature:
		t.hushProto.Errorf("invalid signature on incoming individual session proposal")
		err := t.hushProto.store.DeleteIncomingIndividualSessionProposal(proposal)
		if err != nil {
			t.hushProto.Errorf("while deleting incoming individual session proposal: %v", err)
			return false
		}
		return false
	case ErrEpochTooLow:
		return false

	case ErrInvalidDHPubkey:
		t.hushProto.Debugf("ignoring session proposal: DH pubkey %v does not match our latest", proposedSession.RemoteDHPubkey)
		err := t.hushProto.store.DeleteIncomingIndividualSessionProposal(proposal)
		if err != nil {
			t.hushProto.Errorf("while deleting incoming individual session proposal: %v", err)
		}
		// t.proposeReplacementSession(myAddr, proposal, proposedSession)
		return false

	default:
		t.hushProto.Errorf("while validating incoming individual session proposal: %v", err)
		return true

	case nil:
		// Yay
	}

	// // If this is a replacement session proposal, delete our original proposal from the DB
	// if (proposedSession.ReplacesSessionWithHash != types.Hash{}) {
	// 	err := t.hushProto.store.DeleteOutgoingIndividualSessionProposal(proposedSession.ReplacesSessionWithHash)
	// 	if err != nil {
	// 		t.hushProto.Errorf("while deleting outgoing individual session proposal: %v", err)
	// 	}
	// }

	dhPair, err := t.hushProto.store.EnsureDHPair()
	if err != nil {
		t.hushProto.Errorf("while fetching DH pair: %v", err)
		return true
	}

	// Save the session to the DB
	_, err = doubleratchet.New(
		proposedSession.SessionID.Bytes(),
		proposedSession.SharedKey[:],
		dhPair,
		t.hushProto.store.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(t.hushProto.store.RatchetKeyStore()),
	)
	if err != nil {
		t.hushProto.Errorf("while initializing double ratchet session: %v", err)
		return true
	}

	err = t.hushProto.store.SaveApprovedIndividualSession(proposedSession)
	if err != nil {
		t.hushProto.Errorf("while saving approved individual session: %v", err)
		return true
	}

	t.hushProto.onSessionOpened(t.proposalHash, proposedSession.SessionID.AliceAddr)

	// Prepare the approval message for sending
	proposalHash, err := proposedSession.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing individual session proposal: %v", err)
		return true
	}
	sig, err := identity.SignHash(proposalHash)
	if err != nil {
		t.hushProto.Errorf("while signing individual session proposal hash: %v", err)
		return true
	}
	approval := IndividualSessionApproval{
		ProposalHash: proposalHash,
		BobSig:       sig,
	}
	err = t.hushProto.store.SaveOutgoingIndividualSessionApproval(proposedSession.SessionID.AliceAddr, approval)
	if err != nil {
		t.hushProto.Errorf("while saving outgoing individual session approval: %v", err)
		return true
	}

	// Delete the incoming proposal
	err = t.hushProto.store.DeleteIncomingIndividualSessionProposal(proposal)
	if err != nil {
		t.hushProto.Errorf("while deleting incoming individual session proposal: %v", err)
		return true
	}

	t.hushProto.Debugf("Add approveIndividualSession")
	t.hushProto.poolWorker.Add(approveIndividualSession{proposedSession.SessionID.AliceAddr, proposalHash, t.hushProto})
	return false
}

func (t handleIncomingIndividualSession) decryptProposal(proposal EncryptedIndividualSessionProposal) (IndividualSessionProposal, error) {
	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		return IndividualSessionProposal{}, errors.Wrapf(err, "while fetching default public identity")
	}

	alices := t.hushProto.peerStore.PeersWithAddress(proposal.AliceAddr)
	if len(alices) == 0 {
		return IndividualSessionProposal{}, errors.Wrapf(err, "no known peer with address %v", proposal.AliceAddr)
	}
	alice := alices[0]

	_, aliceAsymEncPubkey := alice.PublicKeys(proposal.AliceAddr)

	sessionBytes, err := t.hushProto.keyStore.OpenMessageFrom(identity.Address(), aliceAsymEncPubkey, proposal.EncryptedProposal)
	if err != nil {
		return IndividualSessionProposal{}, errors.Wrapf(err, "while decrypting shared key proposal")
	}

	var proposedSession IndividualSessionProposal
	err = proposedSession.Unmarshal(sessionBytes)
	if err != nil {
		return IndividualSessionProposal{}, errors.Wrapf(err, "while unmarshaling shared key proposal")
	}
	return proposedSession, nil
}

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidDHPubkey  = errors.New("invalid DH pubkey")
	ErrEpochTooLow      = errors.New("epoch too low")
)

func (t handleIncomingIndividualSession) validateProposal(sender types.Address, proposedSession IndividualSessionProposal) error {
	peers := t.hushProto.peerStore.PeersWithAddress(sender)
	if len(peers) == 0 {
		return errors.Errorf("no known peer with address %v", sender)
	}
	peer := peers[0]
	peerSigPubkey, _ := peer.PublicKeys(sender)

	// Check the signature
	sessionHash, err := proposedSession.Hash()
	if err != nil {
		return errors.Wrapf(err, "while hashing incoming individual session proposal")
	} else if !peerSigPubkey.VerifySignature(sessionHash, proposedSession.AliceSig) {
		return ErrInvalidSignature
	}

	// Check the DH pubkey against our latest
	dhPair, err := t.hushProto.store.EnsureDHPair()
	if err != nil {
		return errors.Wrapf(err, "while fetching DH keypair from database")
	}
	if !bytes.Equal(dhPair.Public, proposedSession.RemoteDHPubkey) {
		return ErrInvalidDHPubkey
	}

	// Check to see if this is a re-send of a known proposal (same sessionID and shared key)
	var sessionExists bool
	existingSession, err := t.hushProto.store.IndividualSessionByID(proposedSession.SessionID)
	if errors.Cause(err) == errors.Err404 {
		// Do nothing
	} else if err != nil {
		return errors.Wrapf(err, "while checking database for session")
	} else {
		sessionExists = true
	}

	if sessionExists {
		if existingSession.SharedKey != proposedSession.SharedKey {
			return ErrEpochTooLow
		}

	} else {
		// Check to see if the epoch is high enough
		latestSession, err := t.hushProto.store.LatestIndividualSessionWithUsers(proposedSession.SessionID.SessionType, proposedSession.SessionID.AliceAddr, proposedSession.SessionID.BobAddr)
		if errors.Cause(err) == errors.Err404 {
			// We have no sessions, no issue with epoch
			return nil
		} else if err != nil {
			return errors.Wrapf(err, "while checking database for latest session")
		}

		if proposedSession.SessionID.Epoch <= latestSession.SessionID.Epoch {
			return ErrEpochTooLow
		}
	}

	return nil
}

func (t handleIncomingIndividualSession) proposeReplacementSession(myAddr types.Address, proposal EncryptedIndividualSessionProposal, proposedSession IndividualSessionProposal) {
	var replacementSessionID IndividualSessionID
	_, err := t.hushProto.store.LatestIndividualSessionWithUsers(proposedSession.SessionID.SessionType, myAddr, proposal.AliceAddr)
	if errors.Cause(err) == errors.Err404 {
		replacementSessionID = IndividualSessionID{
			SessionType: proposedSession.SessionID.SessionType,
			AliceAddr:   myAddr,
			BobAddr:     proposedSession.SessionID.AliceAddr,
			Epoch:       0,
		}

	} else if err != nil {
		t.hushProto.Errorf("while checking database for latest session ID: %v", err)
		return

	} else {
		replacementSessionID = IndividualSessionID{
			SessionType: proposedSession.SessionID.SessionType,
			AliceAddr:   myAddr,
			BobAddr:     proposedSession.SessionID.AliceAddr,
			Epoch:       proposedSession.SessionID.Epoch + 1,
		}
	}

	bobDHPubkeyAttestation, err := t.hushProto.store.LatestDHPubkeyFor(proposal.AliceAddr)
	if err != nil {
		t.hushProto.Errorf("while fetching DH pubkey for %v from database: %v", proposal.AliceAddr, err)
		return
	}

	proposedSessionHash, err := proposedSession.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing proposed session: %v", err)
		return
	}

	replacementSession := IndividualSessionProposal{
		SessionID:               replacementSessionID,
		SharedKey:               proposedSession.SharedKey,
		AliceSig:                nil,
		ReplacesSessionWithHash: proposedSessionHash,
		RemoteDHPubkey:          bobDHPubkeyAttestation.Pubkey,
	}

	replacementSessionHash, err := replacementSession.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing individual session proposal: %v", err)
		return
	}

	aliceSig, err := t.hushProto.keyStore.SignHash(myAddr, replacementSessionHash)
	if err != nil {
		t.hushProto.Errorf("while signing hash of individual session proposal: %v", err)
		return
	}
	replacementSession.AliceSig = aliceSig

	err = t.hushProto.store.SaveOutgoingIndividualSessionProposal(replacementSession)
	if err != nil {
		t.hushProto.Errorf("while suggesting next session ID: %v", err)
		return
	}

	err = t.hushProto.store.DeleteIncomingIndividualSessionProposal(proposal)
	if err != nil {
		t.hushProto.Errorf("while deleting incoming individual session proposal: %v", err)
		return
	}
}

type approveIndividualSession struct {
	aliceAddr    types.Address
	proposalHash types.Hash
	hushProto    *hushProtocol
}

var _ process.PoolWorkerItem = approveIndividualSession{}

func (t approveIndividualSession) ID() process.PoolUniqueID { return t }

func (t approveIndividualSession) Work(ctx context.Context) (retry bool) {
	approval, err := t.hushProto.store.OutgoingIndividualSessionApproval(t.aliceAddr, t.proposalHash)
	if errors.Cause(err) == errors.Err404 {
		// Done
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching incoming individual session approval from database: %v", err)
		return true
	}

	t.hushProto.Warnf("approving individual incoming session with %v (hash: %v)", t.aliceAddr, t.proposalHash)
	defer func() {
		if retry {
			t.hushProto.Warnf("approve incoming individual session %v: retrying later", t.proposalHash)
		} else {
			t.hushProto.Successf("approve incoming individual session %v: done", t.proposalHash)
		}
	}()

	t.hushProto.withPeers(ctx, 3*time.Second, t.hushProto.peerStore.PeersWithAddress(t.aliceAddr), func(ctx context.Context, alice HushPeerConn) error {
		err = alice.ApproveIndividualSession(ctx, approval)
		if err != nil {
			return err
		}
		err = t.hushProto.store.DeleteOutgoingIndividualSessionApproval(t.aliceAddr, approval.ProposalHash)
		if err != nil {
			return err
		}
		return nil
	})
	return false
}

type sendIndividualMessage struct {
	sessionType string
	recipient   types.Address
	id          types.ID
	hushProto   *hushProtocol
}

var _ process.PoolWorkerItem = sendIndividualMessage{}

func (t sendIndividualMessage) ID() process.PoolUniqueID { return t }

func (t sendIndividualMessage) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("send individual message %v: retrying later", t.id)
		} else {
			t.hushProto.Successf("send individual message %v: done", t.id)
		}
	}()

	intent, err := t.hushProto.store.OutgoingIndividualMessageIntent(t.sessionType, t.recipient, t.id)
	if errors.Cause(err) == errors.Err404 {
		// Done
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching message intent from database: %v", err)
		return true
	}

	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.hushProto.Errorf("while fetching default public identity from database: %v", err)
		return true
	}

	session, err := t.hushProto.store.LatestIndividualSessionWithUsers(intent.SessionType, identity.Address(), intent.Recipient)
	if errors.Cause(err) == errors.Err404 {
		// No established session, now check if we have a proposed session
		_, err = t.hushProto.store.OutgoingIndividualSessionProposalByUsersAndType(intent.SessionType, identity.Address(), intent.Recipient)
		if errors.Cause(err) == errors.Err404 {
			t.hushProto.Debugf("no individual session exists with %v, proposing epoch 0", intent.Recipient)

			err := t.hushProto.ProposeNextIndividualSession(ctx, intent.SessionType, intent.Recipient)
			if err != nil {
				t.hushProto.Errorf("while proposing next individual session with %v: %v", intent.Recipient, err)
				return true
			}

		} else if err != nil {
			t.hushProto.Errorf("while looking up session with ID: %v", err)
			return true
		}
		// Retry later once we have an established session
		return true

	} else if err != nil {
		t.hushProto.Errorf("while looking up session with ID: %v", err)
		return true
	}

	sessionHash, err := session.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing latest session %v: %v", session.SessionID, err)
		return true
	}
	t.hushProto.Successf("sending message (sessionHash=%v)", sessionHash)

	drsession, err := doubleratchet.Load(session.SessionID.Bytes(), t.hushProto.store.RatchetSessionStore())
	if err != nil {
		t.hushProto.Errorf("while loading doubleratchet session %v: %v", session.SessionID, err)
		return true
	}

	msg, err := drsession.RatchetEncrypt(intent.Plaintext, nil)
	if err != nil {
		t.hushProto.Errorf("while encrypting outgoing individual message (sessionID=%v): %v", session.SessionID, err)
		return true
	}

	t.hushProto.withPeers(ctx, 3*time.Second, t.hushProto.peerStore.PeersWithAddress(intent.Recipient), func(ctx context.Context, recipient HushPeerConn) error {
		err = recipient.SendHushIndividualMessage(ctx, IndividualMessageFromDoubleRatchetMessage(msg, sessionHash))
		if err != nil {
			return err
		}
		t.hushProto.Debugf("sent individual message to %v", intent.Recipient)

		err = t.hushProto.store.DeleteOutgoingIndividualMessageIntent(intent)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.hushProto.Errorf("while sending outgoing individual message (sessionID=%v): %v", session.SessionID, err)
		return true
	}
	return false
}

type decryptIncomingIndividualMessagesTask struct {
	process.PeriodicTask
	log.Logger
	hushProto  *hushProtocol
	store      Store
	peerStore  swarm.PeerStore
	keyStore   identity.KeyStore
	transports map[string]HushTransport
}

func NewDecryptIncomingIndividualMessagesTask(
	interval time.Duration,
	hushProto *hushProtocol,
) *decryptIncomingIndividualMessagesTask {
	t := &decryptIncomingIndividualMessagesTask{
		Logger:    log.NewLogger(ProtocolName),
		hushProto: hushProto,
	}
	t.PeriodicTask = *process.NewPeriodicTask("DecryptIncomingIndividualMessagesTask", utils.NewStaticTicker(interval), t.decryptIncomingIndividualMessages)
	return t
}

func (t *decryptIncomingIndividualMessagesTask) decryptIncomingIndividualMessages(ctx context.Context) {
	messages, err := t.hushProto.store.IncomingIndividualMessages()
	if err != nil {
		t.Errorf("while fetching incoming messages from database: %v", err)
		return
	}

	for _, msg := range messages {
		name := fmt.Sprintf("decrypt incoming message")
		msg := msg

		t.Process.Go(ctx, name, func(ctx context.Context) {
			t.decryptIncomingIndividualMessage(ctx, msg)
		})
	}
}

func (t *decryptIncomingIndividualMessagesTask) decryptIncomingIndividualMessage(ctx context.Context, msg IndividualMessage) {
	sessionID, err := t.hushProto.store.IndividualSessionIDBySessionHash(msg.SessionHash)
	if err != nil {
		t.Errorf("while looking up session with ID: %v", err)
		return
	}

	t.Debugf("decrypting incoming individual message (sessionID=%v)", sessionID)

	session, err := doubleratchet.Load(sessionID.Bytes(), t.hushProto.store.RatchetSessionStore())
	if err != nil {
		t.Errorf("while loading doubleratchet session %v: %v", sessionID, err)
		return
	}
	plaintext, err := session.RatchetDecrypt(msg.ToDoubleRatchetMessage(), nil)
	if err != nil {
		t.Errorf("while decrypting incoming individual message (sessionID=%v): %v", sessionID, err)
		return
	}
	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.Errorf("while fetching identity from key store: %v", err)
		return
	}

	var sender types.Address
	if sessionID.AliceAddr == identity.Address() {
		sender = sessionID.BobAddr
	} else {
		sender = sessionID.AliceAddr
	}

	t.Successf("decrypted incoming individual message (sessionID=%v): %v", sessionID, string(plaintext))

	t.hushProto.notifyIndividualMessageDecryptedListeners(sessionID.SessionType, sender, plaintext, msg)

	err = t.hushProto.store.DeleteIncomingIndividualMessage(msg)
	if err != nil {
		t.Errorf("while deleting incoming individual message (sessionID=%v): %v", sessionID, err)
		return
	}
}

type encryptGroupMessage struct {
	sessionType string
	id          string
	hushProto   *hushProtocol
}

var _ process.PoolWorkerItem = encryptGroupMessage{}

func (t encryptGroupMessage) ID() process.PoolUniqueID { return t }

func (t encryptGroupMessage) Work(ctx context.Context) (retry bool) {
	defer func() {
		if retry {
			t.hushProto.Warnf("encrypt group message %v-%v: retrying later", t.sessionType, t.id)
		} else {
			t.hushProto.Successf("encrypt group message %v-%v: done", t.sessionType, t.id)
		}
	}()

	err := t.hushProto.ensureSessionWithSelf(t.sessionType)
	if err != nil {
		t.hushProto.Errorf("while ensuring session with self: %v", err)
		return true
	}

	intent, err := t.hushProto.store.OutgoingGroupMessageIntent(t.sessionType, t.id)
	if errors.Cause(err) == errors.Err404 {
		// Done
		return false
	} else if err != nil {
		t.hushProto.Errorf("while fetching outgoing group message intent: %v", err)
		return true
	}

	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.hushProto.Errorf("while fetching default public identity from database: %v", err)
		return true
	}

	retry = t.ensureAllIndividualSessions(ctx, identity, intent)
	if retry {
		return true
	}

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

	var encryptionKeys []GroupMessage_EncryptionKey
	for _, recipient := range intent.Recipients {
		var session IndividualSessionProposal
		if recipient == identity.Address() {
			session, err = t.hushProto.store.IndividualSessionByID(IndividualSessionID{t.sessionType + ":self:send", recipient, recipient, 0})
			if err != nil {
				t.hushProto.Errorf("while fetching group:self:send session: %v", err)
				retry = true
				continue
			}
		} else {
			session, err = t.hushProto.store.LatestIndividualSessionWithUsers(t.sessionType, identity.Address(), recipient)
			if err != nil {
				t.hushProto.Errorf("while fetching group session for group message: %v", err)
				retry = true
				continue
			}
		}
		sessionHash, err := session.Hash()
		if err != nil {
			t.hushProto.Errorf("while hashing latest session %v: %v", session.SessionID, err)
			retry = true
			continue
		}

		drsession, err := doubleratchet.Load(session.SessionID.Bytes(), t.hushProto.store.RatchetSessionStore())
		if err != nil {
			t.hushProto.Errorf("while loading doubleratchet session %v: %v", session.SessionID, err)
			retry = true
			continue
		}
		msg, err := drsession.RatchetEncrypt(symEncKey.Bytes(), nil)
		if err != nil {
			t.hushProto.Errorf("while encrypting outgoing group message (sessionID=%v): %v", session.SessionID, err)
			retry = true
			continue
		}
		encryptionKeys = append(encryptionKeys, GroupMessage_EncryptionKey{
			Recipient: recipient,
			Key:       IndividualMessageFromDoubleRatchetMessage(msg, sessionHash),
		})
	}
	if retry {
		return true
	}

	msg := GroupMessage{
		SessionType:    intent.SessionType,
		ID:             intent.ID,
		EncryptionKeys: encryptionKeys,
		Ciphertext:     symEncMsg.Bytes(),
	}
	hash, err := msg.Hash()
	if err != nil {
		t.hushProto.Errorf("while hashing outgoing group message: %v", err)
		return true
	}
	sig, err := identity.SignHash(hash)
	if err != nil {
		t.hushProto.Errorf("while signing hash of outgoing group message: %v", err)
		return true
	}
	msg.Sig = sig

	t.hushProto.notifyGroupMessageEncryptedListeners(msg)

	err = t.hushProto.store.DeleteOutgoingGroupMessageIntent(intent.SessionType, intent.ID)
	if err != nil {
		t.hushProto.Errorf("while deleting outgoing group message: %v", err)
		return true
	}
	return false
}

func (t encryptGroupMessage) ensureAllIndividualSessions(ctx context.Context, identity identity.Identity, intent GroupMessageIntent) (retry bool) {
	for _, recipient := range intent.Recipients {
		if recipient == identity.Address() {
			_, err := t.hushProto.store.IndividualSessionByID(IndividualSessionID{intent.SessionType + ":self:send", recipient, recipient, 0})
			if err != nil {
				t.hushProto.Errorf("while fetching group:self:send session: %v", err)
				panic(err)
			}
		} else {
			_, err := t.hushProto.store.LatestIndividualSessionWithUsers(intent.SessionType, identity.Address(), recipient)
			if errors.Cause(err) == errors.Err404 {
				err = t.hushProto.ProposeIndividualSession(ctx, intent.SessionType, recipient, 0)
				if err != nil {
					t.hushProto.Errorf("while proposing individual session: %v", err)
				}
				retry = true
				continue

			} else if err != nil {
				t.hushProto.Errorf("while fetching group session for group message: %v", err)
				retry = true
				continue
			}
		}
	}
	return
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
		name := fmt.Sprintf("decrypt incoming message")
		msg := msg

		t.Process.Go(ctx, name, func(ctx context.Context) {
			t.decryptIncomingGroupMessage(ctx, msg)
		})
	}
}

func (t *decryptIncomingGroupMessagesTask) decryptIncomingGroupMessage(ctx context.Context, msg GroupMessage) {
	identity, err := t.hushProto.keyStore.DefaultPublicIdentity()
	if err != nil {
		t.hushProto.Errorf("while fetching default public identity from keystore: %v", err)
		return
	}

	var encEncKey GroupMessage_EncryptionKey
	var found bool
	for _, key := range msg.EncryptionKeys {
		if key.Recipient == identity.Address() {
			encEncKey = key
			found = true
			break
		}
	}
	if !found {
		return
	}

	sessionID, err := t.hushProto.store.IndividualSessionIDBySessionHash(encEncKey.Key.SessionHash)
	if err != nil {
		t.hushProto.Errorf("while looking up session with ID: %v", err)
		return
	}

	sender, err := t.hushProto.otherPartyInIndividualSession(sessionID)
	if err != nil {
		t.hushProto.Errorf("while determining other party in individual session: %v", err)
		return
	}

	if sender == identity.Address() {
		sessionID = IndividualSessionID{msg.SessionType + ":self:recv", identity.Address(), identity.Address(), 0}
	}

	t.hushProto.Debugf("decrypting incoming group message (sessionID=%v)", sessionID)

	session, err := doubleratchet.Load(sessionID.Bytes(), t.hushProto.store.RatchetSessionStore())
	if err != nil {
		t.hushProto.Errorf("while loading doubleratchet session %v: %v", sessionID, err)
		return
	}
	encKeyBytes, err := session.RatchetDecrypt(encEncKey.Key.ToDoubleRatchetMessage(), nil)
	if err != nil {
		t.hushProto.Errorf("while decrypting incoming group message (sessionID=%v): %v", sessionID, err)
		return
	}

	symEncKey := crypto.SymEncKeyFromBytes(encKeyBytes)
	symEncMsg := crypto.SymEncMsgFromBytes(msg.Ciphertext)

	plaintext, err := symEncKey.Decrypt(symEncMsg)
	if err != nil {
		t.hushProto.Errorf("while decrypting incoming group message (sessionID=%v): %v", sessionID, err)
		return
	}

	t.hushProto.notifyGroupMessageDecryptedListeners(sender, plaintext, msg)

	t.Successf("decrypted incoming group message (sessionID=%v)", sessionID)

	err = t.hushProto.store.DeleteIncomingGroupMessage(msg)
	if err != nil {
		t.Errorf("while deleting incoming group message (sessionID=%v): %v", sessionID, err)
		return
	}
}

func (hp *hushProtocol) otherPartyInIndividualSession(sessionID IndividualSessionID) (types.Address, error) {
	identity, err := hp.keyStore.DefaultPublicIdentity()
	if err != nil {
		return types.Address{}, errors.Wrapf(err, "while fetching identity from key store")
	}
	if sessionID.AliceAddr == identity.Address() {
		return sessionID.BobAddr, nil
	}
	return sessionID.AliceAddr, nil
}

func (hp *hushProtocol) onSessionOpened(proposalHash types.Hash, peerAddr types.Address) {
	sessionID, err := hp.store.IndividualSessionIDBySessionHash(proposalHash)
	if err != nil {
		hp.Errorf("while fetching individual session ID from database: %v", err)
		return
	}

	// Outgoing individual messages
	iids, err := hp.store.OutgoingIndividualMessageIntentIDsForTypeAndRecipient(sessionID.SessionType, peerAddr)
	if err != nil {
		hp.Errorf("while fetching individual message IDs from database: %v", err)
		return
	}
	for id := range iids {
		hp.poolWorker.ForceRetry(sendIndividualMessage{sessionID.SessionType, peerAddr, id, hp})
	}

	// Outgoing group messages
	gids, err := hp.store.OutgoingGroupMessageIntentIDsForSessionType(sessionID.SessionType)
	if err != nil {
		hp.Errorf("while fetching group message IDs from database: %v", err)
		return
	}
	for id := range gids {
		hp.poolWorker.ForceRetry(encryptGroupMessage{sessionID.SessionType, id, hp})
	}

	// Incoming individual + group messages
	hp.decryptIncomingIndividualMessagesTask.Enqueue()
	hp.decryptIncomingGroupMessagesTask.Enqueue()
}

func (hp *hushProtocol) withPeers(
	ctx context.Context,
	attemptTimeout time.Duration,
	peers []swarm.PeerInfo,
	fn func(ctx context.Context, hushPeerConn HushPeerConn) error,
) {
	tpts := make(map[string]swarm.Transport)
	for k, v := range hp.transports {
		tpts[k] = v
	}

	var pools []*swarm.EndpointPool
	for _, peer := range peers {
		pool := swarm.NewEndpointPool("", peer, tpts, attemptTimeout, 3*time.Second, func(ctx context.Context, peerConn swarm.PeerConn) error {
			return fn(ctx, peerConn.(HushPeerConn))
		})

		err := hp.SpawnChild(ctx, pool)
		if err != nil {
			hp.Errorf("while spawning endpoint pool: %v", err)
			continue
		}
		pools = append(pools, pool)
	}

	go func() {
		<-ctx.Done()
		for _, pool := range pools {
			pool.Close()
		}

	}()
	for _, pool := range pools {
		<-pool.Done()
	}
}
