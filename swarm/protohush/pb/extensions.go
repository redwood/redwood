package pb

import (
	"bytes"
	"crypto/ecdsa"
	"strconv"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/status-im/doubleratchet"
	"golang.org/x/crypto/curve25519"

	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/types"
)

func (b KeyBundle) ID() types.Hash {
	return b.Hash()
}

func (b KeyBundle) Hash() types.Hash {
	parts := [][]byte{
		b.IdentityKey.PublicKeyBytes(),
		b.SignedPreKey.PublicKeyBytes(),
		[]byte(b.RatchetKey.PublicKey()),
		[]byte(strconv.FormatUint(b.CreatedAt, 10)),
	}
	return types.HashBytes(bytes.Join(parts, nil))
}

func (b KeyBundle) ToPubkeyBundle(identity identity.Identity) (PubkeyBundle, error) {
	pubkeyBundle := PubkeyBundle{
		IdentityPubkey:     b.IdentityKey.Public(),
		SignedPreKeyPubkey: b.SignedPreKey.Public(),
		RatchetPubkey:      b.RatchetKey.Public(),
		Revoked:            false,
		CreatedAt:          b.CreatedAt,
	}
	sig, err := identity.SignHash(pubkeyBundle.Hash())
	if err != nil {
		return PubkeyBundle{}, err
	}
	pubkeyBundle.Sig = sig
	return pubkeyBundle, nil
}

func (b PubkeyBundle) ID() types.Hash {
	return b.Hash()
}

func (b PubkeyBundle) Hash() types.Hash {
	parts := [][]byte{
		b.IdentityPubkey.Bytes(),
		b.SignedPreKeyPubkey.Bytes(),
		b.RatchetPubkey.Bytes(),
		[]byte(strconv.FormatUint(b.CreatedAt, 10)),
	}
	return types.HashBytes(bytes.Join(parts, nil))
}

func (b PubkeyBundle) Address() (types.Address, error) {
	pubkey, err := crypto.RecoverSigningPubkey(b.Hash(), b.Sig)
	if err != nil {
		return types.Address{}, err
	}
	return pubkey.Address(), nil
}

func (m GroupMessage) Hash() (types.Hash, error) {
	bs, err := m.Marshal()
	if err != nil {
		return types.Hash{}, err
	}
	return types.HashBytes(bs), nil
}

func (m GroupMessage) Copy() GroupMessage {
	// @@TODO
	return m
}

type X3DHPrivateKey ecdsa.PrivateKey

func GenerateX3DHPrivateKey() (*X3DHPrivateKey, error) {
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	return (*X3DHPrivateKey)(key), nil
}

func (k *X3DHPrivateKey) PublicKeyBytes() []byte {
	return gethcrypto.CompressPubkey(k.Public().ECDSA())
}

func (k *X3DHPrivateKey) PrivateKeyBytes() []byte {
	return gethcrypto.FromECDSA((*ecdsa.PrivateKey)(k))
}

func (k *X3DHPrivateKey) ECDSA() *ecdsa.PrivateKey {
	return (*ecdsa.PrivateKey)(k)
}

func (k *X3DHPrivateKey) Public() *X3DHPublicKey {
	return (*X3DHPublicKey)((*ecdsa.PrivateKey)(k).Public().(*ecdsa.PublicKey))
}

func (k *X3DHPrivateKey) Unmarshal(bs []byte) error {
	key, err := gethcrypto.ToECDSA(bs)
	if err != nil {
		return err
	}
	*k = X3DHPrivateKey(*key)
	return nil
}

func (k *X3DHPrivateKey) UnmarshalStateBytes(bs []byte) error {
	return k.Unmarshal(bs)
}

func (k *X3DHPrivateKey) MarshalStateBytes() ([]byte, error) {
	return k.PrivateKeyBytes(), nil
}

func (k *X3DHPrivateKey) MarshalTo(data []byte) (n int, err error) {
	return copy(data, k.PrivateKeyBytes()), nil
}

func (k *X3DHPrivateKey) Size() int { return len(k.PrivateKeyBytes()) }

type X3DHPublicKey ecdsa.PublicKey

func (k *X3DHPublicKey) ECDSA() *ecdsa.PublicKey {
	return (*ecdsa.PublicKey)(k)
}

func (k *X3DHPublicKey) Bytes() []byte {
	return gethcrypto.CompressPubkey((*ecdsa.PublicKey)(k))
}

func (k *X3DHPublicKey) Unmarshal(bs []byte) error {
	key, err := gethcrypto.DecompressPubkey(bs)
	if err != nil {
		return err
	}
	*k = X3DHPublicKey(*key)
	return nil
}

func (k *X3DHPublicKey) MarshalTo(data []byte) (n int, err error) {
	return copy(data, k.Bytes()), nil
}

func (k *X3DHPublicKey) Size() int { return len(k.Bytes()) }

func (k *X3DHPublicKey) UnmarshalStateBytes(bs []byte) error {
	return k.Unmarshal(bs)
}

func (k *X3DHPublicKey) MarshalStateBytes() ([]byte, error) {
	return k.Bytes(), nil
}

type RatchetPrivateKey struct {
	K []byte
}

var _ doubleratchet.DHPair = (*RatchetPrivateKey)(nil)

