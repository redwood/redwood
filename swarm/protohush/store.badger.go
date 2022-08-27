package protohush

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/status-im/doubleratchet"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

type store struct {
	log.Logger
	db *state.DBTree
}

var _ Store = (*store)(nil)

func NewStore(db *state.DBTree) *store {
	return &store{
		Logger: log.NewLogger("store"),
		db:     db,
	}
}

func (s *store) KeyBundle(id types.Hash) (KeyBundle, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := keyBundleKeypathForID(id)

	exists, err := node.Exists(keypath)
	if err != nil {
		return KeyBundle{}, err
	} else if !exists {
		return KeyBundle{}, errors.Err404
	}

	var keyBundle KeyBundle
	err = node.NodeAt(keypath, nil).Scan(&keyBundle)
	if err != nil {
		return KeyBundle{}, err
	}
	return keyBundle, nil
}

func (s *store) KeyBundles() ([]KeyBundle, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := keyBundleKeypath.Copy()

	// exists, err := node.Exists(keypath)
	// if err != nil {
	// 	return nil, err
	// } else if !exists {
	// 	return nil, nil
	// }

	var keyBundles map[string]KeyBundle
	err := node.NodeAt(keypath, nil).Scan(&keyBundles)
	if err != nil {
		return nil, err
	}
	return Values(keyBundles), nil
}

func (s *store) LatestValidKeyBundleFor(addr types.Address) (KeyBundle, error) {
	bundles, err := s.KeyBundles()
	if err != nil {
		return KeyBundle{}, err
	}

	bundles = Filter(bundles, func(bundle KeyBundle) bool { return !bundle.Revoked && bundle.Address == addr })
	latest, ok := MaxFunc(bundles, func(bundle KeyBundle) uint64 { return bundle.CreatedAt })
	if !ok {
		return KeyBundle{}, errors.Err404
	}
	return latest, nil
}

func (s *store) CreateKeyBundle(identity identity.Identity) (KeyBundle, PubkeyBundle, error) {
	idKey, err := GenerateX3DHPrivateKey()
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}
	spKey, err := GenerateX3DHPrivateKey()
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}
	rKey, err := GenerateRatchetPrivateKey()
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}

	now := uint64(time.Now().UTC().Unix())

	keyBundle := KeyBundle{
		IdentityKey:  idKey,
		SignedPreKey: spKey,
		RatchetKey:   rKey,
		Revoked:      false,
		Address:      identity.Address(),
		CreatedAt:    now,
	}

	pubkeyBundle, err := keyBundle.ToPubkeyBundle(identity)
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(keyBundleKeypathForID(keyBundle.ID()), nil, keyBundle)
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}
	err = node.Set(pubkeyBundleKeypathForID(pubkeyBundle.ID()), nil, pubkeyBundle)
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}
	err = node.Save()
	if err != nil {
		return KeyBundle{}, PubkeyBundle{}, err
	}
	return keyBundle, pubkeyBundle, nil
}

func (s *store) PubkeyBundle(id types.Hash) (PubkeyBundle, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := pubkeyBundleKeypathForID(id)

	exists, err := node.Exists(keypath)
	if err != nil {
		return PubkeyBundle{}, err
	} else if !exists {
		return PubkeyBundle{}, errors.Err404
	}

	var pubkeyBundle PubkeyBundle
	err = node.NodeAt(keypath, nil).Scan(&pubkeyBundle)
	if err != nil {
		return PubkeyBundle{}, err
	}
	return pubkeyBundle, nil
}

func (s *store) PubkeyBundles() ([]PubkeyBundle, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := pubkeyBundleKeypath.Copy()

	var pubkeyBundles map[string]PubkeyBundle
	err := node.NodeAt(keypath, nil).Scan(&pubkeyBundles)
	if err != nil {
		return nil, err
	}
	return Values(pubkeyBundles), nil
}

