package protohush

import (
	"time"

	"github.com/status-im/doubleratchet"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

type Store interface {
	KeyBundle(id types.Hash) (KeyBundle, error)
	KeyBundles() ([]KeyBundle, error)
	LatestValidKeyBundleFor(addr types.Address) (KeyBundle, error)
	CreateKeyBundle(identity identity.Identity) (KeyBundle, PubkeyBundle, error)

	PubkeyBundle(id types.Hash) (PubkeyBundle, error)
	PubkeyBundles() ([]PubkeyBundle, error)
	LatestValidPubkeyBundleFor(addr types.Address) (PubkeyBundle, error)
	SavePubkeyBundles(bundles []PubkeyBundle) error
	PubkeyBundleAddresses() ([]types.Address, error)

	Session(id types.Hash) (Session, doubleratchet.Session, error)
	StartOutgoingSession(myBundle KeyBundle, remoteBundle PubkeyBundle, ephemeralPubkey *X3DHPublicKey, sharedKey []byte) (Session, doubleratchet.Session, error)
	StartIncomingSession(myBundle KeyBundle, remoteBundle PubkeyBundle, ephemeralPubkey *X3DHPublicKey, sharedKey []byte) (Session, doubleratchet.Session, error)
	LatestValidSessionWithUsers(a, b types.Address) (Session, doubleratchet.Session, error)

	SymEncKeyAndMessage(messageType, messageID string) (crypto.SymEncKey, crypto.SymEncMsg, error)
	SaveSymEncKeyAndMessage(messageType, messageID string, key crypto.SymEncKey, msg crypto.SymEncMsg) error

	IncomingGroupMessages() ([]IncomingGroupMessage, error)
	SaveIncomingGroupMessage(msg IncomingGroupMessage) error
	DeleteIncomingGroupMessage(id string) error

	OutgoingGroupMessage(id string) (OutgoingGroupMessage, error)
	OutgoingGroupMessageIDs() []string
	SaveOutgoingGroupMessage(intent OutgoingGroupMessage) error
	DeleteOutgoingGroupMessage(id string) error

	// Double ratchet message keys + sessions
	RatchetSessionStore() RatchetSessionStore
	RatchetKeyStore() RatchetKeyStore

	// Vault
	Vaults() Set[string]
	AddVault(host string) error
	RemoveVault(host string) error
	LatestMtimeForVaultAndCollection(vaultHost, collectionID string) time.Time
	MaybeSaveLatestMtimeForVaultAndCollection(vaultHost, collectionID string, mtime time.Time) error

	DebugPrint()
}

type RatchetSessionStore interface {
	doubleratchet.SessionStorage
}

type RatchetKeyStore interface {
	doubleratchet.KeysStorage
}
