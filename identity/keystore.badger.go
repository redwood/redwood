package identity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/pkg/errors"
	"github.com/status-im/doubleratchet"

	"redwood.dev/crypto"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type BadgerKeyStore struct {
	log.Logger
	dbFilename   string
	db           *state.DBTree
	scryptParams ScryptParams
	unlockedUser *badgerUser
	mu           sync.RWMutex
}

type badgerUser struct {
	Password           string
	Mnemonic           string
	LocalSymEncKey     crypto.SymEncKey
	Identities         []Identity
	PublicIdentities   map[uint32]struct{}
	AddressesToIndices map[types.Address]uint32
	Extra              map[string]interface{}
}

type ScryptParams struct{ N, P int }

var (
	DefaultScryptParams  = ScryptParams{N: keystore.StandardScryptN, P: keystore.StandardScryptP}
	InsecureScryptParams = ScryptParams{N: 2, P: 1}
)

func NewBadgerKeyStore(dbFilename string, scryptParams ScryptParams) *BadgerKeyStore {
	return &BadgerKeyStore{
		Logger:       log.NewLogger("keystore"),
		dbFilename:   dbFilename,
		scryptParams: scryptParams,
	}
}

// Loads the user's keys from the DB and decrypts them.
func (ks *BadgerKeyStore) Unlock(password string, userMnemonic string) (err error) {
	defer utils.WithStack(&err)

	ks.mu.Lock()
	defer ks.mu.Unlock()

	db, err := state.NewDBTree(ks.dbFilename, nil)
	if err != nil {
		return err
	}
	ks.db = db
	defer func() {
		if err != nil {
			_ = ks.db.Close()
			ks.db = nil
			ks.unlockedUser = nil
		}
	}()

	node := ks.db.State(false)
	defer node.Close()

	user, err := ks.loadUser(password)
	if errors.Cause(err) == ErrNoUser {
		var mnemonic string
		var err error

		// Add user mnemonic or create if it doesn't exist
		if len(userMnemonic) > 0 {
			mnemonic = userMnemonic
		} else {
			mnemonic, err = crypto.GenerateMnemonic()
			if err != nil {
				return err
			}
		}

		sigkeys, err := crypto.SigKeypairFromHDMnemonic(mnemonic, 0)
		if err != nil {
			return err
		}

		enckeys, err := crypto.GenerateAsymEncKeypair()
		if err != nil {
			return err
		}

		symenckey, err := crypto.NewSymEncKey()
		if err != nil {
			return err
		}

		ks.unlockedUser = &badgerUser{
			Password:           password,
			Mnemonic:           mnemonic,
			LocalSymEncKey:     symenckey,
			Identities:         []Identity{{Public: true, SigKeypair: sigkeys, AsymEncKeypair: enckeys}},
			PublicIdentities:   map[uint32]struct{}{0: struct{}{}},
			AddressesToIndices: map[types.Address]uint32{sigkeys.Address(): 0},
			Extra:              map[string]interface{}{},
		}
		return ks.saveUser(ks.unlockedUser, password)

	} else if err != nil {
		return err
	}

	if user.Extra == nil {
		user.Extra = make(map[string]interface{})
	}

	ks.Infof(0, "keystore unlocked")

	ks.unlockedUser = user
	return ks.saveUser(ks.unlockedUser, password)
}

func (ks *BadgerKeyStore) Close() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	err := ks.db.Close()
	ks.db = nil
	ks.unlockedUser = nil
	return err
}

func (ks *BadgerKeyStore) Mnemonic() (string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return "", errors.WithStack(ErrLocked)
	}
	return ks.unlockedUser.Mnemonic, nil
}

func (ks *BadgerKeyStore) Identities() (_ []Identity, err error) {
	defer utils.WithStack(&err)

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return nil, errors.WithStack(ErrLocked)
	}
	identities := make([]Identity, len(ks.unlockedUser.Identities))
	copy(identities, ks.unlockedUser.Identities)
	return identities, nil
}

func (ks *BadgerKeyStore) PublicIdentities() (_ []Identity, err error) {
	defer utils.WithStack(&err)

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return nil, errors.WithStack(ErrLocked)
	}

	var idxs []uint32
	for idx := range ks.unlockedUser.PublicIdentities {
		idxs = append(idxs, idx)
	}
	sort.Slice(idxs, func(i, j int) bool { return idxs[i] < idxs[j] })

	var publicIdentities []Identity
	for idx := range idxs {
		publicIdentities = append(publicIdentities, ks.unlockedUser.Identities[idx])
	}
	return publicIdentities, nil
}

