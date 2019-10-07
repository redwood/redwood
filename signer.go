package redwood

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
)

type PrivateKey interface {
	Sign(data []byte) ([]byte, error)
	Address() Address
	PublicKey() *publicKey
	String() string
}

type PublicKey interface {
	Address() Address
	Bytes() []byte
	String() string
}

type privateKey struct {
	*ecdsa.PrivateKey
}

type publicKey struct {
	*ecdsa.PublicKey
}

func (pk privateKey) Address() Address {
	return pk.PublicKey().Address()
}

func (pk publicKey) String() string {
	return hex.EncodeToString(pk.Bytes())
}

func (pk publicKey) Address() Address {
	lenX := len(pk.X.Bytes())
	lenY := len(pk.Y.Bytes())
	bs := make([]byte, lenX+lenY)
	copy(bs, pk.X.Bytes())
	copy(bs[lenX:], pk.Y.Bytes())
	digest := HashBytes(bs)
	return Address(digest)
}

func (pk *publicKey) Bytes() []byte {
	return crypto.FromECDSAPub(pk.PublicKey)
}

func (pk privateKey) SignHash(hash Hash) ([]byte, error) {
	return crypto.Sign(hash[:], pk.PrivateKey)
}

func (pk privateKey) PublicKey() *publicKey {
	return &publicKey{&pk.PrivateKey.PublicKey}
}

func HashBytes(bs []byte) Hash {
	hash := crypto.Keccak256Hash(bs)
	var h Hash
	copy(h[:], hash.Bytes())
	return h
}

func GenerateKeypair() (*privateKey, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	return &privateKey{pk}, nil
}

func KeypairFromHex(s string) (*privateKey, error) {
	pk, err := crypto.HexToECDSA(s)
	if err != nil {
		return nil, err
	}
	return &privateKey{pk}, nil
}

func RecoverPubkey(hash Hash, signature []byte) (*publicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return nil, err
	}
	return &publicKey{ecdsaPubkey}, nil
}

func VerifySignature(pk *publicKey, hash Hash, signature []byte) bool {
	signatureNoRecoverID := signature[:len(signature)-1] // remove recovery id
	return crypto.VerifySignature(pk.Bytes(), hash[:], signatureNoRecoverID)
}
