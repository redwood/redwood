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
		ID           ID
		Parents      []ID
		PatchStrings []string
		Patches      []Patch `json:"-"`
	}

	Patch struct {
		Keys  []string
		Range *[2]int64
		Val   interface{}
	}
)

func RandomID() ID {
	var id ID
	rand.Read(id[:])
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

func (tx Tx) MarshalJSON() ([]byte, error) {
	type TxMsg Tx
	var txmsg TxMsg

	txmsg.ID = tx.ID
	txmsg.Parents = tx.Parents
	txmsg.PatchStrings = make([]string, len(tx.Patches))
	for i, patch := range tx.Patches {
		txmsg.PatchStrings[i] = patch.String()
	}
	return json.Marshal(txmsg)
}

func (tx *Tx) UnmarshalJSON(bs []byte) error {
	type TxMsg Tx
	var txmsg TxMsg
	err := json.Unmarshal(bs, &txmsg)
	if err != nil {
		return err
	}

	tx.ID = txmsg.ID
	tx.Parents = txmsg.Parents
	tx.PatchStrings = txmsg.PatchStrings

	tx.Patches = make([]Patch, len(tx.PatchStrings))
	for i, patchStr := range tx.PatchStrings {
		patch, err := ParsePatch(patchStr)
		if err != nil {
			return err
		}
		tx.Patches[i] = patch
	}
	return nil
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
