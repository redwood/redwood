package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	"github.com/tyler-smith/go-bip39"

	"redwood.dev/types"
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
		Hex() string
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

func (pubkey *signingPublicKey) Hex() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (pubkey *signingPublicKey) String() string {
	return pubkey.Hex()
}

func (pubkey *signingPublicKey) UnmarshalText(bs []byte) error {
	pk, err := crypto.UnmarshalPubkey(bs)
	if err != nil {
		return err
	}
	pubkey.PublicKey = pk
	return nil
}

func (pubkey *signingPublicKey) MarshalText() ([]byte, error) {
	return crypto.FromECDSAPub(pubkey.PublicKey), nil
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

func (privkey *signingPrivateKey) Hex() string {
	return hex.EncodeToString(privkey.Bytes())
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

func SigningPublicKeyFromBytes(bs []byte) (SigningPublicKey, error) {
	var sigpubkey signingPublicKey
	err := sigpubkey.UnmarshalText(bs)
	return &sigpubkey, err
}

func RecoverSigningPubkey(hash types.Hash, signature []byte) (SigningPublicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &signingPublicKey{ecdsaPubkey}, nil
}

func GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}
	return mnemonic, nil
}

var DefaultHDDerivationPath = "m/44'/60'/0'/0/0"

func SigningKeypairFromHDMnemonic(mnemonic string, derivationPath string) (*SigningKeypair, error) {
	if mnemonic == "" {
		return nil, errors.New("mnemonic is required")
	} else if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("mnemonic is invalid")
	}
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, err
	}
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, err
	}
	path, err := accounts.ParseDerivationPath(derivationPath)
	if err != nil {
		return nil, err
	}
	privKey, err := derivePrivateKey(masterKey, path)
	if err != nil {
		return nil, err
	}
	return &SigningKeypair{
		SigningPrivateKey: &signingPrivateKey{privKey},
		SigningPublicKey:  &signingPublicKey{&privKey.PublicKey},
	}, nil
}

// DerivePrivateKey derives the private key of the derivation path.
func derivePrivateKey(masterKey *hdkeychain.ExtendedKey, path accounts.DerivationPath) (*ecdsa.PrivateKey, error) {
	var err error
	key := masterKey
	for _, n := range path {
		key, err = key.Child(n)
		if err != nil {
			return nil, err
		}
	}

	privateKey, err := key.ECPrivKey()
	privateKeyECDSA := privateKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	return privateKeyECDSA, nil
}
