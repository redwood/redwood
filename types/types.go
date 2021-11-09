package types

import (
	"bytes"
	"encoding/hex"
	"math/rand"

	"github.com/ethereum/go-ethereum/crypto"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types/pb"
	"redwood.dev/utils"
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
	if id == nil {
		id = &ID{}
	}
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.WithStack(err)
	}
	copy((*id)[:], bs)
	return nil
}

func (id ID) Marshal() ([]byte, error) { return id[:], nil }

func (id *ID) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*id)[:])
	return len(data), nil
}

func (id *ID) Unmarshal(data []byte) error {
	*id = ID{}
	copy((*id)[:], data)
	return nil
}

func (id *ID) Size() int { return len(*id) }
func (id ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.Hex() + `"`), nil
}
func (id *ID) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		*id = ID{}
		return nil
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*id = IDFromBytes(bs)
	return err
}
func (id ID) Compare(other ID) int { return bytes.Compare(id[:], other[:]) }
func (id ID) Equal(other ID) bool  { return bytes.Equal(id[:], other[:]) }

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

func RandomAddress() Address {
	return AddressFromBytes(utils.RandomBytes(len(Address{})))
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

func (a Address) Marshal() ([]byte, error) { return a[:], nil }

func (a *Address) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*a)[:])
	return len(data), nil
}

func (a *Address) Unmarshal(data []byte) error {
	*a = Address{}
	copy((*a)[:], data)
	return nil
}

func (a *Address) Size() int { return len(*a) }
func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.Hex() + `"`), nil
}
func (a *Address) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		*a = Address{}
		return nil
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*a = AddressFromBytes(bs)
	return nil
}
func (a Address) Compare(other Address) int { return bytes.Compare(a[:], other[:]) }
func (a Address) Equal(other Address) bool  { return bytes.Equal(a[:], other[:]) }

func (a Address) MapKey() (state.Keypath, error) {
	return state.Keypath(a.Hex()), nil
}

func (a *Address) ScanMapKey(keypath state.Keypath) error {
	addr, err := AddressFromHex(string(keypath))
	if err != nil {
		return err
	}
	*a = addr
	return nil
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

func (sig Signature) Copy() Signature {
	cp := make(Signature, len(sig))
	copy(cp, sig)
	return cp
}

func (sig Signature) Marshal() ([]byte, error) {
	sig2 := make(Signature, len(sig))
	copy(sig2, sig)
	return sig2, nil
}

func (sig *Signature) MarshalTo(data []byte) (n int, err error) {
	copy(data, *sig)
	return len(data), nil
}

func (sig *Signature) Unmarshal(data []byte) error {
	*sig = make(Signature, len(data))
	copy(*sig, data)
	return nil
}

func (sig *Signature) Size() int { return len(*sig) }
func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + sig.Hex() + `"`), nil
}
func (sig *Signature) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.Errorf(`bad JSON for types.Signature: %v`, string(data))
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*sig = bs
	return err
}
func (sig Signature) Compare(other Signature) int { return bytes.Compare(sig[:], other[:]) }
func (sig Signature) Equal(other Signature) bool  { return bytes.Equal(sig[:], other[:]) }

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

func (h Hash) Marshal() ([]byte, error) { return h[:], nil }

func (h *Hash) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*h)[:])
	return len(data), nil
}

func (h *Hash) Unmarshal(data []byte) error {
	*h = Hash{}
	copy((*h)[:], data)
	return nil
}

func (h *Hash) Size() int { return len(*h) }
func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + h.Hex() + `"`), nil
}
func (h *Hash) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		*h = Hash{}
		return nil
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*h, err = HashFromBytes(bs)
	return err
}
func (h Hash) Compare(other Hash) int { return bytes.Compare(h[:], other[:]) }
func (h Hash) Equal(other Hash) bool  { return bytes.Equal(h[:], other[:]) }

type HashAlg int

const (
	HashAlgUnknown HashAlg = iota
	SHA1
	SHA3
)

func HashAlgFromProtobuf(proto pb.HashAlg) HashAlg {
	return HashAlg(proto)
}

func (alg HashAlg) ToProtobuf() pb.HashAlg {
	return pb.HashAlg(alg)
}

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

type gogoprotobufTest interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func NewPopulatedID(_ gogoprotobufTest) *ID {
	var id ID
	copy(id[:], utils.RandomBytes(32))
	return &id
}

func NewPopulatedHash(_ gogoprotobufTest) *Hash {
	var h Hash
	copy(h[:], utils.RandomBytes(32))
	return &h
}

func NewPopulatedAddress(_ gogoprotobufTest) *Address {
	var a Address
	copy(a[:], utils.RandomBytes(20))
	return &a
}

func NewPopulatedSignature(_ gogoprotobufTest) *Signature {
	var sig Signature
	copy(sig[:], utils.RandomBytes(32))
	return &sig
}
