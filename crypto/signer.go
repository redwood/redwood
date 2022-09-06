package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

func (pubkey *SigningPublicKey) VerifySignature(hash types.Hash, signature Signature) bool {
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

func (privkey *SigningPrivateKey) SignHash(hash types.Hash) (Signature, error) {
	sig, err := crypto.Sign(hash[:], privkey.PrivateKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return Signature(sig), nil
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

func RecoverSigningPubkey(hash types.Hash, signature Signature) (*SigningPublicKey, error) {
	ecdsaPubkey, err := crypto.SigToPub(hash[:], []byte(signature))
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

type ChallengeMsg []byte

const challengeMsgLength = 128

func GenerateChallengeMsg() (ChallengeMsg, error) {
	challengeMsg := make([]byte, challengeMsgLength)
	n, err := rand.Read(challengeMsg)
	if err != nil {
		return nil, err
	} else if n < challengeMsgLength {
		return nil, errors.Errorf("could not read enough entropy")
	}
	return ChallengeMsg(challengeMsg), nil
}

func (challenge ChallengeMsg) String() string {
	return challenge.Hex()
}

func (challenge ChallengeMsg) Bytes() []byte {
	return []byte(challenge.Copy())
}

func (challenge ChallengeMsg) Hex() string {
	return hex.EncodeToString(challenge)
}

func (challenge ChallengeMsg) Hash() types.Hash {
	return types.HashBytes([]byte(challenge))
}

func (challenge ChallengeMsg) Copy() ChallengeMsg {
	cp := make(ChallengeMsg, len(challenge))
	copy(cp, challenge)
	return cp
}

func (challenge ChallengeMsg) Marshal() ([]byte, error) {
	return challenge.Copy().Bytes(), nil
}

func (challenge *ChallengeMsg) MarshalTo(data []byte) (n int, err error) {
	if len(data) < challengeMsgLength {
		return 0, errors.Errorf("bad length for crypto.ChallengeMsg (%v)", len(data))
	}
	return copy(data, *challenge), nil
}

func (challenge *ChallengeMsg) Unmarshal(data []byte) error {
	if len(data) != challengeMsgLength {
		return errors.Errorf("bad length for crypto.ChallengeMsg (%v)", len(data))
	}
	*challenge = make(ChallengeMsg, len(data))
	copy(*challenge, data)
	return nil
}

func (challenge *ChallengeMsg) UnmarshalText(bs []byte) error {
	bytes, err := hex.DecodeString(string(bs))
	if err != nil {
		return err
	}
	return challenge.Unmarshal(bytes)
}

func (challenge ChallengeMsg) MarshalText() ([]byte, error) {
	return []byte(challenge.Hex()), nil
}

func (challenge ChallengeMsg) MarshalJSON() ([]byte, error) {
	return []byte(`"` + challenge.Hex() + `"`), nil
}
func (challenge *ChallengeMsg) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	return challenge.UnmarshalText([]byte(s))
}

func (challenge ChallengeMsg) Compare(other ChallengeMsg) int {
	return bytes.Compare(challenge[:], other[:])
}

func (challenge ChallengeMsg) Equal(other ChallengeMsg) bool {
	return bytes.Equal(challenge[:], other[:])
}

func (challenge ChallengeMsg) Length() int { return len(challenge) }

func (challenge *ChallengeMsg) Size() int { return len(*challenge) }

func (challenge *ChallengeMsg) UnmarshalHTTPHeader(header string) error {
	return challenge.UnmarshalText([]byte(header))
}

type Signature []byte

func SignatureFromBytes(bs []byte) (Signature, error) {
	return Signature(bs).Copy(), nil
}

func SignatureFromHex(hx string) (Signature, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return SignatureFromBytes(bs)
}

func (sig Signature) String() string {
	return sig.Hex()
}

func (sig Signature) Hex() string {
	return hex.EncodeToString(sig)
}

func (sig Signature) Copy() Signature {
	cp := make(Signature, len(sig))
	copy(cp, sig)
	return cp
}

func (sig Signature) Marshal() ([]byte, error) {
	return []byte(sig.Copy()), nil
}

func (sig *Signature) MarshalTo(data []byte) (n int, err error) {
	return copy(data, *sig), nil
}

func (sig *Signature) Unmarshal(data []byte) (err error) {
	*sig, err = SignatureFromBytes(data)
	return err
}

func (sig *Signature) UnmarshalText(bs []byte) error {
	bytes, err := hex.DecodeString(string(bs))
	if err != nil {
		return err
	}
	return sig.Unmarshal(bytes)
}

func (sig Signature) Length() int { return len(sig) }

func (sig *Signature) Size() int { return len(*sig) }

func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + sig.Hex() + `"`), nil
}

func (sig *Signature) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*sig, err = SignatureFromHex(s)
	return err
}

func (sig Signature) Compare(other Signature) int { return bytes.Compare(sig[:], other[:]) }

func (sig Signature) Equal(other Signature) bool { return bytes.Equal(sig[:], other[:]) }

func (sig *Signature) UnmarshalHTTPHeader(header string) error {
	bs, err := hex.DecodeString(header)
	if err != nil {
		return err
	}
	*sig = Signature(bs)
	return nil
}

type gogoprotobufTest interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func NewPopulatedChallengeMsg(_ gogoprotobufTest) *ChallengeMsg {
	challenge, err := GenerateChallengeMsg()
	if err != nil {
		panic(err)
	}
	return &challenge
}

func NewPopulatedSignature(_ gogoprotobufTest) *Signature {
	sig, err := SignatureFromBytes(types.RandomBytes(32))
	if err != nil {
		panic(err)
	}
	return &sig
}
