package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip39"

	"redwood.dev/errors"
	"redwood.dev/types"
)

type (
	SigKeypair struct {
		*SigningPrivateKey
		*SigningPublicKey
	}

	SigningPrivateKey struct {
		*ecdsa.PrivateKey
	}

	SigningPublicKey struct {
		*ecdsa.PublicKey
	}
)

func (pubkey *SigningPublicKey) VerifySignature(hash types.Hash, signature []byte) bool {
	signatureNoRecoverID := signature[:len(signature)-1] // remove recovery id
	return crypto.VerifySignature(pubkey.Bytes(), hash[:], signatureNoRecoverID)
}

func (pubkey *SigningPublicKey) Address() types.Address {
	ethAddr := crypto.PubkeyToAddress(*pubkey.PublicKey)
	var a types.Address
	copy(a[:], ethAddr[:])
	return a
}

func (pubkey *SigningPublicKey) Bytes() []byte {
	return crypto.FromECDSAPub(pubkey.PublicKey)
}

func (pubkey *SigningPublicKey) Hex() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (pubkey *SigningPublicKey) String() string {
	return pubkey.Hex()
}

func (pubkey *SigningPublicKey) UnmarshalText(bs []byte) error {
	pk, err := crypto.UnmarshalPubkey(bs)
	if err != nil {
		return err
	}
	pubkey.PublicKey = pk
	return nil
}

func (pubkey *SigningPublicKey) MarshalText() ([]byte, error) {
	return crypto.FromECDSAPub(pubkey.PublicKey), nil
}

func (pubkey *SigningPublicKey) UnmarshalBinary(bs []byte) error {
	return pubkey.UnmarshalText(bs)
}

func (pubkey *SigningPublicKey) MarshalBinary() ([]byte, error) {
	return pubkey.MarshalText()
}

func (pubkey *SigningPublicKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pubkey.Hex() + `"`), nil
}

func (pubkey *SigningPublicKey) UnmarshalStateBytes(bs []byte) error {
	return pubkey.UnmarshalText(bs)
}

func (pubkey SigningPublicKey) MarshalStateBytes() ([]byte, error) {
	return pubkey.MarshalText()
}

func (privkey *SigningPrivateKey) SignHash(hash types.Hash) ([]byte, error) {
	sig, err := crypto.Sign(hash[:], privkey.PrivateKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return sig, nil
}

func (privkey *SigningPrivateKey) Bytes() []byte {
	return crypto.FromECDSA(privkey.PrivateKey)
}

func (privkey *SigningPrivateKey) Hex() string {
	return hex.EncodeToString(privkey.Bytes())
}

func (privkey *SigningPrivateKey) String() string {
	return hex.EncodeToString(privkey.Bytes())
}

func GenerateSigKeypair() (*SigKeypair, error) {
	pk, err := crypto.GenerateKey()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &SigKeypair{
		SigningPrivateKey: &SigningPrivateKey{pk},
		SigningPublicKey:  &SigningPublicKey{&pk.PublicKey},
	}, nil
}

func SigKeypairFromHex(s string) (*SigKeypair, error) {
	pk, err := crypto.HexToECDSA(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &SigKeypair{
		SigningPrivateKey: &SigningPrivateKey{pk},
		SigningPublicKey:  &SigningPublicKey{&pk.PublicKey},
	}, nil
}

func SigningPublicKeyFromBytes(bs []byte) (*SigningPublicKey, error) {
	var sigpubkey SigningPublicKey
	err := sigpubkey.UnmarshalText(bs)
	return &sigpubkey, err
}

func RecoverSigningPubkey(hash types.Hash, signature []byte) (*SigningPublicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &SigningPublicKey{ecdsaPubkey}, nil
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

var DefaultHDDerivationPathPrefix = accounts.DerivationPath{44, 60, 0, 0}

func SigKeypairFromHDMnemonic(mnemonic string, accountIndex uint32) (*SigKeypair, error) {
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
	path, err := accounts.ParseDerivationPath(fmt.Sprintf("m/44'/60'/0'/0/%d", accountIndex))
	if err != nil {
		return nil, err
	}
	privKey, err := derivePrivateKey(masterKey, path)
	if err != nil {
		return nil, err
	}

	ecPrivKey, err := privKey.ECPrivKey()
	if err != nil {
		return nil, err
	}
	ecdsaPrivKey := ecPrivKey.ToECDSA()

	return &SigKeypair{
		SigningPrivateKey: &SigningPrivateKey{ecdsaPrivKey},
		SigningPublicKey:  &SigningPublicKey{&ecdsaPrivKey.PublicKey},
	}, nil
}

// DerivePrivateKey derives the private key of the derivation path.
func derivePrivateKey(masterKey *hdkeychain.ExtendedKey, path accounts.DerivationPath) (*hdkeychain.ExtendedKey, error) {
	var err error
	key := masterKey
	for _, n := range path {
		key, err = key.Derive(n)
		if err != nil {
			return nil, err
		}
	}
	return key, nil
}
