package redwood

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

type (
	ID        [32]byte
	Address   [20]byte
	Hash      [32]byte
	Signature []byte

	Tx struct {
		ID         ID        `json:"id"`
		Parents    []Hash    `json:"parents"`
		From       Address   `json:"from"`
		Sig        Signature `json:"sig"`
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
	GenesisTxHash = HashBytes([]byte("genesis"))
	EmptyHash     = Hash{}
)

const (
	KeypathSeparator = "."
)

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

func AddressFromHex(hx string) (Address, error) {
	bs, err := hex.DecodeString(hx)
	if err != nil {
		return Address{}, err
	}
	var addr Address
	copy(addr[:], bs)
	return addr, nil
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

func (p Patch) Copy() Patch {
	keys := make([]string, len(p.Keys))
	copy(keys, p.Keys)

	var rng *Range
	if p.Range != nil {
		*rng = *p.Range
	}

	return Patch{
		Keys:  keys,
		Range: rng,
	}
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

func IDFromString(s string) ID {
	var id ID
	copy(id[:], []byte(s))
	return id
}

func (id ID) Pretty() string {
	return id.String()[:6]
}

func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

func (tx *Tx) PrettyJSON() string {
	j, _ := json.MarshalIndent(tx, "", "    ")
	return string(j)
}

// var (
// 	patchRegexp = regexp.MustCompile(`\.?([^\.\[ =]+)|\[((\-?\d+)(:\-?\d+)?|'(\\'|[^'])*'|"(\\"|[^"])*")\]|\s*=\s*([.\n]*)`)
// 	ErrBadPatch = errors.New("bad patch string")
// )

// func ParsePatch(txt string) (Patch, error) {
// 	matches := patchRegexp.FindAllStringSubmatch(txt, -1)

// 	for i, m := range matches {
// 		fmt.Println(i, "---------------------")
// 		for j := range m {
// 			fmt.Println("  - ", j, m[j])
// 		}
// 	}

// 	var patch Patch
// 	for i, m := range matches {
// 		switch {
// 		case len(m[1]) > 0:
// 			patch.Keys = append(patch.Keys, m[1])

// 		case len(m[2]) > 0 && len(m[4]) > 0:
// 			start, err := strconv.ParseInt(m[3], 10, 64)
// 			if err != nil {
// 				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
// 			}
// 			end, err := strconv.ParseInt(m[4][1:], 10, 64)
// 			if err != nil {
// 				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
// 			}

// 			patch.Range = &[2]int64{start, end}

// 		case len(m[2]) > 0:
// 			patch.Keys = append(patch.Keys, m[2][1:len(m[2])-1])

// 		case len(m[7]) > 0:
// 			err := json.Unmarshal([]byte(m[7]), &patch.Val)
// 			if err != nil {
// 				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
// 			}

// 		default:
// 			return Patch{}, errors.Wrap(ErrBadPatch, "syntax error: "+txt)
// 		}
// 	}

// 	return patch, nil
// }

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
