package identity

import (
	"encoding/json"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type BadgerKeyStore struct {
	db                *tree.DBTree
	scryptParams      ScryptParams
	unlockedUser      *badgerUser
	loadUserCallbacks []func(user User) error
	saveUserCallbacks []func(user User) error
	mu                sync.RWMutex
}

type badgerUser struct {
	Password               string
	Mnemonic               string
	Identities             []Identity
	PublicIdentities       map[uint32]struct{}
	AddressesToIndices     map[types.Address]uint32
	LocalEncryptingKeypair *crypto.EncryptingKeypair
	Extra                  map[string]interface{}
}

type ScryptParams struct{ N, P int }

var (
	DefaultScryptParams = ScryptParams{N: keystore.StandardScryptN, P: keystore.StandardScryptP}
	FastScryptParams    = ScryptParams{N: 2, P: 1}
)

func NewBadgerKeyStore(db *tree.DBTree, scryptParams ScryptParams) *BadgerKeyStore {
	return &BadgerKeyStore{
		db:           db,
		scryptParams: scryptParams,
	}
}

// Loads the user's keys from the DB and decrypts them.
func (ks *BadgerKeyStore) Unlock(password string, userMnemonic string) (err error) {
	defer utils.WithStack(&err)

	ks.mu.Lock()
	defer ks.mu.Unlock()

	state := ks.db.State(false)
	defer state.Close()

	user, err := ks.loadUser(password)

	userMnemonicExists := len(userMnemonic) > 0

	if errors.Cause(err) == ErrNoUser {
		var mnemonic string
		var err error

		// Add user mnemonic or create if it doesn't exist
		if userMnemonicExists {
			mnemonic = userMnemonic
		} else {
			mnemonic, err = crypto.GenerateMnemonic()
		}

		if err != nil {
			return err
		}

		sigkeys, err := crypto.SigningKeypairFromHDMnemonic(mnemonic, 0)
		if err != nil {
			return err
		}

		enckeys, err := crypto.GenerateEncryptingKeypair()
		if err != nil {
			return err
		}

		localEnckeys, err := crypto.GenerateEncryptingKeypair()
		if err != nil {
			return err
		}

		ks.unlockedUser = &badgerUser{
			Password:               password,
			Mnemonic:               mnemonic,
			Identities:             []Identity{{Public: true, Signing: sigkeys, Encrypting: enckeys}},
			PublicIdentities:       map[uint32]struct{}{0: struct{}{}},
			AddressesToIndices:     map[types.Address]uint32{sigkeys.Address(): 0},
			LocalEncryptingKeypair: localEnckeys,
			Extra:                  map[string]interface{}{},
		}

		for _, fn := range ks.loadUserCallbacks {
			err := fn(ks.unlockedUser)
			if err != nil {
				return err
			}
		}

		return ks.saveUser(ks.unlockedUser, password)

	} else if err != nil {
		return err
	}

	if user.Extra == nil {
		user.Extra = make(map[string]interface{})
	}

	ks.unlockedUser = user
	return ks.saveUser(ks.unlockedUser, password)
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
	sigkeys, err := crypto.SigningKeypairFromHDMnemonic(ks.unlockedUser.Mnemonic, numIdentities)
	if err != nil {
		return Identity{}, err
	}
	enckeys, err := crypto.GenerateEncryptingKeypair()
	if err != nil {
		return Identity{}, err
	}
	identity := Identity{
		Public:     public,
		Signing:    sigkeys,
		Encrypting: enckeys,
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
	return identity.Signing.SignHash(data)
}

func (ks *BadgerKeyStore) VerifySignature(usingIdentity types.Address, hash types.Hash, signature []byte) (_ bool, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return false, err
	}
	return identity.Signing.VerifySignature(hash, signature), nil
}

func (ks *BadgerKeyStore) SealMessageFor(usingIdentity types.Address, recipientPubKey crypto.EncryptingPublicKey, msg []byte) (_ []byte, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return nil, err
	}
	return identity.Encrypting.SealMessageFor(recipientPubKey, msg)
}

func (ks *BadgerKeyStore) OpenMessageFrom(usingIdentity types.Address, senderPublicKey crypto.EncryptingPublicKey, msgEncrypted []byte) (_ []byte, err error) {
	defer utils.WithStack(&err)

	identity, err := ks.IdentityWithAddress(usingIdentity)
	if err != nil {
		return nil, err
	}
	return identity.Encrypting.OpenMessageFrom(senderPublicKey, msgEncrypted)
}

func (ks *BadgerKeyStore) OnLoadUser(fn UserCallback) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.loadUserCallbacks = append(ks.loadUserCallbacks, fn)
}