func (ks *BadgerKeyStore) DefaultPublicIdentity() (_ Identity, err error) {
	defer utils.WithStack(&err)

	publicIdentities, err := ks.PublicIdentities()
	if err != nil {
		return Identity{}, err
	} else if len(publicIdentities) == 0 {
		return Identity{}, ErrAccountDoesNotExist
	}
	return publicIdentities[0], nil
}

func (ks *BadgerKeyStore) IdentityWithAddress(address types.Address) (_ Identity, err error) {
	defer utils.WithStack(&err)

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return Identity{}, errors.WithStack(ErrLocked)
	}

	idx, exists := ks.unlockedUser.AddressesToIndices[address]
	if !exists || idx > uint32(len(ks.unlockedUser.Identities)-1) {
		return Identity{}, ErrAccountDoesNotExist
	}
	return ks.unlockedUser.Identities[idx], nil
}

func (ks *BadgerKeyStore) IdentityExists(address types.Address) (_ bool, err error) {
	_, err = ks.IdentityWithAddress(address)
	if errors.Cause(err) == ErrAccountDoesNotExist {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (ks *BadgerKeyStore) NewIdentity(public bool) (_ Identity, err error) {
	defer utils.WithStack(&err)

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return Identity{}, errors.WithStack(ErrLocked)
	}

	numIdentities := uint32(len(ks.unlockedUser.Identities))
	sigkeys, err := crypto.SigKeypairFromHDMnemonic(ks.unlockedUser.Mnemonic, numIdentities)
	if err != nil {
		return Identity{}, err
	}
	enckeys, err := crypto.GenerateAsymEncKeypair()
	if err != nil {
		return Identity{}, err
	}
	identity := Identity{
		Public:         public,
		SigKeypair:     sigkeys,
		AsymEncKeypair: enckeys,
	}
	ks.unlockedUser.Identities = append(ks.unlockedUser.Identities, identity)
	ks.unlockedUser.AddressesToIndices[sigkeys.Address()] = numIdentities
	if public {
		ks.unlockedUser.PublicIdentities[numIdentities] = struct{}{}
	}

	err = ks.saveUser(ks.unlockedUser, ks.unlockedUser.Password)
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

func (ks *BadgerKeyStore) SignHash(usingIdentity types.Address, data types.Hash) (_ []byte, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return nil, err
	}
	return identity.SigKeypair.SignHash(data)
}

func (ks *BadgerKeyStore) VerifySignature(usingIdentity types.Address, hash types.Hash, signature []byte) (_ bool, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return false, err
	}
	return identity.SigKeypair.VerifySignature(hash, signature), nil
}

func (ks *BadgerKeyStore) SealMessageFor(usingIdentity types.Address, recipientPubKey crypto.AsymEncPubkey, msg []byte) (_ []byte, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return nil, err
	}
	return identity.AsymEncKeypair.SealMessageFor(recipientPubKey, msg)
}

func (ks *BadgerKeyStore) OpenMessageFrom(usingIdentity types.Address, senderPublicKey crypto.AsymEncPubkey, msgEncrypted []byte) (_ []byte, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return nil, err
	}
	return identity.AsymEncKeypair.OpenMessageFrom(senderPublicKey, msgEncrypted)
}

func (ks *BadgerKeyStore) LocalSymEncKey() crypto.SymEncKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.unlockedUser.LocalSymEncKey
}

func (ks *BadgerKeyStore) SymmetricallyEncrypt(plaintext []byte) (crypto.SymEncMsg, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.unlockedUser.LocalSymEncKey.Encrypt(plaintext)
}

func (ks *BadgerKeyStore) SymmetricallyDecrypt(ciphertext crypto.SymEncMsg) ([]byte, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.unlockedUser.LocalSymEncKey.Decrypt(ciphertext)
}

func (ks *BadgerKeyStore) ExtraUserData(key string) (interface{}, bool, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.unlockedUser == nil {
		return nil, false, ErrLocked
	}
	val, exists := ks.unlockedUser.Extra[key]
	return val, exists, nil
}

func (ks *BadgerKeyStore) SaveExtraUserData(key string, value interface{}) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.unlockedUser == nil {
		return ErrLocked
	}
	ks.unlockedUser.Extra[key] = value
	return ks.saveUser(ks.unlockedUser, ks.unlockedUser.Password)
}

type encryptedBadgerUser struct {
	Mnemonic         string
	NumIdentities    uint32
	PublicIdentities map[uint32]struct{}
	AsymEncKeypair   map[uint32]dbAsymEncKeypair
	LocalSymEncKey   []byte
	Extra            map[string]interface{}
}

type dbAsymEncKeypair struct {
	Public  []byte
	Private []byte
}

