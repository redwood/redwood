package pb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

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

	// var recipients []types.Address
	// if len(tx.Recipients) > 0 {
	//  recipients = make([]types.Address, len(tx.Recipients))
	//  for i, addr := range tx.Recipients {
	//      recipients[i] = addr
	//  }
	// }

	attachment := make([]byte, len(tx.Attachment))
	copy(attachment, tx.Attachment)

	return Tx{
		ID:       tx.ID,
		Parents:  parents,
		Children: children,
		From:     tx.From,
		Sig:      tx.Sig.Copy(),
		StateURI: tx.StateURI,
		Patches:  patches,
		// Recipients: recipients,
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
	parts := p.Keypath.Parts()
	var keypathParts []string
	for _, key := range parts {
		if bytes.IndexByte(key, '.') > -1 {
			keypathParts = append(keypathParts, `["`+string(key)+`"]`)
		} else {
			keypathParts = append(keypathParts, KeypathSeparator+string(key))
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

	return s
}

func (p Patch) Copy() Patch {
	val := make([]byte, len(p.ValueJSON))
	copy(val, p.ValueJSON)
	return Patch{
		Keypath:   p.Keypath.Copy(),
		Range:     p.Range.Copy(),
		ValueJSON: val,
	}
}

func (p *Patch) UnmarshalJSON(bs []byte) error {
	var s string
	err := json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	*p, err = ParsePatch([]byte(s))
	return err
}

func (p Patch) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

// func (status TxStatus) Marshal() ([]byte, error) { return []byte(status), nil }

// func (status *TxStatus) MarshalTo(data []byte) (n int, err error) {
// 	if len(data) < len(*status) {
// 		return 0, io.ErrUnexpectedEOF
// 	}
// 	copy(data, []byte(*status))
// 	return len(*status), nil
// }

// func (status *TxStatus) Unmarshal(data []byte) error {
// 	switch TxStatus(data) {
// 	case TxStatusUnknown,
// 		TxStatusInMempool,
// 		TxStatusInvalid,
// 		TxStatusValid:
// 		*status = TxStatus(data)
// 		return nil
// 	default:
// 		return errors.Errorf("bad txstatus value: %v", string(data))
// 	}
// }

// func (status *TxStatus) Size() int { return len(*status) }
// func (status TxStatus) MarshalJSON() ([]byte, error) {
// 	return []byte(`"` + string(status) + `"`), nil
// }
// func (status *TxStatus) UnmarshalJSON(data []byte) error {
// 	if len(data) < 3 {
// 		return errors.Errorf("bad txstatus JSON value: %v", string(data))
// 	}
// 	return status.Unmarshal(data[1 : len(data)-1])
// }
// func (status TxStatus) Compare(other TxStatus) int {
// 	return strings.Compare(string(status), string(other))
// }
// func (status TxStatus) Equal(other TxStatus) bool {
// 	return string(status) == string(other)
// }

// type gogoprotobufTest interface {
// 	Float32() float32
// 	Float64() float64
// 	Int63() int64
// 	Int31() int32
// 	Uint32() uint32
// 	Intn(n int) int
// }

// func NewPopulatedTxStatus(_ gogoprotobufTest) *TxStatus {
// 	var s TxStatus
// 	switch rand.Intn(4) {
// 	case 0:
// 		s = TxStatusUnknown
// 	case 1:
// 		s = TxStatusInMempool
// 	case 2:
// 		s = TxStatusInvalid
// 	case 3:
// 		s = TxStatusValid
// 	default:
// 		panic("invariant violation")
// 	}
// 	return &s
// }