func RatchetPrivateKeyFromBytes(bs []byte) *RatchetPrivateKey {
	if len(bs) != 32 {
		panic("bad length for ratchet private key")
	}
	return &RatchetPrivateKey{bs}
}

func GenerateRatchetPrivateKey() (*RatchetPrivateKey, error) {
	dhpair, err := doubleratchet.DefaultCrypto{}.GenerateDH()
	if err != nil {
		return nil, err
	}
	return &RatchetPrivateKey{K: []byte(dhpair.PrivateKey())}, nil
}

func (k *RatchetPrivateKey) Public() *RatchetPublicKey {
	if len(k.K) < 32 {
		panic("bad length for ratchet private key")
	}
	var pubkey [32]byte
	var privkey [32]byte
	copy(privkey[:], k.K)
	curve25519.ScalarBaseMult(&pubkey, &privkey)
	pub := &RatchetPublicKey{K: make([]byte, 32)}
	copy(pub.K, pubkey[:])
	return pub
}

func (k *RatchetPrivateKey) PrivateKey() doubleratchet.Key {
	if k == nil {
		return nil
	}
	if len(k.K) < 32 {
		panic("bad length for ratchet private key")
	}
	k2 := make(doubleratchet.Key, 32)
	copy(k2, k.K)
	return k2
}

func (k *RatchetPrivateKey) PublicKey() doubleratchet.Key {
	if k == nil {
		return nil
	}
	if len(k.K) < 32 {
		panic("bad length for ratchet private key")
	}
	var pubkey [32]byte
	var privkey [32]byte
	copy(privkey[:], k.K)
	curve25519.ScalarBaseMult(&pubkey, &privkey)
	return doubleratchet.Key(pubkey[:])
}

func (k *RatchetPrivateKey) Unmarshal(bs []byte) error {
	if len(bs) != 32 {
		panic("bad length for ratchet private key")
	}
	if k == nil {
		*k = RatchetPrivateKey{}
	}
	k.K = make([]byte, 32)
	copy(k.K, bs)
	return nil
}

func (k *RatchetPrivateKey) MarshalTo(data []byte) (n int, err error) {
	if len(data) < 32 {
		panic("bad length for ratchet private key")
	}
	return copy(data, k.K), nil
}

func (k *RatchetPrivateKey) Size() int { return 32 }

func (k *RatchetPrivateKey) UnmarshalStateBytes(bs []byte) error {
	return k.Unmarshal(bs)
}

func (k *RatchetPrivateKey) MarshalStateBytes() ([]byte, error) {
	return []byte(k.PrivateKey()), nil
}

type RatchetPublicKey struct {
	K []byte
}

func RatchetPublicKeyFromBytes(bs []byte) *RatchetPublicKey {
	if len(bs) != 32 {
		panic("bad length for ratchet public key")
	}
	return &RatchetPublicKey{bs}
}

func (k *RatchetPublicKey) Unmarshal(bs []byte) error {
	if len(bs) != 32 {
		panic("bad length for ratchet public key")
	}
	if k == nil {
		*k = RatchetPublicKey{}
	}
	k.K = make([]byte, 32)
	copy(k.K, bs)
	return nil
}

func (k *RatchetPublicKey) Marshal() ([]byte, error) {
	if len(k.K) < 32 {
		panic("bad length for ratchet public key")
	}
	bs := make([]byte, 32)
	copy(bs, k.K)
	return bs, nil
}

func (k *RatchetPublicKey) Bytes() []byte {
	if len(k.K) < 32 {
		panic("bad length for ratchet public key")
	}
	bs := make([]byte, len(k.K))
	copy(bs, k.K)
	return bs
}

func (k *RatchetPublicKey) MarshalTo(data []byte) (n int, err error) {
	if len(data) < 32 {
		panic("bad length for ratchet public key")
	}
	if len(k.K) < 32 {
		panic("bad length for ratchet public key")
	}
	return copy(data, k.K), nil
}

func (k *RatchetPublicKey) Size() int { return 32 }

func (k *RatchetPublicKey) UnmarshalStateBytes(bs []byte) error {
	if len(bs) < 32 {
		panic("bad length for ratchet public key")
	}
	return k.Unmarshal(bs)
}

func (k *RatchetPublicKey) MarshalStateBytes() ([]byte, error) {
	if len(k.K) < 32 {
		panic("bad length for ratchet public key")
	}
	return k.Bytes(), nil
}

//
// Protobuf testing stuff
//

func NewPopulatedX3DHPrivateKey(r randyHush) *X3DHPrivateKey {
	k, err := GenerateX3DHPrivateKey()
	if err != nil {
		panic(err)
	}
	return k
}

func NewPopulatedX3DHPublicKey(r randyHush) *X3DHPublicKey {
	k, err := GenerateX3DHPrivateKey()
	if err != nil {
		panic(err)
	}
	return (*X3DHPublicKey)(k.Public())
}

func NewPopulatedRatchetPrivateKey(r randyHush) *RatchetPrivateKey {
	k, err := GenerateRatchetPrivateKey()
	if err != nil {
		panic(err)
	}
	return k
}

func NewPopulatedRatchetPublicKey(r randyHush) *RatchetPublicKey {
	k, err := GenerateRatchetPrivateKey()
	if err != nil {
		panic(err)
	}
	return &RatchetPublicKey{K: []byte(k.PublicKey())}
}