func (s *store) LatestValidPubkeyBundleFor(addr types.Address) (PubkeyBundle, error) {
	bundles, err := s.PubkeyBundles()
	if err != nil {
		return PubkeyBundle{}, err
	}

	bundles = Filter(bundles, func(bundle PubkeyBundle) bool {
		bundleAddr, err := bundle.Address()
		return err == nil && !bundle.Revoked && bundleAddr == addr
	})
	latest, ok := MaxFunc(bundles, func(bundle PubkeyBundle) uint64 { return bundle.CreatedAt })
	if !ok {
		return PubkeyBundle{}, errors.Err404
	}
	return latest, nil
}

func (s *store) SavePubkeyBundles(bundles []PubkeyBundle) error {
	node := s.db.State(true)
	defer node.Close()

	for _, bundle := range bundles {
		err := node.Set(pubkeyBundleKeypathForID(bundle.ID()), nil, bundle)
		if err != nil {
			return err
		}
	}
	return node.Save()
}

func (s *store) PubkeyBundleAddresses() ([]types.Address, error) {
	bundles, err := s.PubkeyBundles()
	if err != nil {
		return nil, err
	}
	return MapWithError(bundles, func(bundle PubkeyBundle) (types.Address, error) { return bundle.Address() })
}

func (s *store) Session(id types.Hash) (Session, doubleratchet.Session, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := sessionKeypathForID(id)

	exists, err := node.Exists(keypath)
	if err != nil {
		return Session{}, nil, err
	} else if !exists {
		return Session{}, nil, errors.Err404
	}

	var session Session
	err = node.NodeAt(keypath, nil).Scan(&session)
	if err != nil {
		return Session{}, nil, err
	}

	drsession, err := doubleratchet.Load(session.ID().Bytes(), s.RatchetSessionStore())
	if err != nil {
		return Session{}, nil, err
	}

	return session, drsession, nil
}

func (s *store) SaveSession(session Session) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(sessionKeypathForID(session.ID()), nil, session)
	if err != nil {
		return err
	}

	err = node.Set(sessionsKeypathForUsers(session.MyAddress, session.RemoteAddress).Pushs(session.ID().Hex()), nil, true)
	if err != nil {
		return err
	}

	return node.Save()
}

