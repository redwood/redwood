package protohush

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"

	"github.com/status-im/doubleratchet"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type store struct {
	log.Logger
	db *state.DBTree
	mu sync.RWMutex
}

var _ Store = (*store)(nil)

func NewStore(db *state.DBTree) *store {
	return &store{
		Logger: log.NewLogger("store"),
		db:     db,
	}
}

func (s *store) EnsureDHPair() (DHPair, error) {
	dhPair, err := s.getDHPair()
	if err == nil {
		return dhPair, nil
	} else if errors.Cause(err) == errors.Err404 {
		// no-op
	} else if err != nil {
		return DHPair{}, err
	}

	dh, err := doubleratchet.DefaultCrypto{}.GenerateDH()
	if err != nil {
		return DHPair{}, errors.Wrapf(err, "while generating DH keypair")
	}

	dhPair = DHPair{Public: dh.PublicKey(), Private: dh.PrivateKey()}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(dhPairKeypath.Copy(), nil, dhPair)
	if err != nil {
		return DHPair{}, err
	}
	err = node.Save()
	if err != nil {
		return DHPair{}, err
	}

	dhPair, err = s.getDHPair()
	if err != nil {
		return DHPair{}, err
	}

	return dhPair, nil
}

func (s *store) getDHPair() (DHPair, error) {
	node := s.db.State(false)
	defer node.Close()

	exists, err := node.Exists(dhPairKeypath.Copy())
	if err != nil {
		return DHPair{}, err
	} else if !exists {
		return DHPair{}, errors.Err404
	}

	var dhPair DHPair
	err = node.NodeAt(dhPairKeypath.Copy(), nil).Scan(&dhPair)
	if err != nil {
		return DHPair{}, err
	}
	return dhPair, nil
}

func (s *store) DHPubkeyAttestations() ([]DHPubkeyAttestation, error) {
	node := s.db.State(false)
	defer node.Close()

	var attestationsByAddr map[string]map[uint64]DHPubkeyAttestation
	err := node.NodeAt(dhPubkeyAttestationsKeypath.Copy(), nil).Scan(&attestationsByAddr)
	if err != nil {
		return nil, err
	}

	var attestations []DHPubkeyAttestation
	for _, as := range attestationsByAddr {
		for _, a := range as {
			attestations = append(attestations, a)
		}
	}
	return attestations, nil
}

func (s *store) LatestDHPubkeyFor(addr types.Address) (DHPubkeyAttestation, error) {
	node := s.db.State(false)
	defer node.Close()

	var attestations map[uint64]DHPubkeyAttestation
	err := node.NodeAt(dhPubkeyAttestationsKeypathFor(addr), nil).Scan(&attestations)
	if err != nil {
		return DHPubkeyAttestation{}, err
	}

	if len(attestations) == 0 {
		return DHPubkeyAttestation{}, errors.Err404
	}

	var maxEpoch uint64
	for epoch := range attestations {
		if epoch > maxEpoch {
			maxEpoch = epoch
		}
	}
	return attestations[maxEpoch], nil
}

func (s *store) SaveDHPubkeyAttestations(attestations []DHPubkeyAttestation) error {
	node := s.db.State(true)
	defer node.Close()

	for _, a := range attestations {
		addr, err := a.Address()
		if err != nil {
			s.Errorf("bad DH pubkey attestation: %v", a)
			continue
		}

		err = node.Set(dhPubkeyAttestationsKeypathFor(addr).Pushs(fmt.Sprintf("%v", a.Epoch)), nil, a)
		if err != nil {
			s.Errorf("error saving DH pubkey attestation %v: %v", a, err)
			continue
		}
	}
	return node.Save()
}

func (s *store) OutgoingIndividualSessionProposalHashes() (types.HashSet, error) {
	node := s.db.State(false)
	defer node.Close()

	hashes := types.NewHashSet(nil)
	for _, subkey := range node.NodeAt(outgoingIndividualSessionProposalsByHashKeypath.Copy(), nil).Subkeys() {
		hash, err := types.HashFromHex(subkey.String())
		if err != nil {
			return nil, err
		}
		hashes.Add(hash)
	}
	return hashes, nil
}