func (ks *BadgerKeyStore) saveUser(user *badgerUser, password string) error {
	publicIdentities := make(map[uint32]struct{})
	asymEncKeys := make(map[uint32]dbAsymEncKeypair, len(user.Identities))
	for i, identity := range user.Identities {
		if identity.Public {
			publicIdentities[uint32(i)] = struct{}{}
		}
		asymEncKeys[uint32(i)] = dbAsymEncKeypair{
			Public:  identity.AsymEncKeypair.AsymEncPubkey.Bytes(),
			Private: identity.AsymEncKeypair.AsymEncPrivkey.Bytes(),
		}
	}

	encryptedUser := encryptedBadgerUser{
		Mnemonic:         user.Mnemonic,
		NumIdentities:    uint32(len(user.Identities)),
		PublicIdentities: publicIdentities,
		AsymEncKeypair:   asymEncKeys,
		LocalSymEncKey:   user.LocalSymEncKey[:],
		Extra:            user.Extra,
	}

	bs, err := json.Marshal(encryptedUser)
	if err != nil {
		return err
	}

	cryptoJSON, err := keystore.EncryptDataV3(bs, []byte(password), ks.scryptParams.N, ks.scryptParams.P)
	if err != nil {
		return err
	}

	node := ks.db.State(true)
	defer node.Close()

	err = node.Set(state.Keypath("keystore"), nil, cryptoJSON)
	if err != nil {
		return err
	}

	return node.Save()
}

func (ks *BadgerKeyStore) loadUser(password string) (_ *badgerUser, err error) {
	defer utils.WithStack(&err)

	node := ks.db.State(false)
	defer node.Close()

	exists, err := node.Exists(state.Keypath("keystore"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, ErrNoUser
	}

	var cryptoJSON keystore.CryptoJSON
	err = node.NodeAt(state.Keypath("keystore"), nil).Scan(&cryptoJSON)
	if err != nil {
		return nil, err
	}

	// Geth's keystore requires these parameters to be ints or float64s
	keys := []string{"dklen", "n", "r", "p"}
	if cryptoJSON.KDF == "pbkdf2" {
		keys = append(keys, "c")
	}
	for _, key := range keys {
		cryptoJSON.KDFParams[key] = int(cryptoJSON.KDFParams[key].(int64))
	}

	bs, err := keystore.DecryptDataV3(cryptoJSON, password)
	if err != nil {
		return nil, err
	}

	var encryptedUser encryptedBadgerUser
	err = json.Unmarshal(bs, &encryptedUser)
	if err != nil {
		return nil, err
	}

	var (
		identities         = make([]Identity, encryptedUser.NumIdentities)
		publicIdentities   = make(map[uint32]struct{})
		addressesToIndices = make(map[types.Address]uint32, encryptedUser.NumIdentities)
	)
	for i := uint32(0); i < encryptedUser.NumIdentities; i++ {
		sigkeys, err := crypto.SigKeypairFromHDMnemonic(encryptedUser.Mnemonic, i)
		if err != nil {
			return nil, err
		}
		var public bool
		if _, exists := encryptedUser.PublicIdentities[i]; exists {
			public = true
			publicIdentities[i] = struct{}{}
		}
		identities[i] = Identity{
			Public:     public,
			SigKeypair: sigkeys,
			AsymEncKeypair: &crypto.AsymEncKeypair{
				AsymEncPubkey:  crypto.AsymEncPubkeyFromBytes(encryptedUser.AsymEncKeypair[i].Public),
				AsymEncPrivkey: crypto.AsymEncPrivkeyFromBytes(encryptedUser.AsymEncKeypair[i].Private),
			},
		}
		addressesToIndices[sigkeys.Address()] = i
	}

	var localSymEncKey crypto.SymEncKey
	copy(localSymEncKey[:], encryptedUser.LocalSymEncKey)

	user := &badgerUser{
		Password:           password,
		Mnemonic:           encryptedUser.Mnemonic,
		LocalSymEncKey:     localSymEncKey,
		Identities:         identities,
		PublicIdentities:   publicIdentities,
		AddressesToIndices: addressesToIndices,
		Extra:              encryptedUser.Extra,
	}
	return user, nil
}

func (ks *BadgerKeyStore) RatchetSessionStore() RatchetSessionStore {
	return (*ratchetSessionStore)(ks)
}

func (ks *BadgerKeyStore) RatchetKeyStore() RatchetKeyStore {
	return (*ratchetKeyStore)(ks)
}

type ratchetSessionStore BadgerKeyStore

var _ RatchetSessionStore = (*ratchetSessionStore)(nil)

type ratchetSessionCodec struct {
	DHr                      doubleratchet.Key `tree:"DHr"`
	DHs                      dhPair            `tree:"DHs"`
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

type dhPair struct {
	Private doubleratchet.Key `tree:"PrivateKey"`
	Public  doubleratchet.Key `tree:"PublicKey"`
}

func (p dhPair) PrivateKey() doubleratchet.Key {
	return p.Private
}

func (p dhPair) PublicKey() doubleratchet.Key {
	return p.Public
}

func (p dhPair) String() string {
	return fmt.Sprintf("{privateKey: %s publicKey: %s}", p.Private, p.Public)
}

func (s *ratchetSessionStore) ratchetSessionKeypath(sessionID []byte) state.Keypath {
	return state.Keypath("ratchet").Push(sessionID)
}

func (s *ratchetSessionStore) ratchetSessionSharedKeyKeypath(sessionID []byte) state.Keypath {
	return state.Keypath("ratchet").Push(sessionID).Pushs("SharedKey")
}

func (s *ratchetSessionStore) LoadSharedKey(sessionID []byte) ([]byte, error) {
	node := s.db.State(false)
	defer node.Close()

	sharedKey, is, err := node.BytesValue(s.ratchetSessionSharedKeyKeypath(sessionID))
	if err != nil {
		return nil, err
	} else if !is {
		return nil, errors.Errorf("SharedKey: wrong type")
	}
	return sharedKey, nil
}

func (s *ratchetSessionStore) SaveSharedKey(sessionID, sharedKey []byte) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.ratchetSessionSharedKeyKeypath(sessionID), nil, sharedKey)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *ratchetSessionStore) Load(sessionID []byte) (*doubleratchet.State, error) {
	node := s.db.State(false)
	defer node.Close()

	sharedKey, err := s.LoadSharedKey(sessionID)
	if err != nil {
		return nil, err
	}

	var codec ratchetSessionCodec
	err = node.NodeAt(s.ratchetSessionKeypath(sessionID), nil).Scan(&codec)
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
		DHs:                      dhPair{Private: state.DHs.PrivateKey(), Public: state.DHs.PublicKey()},
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

	err := node.Set(s.ratchetSessionKeypath(sessionID), nil, codec)
	if err != nil {
		return err
	}
	return node.Save()
}