func (s *store) StartOutgoingSession(myBundle KeyBundle, remoteBundle PubkeyBundle, ephemeralPubkey *X3DHPublicKey, sharedKey []byte) (_ Session, _ doubleratchet.Session, err error) {
	defer errors.AddStack(&err)

	remoteAddress, err := remoteBundle.Address()
	if err != nil {
		return Session{}, nil, err
	}
	session := Session{
		MyAddress:       myBundle.Address,
		MyBundleID:      myBundle.ID(),
		RemoteAddress:   remoteAddress,
		RemoteBundleID:  remoteBundle.ID(),
		EphemeralPubkey: ephemeralPubkey,
		SharedKey:       sharedKey,
		Revoked:         false,
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(sessionKeypathForID(session.ID()), nil, session)
	if err != nil {
		return Session{}, nil, err
	}
	err = node.Set(sessionsKeypathForUsers(session.MyAddress, session.RemoteAddress).Pushs(session.ID().Hex()), nil, true)
	if err != nil {
		return Session{}, nil, err
	}
	err = node.Save()
	if err != nil {
		return Session{}, nil, err
	}

	drsession, err := doubleratchet.NewWithRemoteKey(
		session.ID().Bytes(),
		session.SharedKey,
		doubleratchet.Key(remoteBundle.RatchetPubkey.Bytes()),
		s.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(s.RatchetKeyStore()),
	)
	return session, drsession, err
}

func (s *store) StartIncomingSession(myBundle KeyBundle, remoteBundle PubkeyBundle, ephemeralPubkey *X3DHPublicKey, sharedKey []byte) (_ Session, _ doubleratchet.Session, err error) {
	defer errors.AddStack(&err)

	remoteAddress, err := remoteBundle.Address()
	if err != nil {
		return Session{}, nil, err
	}
	session := Session{
		MyAddress:       myBundle.Address,
		MyBundleID:      myBundle.ID(),
		RemoteAddress:   remoteAddress,
		RemoteBundleID:  remoteBundle.ID(),
		EphemeralPubkey: ephemeralPubkey,
		SharedKey:       sharedKey,
		Revoked:         false,
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(sessionKeypathForID(session.ID()), nil, session)
	if err != nil {
		return Session{}, nil, err
	}
	err = node.Set(sessionsKeypathForUsers(session.MyAddress, session.RemoteAddress).Pushs(session.ID().Hex()), nil, true)
	if err != nil {
		return Session{}, nil, err
	}
	err = node.Save()
	if err != nil {
		return Session{}, nil, err
	}

	drsession, err := doubleratchet.New(
		session.ID().Bytes(),
		sharedKey,
		myBundle.RatchetKey,
		s.RatchetSessionStore(),
		doubleratchet.WithKeysStorage(s.RatchetKeyStore()),
	)
	return session, drsession, err
}

func (s *store) LatestValidSessionWithUsers(a, b types.Address) (_ Session, _ doubleratchet.Session, err error) {
	defer errors.AddStack(&err)

	node := s.db.State(false)
	defer node.Close()

	sessionIDs := node.NodeAt(sessionsKeypathForUsers(a, b), nil).Subkeys()

	sessions := Reduce(sessionIDs, func(into []Session, idKeypath state.Keypath) []Session {
		id, err := types.HashFromHex(string(idKeypath))
		if err != nil {
			s.Errorf("while decoding session keypath '%v': %v", idKeypath.String(), err)
			return into
		}

		keypath := sessionKeypathForID(id)

		exists, err := node.Exists(keypath)
		if err != nil {
			s.Errorf("while loading session '%v': %v", idKeypath.String(), err)
			return into
		} else if !exists {
			return into
		}

		var session Session
		err = node.NodeAt(keypath, nil).Scan(&session)
		if err != nil {
			s.Errorf("while loading session '%v': %v", idKeypath.String(), err)
			return into
		} else if session.Revoked {
			return into
		}
		return append(into, session)
	}, nil)

	session, ok := MaxFunc(sessions, func(session Session) uint64 { return session.CreatedAt })
	if !ok {
		return Session{}, nil, errors.Err404
	}

	drsession, err := doubleratchet.Load(session.ID().Bytes(), s.RatchetSessionStore())
	if err != nil {
		return Session{}, nil, err
	}

	return session, drsession, nil
}

func (s *store) SymEncKeyAndMessage(messageType, messageID string) (crypto.SymEncKey, crypto.SymEncMsg, error) {
	node := s.db.State(false)
	defer node.Close()

	exists, err := node.Exists(nil)
	if err != nil {
		return crypto.SymEncKey{}, crypto.SymEncMsg{}, err
	} else if !exists {
		return crypto.SymEncKey{}, crypto.SymEncMsg{}, errors.Err404
	}

	var msg SymEncKeyAndMessage
	err = node.NodeAt(symEncKeyAndMessageKeypathFor(messageType, messageID), nil).Scan(&msg)
	if err != nil {
		return crypto.SymEncKey{}, crypto.SymEncMsg{}, err
	}
	return msg.Key, msg.Message, nil
}

func (s *store) SaveSymEncKeyAndMessage(messageType, messageID string, key crypto.SymEncKey, msg crypto.SymEncMsg) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(symEncKeyAndMessageKeypathFor(messageType, messageID), nil, SymEncKeyAndMessage{key, msg})
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) IncomingGroupMessages() ([]IncomingGroupMessage, error) {
	node := s.db.State(false)
	defer node.Close()

	var messages map[string]IncomingGroupMessage
	err := node.NodeAt(incomingGroupMessagesKeypath.Copy(), nil).Scan(&messages)
	if err != nil {
		return nil, err
	}

	return Values(messages), nil
}

func (s *store) SaveIncomingGroupMessage(msg IncomingGroupMessage) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(incomingGroupMessageKeypathForID(msg.ID), nil, msg)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteIncomingGroupMessage(id string) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(incomingGroupMessageKeypathForID(id), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) OutgoingGroupMessage(id string) (OutgoingGroupMessage, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := outgoingGroupMessageKeypathForID(id)

	exists, err := node.Exists(keypath)
	if err != nil {
		return OutgoingGroupMessage{}, err
	} else if !exists {
		return OutgoingGroupMessage{}, errors.Err404
	}

	var msg OutgoingGroupMessage
	err = node.NodeAt(keypath, nil).Scan(&msg)
	if err != nil {
		return OutgoingGroupMessage{}, err
	}
	return msg, nil
}

func (s *store) OutgoingGroupMessageIDs() []string {
	node := s.db.State(false)
	defer node.Close()
	return Map(node.NodeAt(outgoingGroupMessagesKeypath.Copy(), nil).Subkeys(), func(kp state.Keypath) string { return string(kp) })
}

func (s *store) SaveOutgoingGroupMessage(msg OutgoingGroupMessage) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(outgoingGroupMessageKeypathForID(msg.ID), nil, msg)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) DeleteOutgoingGroupMessage(id string) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(outgoingGroupMessageKeypathForID(id), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) Vaults() Set[string] {
	node := s.db.State(false)
	defer node.Close()

	hostsEscaped := node.NodeAt(vaultsKeypath.Copy(), nil).Subkeys()

	hosts, err := MapWithError(hostsEscaped, func(kp state.Keypath) (string, error) {
		return url.QueryUnescape(string(kp))
	})
	if err != nil {
		return nil
	}
	return NewSet(hosts)
}

