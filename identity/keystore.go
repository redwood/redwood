package identity

import (
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/types"
)

//go:generate mockery --name KeyStore --output ./mocks/ --case=underscore
type KeyStore interface {
	Unlock(password string, userMnemonic string) error
	Close() error

	Mnemonic() (string, error)
	Identities() ([]Identity, error)
	Addresses() (types.Set[types.Address], error)
	PublicIdentities() ([]Identity, error)
	DefaultPublicIdentity() (Identity, error)
	IdentityWithAddress(address types.Address) (Identity, error)
	IdentityExists(address types.Address) (bool, error)
	NewIdentity(public bool) (Identity, error)
	SignHash(usingIdentity types.Address, data types.Hash) ([]byte, error)
	VerifySignature(usingIdentity types.Address, hash types.Hash, signature []byte) (bool, error)
	SealMessageFor(usingIdentity types.Address, recipientPubKey *crypto.AsymEncPubkey, msg []byte) ([]byte, error)
	OpenMessageFrom(usingIdentity types.Address, senderPublicKey *crypto.AsymEncPubkey, msgEncrypted []byte) ([]byte, error)
	LocalSymEncKey() crypto.SymEncKey

	ExtraUserData(key string) (interface{}, bool, error)
	SaveExtraUserData(key string, value interface{}) error
}

var (
	ErrNoUser              = errors.New("no user in DB")
	ErrLocked              = errors.New("keystore is locked")
	ErrAccountDoesNotExist = errors.New("account does not exist")
)

type Identity struct {
	Public         bool
	SigKeypair     *crypto.SigKeypair
	AsymEncKeypair *crypto.AsymEncKeypair
}

func (i Identity) Address() types.Address {
	return i.SigKeypair.Address()
}

func (i Identity) SignHash(hash types.Hash) ([]byte, error) {
	return i.SigKeypair.SignHash(hash)
}

func (i Identity) VerifySignature(hash types.Hash, signature []byte) bool {
	return i.SigKeypair.VerifySignature(hash, signature)
}
