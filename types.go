package redwood

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

type (
	ID           [32]byte
	Address      [20]byte
	Hash         [32]byte
	Signature    []byte
	ChallengeMsg []byte

	Tx struct {
		ID         ID        `json:"id"`
		Parents    []ID      `json:"parents"`
		From       Address   `json:"from"`
		Sig        Signature `json:"sig,omitempty"`
		URL        string    `json:"url"`
		Patches    []Patch   `json:"patches"`
		Recipients []Address `json:"recipients,omitempty"`

		Valid bool `json:"-"`
		hash  Hash `json:"-"`
	}

	Patch struct {
		Keys  []string
		Range *Range
		Val   interface{}
	}

	Range struct {
		Start int64
		End   int64
	}
)

var (
	GenesisTxID = IDFromString("genesis")
	EmptyHash   = Hash{}
)

const (
	KeypathSeparator = "."
)

func SignatureFromHex(hx string) (Signature, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return nil, err
	}
	return Signature(bs), nil
}

func (sig Signature) String() string {
	return hex.EncodeToString(sig)
}

func (sig Signature) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(sig) + `"`), nil
}

func (sig *Signature) UnmarshalJSON(bs []byte) error {
	bs, err := hex.DecodeString(string(bs[1 : len(bs)-1]))
	if err != nil {
		return err
	}
	*sig = bs
	return nil
}

func (c ChallengeMsg) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(c) + `"`), nil
}

func (c *ChallengeMsg) UnmarshalJSON(bs []byte) error {
	bs, err := hex.DecodeString(string(bs[1 : len(bs)-1]))
	if err != nil {
		return err
	}
	*c = bs
	return nil
}

func AddressFromHex(hx string) (Address, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return Address{}, err
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
		return err
	}
	copy((*a)[:], bs)
	return nil
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
	return "private" + HashBytes(bs).String()
}

func (tx Tx) PrivateRootKey() string {
	return PrivateRootKeyForRecipients(tx.Recipients)
}

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

func (h *Hash) UnmarshalText(text []byte) error {
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return err
	}
	copy((*h)[:], bs)
	return nil
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) Pretty() string {
	return h.String()[:6]
}

func (p Patch) String() string {
	s := KeypathSeparator + strings.Join(p.Keys, KeypathSeparator)

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

func (id *ID) UnmarshalText(text []byte) error {
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return err
	}
	copy((*id)[:], bs)
	return nil
}

func (id ID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(id[:])), nil
}

func RandomID() ID {
	var id ID
	rand.Read(id[:])
	return id
}

func IDFromHex(h string) (ID, error) {
	bs, err := hex.DecodeString(h)
	if err != nil {
		return ID{}, err
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

func (id ID) Pretty() string {
	return id.String()[:6]
}

func (id ID) Hex() string {
	return hex.EncodeToString(id[:])
}

func (id ID) String() string {
	return id.Hex()
}

func (tx *Tx) PrettyJSON() string {
	j, _ := json.MarshalIndent(tx, "", "    ")
	return string(j)
}