func (s *store) AddVault(host string) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(vaultsKeypathForHost(host), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveVault(host string) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(vaultsKeypathForHost(host), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) LatestMtimeForVaultAndCollection(vaultHost, collectionID string) time.Time {
	node := s.db.State(false)
	defer node.Close()

	var mtime uint64
	err := node.NodeAt(keypathForLatestMtimeForVaultAndCollection(vaultHost, collectionID), nil).Scan(&mtime)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(int64(mtime), 0)
}

func (s *store) MaybeSaveLatestMtimeForVaultAndCollection(vaultHost, collectionID string, mtime time.Time) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(keypathForLatestMtimeForVaultAndCollection(vaultHost, collectionID), nil, uint64(mtime.UTC().Unix()))
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

func (s *store) RatchetSessionStore() RatchetSessionStore {
	return (*ratchetSessionStore)(s)
}

func (s *store) RatchetKeyStore() RatchetKeyStore {
	return (*ratchetKeyStore)(s)
}

type ratchetSessionStore store

var _ RatchetSessionStore = (*ratchetSessionStore)(nil)

type ratchetSessionCodec struct {
	DHr                      doubleratchet.Key  `tree:"DHr"`
	DHs                      *RatchetPrivateKey `tree:"DHs"`
	RootChainKey             doubleratchet.Key  `tree:"RootChainKey"`
	SendChainKey             doubleratchet.Key  `tree:"SendChainKey"`
	SendChainN               uint64             `tree:"SendChainN"`
	RecvChainKey             doubleratchet.Key  `tree:"RecvChainKey"`
	RecvChainN               uint64             `tree:"RecvChainN"`
	PN                       uint64             `tree:"PN"`
	MaxSkip                  uint64             `tree:"MaxSkip"`
	HKr                      doubleratchet.Key  `tree:"HKr"`
	NHKr                     doubleratchet.Key  `tree:"NHKr"`
	HKs                      doubleratchet.Key  `tree:"HKs"`
	NHKs                     doubleratchet.Key  `tree:"NHKs"`
	MaxKeep                  uint64             `tree:"MaxKeep"`
	MaxMessageKeysPerSession int64              `tree:"MaxMessageKeysPerSession"`
	Step                     uint64             `tree:"Step"`
	KeysCount                uint64             `tree:"KeysCount"`
}

func (s *ratchetSessionStore) Load(sessionIDBytes []byte) (*doubleratchet.State, error) {
	node := s.db.State(false)
	defer node.Close()

	id, err := types.HashFromBytes(sessionIDBytes)
	if err != nil {
		return nil, err
	}

	keypath := sessionKeypathForID(id).Pushs("sharedKey")

	exists, err := node.Exists(keypath)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.Err404
	}

	var sharedKey []byte
	err = node.NodeAt(keypath, nil).Scan(&sharedKey)
	if err != nil {
		return nil, err
	}

	keypath = ratchetSessionKeypathFor(sessionIDBytes)

	exists, err = node.Exists(keypath)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.Err404
	}

	var codec ratchetSessionCodec
	err = node.NodeAt(keypath, nil).Scan(&codec)
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
		DHs:                      RatchetPrivateKeyFromBytes([]byte(state.DHs.PrivateKey())),
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
	rootKeypath                  = state.Keypath("protohush")
	keyBundleKeypath             = rootKeypath.Copy().Pushs("key-bundles")
	pubkeyBundleKeypath          = rootKeypath.Copy().Pushs("pubkey-bundles")
	sessionsKeypath              = rootKeypath.Copy().Pushs("sessions")
	symEncKeyAndMessageKeypath   = rootKeypath.Copy().Pushs("sym-enc-messages-and-keys")
	incomingGroupMessagesKeypath = rootKeypath.Copy().Pushs("messages-in")
	outgoingGroupMessagesKeypath = rootKeypath.Copy().Pushs("messages-out")
	sharedKeysKeypath            = rootKeypath.Copy().Pushs("sks")
	drsessionsKeypath            = rootKeypath.Copy().Pushs("dr-sessions")
	messageKeysKeypath           = rootKeypath.Copy().Pushs("mks")
	vaultsKeypath                = rootKeypath.Copy().Pushs("vaults")
	vaultMtimeKeypath            = rootKeypath.Copy().Pushs("latestMtimeForVaultAndCollection")
)