func (ks *BadgerKeyStore) OnSaveUser(fn UserCallback) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.saveUserCallbacks = append(ks.saveUserCallbacks, fn)
}

func (user *badgerUser) ExtraData(key string) (interface{}, bool) {
	val, exists := user.Extra[key]
	return val, exists
}

func (user *badgerUser) SaveExtraData(key string, value interface{}) {
	user.Extra[key] = value
}

type encryptedBadgerUser struct {
	Mnemonic               string
	NumIdentities          uint32
	PublicIdentities       map[uint32]struct{}
	EncryptingKeys         map[uint32]dbEncryptingKeypair
	LocalEncryptingKeypair dbEncryptingKeypair
	Extra                  map[string]interface{}
}

type dbEncryptingKeypair struct {
	Public  []byte
	Private []byte
}

func (ks *BadgerKeyStore) saveUser(user *badgerUser, password string) error {
	// Allow other parts of the codebase to add data to the encrypted payload
	for _, fn := range ks.saveUserCallbacks {
		err := fn(user)
		if err != nil {
			return err
		}
	}

	publicIdentities := make(map[uint32]struct{})
	encryptingKeys := make(map[uint32]dbEncryptingKeypair, len(user.Identities))
	for i, identity := range user.Identities {
		if identity.Public {
			publicIdentities[uint32(i)] = struct{}{}
		}
		encryptingKeys[uint32(i)] = dbEncryptingKeypair{
			Public:  identity.Encrypting.EncryptingPublicKey.Bytes(),
			Private: identity.Encrypting.EncryptingPrivateKey.Bytes(),
		}
	}

	encryptedUser := encryptedBadgerUser{
		Mnemonic:         user.Mnemonic,
		NumIdentities:    uint32(len(user.Identities)),
		PublicIdentities: publicIdentities,
		EncryptingKeys:   encryptingKeys,
		LocalEncryptingKeypair: dbEncryptingKeypair{
			Public:  user.LocalEncryptingKeypair.EncryptingPublicKey.Bytes(),
			Private: user.LocalEncryptingKeypair.EncryptingPrivateKey.Bytes(),
		},
		Extra: user.Extra,
	}

	bs, err := json.Marshal(encryptedUser)
	if err != nil {
		return err
	}

	cryptoJSON, err := keystore.EncryptDataV3(bs, []byte(password), ks.scryptParams.N, ks.scryptParams.P)
	if err != nil {
		return err
	}

	state := ks.db.State(true)
	defer state.Close()

	err = state.Set(tree.Keypath("keystore"), nil, cryptoJSON)
	if err != nil {
		return err
	}

	return state.Save()
}

func (ks *BadgerKeyStore) loadUser(password string) (_ *badgerUser, err error) {
	defer utils.WithStack(&err)

	state := ks.db.State(false)
	defer state.Close()

	exists, err := state.Exists(tree.Keypath("keystore"))
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, ErrNoUser
	}

	var cryptoJSON keystore.CryptoJSON
	err = state.NodeAt(tree.Keypath("keystore"), nil).Scan(&cryptoJSON)
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
		sigkeys, err := crypto.SigningKeypairFromHDMnemonic(encryptedUser.Mnemonic, i)
		if err != nil {
			return nil, err
		}
		var public bool
		if _, exists := encryptedUser.PublicIdentities[i]; exists {
			public = true
			publicIdentities[i] = struct{}{}
		}
		identities[i] = Identity{
			Public:  public,
			Signing: sigkeys,
			Encrypting: &crypto.EncryptingKeypair{
				EncryptingPublicKey:  crypto.EncryptingPublicKeyFromBytes(encryptedUser.EncryptingKeys[i].Public),
				EncryptingPrivateKey: crypto.EncryptingPrivateKeyFromBytes(encryptedUser.EncryptingKeys[i].Private),
			},
		}
		addressesToIndices[sigkeys.Address()] = i
	}

	user := &badgerUser{
		Password:           password,
		Mnemonic:           encryptedUser.Mnemonic,
		Identities:         identities,
		PublicIdentities:   publicIdentities,
		AddressesToIndices: addressesToIndices,
		LocalEncryptingKeypair: &crypto.EncryptingKeypair{
			EncryptingPublicKey:  crypto.EncryptingPublicKeyFromBytes(encryptedUser.LocalEncryptingKeypair.Public),
			EncryptingPrivateKey: crypto.EncryptingPrivateKeyFromBytes(encryptedUser.LocalEncryptingKeypair.Private),
		},
		Extra: encryptedUser.Extra,
	}

	// Allow other parts of the codebase to read data from the encrypted payload
	for _, fn := range ks.loadUserCallbacks {
		err := fn(user)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}