type ratchetKeyStore BadgerKeyStore

var _ RatchetKeyStore = (*ratchetKeyStore)(nil)

type ratchetKeyCodec struct {
	MessageKey doubleratchet.Key
	SeqNum     uint64
	SessionID  []byte
}

var ratchetKeystoreRootKeypath = state.Keypath("rk")

func (s *ratchetKeyStore) ratchetPubkeyKeypath(pubKey doubleratchet.Key) state.Keypath {
	hexKey := hex.EncodeToString([]byte(pubKey))
	return ratchetKeystoreRootKeypath.Pushs(hexKey)
}

func (s *ratchetKeyStore) ratchetMsgkeyKeypath(pubKey doubleratchet.Key, msgNum uint) state.Keypath {
	strMsgNum := strconv.FormatUint(uint64(msgNum), 10)
	return s.ratchetPubkeyKeypath(pubKey).Pushs(strMsgNum)
}

// Get returns a message key by the given key and message number.
func (s *ratchetKeyStore) Get(pubKey doubleratchet.Key, msgNum uint) (mk doubleratchet.Key, ok bool, err error) {
	node := s.db.State(false)
	defer node.Close()

	innerNode := node.NodeAt(s.ratchetMsgkeyKeypath(pubKey, msgNum), nil)

	exists, err := innerNode.Exists(nil)
	if err != nil {
		return nil, false, err
	} else if !exists {
		return nil, false, nil
	}

	var codec ratchetKeyCodec
	err = innerNode.Scan(&codec)
	if errors.Cause(err) == types.Err404 {
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

	err := node.Set(s.ratchetMsgkeyKeypath(pubKey, msgNum), nil, codec)
	if err != nil {
		return err
	}
	return node.Save()
}

// DeleteMk ensures there's no message key under the specified key and msgNum.
func (s *ratchetKeyStore) DeleteMk(k doubleratchet.Key, msgNum uint) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.ratchetMsgkeyKeypath(k, msgNum), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

// DeleteOldMKeys deletes old message keys for a session.
func (s *ratchetKeyStore) DeleteOldMks(sessionID []byte, deleteUntilSeqKey uint) error {
	// node := s.db.State(true)
	// defer node.Close()

	// node.ChildIterator(s.ratchetPubkeyKeypath(k), prefetchValues, prefetchSize)
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

	n := node.NodeAt(s.ratchetPubkeyKeypath(pubKey), nil).NumSubkeys()
	return uint(n), nil
}

// All returns all the keys
func (s *ratchetKeyStore) All() (map[string]map[uint]doubleratchet.Key, error) {
	node := s.db.State(false)
	defer node.Close()

	m := make(map[string]map[uint]doubleratchet.Key)

	iter := node.ChildIterator(ratchetKeystoreRootKeypath, true, 10)
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