func keyBundleKeypathForID(id types.Hash) state.Keypath {
	return keyBundleKeypath.Copy().Pushs(id.Hex())
}

func pubkeyBundleKeypathForID(id types.Hash) state.Keypath {
	return pubkeyBundleKeypath.Copy().Pushs(id.Hex())
}

func sessionKeypathForID(id types.Hash) state.Keypath {
	return sessionsKeypath.Copy().Pushs("by-id").Pushs(id.Hex())
}

func sessionsKeypathForUsers(a, b types.Address) state.Keypath {
	if bytes.Compare(a.Bytes(), b.Bytes()) > 0 {
		a, b = b, a
	}
	return sessionsKeypath.Copy().Pushs("by-users").Pushs(a.Hex()).Pushs(b.Hex())
}

func symEncKeyAndMessageKeypathFor(messageType, messageID string) state.Keypath {
	return symEncKeyAndMessageKeypath.Copy().Pushs(messageType).Pushs(messageID)
}

func incomingGroupMessageKeypathForID(id string) state.Keypath {
	return incomingGroupMessagesKeypath.Copy().Pushs(id)
}

func outgoingGroupMessageKeypathForID(id string) state.Keypath {
	return outgoingGroupMessagesKeypath.Copy().Pushs(id)
}

func ratchetSessionKeypathFor(sessionID []byte) state.Keypath {
	return drsessionsKeypath.Copy().Pushs(hex.EncodeToString(sessionID))
}

func ratchetPubkeyKeypathFor(pubKey doubleratchet.Key) state.Keypath {
	hexKey := hex.EncodeToString([]byte(pubKey))
	return messageKeysKeypath.Copy().Pushs(hexKey)
}

func ratchetMsgkeyKeypath(pubKey doubleratchet.Key, msgNum uint) state.Keypath {
	strMsgNum := strconv.FormatUint(uint64(msgNum), 10)
	return ratchetPubkeyKeypathFor(pubKey).Pushs(strMsgNum)
}

func vaultsKeypathForHost(host string) state.Keypath {
	return vaultsKeypath.Copy().Pushs(url.QueryEscape(host))
}

func keypathForLatestMtimeForVaultAndCollection(vaultHost, collectionID string) state.Keypath {
	return vaultMtimeKeypath.Copy().Pushs(url.QueryEscape(vaultHost)).Pushs(collectionID)
}