func (s *store) OutgoingIndividualSessionProposalsForUsers(aliceAddr, bobAddr types.Address) ([]IndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	var m map[string]types.Hash
	err := node.NodeAt(outgoingIndividualSessionProposalKeypathForUsers(aliceAddr, bobAddr), nil).Scan(&m)
	if err != nil {
		return nil, err
	}

	var proposals []IndividualSessionProposal
	for _, hash := range m {
		proposal, err := s.OutgoingIndividualSessionProposalByHash(hash)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	}
	return proposals, nil
}

func (s *store) OutgoingIndividualSessionProposalByHash(proposalHash types.Hash) (p IndividualSessionProposal, err error) {
	node := s.db.State(false)
	defer node.Close()

	exists, err := node.Exists(outgoingIndividualSessionProposalKeypathForHash(proposalHash))
	if err != nil {
		return IndividualSessionProposal{}, err
	} else if !exists {
		return IndividualSessionProposal{}, errors.Err404
	}

	var proposal IndividualSessionProposal
	err = node.NodeAt(outgoingIndividualSessionProposalKeypathForHash(proposalHash), nil).Scan(&proposal)
	if err != nil {
		return IndividualSessionProposal{}, err
	}
	return proposal, nil
}

func (s *store) OutgoingIndividualSessionProposalByUsersAndType(sessionType string, aliceAddr, bobAddr types.Address) (IndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := outgoingIndividualSessionProposalKeypathForUsersAndType(sessionType, aliceAddr, bobAddr)

	exists, err := node.Exists(keypath)
	if err != nil {
		return IndividualSessionProposal{}, err
	} else if !exists {
		return IndividualSessionProposal{}, errors.Err404
	}

	var proposalHash types.Hash
	err = node.NodeAt(outgoingIndividualSessionProposalKeypathForUsersAndType(sessionType, aliceAddr, bobAddr), nil).Scan(&proposalHash)
	if err != nil {
		return IndividualSessionProposal{}, err
	}
	return s.OutgoingIndividualSessionProposalByHash(proposalHash)
}

