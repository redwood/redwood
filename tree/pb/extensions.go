package pb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

const (
	KeypathSeparator = "."
)

func (tx Tx) Hash() types.Hash {
	var txBytes []byte

	txBytes = append(txBytes, tx.ID.Bytes()...)

	for i := range tx.Parents {
		txBytes = append(txBytes, tx.Parents[i].Bytes()...)
	}

	txBytes = append(txBytes, []byte(tx.StateURI)...)

	for i := range tx.Patches {
		txBytes = append(txBytes, []byte(tx.Patches[i].String())...)
	}
	return types.HashBytes(txBytes)
}

func (tx Tx) Copy() Tx {
	var parents []state.Version
	if len(tx.Parents) > 0 {
		parents = make([]state.Version, len(tx.Parents))
		for i, id := range tx.Parents {
			parents[i] = id
		}
	}

	var children []state.Version
	if len(tx.Children) > 0 {
		children = make([]state.Version, len(tx.Children))
		for i, id := range tx.Children {
			children[i] = id
		}
	}

	var patches []Patch
	if len(tx.Patches) > 0 {
		patches = make([]Patch, len(tx.Patches))
		for i, p := range tx.Patches {
			patches[i] = p.Copy()
		}
	}

	attachment := make([]byte, len(tx.Attachment))
	copy(attachment, tx.Attachment)

	return Tx{
		ID:         tx.ID,
		Parents:    parents,
		Children:   children,
		From:       tx.From,
		Sig:        tx.Sig.Copy(),
		StateURI:   tx.StateURI,
		Patches:    patches,
		Checkpoint: tx.Checkpoint,
		Attachment: attachment,
		Status:     tx.Status,
	}
}

func (p Patch) Value() (interface{}, error) {
	var val interface{}
	err := json.Unmarshal(p.ValueJSON, &val)
	return val, err
}

func (p Patch) String() string {
	bs, err := p.MarshalText()
	if err != nil {
		return fmt.Sprintf("<ERR while marshaling patch keypath '"+string(p.Keypath)+"': %v>", p.Keypath)
	}
	return string(bs)
}

var ErrBadPatch = errors.New("bad patch string")
var equalsSign byte = '='

func (p Patch) MarshalText() ([]byte, error) {
	parts := p.Keypath.Parts()
	var keypathParts []string
	for _, part := range parts {
		if bytes.IndexByte(part, KeypathSeparator[0]) > -1 {
			keypathParts = append(keypathParts, `["`+string(part)+`"]`)
		} else {
			keypathParts = append(keypathParts, KeypathSeparator+string(part))
		}
	}
	s := strings.Join(keypathParts, "")

	if p.Range != nil {
		if p.Range.Reverse {
			s += fmt.Sprintf("[-%v:-%v]", p.Range.Start, p.Range.End)
		} else {
			s += fmt.Sprintf("[%v:%v]", p.Range.Start, p.Range.End)
		}
	}

	s += " = " + string(p.ValueJSON)

	return []byte(s), nil
}

func (p *Patch) UnmarshalText(bs []byte) error {
	idx := bytes.IndexByte(bs, equalsSign)
	if idx < 0 {
		return errors.Wrapf(ErrBadPatch, "no '=' sign")
	}

	keypath, rng, err := state.ParseKeypathAndRange(bs[:idx], KeypathSeparator[0])
	if err != nil {
		return errors.Wrapf(err, "%v", ErrBadPatch)
	}

	bs = bytes.TrimSpace(bs[idx+1:])
	if len(bs) == 0 {
		return errors.Wrapf(ErrBadPatch, "no value")
	}
	valueJSON := make([]byte, len(bs))
	copy(valueJSON, bs)

	*p = Patch{
		Keypath:   keypath,
		Range:     rng,
		ValueJSON: valueJSON,
	}
	return nil
}

func (p Patch) MarshalJSON() ([]byte, error) {
	bs, err := p.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(bs))
}

func (p *Patch) UnmarshalJSON(bs []byte) error {
	var s string
	err := json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	return p.UnmarshalText([]byte(s))
}

func (p Patch) Copy() Patch {
	valueJSON := make([]byte, len(p.ValueJSON))
	copy(valueJSON, p.ValueJSON)
	return Patch{
		Keypath:   p.Keypath,
		Range:     p.Range.Copy(),
		ValueJSON: valueJSON,
	}
}
