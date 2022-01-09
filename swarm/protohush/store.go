package protohush

import (
	"github.com/status-im/doubleratchet"

	"redwood.dev/types"
)

type Store interface {
	// Diffie-Hellman keys
	EnsureDHPair() (DHPair, error)
	DHPubkeyAttestations() ([]DHPubkeyAttestation, error)
	LatestDHPubkeyFor(addr types.Address) (DHPubkeyAttestation, error)
	SaveDHPubkeyAttestations(attestations []DHPubkeyAttestation) error

	// Individual session handshake protocol
	OutgoingIndividualSessionProposalHashes() (types.HashSet, error)
	OutgoingIndividualSessionProposalByHash(proposalHash types.Hash) (IndividualSessionProposal, error)
	OutgoingIndividualSessionProposalsForUsers(aliceAddr, bobAddr types.Address) ([]IndividualSessionProposal, error)
	OutgoingIndividualSessionProposalByUsersAndType(sessionType string, aliceAddr, bobAddr types.Address) (IndividualSessionProposal, error)
	SaveOutgoingIndividualSessionProposal(proposal IndividualSessionProposal) error
	DeleteOutgoingIndividualSessionProposal(sessionHash types.Hash) error

	IncomingIndividualSessionProposals() (map[types.Hash]EncryptedIndividualSessionProposal, error)
	IncomingIndividualSessionProposal(hash types.Hash) (EncryptedIndividualSessionProposal, error)
	SaveIncomingIndividualSessionProposal(proposal EncryptedIndividualSessionProposal) error
	DeleteIncomingIndividualSessionProposal(proposal EncryptedIndividualSessionProposal) error

	OutgoingIndividualSessionResponses() (map[types.Address]map[types.Hash]IndividualSessionResponse, error)
	OutgoingIndividualSessionResponsesForUser(aliceAddr types.Address) ([]IndividualSessionResponse, error)
	OutgoingIndividualSessionResponse(aliceAddr types.Address, proposalHash types.Hash) (IndividualSessionResponse, error)
	SaveOutgoingIndividualSessionResponse(sender types.Address, approval IndividualSessionResponse) error
	DeleteOutgoingIndividualSessionResponse(aliceAddr types.Address, proposalHash types.Hash) error

	// Established individual sessions
	LatestIndividualSessionWithUsers(sessionType string, aliceAddr, bobAddr types.Address) (IndividualSessionProposal, error)
	IndividualSessionByID(id IndividualSessionID) (IndividualSessionProposal, error)
	IndividualSessionIDBySessionHash(hash types.Hash) (IndividualSessionID, error)
	SaveApprovedIndividualSession(session IndividualSessionProposal) error

	// Individual messages
	OutgoingIndividualMessageIntents() ([]IndividualMessageIntent, error)
	OutgoingIndividualMessageIntentIDsForTypeAndRecipient(sessionType string, recipient types.Address) (types.IDSet, error)
	OutgoingIndividualMessageIntent(sessionType string, recipient types.Address, id types.ID) (IndividualMessageIntent, error)
	SaveOutgoingIndividualMessageIntent(intent IndividualMessageIntent) error
	DeleteOutgoingIndividualMessageIntent(intent IndividualMessageIntent) error

	IncomingIndividualMessages() ([]IndividualMessage, error)
	SaveIncomingIndividualMessage(sender types.Address, msg IndividualMessage) error
	DeleteIncomingIndividualMessage(msg IndividualMessage) error

	// Group messages
	OutgoingGroupMessageIntentIDs(sessionType string) (types.StringSet, error)
	OutgoingGroupMessageIntent(sessionType, id string) (GroupMessageIntent, error)
	SaveOutgoingGroupMessageIntent(intent GroupMessageIntent) error
	DeleteOutgoingGroupMessageIntent(sessionType, id string) error

	IncomingGroupMessages() ([]GroupMessage, error)
	SaveIncomingGroupMessage(sender types.Address, msg GroupMessage) error
	DeleteIncomingGroupMessage(msg GroupMessage) error

	// Double ratchet message keys + sessions
	RatchetSessionStore() RatchetSessionStore
	RatchetKeyStore() RatchetKeyStore

	DebugPrint()
}

type RatchetSessionStore interface {
	doubleratchet.SessionStorage
}

type RatchetKeyStore interface {
	doubleratchet.KeysStorage
}