func (s *store) SaveOutgoingIndividualSessionProposal(proposal IndividualSessionProposal) error {
	node := s.db.State(true)
	defer node.Close()

	proposalHash, err := proposal.Hash()
	if err != nil {
		return err
	}

	id := proposal.SessionID
	err = node.Set(outgoingIndividualSessionProposalKeypathForUsersAndType(id.SessionType, id.AliceAddr, id.BobAddr), nil, proposalHash)
	if err != nil {
		return err
	}
	err = node.Set(outgoingIndividualSessionProposalKeypathForHash(proposalHash), nil, proposal)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteOutgoingIndividualSessionProposal(proposalHash types.Hash) error {
	proposal, err := s.OutgoingIndividualSessionProposalByHash(proposalHash)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	id := proposal.SessionID
	err = node.Delete(outgoingIndividualSessionProposalKeypathForUsersAndType(id.SessionType, id.AliceAddr, id.BobAddr), nil)
	if err != nil {
		return err
	}

	err = node.Delete(outgoingIndividualSessionProposalKeypathForHash(proposalHash), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) IncomingIndividualSessionProposals() (map[types.Hash]EncryptedIndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	var proposals map[types.Hash]EncryptedIndividualSessionProposal
	err := node.NodeAt(incomingIndividualSessionProposalsKeypath.Copy(), nil).Scan(&proposals)
	if err != nil {
		return nil, err
	}
	return proposals, nil
}

func (s *store) IncomingIndividualSessionProposal(hash types.Hash) (EncryptedIndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := incomingIndividualSessionProposalKeypathFor(hash)
	exists, err := node.Exists(keypath)
	if err != nil {
		return EncryptedIndividualSessionProposal{}, err
	} else if !exists {
		return EncryptedIndividualSessionProposal{}, errors.Err404
	}

	var proposal EncryptedIndividualSessionProposal
	err = node.NodeAt(keypath, nil).Scan(&proposal)
	if err != nil {
		return EncryptedIndividualSessionProposal{}, err
	}
	return proposal, nil
}

func (s *store) SaveIncomingIndividualSessionProposal(proposal EncryptedIndividualSessionProposal) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(incomingIndividualSessionProposalKeypathFor(proposal.Hash()), nil, proposal)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteIncomingIndividualSessionProposal(proposal EncryptedIndividualSessionProposal) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(incomingIndividualSessionProposalKeypathFor(proposal.Hash()), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) OutgoingIndividualSessionResponses() (map[types.Address]map[types.Hash]IndividualSessionResponse, error) {
	node := s.db.State(false)
	defer node.Close()

	var approvals_ map[string]map[types.Hash]IndividualSessionResponse
	err := node.NodeAt(outgoingIndividualSessionResponsesKeypath.Copy(), nil).Scan(&approvals_)
	if err != nil {
		return nil, err
	}

	approvals := make(map[types.Address]map[types.Hash]IndividualSessionResponse)
	for a, x := range approvals_ {
		addr, err := types.AddressFromHex(a)
		if err != nil {
			return nil, err
		}
		approvals[addr] = x
	}
	return approvals, nil
}

func (s *store) OutgoingIndividualSessionResponse(aliceAddr types.Address, proposalHash types.Hash) (IndividualSessionResponse, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := outgoingIndividualSessionResponseKeypathFor(aliceAddr, proposalHash)
	exists, err := node.Exists(keypath)
	if err != nil {
		return IndividualSessionResponse{}, err
	} else if !exists {
		return IndividualSessionResponse{}, errors.Err404
	}

	var approval IndividualSessionResponse
	err = node.NodeAt(keypath, nil).Scan(&approval)
	if err != nil {
		return IndividualSessionResponse{}, err
	}
	return approval, nil
}

func (s *store) OutgoingIndividualSessionResponsesForUser(aliceAddr types.Address) ([]IndividualSessionResponse, error) {
	node := s.db.State(false)
	defer node.Close()

	var approvalsMap map[string]IndividualSessionResponse
	err := node.NodeAt(outgoingIndividualSessionResponsesKeypathForUser(aliceAddr), nil).Scan(&approvalsMap)
	if err != nil {
		return nil, err
	}
	var approvals []IndividualSessionResponse
	for _, a := range approvalsMap {
		approvals = append(approvals, a)
	}
	return approvals, nil
}

func (s *store) SaveOutgoingIndividualSessionResponse(aliceAddr types.Address, approval IndividualSessionResponse) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(outgoingIndividualSessionResponseKeypathFor(aliceAddr, approval.ProposalHash), nil, approval)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteOutgoingIndividualSessionResponse(aliceAddr types.Address, proposalHash types.Hash) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(outgoingIndividualSessionResponseKeypathFor(aliceAddr, proposalHash), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) SaveApprovedIndividualSession(session IndividualSessionProposal) error {
	hash, err := session.Hash()
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(individualSessionIDKeypathForHash(hash), nil, session.SessionID)
	if err != nil {
		return err
	}
	err = node.Set(individualSessionKeypathForID(session.SessionID), nil, session)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) IndividualSessionIDBySessionHash(hash types.Hash) (IndividualSessionID, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := individualSessionIDKeypathForHash(hash)

	exists, err := node.Exists(keypath)
	if err != nil {
		return IndividualSessionID{}, err
	} else if !exists {
		return IndividualSessionID{}, errors.Err404
	}

	var sessionID IndividualSessionID
	err = node.NodeAt(keypath, nil).Scan(&sessionID)
	if err != nil {
		return IndividualSessionID{}, err
	}
	return sessionID, nil
}

func (s *store) LatestIndividualSessionWithUsers(sessionType string, aliceAddr, bobAddr types.Address) (IndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := individualSessionsKeypathForUsersAndType(sessionType, aliceAddr, bobAddr)

	exists, err := node.Exists(keypath)
	if err != nil {
		return IndividualSessionProposal{}, err
	} else if !exists {
		return IndividualSessionProposal{}, errors.Err404
	}

	iter := node.ChildIterator(individualSessionsKeypathForUsersAndType(sessionType, aliceAddr, bobAddr), false, 0)
	defer iter.Close()

	var maxEpoch uint64
	for iter.Rewind(); iter.Valid(); iter.Next() {
		epoch := state.DecodeSliceIndex(iter.Node().Keypath().Part(-1))
		if epoch > maxEpoch {
			maxEpoch = epoch
		}
	}

	session, err := s.IndividualSessionByID(IndividualSessionID{sessionType, aliceAddr, bobAddr, maxEpoch})
	if err != nil {
		return IndividualSessionProposal{}, err
	}
	return session, nil
}

func (s *store) IndividualSessionByID(id IndividualSessionID) (IndividualSessionProposal, error) {
	node := s.db.State(false)
	defer node.Close()

	exists, err := node.Exists(individualSessionKeypathForID(id))
	if err != nil {
		return IndividualSessionProposal{}, err
	} else if !exists {
		return IndividualSessionProposal{}, errors.Err404
	}

	var session IndividualSessionProposal
	err = node.NodeAt(individualSessionKeypathForID(id), nil).Scan(&session)
	if err != nil {
		return IndividualSessionProposal{}, err
	}
	return session, nil
}

func (s *store) OutgoingIndividualMessageIntents() ([]IndividualMessageIntent, error) {
	node := s.db.State(false)
	defer node.Close()

	var intentsMap map[string]map[string]map[string]IndividualMessageIntent
	err := node.NodeAt(outgoingIndividualMessagesKeypath.Copy(), nil).Scan(&intentsMap)
	if err != nil {
		return nil, err
	}
	var intents []IndividualMessageIntent
	for _, x := range intentsMap {
		for _, y := range x {
			for _, intent := range y {
				intents = append(intents, intent)
			}
		}
	}
	return intents, nil
}

func (s *store) OutgoingIndividualMessageIntent(sessionType string, recipient types.Address, id types.ID) (IndividualMessageIntent, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := outgoingIndividualMessageKeypathFor(IndividualMessageIntent{ID: id, SessionType: sessionType, Recipient: recipient})

	exists, err := node.Exists(keypath)
	if err != nil {
		return IndividualMessageIntent{}, err
	} else if !exists {
		return IndividualMessageIntent{}, errors.Err404
	}

	var intent IndividualMessageIntent
	err = node.NodeAt(keypath, nil).Scan(&intent)
	if err != nil {
		return IndividualMessageIntent{}, err
	}
	return intent, nil
}

func (s *store) OutgoingIndividualMessageIntentIDsForTypeAndRecipient(sessionType string, recipient types.Address) (types.IDSet, error) {
	node := s.db.State(false)
	defer node.Close()

	ids := types.NewIDSet(nil)
	for _, subkey := range node.NodeAt(outgoingIndividualMessagesKeypathForTypeAndRecipient(sessionType, recipient), nil).Subkeys() {
		hash, err := types.IDFromHex(subkey.String())
		if err != nil {
			return nil, err
		}
		ids.Add(hash)
	}
	return ids, nil
}

func (s *store) SaveOutgoingIndividualMessageIntent(intent IndividualMessageIntent) error {
	if (intent.ID == types.ID{}) {
		return errors.New("message has no id")
	}

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(outgoingIndividualMessageKeypathFor(intent), nil, intent)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteOutgoingIndividualMessageIntent(intent IndividualMessageIntent) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(outgoingIndividualMessageKeypathFor(intent), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) IncomingIndividualMessages() ([]IndividualMessage, error) {
	node := s.db.State(false)
	defer node.Close()

	var mss map[string]map[uint64]IndividualMessage
	err := node.NodeAt(incomingIndividualMessagesKeypath.Copy(), nil).Scan(&mss)
	if err != nil {
		return nil, err
	}
	var messages []IndividualMessage
	for _, ms := range mss {
		for _, m := range ms {
			messages = append(messages, m)
		}
	}
	return messages, nil
}

func (s *store) SaveIncomingIndividualMessage(sender types.Address, msg IndividualMessage) error {

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(incomingIndividualMessageKeypathFor(msg), nil, msg)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteIncomingIndividualMessage(msg IndividualMessage) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(incomingIndividualMessageKeypathFor(msg), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) OutgoingGroupMessageIntentIDs(sessionType string) (types.StringSet, error) {
	node := s.db.State(false)
	defer node.Close()

	ids := types.NewStringSet(nil)
	for _, subkey := range node.Subkeys() {
		ids.Add(subkey.String())
	}
	return ids, nil
}

func (s *store) OutgoingGroupMessageIntent(sessionType, id string) (GroupMessageIntent, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := outgoingGroupMessageIntentKeypathFor(sessionType, id)

	exists, err := node.Exists(keypath)
	if err != nil {
		return GroupMessageIntent{}, err
	} else if !exists {
		return GroupMessageIntent{}, errors.Err404
	}

	var intent GroupMessageIntent
	err = node.NodeAt(keypath, nil).Scan(&intent)
	if err != nil {
		return GroupMessageIntent{}, err
	}
	return intent, nil
}

func (s *store) SaveOutgoingGroupMessageIntent(intent GroupMessageIntent) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(outgoingGroupMessageIntentKeypathFor(intent.SessionType, intent.ID), nil, intent)
	if err != nil {
		return err
	}

	return node.Save()
}

func (s *store) DeleteOutgoingGroupMessageIntent(sessionType, id string) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(outgoingGroupMessageIntentKeypathFor(sessionType, id), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) IncomingGroupMessages() ([]GroupMessage, error) {
	node := s.db.State(false)
	defer node.Close()

	var messagesMap map[string]GroupMessage
	err := node.NodeAt(incomingGroupMessagesKeypath.Copy(), nil).Scan(&messagesMap)
	if err != nil {
		return nil, err
	}
	var messages []GroupMessage
	for _, msg := range messagesMap {
		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *store) SaveIncomingGroupMessage(sender types.Address, msg GroupMessage) error {
	node := s.db.State(true)
	defer node.Close()

	msgHash, err := msg.Hash()
	if err != nil {
		return err
	}
	err = node.Set(incomingGroupMessageKeypathFor(msgHash), nil, msg)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteIncomingGroupMessage(msg GroupMessage) error {

	node := s.db.State(true)
	defer node.Close()

	msgHash, err := msg.Hash()
	if err != nil {
		return err
	}

	err = node.Delete(incomingGroupMessageKeypathFor(msgHash), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(rootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}

func (ks *store) RatchetSessionStore() RatchetSessionStore {
	return (*ratchetSessionStore)(ks)
}

func (ks *store) RatchetKeyStore() RatchetKeyStore {
	return (*ratchetKeyStore)(ks)
}

type ratchetSessionStore store

var _ RatchetSessionStore = (*ratchetSessionStore)(nil)

type ratchetSessionCodec struct {
	DHr                      doubleratchet.Key `tree:"DHr"`
	DHs                      DHPair            `tree:"DHs"`
	RootChainKey             doubleratchet.Key `tree:"RootChainKey"`
	SendChainKey             doubleratchet.Key `tree:"SendChainKey"`
	SendChainN               uint64            `tree:"SendChainN"`
	RecvChainKey             doubleratchet.Key `tree:"RecvChainKey"`
	RecvChainN               uint64            `tree:"RecvChainN"`
	PN                       uint64            `tree:"PN"`
	MaxSkip                  uint64            `tree:"MaxSkip"`
	HKr                      doubleratchet.Key `tree:"HKr"`
	NHKr                     doubleratchet.Key `tree:"NHKr"`
	HKs                      doubleratchet.Key `tree:"HKs"`
	NHKs                     doubleratchet.Key `tree:"NHKs"`
	MaxKeep                  uint64            `tree:"MaxKeep"`
	MaxMessageKeysPerSession int64             `tree:"MaxMessageKeysPerSession"`
	Step                     uint64            `tree:"Step"`
	KeysCount                uint64            `tree:"KeysCount"`
}

func (s *ratchetSessionStore) loadSharedKey(sessionIDBytes []byte) ([]byte, error) {
	var sessionID IndividualSessionID
	err := sessionID.UnmarshalText(sessionIDBytes)
	if err != nil {
		return nil, err
	}
	session, err := (*store)(s).IndividualSessionByID(sessionID)
	if err != nil {
		return nil, err
	}
	return session.SharedKey[:], nil
}

func (s *ratchetSessionStore) Load(sessionID []byte) (*doubleratchet.State, error) {
	node := s.db.State(false)
	defer node.Close()

	sharedKey, err := s.loadSharedKey(sessionID)
	if err != nil {
		return nil, err
	}

	var codec ratchetSessionCodec
	err = node.NodeAt(ratchetSessionKeypathFor(sessionID), nil).Scan(&codec)
	if err != nil {
		return nil, err
	}

	state := doubleratchet.DefaultState(sharedKey)
	state.Crypto = doubleratchet.DefaultCrypto{}
	state.DHr = codec.DHr
	state.DHs = codec.DHs
	state.RootCh.CK = codec.RootChainKey
	state.SendCh.CK = codec.SendChainKey
	state.SendCh.N = uint32(codec.SendChainN)
	state.RecvCh.CK = codec.RecvChainKey
	state.RecvCh.N = uint32(codec.RecvChainN)
	state.PN = uint32(codec.PN)
	state.MkSkipped = (*ratchetKeyStore)(s)
	state.MaxSkip = uint(codec.MaxSkip)
	state.HKr = codec.HKr
	state.NHKr = codec.NHKr
	state.HKs = codec.HKs
	state.NHKs = codec.NHKs
	state.MaxKeep = uint(codec.MaxKeep)
	state.MaxMessageKeysPerSession = int(codec.MaxMessageKeysPerSession)
	state.Step = uint(codec.Step)
	state.KeysCount = uint(codec.KeysCount)
	return &state, nil
}

func (s *ratchetSessionStore) Save(sessionID []byte, state *doubleratchet.State) error {
	node := s.db.State(true)
	defer node.Close()

	codec := ratchetSessionCodec{
		DHr:                      state.DHr,
		DHs:                      DHPair{Private: state.DHs.PrivateKey(), Public: state.DHs.PublicKey()},
		RootChainKey:             state.RootCh.CK,
		SendChainKey:             state.SendCh.CK,
		SendChainN:               uint64(state.SendCh.N),
		RecvChainKey:             state.RecvCh.CK,
		RecvChainN:               uint64(state.RecvCh.N),
		PN:                       uint64(state.PN),
		MaxSkip:                  uint64(state.MaxSkip),
		HKr:                      state.HKr,
		NHKr:                     state.NHKr,
		HKs:                      state.HKs,
		NHKs:                     state.NHKs,
		MaxKeep:                  uint64(state.MaxKeep),
		MaxMessageKeysPerSession: int64(state.MaxMessageKeysPerSession),
		Step:                     uint64(state.Step),
		KeysCount:                uint64(state.KeysCount),
	}

	err := node.Set(ratchetSessionKeypathFor(sessionID), nil, codec)
	if err != nil {
		return err
	}
	return node.Save()
}

type ratchetKeyStore store

var _ RatchetKeyStore = (*ratchetKeyStore)(nil)

type ratchetKeyCodec struct {
	MessageKey doubleratchet.Key
	SeqNum     uint64
	SessionID  []byte
}

// Get returns a message key by the given key and message number.
func (s *ratchetKeyStore) Get(pubKey doubleratchet.Key, msgNum uint) (mk doubleratchet.Key, ok bool, err error) {
	node := s.db.State(false)
	defer node.Close()

	innerNode := node.NodeAt(ratchetMsgkeyKeypath(pubKey, msgNum), nil)

	exists, err := innerNode.Exists(nil)
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}

	var codec ratchetKeyCodec
	err = innerNode.Scan(&codec)
	if errors.Cause(err) == errors.Err404 {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return codec.MessageKey, true, nil
}

// Put saves the given mk under the specified key and msgNum.
func (s *ratchetKeyStore) Put(sessionID []byte, pubKey doubleratchet.Key, msgNum uint, mk doubleratchet.Key, keySeqNum uint) error {
	node := s.db.State(true)
	defer node.Close()

	codec := ratchetKeyCodec{
		MessageKey: mk,
		SeqNum:     uint64(keySeqNum),
		SessionID:  sessionID,
	}

	err := node.Set(ratchetMsgkeyKeypath(pubKey, msgNum), nil, codec)
	if err != nil {
		return err
	}
	return node.Save()
}

// DeleteMk ensures there's no message key under the specified key and msgNum.
func (s *ratchetKeyStore) DeleteMk(k doubleratchet.Key, msgNum uint) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(ratchetMsgkeyKeypath(k, msgNum), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

// DeleteOldMKeys deletes old message keys for a session.
func (s *ratchetKeyStore) DeleteOldMks(sessionID []byte, deleteUntilSeqKey uint) error {
	// node := s.db.State(true)
	// defer node.Close()

	// node.ChildIterator(ratchetPubkeyKeypathFor(k), prefetchValues, prefetchSize)
	return nil
}

// TruncateMks truncates the number of keys to maxKeys.
func (s *ratchetKeyStore) TruncateMks(sessionID []byte, maxKeys int) error {
	return nil
}

// Count returns number of message keys stored under the specified key.
func (s *ratchetKeyStore) Count(pubKey doubleratchet.Key) (uint, error) {
	node := s.db.State(false)
	defer node.Close()

	n := node.NodeAt(ratchetPubkeyKeypathFor(pubKey), nil).NumSubkeys()
	return uint(n), nil
}

// All returns all the keys
func (s *ratchetKeyStore) All() (map[string]map[uint]doubleratchet.Key, error) {
	node := s.db.State(false)
	defer node.Close()

	m := make(map[string]map[uint]doubleratchet.Key)

	iter := node.ChildIterator(messageKeysKeypath, true, 10)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		var key ratchetKeyCodec
		err := iter.Node().Scan(&key)
		if err != nil {
			return nil, err
		}

		if _, exists := m[string(key.SessionID)]; !exists {
			m[string(key.SessionID)] = make(map[uint]doubleratchet.Key)
		}
		m[string(key.SessionID)][uint(key.SeqNum)] = key.MessageKey
	}
	return m, nil
}

var (
	rootKeypath = state.Keypath("protohush")

	dhPubkeyAttestationsKeypath = rootKeypath.Copy().Pushs("dh-pubkey-attestations")
	dhPairKeypath               = rootKeypath.Copy().Pushs("dh-pairs")

	outgoingIndividualSessionProposalsKeypath        = rootKeypath.Copy().Pushs("proposals-out")
	outgoingIndividualSessionProposalsByHashKeypath  = outgoingIndividualSessionProposalsKeypath.Copy().Pushs("by-hash")
	outgoingIndividualSessionProposalsByUsersKeypath = outgoingIndividualSessionProposalsKeypath.Copy().Pushs("by-users")

	outgoingIndividualSessionResponsesKeypath = rootKeypath.Copy().Pushs("approvals-out")
	incomingIndividualSessionProposalsKeypath = rootKeypath.Copy().Pushs("proposals-in")

	individualSessionsKeypath       = rootKeypath.Copy().Pushs("individual-sessions")
	individualSessionsByIDKeypath   = individualSessionsKeypath.Copy().Pushs("by-id")
	individualSessionsByHashKeypath = individualSessionsKeypath.Copy().Pushs("by-hash")

	outgoingIndividualMessagesKeypath = rootKeypath.Copy().Pushs("outgoing-individual-messages")
	incomingIndividualMessagesKeypath = rootKeypath.Copy().Pushs("incoming-individual-messages")

	outgoingGroupMessagesKeypath       = rootKeypath.Copy().Pushs("outgoing-group-messages")
	outgoingGroupMessageIntentsKeypath = outgoingGroupMessagesKeypath.Copy().Pushs("intents")
	outgoingGroupMessageSendsKeypath   = outgoingGroupMessagesKeypath.Copy().Pushs("sends")

	incomingGroupMessagesKeypath = rootKeypath.Copy().Pushs("incoming-group-messages")

	sharedKeysKeypath  = rootKeypath.Copy().Pushs("sks")
	sessionsKeypath    = rootKeypath.Copy().Pushs("sessions")
	messageKeysKeypath = rootKeypath.Copy().Pushs("mks")
)

func dhPubkeyAttestationsKeypathFor(addr types.Address) state.Keypath {
	return dhPubkeyAttestationsKeypath.Copy().Pushs(addr.Hex())
}

func outgoingIndividualSessionProposalKeypathForHash(proposalHash types.Hash) state.Keypath {
	return outgoingIndividualSessionProposalsByHashKeypath.Copy().Pushs(proposalHash.Hex())
}

func outgoingIndividualSessionProposalKeypathForUsers(aliceAddr, bobAddr types.Address) state.Keypath {
	alice, bob := addrsSorted(aliceAddr, bobAddr)
	return outgoingIndividualSessionProposalsByUsersKeypath.Copy().Pushs(alice.Hex()).Pushs(bob.Hex())
}

func outgoingIndividualSessionProposalKeypathForUsersAndType(sessionType string, aliceAddr, bobAddr types.Address) state.Keypath {
	return outgoingIndividualSessionProposalKeypathForUsers(aliceAddr, bobAddr).Pushs(sessionType)
}

func incomingIndividualSessionProposalKeypathFor(proposalHash types.Hash) state.Keypath {
	return incomingIndividualSessionProposalsKeypath.Copy().Pushs(proposalHash.Hex())
}

func outgoingIndividualSessionResponsesKeypathForUser(aliceAddr types.Address) state.Keypath {
	return outgoingIndividualSessionResponsesKeypath.Copy().Pushs(aliceAddr.Hex())
}

func outgoingIndividualSessionResponseKeypathFor(aliceAddr types.Address, proposalHash types.Hash) state.Keypath {
	return outgoingIndividualSessionResponsesKeypathForUser(aliceAddr).Pushs(proposalHash.Hex())
}

func individualSessionsKeypathForUsers(aliceAddr, bobAddr types.Address) state.Keypath {
	a, b := addrsSorted(aliceAddr, bobAddr)
	return individualSessionsByIDKeypath.Copy().Pushs(a.Hex()).Pushs(b.Hex())
}

func individualSessionsKeypathForUsersAndType(sessionType string, aliceAddr, bobAddr types.Address) state.Keypath {
	return individualSessionsKeypathForUsers(aliceAddr, bobAddr).Pushs(sessionType)
}

func individualSessionKeypathForID(sessionID IndividualSessionID) state.Keypath {
	return individualSessionsKeypathForUsersAndType(sessionID.SessionType, sessionID.AliceAddr, sessionID.BobAddr).PushIndex(sessionID.Epoch)
}

func individualSessionIDKeypathForHash(hash types.Hash) state.Keypath {
	return individualSessionsByHashKeypath.Copy().Pushs(hash.Hex())
}

func outgoingIndividualMessagesKeypathForTypeAndRecipient(sessionType string, recipient types.Address) state.Keypath {
	return outgoingIndividualMessagesKeypath.Copy().Pushs(sessionType).Pushs(recipient.Hex())
}

func outgoingIndividualMessageKeypathFor(intent IndividualMessageIntent) state.Keypath {
	return outgoingIndividualMessagesKeypathForTypeAndRecipient(intent.SessionType, intent.Recipient).Pushs(intent.ID.Hex())
}

func incomingIndividualMessageKeypathFor(msg IndividualMessage) state.Keypath {
	return incomingIndividualMessagesKeypath.Copy().Pushs(msg.SessionHash.Hex()).PushIndex(uint64(msg.Header.N))
}

func outgoingGroupMessageIntentsKeypathForSessionType(sessionType string) state.Keypath {
	return outgoingGroupMessageIntentsKeypath.Copy().Pushs(sessionType)
}

func outgoingGroupMessageIntentKeypathFor(sessionType, id string) state.Keypath {
	return outgoingGroupMessageIntentsKeypathForSessionType(sessionType).Pushs(id)
}

func outgoingGroupMessageKeypathFor(recipient types.Address, id types.ID) state.Keypath {
	return outgoingGroupMessagesKeypathForRecipient(recipient).Pushs(id.Hex())
}

func outgoingGroupMessagesKeypathForRecipient(recipient types.Address) state.Keypath {
	return outgoingGroupMessageSendsKeypath.Copy().Pushs(recipient.Hex())
}

func incomingGroupMessageKeypathFor(msgHash types.Hash) state.Keypath {
	return incomingGroupMessagesKeypath.Copy().Pushs(msgHash.Hex())
}

func ratchetSessionKeypathFor(sessionID []byte) state.Keypath {
	return sessionsKeypath.Push(sessionID)
}

func ratchetPubkeyKeypathFor(pubKey doubleratchet.Key) state.Keypath {
	hexKey := hex.EncodeToString([]byte(pubKey))
	return messageKeysKeypath.Copy().Pushs(hexKey)
}

func ratchetMsgkeyKeypath(pubKey doubleratchet.Key, msgNum uint) state.Keypath {
	strMsgNum := strconv.FormatUint(uint64(msgNum), 10)
	return ratchetPubkeyKeypathFor(pubKey).Pushs(strMsgNum)
}
