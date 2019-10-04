package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	// log "github.com/sirupsen/logrus"
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
		Keys  []string    `lua:"keys"`
		Range *[2]int64   `lua:"range"`
		Val   interface{} `lua:"val"`
	}
)

var GenesisTxID = IDFromString("genesis")

func (p *Patch) UnmarshalJSON(bs []byte) error {
	var err error
	*p, err = ParsePatch(string(bs[1 : len(bs)-1]))
	return err
}

func (p Patch) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

func (p Patch) Copy() Patch {
	keys := make([]string, len(p.Keys))
	copy(keys, p.Keys)

	var rng *[2]int64
	if p.Range != nil {
		r := [2]int64{p.Range[0], p.Range[1]}
		rng = &r
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
	patchRegexp = regexp.MustCompile(`\.?([^\.\[ =]+)|\[((\-?\d+)(:\-?\d+)?|'(\\'|[^'])*'|"(\\"|[^"])*")\]|\s*=\s*(.*)`)
	ErrBadPatch = errors.New("bad patch string")
)

func ParsePatch(txt string) (Patch, error) {
	matches := patchRegexp.FindAllStringSubmatch(txt, -1)

	var patch Patch
	for _, m := range matches {
		switch {
		case len(m[1]) > 0:
			patch.Keys = append(patch.Keys, m[1])

		case len(m[2]) > 0 && len(m[4]) > 0:
			start, err := strconv.ParseInt(m[3], 10, 64)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}
			end, err := strconv.ParseInt(m[4][1:], 10, 64)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}

			patch.Range = &[2]int64{start, end}

		case len(m[2]) > 0:
			patch.Keys = append(patch.Keys, m[2][1:len(m[2])-1])

		case len(m[7]) > 0:
			err := json.Unmarshal([]byte(m[7]), &patch.Val)
			if err != nil {
				return Patch{}, errors.Wrap(ErrBadPatch, err.Error())
			}

		default:
			return Patch{}, errors.Wrap(ErrBadPatch, "syntax error")
		}
	}

	return patch, nil
}

func (p Patch) String() string {
	var s string
	for i := range p.Keys {
		s += "." + p.Keys[i]
	}

	if p.Range != nil {
		s += fmt.Sprintf("[%d:%d]", p.Range[0], p.Range[1])
	}

	val, err := json.Marshal(p.Val)
	if err != nil {
		panic(err)
	}

	s += " = " + string(val)

	return s
}
