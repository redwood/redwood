package redwood

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
)

type (
	SigningKeypair struct {
		SigningPrivateKey
		SigningPublicKey
	}

	SigningPrivateKey interface {
		SignHash(data Hash) ([]byte, error)
		Bytes() []byte
		String() string
	}

	SigningPublicKey interface {
		VerifySignature(hash Hash, signature []byte) bool
		Address() Address
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

func (pubkey *signingPublicKey) VerifySignature(hash Hash, signature []byte) bool {
	signatureNoRecoverID := signature[:len(signature)-1] // remove recovery id
	return crypto.VerifySignature(pubkey.Bytes(), hash[:], signatureNoRecoverID)
}

func (pubkey *signingPublicKey) Address() Address {
	ethAddr := crypto.PubkeyToAddress(*pubkey.PublicKey)
	var a Address
	copy(a[:], ethAddr[:])
	return a
}

func (pubkey *signingPublicKey) Bytes() []byte {
	return crypto.FromECDSAPub(pubkey.PublicKey)
}

func (pubkey *signingPublicKey) String() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (privkey *signingPrivateKey) SignHash(hash Hash) ([]byte, error) {
	return crypto.Sign(hash[:], privkey.PrivateKey)
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
		return nil, err
	}
	return &SigningKeypair{
		SigningPrivateKey: &signingPrivateKey{pk},
		SigningPublicKey:  &signingPublicKey{&pk.PublicKey},
	}, nil
}

func SigningKeypairFromHex(s string) (*SigningKeypair, error) {
	pk, err := crypto.HexToECDSA(s)
	if err != nil {
		return nil, err
	}
	return &SigningKeypair{
		SigningPrivateKey: &signingPrivateKey{pk},
		SigningPublicKey:  &signingPublicKey{&pk.PublicKey},
	}, nil
}

func RecoverSigningPubkey(hash Hash, signature []byte) (SigningPublicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return nil, err
	}
	return &signingPublicKey{ecdsaPubkey}, nil
}
