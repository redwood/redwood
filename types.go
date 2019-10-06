package redwood

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"

	"github.com/pkg/errors"
)

type (
	ID [32]byte

	Tx struct {
		ID      ID      `json:"id"`
		Parents []ID    `json:"parents"`
		From    ID      `json:"from"`
		URL     string  `json:"url"`
		Patches []Patch `json:"patches"`
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

var GenesisTxID = IDFromString("genesis")

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
	s := hex.EncodeToString(id[:])
	return s[:6]
}

func (id ID) String() string {
	return id.Pretty()
}

func (tx *Tx) PrettyJSON() string {
	j, _ := json.MarshalIndent(tx, "", "    ")
	return string(j)
}

var (
	patchRegexp = regexp.MustCompile(`\.?([^\.\[ =]+)|\[((\-?\d+)(:\-?\d+)?|'(\\'|[^'])*'|"(\\"|[^"])*")\]|\s*=\s*([.\n]*)`)
	ErrBadPatch = errors.New("bad patch string")
)

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
	var s string
	for i := range p.Keys {
		s += "." + p.Keys[i]
	}

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
