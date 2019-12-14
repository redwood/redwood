package redwood

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

var (
	GenesisTxID = IDFromString("genesis")
	EmptyHash   = Hash{}
)

const (
	KeypathSeparator = "."
)

type Tx struct {
	ID         ID        `json:"id"`
	Parents    []ID      `json:"parents"`
	From       Address   `json:"from"`
	Sig        Signature `json:"sig,omitempty"`
	URL        string    `json:"url"`
	Patches    []Patch   `json:"patches"`
	Recipients []Address `json:"recipients,omitempty"`
	Checkpoint bool      `json:"checkpoint"` // @@TODO: probably not ideal

	Valid bool `json:"-"`
	hash  Hash `json:"-"`
}

func (tx Tx) Hash() Hash {
	if tx.hash == EmptyHash {
		var txBytes []byte

		txBytes = append(txBytes, tx.ID[:]...)

		for i := range tx.Parents {
			txBytes = append(txBytes, tx.Parents[i][:]...)
		}

		txBytes = append(txBytes, []byte(tx.URL)...)

		for i := range tx.Patches {
			txBytes = append(txBytes, []byte(tx.Patches[i].String())...)
		}

		for i := range tx.Recipients {
			txBytes = append(txBytes, tx.Recipients[i][:]...)
		}

		tx.hash = HashBytes(txBytes)
	}

	return tx.hash
}

func (tx Tx) IsPrivate() bool {
	return len(tx.Recipients) > 0
}

func PrivateRootKeyForRecipients(recipients []Address) string {
	var bs []byte
	for _, r := range recipients {
		bs = append(bs, r[:]...)
	}
	return "private-" + HashBytes(bs).Hex()
}

func (tx Tx) PrivateRootKey() string {
	return PrivateRootKeyForRecipients(tx.Recipients)
}

type Patch struct {
	Keys  []string
	Range *Range
	Val   interface{}
}

type Range struct {
	Start int64
	End   int64
}

func (p Patch) String() string {
	var keypathParts []string
	for _, key := range p.Keys {
		if strings.Contains(key, ".") {
			keypathParts = append(keypathParts, `["`+key+`"]`)
		} else {
			keypathParts = append(keypathParts, KeypathSeparator+key)
		}
	}
	s := strings.Join(keypathParts, "")

	if p.Range != nil {
		s += fmt.Sprintf("[%v:%v]", p.Range.Start, p.Range.End)
	}

	val, err := json.Marshal(p.Val)
	if err != nil {
		panic(err)
	}

	s += " = " + string(val)

	return s
}

func (p Patch) Copy() Patch {
	keys := make([]string, len(p.Keys))
	copy(keys, p.Keys)

	var rng *Range
	if p.Range != nil {
		r := *p.Range
		rng = &r
	}

	// @@TODO: val?

	return Patch{
		Keys:  keys,
		Range: rng,
		Val:   DeepCopyJSValue(p.Val),
	}
}

func (p Patch) RangeStart() int64 {
	if p.Range == nil {
		return -1
	}
	return p.Range.Start
}

func (p Patch) RangeEnd() int64 {
	if p.Range == nil {
		return -1
	}
	return p.Range.End
}

func (p *Patch) UnmarshalJSON(bs []byte) error {
	var err error
	var s string
	err = json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	*p, err = ParsePatch(s)
	return err
}

func (p Patch) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

type ID [32]byte

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

type ChallengeMsg []byte

func (c ChallengeMsg) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(c) + `"`), nil
}

func (c *ChallengeMsg) UnmarshalJSON(bs []byte) error {
	bs, err := hex.DecodeString(string(bs[1 : len(bs)-1]))
	if err != nil {
		return errors.WithStack(err)
	}
	*c = bs
	return nil
}

type Hash [32]byte

func HashBytes(bs []byte) Hash {
	hash := crypto.Keccak256(bs)
	var h Hash
	copy(h[:], hash)
	return h
}

func HashFromHex(hexStr string) (Hash, error) {
	var hash Hash
	err := hash.UnmarshalText([]byte(hexStr))
	return hash, err
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

type LinkType int

const (
	LinkTypeUnknown LinkType = iota
	LinkTypeRef
	LinkTypePath
	LinkTypeURL // @@TODO
)

func DetermineLinkType(linkStr string) (LinkType, string) {
	if strings.HasPrefix(linkStr, "ref:") {
		return LinkTypeRef, linkStr[len("ref:"):]
	} else if strings.HasPrefix(linkStr, "state:") {
		return LinkTypePath, linkStr[len("state:"):]
	}
	return LinkTypeUnknown, linkStr
}
