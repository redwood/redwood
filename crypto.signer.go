package redwood

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/types"
)

type (
	SigningKeypair struct {
		SigningPrivateKey
		SigningPublicKey
	}

	SigningPrivateKey interface {
		SignHash(data types.Hash) ([]byte, error)
		Bytes() []byte
		String() string
	}

	SigningPublicKey interface {
		VerifySignature(hash types.Hash, signature []byte) bool
		Address() types.Address
		Bytes() []byte
		String() string
	}

	signingPrivateKey struct {
		*ecdsa.PrivateKey
	}

	signingPublicKey struct {
		*ecdsa.PublicKey
	}
)

func (pubkey *signingPublicKey) VerifySignature(hash types.Hash, signature []byte) bool {
	signatureNoRecoverID := signature[:len(signature)-1] // remove recovery id
	return crypto.VerifySignature(pubkey.Bytes(), hash[:], signatureNoRecoverID)
}

func (pubkey *signingPublicKey) Address() types.Address {
	ethAddr := crypto.PubkeyToAddress(*pubkey.PublicKey)
	var a types.Address
	copy(a[:], ethAddr[:])
	return a
}

func (pubkey *signingPublicKey) Bytes() []byte {
	return crypto.FromECDSAPub(pubkey.PublicKey)
}

func (pubkey *signingPublicKey) String() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (privkey *signingPrivateKey) SignHash(hash types.Hash) ([]byte, error) {
	sig, err := crypto.Sign(hash[:], privkey.PrivateKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return sig, nil
}

func (privkey *signingPrivateKey) Bytes() []byte {
	return crypto.FromECDSA(privkey.PrivateKey)
}

func (privkey *signingPrivateKey) String() string {
	return hex.EncodeToString(privkey.Bytes())
}

func GenerateSigningKeypair() (*SigningKeypair, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &SigningKeypair{
		SigningPrivateKey: &signingPrivateKey{pk},
		SigningPublicKey:  &signingPublicKey{&pk.PublicKey},
	}, nil
}

func SigningKeypairFromHex(s string) (*SigningKeypair, error) {
	pk, err := crypto.HexToECDSA(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &SigningKeypair{
		SigningPrivateKey: &signingPrivateKey{pk},
		SigningPublicKey:  &signingPublicKey{&pk.PublicKey},
	}, nil
}

func RecoverSigningPubkey(hash types.Hash, signature []byte) (SigningPublicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &signingPublicKey{ecdsaPubkey}, nil
}
