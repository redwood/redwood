package types

import (
	"encoding/hex"
	"math/rand"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

type ID [32]byte

var EmptyID = ID{}

func RandomID() ID {
	var id ID
	rand.Read(id[:])
	return id
}

func IDFromHex(h string) (ID, error) {
	bs, err := hex.DecodeString(h)
	if err != nil {
		return ID{}, errors.WithStack(err)
	}
	var id ID
	copy(id[:], bs)
	return id, nil
}

func IDFromString(s string) ID {
	var id ID
	copy(id[:], []byte(s))
	return id
}

func IDFromBytes(bs []byte) ID {
	var id ID
	copy(id[:], bs)
	return id
}

func (id ID) String() string {
	return id.Hex()
}

func (id ID) Bytes() []byte {
	return id[:]
}

func (id ID) Pretty() string {
	return id.String()[:6]
}

func (id ID) Hex() string {
	return hex.EncodeToString(id[:])
}

func (id ID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(id[:])), nil
}

func (id *ID) UnmarshalText(text []byte) error {
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.WithStack(err)
	}
	copy((*id)[:], bs)
	return nil
}

type Address [20]byte

func AddressFromHex(hx string) (Address, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return Address{}, errors.WithStack(err)
	}
	var addr Address
	copy(addr[:], bs)
	return addr, nil
}

func AddressFromBytes(bs []byte) Address {
	var addr Address
	copy(addr[:], bs)
	return addr
}

func (a Address) IsZero() bool {
	return a == Address{}
}

func (a Address) Bytes() []byte {
	bs := make([]byte, len(a))
	copy(bs, a[:])
	return bs
}

func (a Address) String() string {
	return a.Hex()
}

func (a Address) Pretty() string {
	return a.Hex()[:6]
}

func (a Address) Hex() string {
	return hex.EncodeToString(a[:])
}

func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *Address) UnmarshalText(asHex []byte) error {
	bs, err := hex.DecodeString(string(asHex))
	if err != nil {
		return errors.WithStack(err)
	}
	copy((*a)[:], bs)
	return nil
}

func OverlappingAddresses(one, two []Address) []Address {
	var overlap []Address
	for _, a := range one {
		for _, b := range two {
			if a == b {
				overlap = append(overlap, a)
			}
		}
	}
	return overlap
}

type Signature []byte

func SignatureFromHex(hx string) (Signature, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return Signature(bs), nil
}

func (sig Signature) String() string {
	return sig.Hex()
}

func (sig Signature) Hex() string {
	return hex.EncodeToString(sig)
}

func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(sig) + `"`), nil
}

func (sig *Signature) UnmarshalJSON(bs []byte) error {
	bs, err := hex.DecodeString(string(bs[1 : len(bs)-1]))
	if err != nil {
		return errors.WithStack(err)
	}
	*sig = bs
	return nil
}

func (sig Signature) Copy() Signature {
	cp := make(Signature, len(sig))
	copy(cp, sig)
	return cp
}

type Hash [32]byte

var (
	EmptyHash = Hash{}
)

func HashBytes(bs []byte) Hash {
	hash := crypto.Keccak256(bs)
	var h Hash
	copy(h[:], hash)
	return h
}

func HashFromBytes(bs []byte) (Hash, error) {
	if len(bs) > 32 {
		return Hash{}, errors.Errorf("bad input to HashFromBytes (expected 32 or fewer bytes, got %v)", len(bs))
	}
	var h Hash
	copy(h[:], bs)
	return h, nil
}

func HashFromHex(hexStr string) (Hash, error) {
	var hash Hash
	err := hash.UnmarshalText([]byte(hexStr))
	return hash, err
}

func (h Hash) Copy() Hash {
	var out Hash
	copy(out[:], h[:])
	return out
}

func (h Hash) Bytes() []byte {
	out := h.Copy()
	return out[:]
}

func (h Hash) String() string {
	return h.Hex()
}

func (h Hash) Pretty() string {
	return h.String()[:6]
}

func (h Hash) Hex() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}

func (h *Hash) UnmarshalText(text []byte) error {
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.WithStack(err)
	}
	copy((*h)[:], bs)
	return nil
}

type HashAlg int

const (
	HashAlgUnknown HashAlg = iota
	SHA1
	SHA3
)

func (alg HashAlg) String() string {
	switch alg {
	case HashAlgUnknown:
		return "unknown"
	case SHA1:
		return "sha1"
	case SHA3:
		return "sha3"
	default:
		return "ERR:(bad value for HashAlg)"
	}
}
