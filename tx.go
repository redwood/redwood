package redwood

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	proto "github.com/golang/protobuf/proto"
	any "github.com/golang/protobuf/ptypes/any"

	"github.com/brynbellomy/redwood/pb"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

var (
	GenesisTxID = types.IDFromString("genesis")
	EmptyHash   = types.Hash{}
)

const (
	KeypathSeparator = "."
)

type Tx struct {
	ID         types.ID        `json:"id"`
	Parents    []types.ID      `json:"parents"`
	Children   []types.ID      `json:"children"`
	From       types.Address   `json:"from"`
	Sig        types.Signature `json:"sig,omitempty"`
	StateURI   string          `json:"stateURI"`
	Patches    []Patch         `json:"patches"`
	Recipients []types.Address `json:"recipients,omitempty"`
	Checkpoint bool            `json:"checkpoint"` // @@TODO: probably not ideal
	Attachment []byte          `json:"attachment,omitempty"`

	Status TxStatus   `json:"status"`
	hash   types.Hash `json:"-"`
}

type TxStatus string

const (
	TxStatusUnknown   TxStatus = ""
	TxStatusInMempool TxStatus = "in mempool"
	TxStatusInvalid   TxStatus = "invalid"
	TxStatusValid     TxStatus = "valid"
)

func (tx Tx) Hash() types.Hash {
	if tx.hash == types.EmptyHash {
		var txBytes []byte

		txBytes = append(txBytes, tx.ID[:]...)

		for i := range tx.Parents {
			txBytes = append(txBytes, tx.Parents[i][:]...)
		}

		txBytes = append(txBytes, []byte(tx.StateURI)...)

		for i := range tx.Patches {
			txBytes = append(txBytes, []byte(tx.Patches[i].String())...)
		}

		for i := range tx.Recipients {
			txBytes = append(txBytes, tx.Recipients[i][:]...)
		}

		tx.hash = types.HashBytes(txBytes)
	}

	return tx.hash
}

func (tx Tx) IsPrivate() bool {
	return len(tx.Recipients) > 0
}

func (tx *Tx) Copy() *Tx {
	var parents []types.ID
	if len(tx.Parents) > 0 {
		parents = make([]types.ID, len(tx.Parents))
		for i, id := range tx.Parents {
			parents[i] = id
		}
	}

	var children []types.ID
	if len(tx.Children) > 0 {
		children = make([]types.ID, len(tx.Children))
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

	var recipients []types.Address
	if len(tx.Recipients) > 0 {
		recipients := make([]types.Address, len(tx.Recipients))
		for i, addr := range tx.Recipients {
			recipients[i] = addr
		}
	}

	attachment := make([]byte, len(tx.Attachment))
	copy(attachment, tx.Attachment)

	return &Tx{
		ID:         tx.ID,
		Parents:    parents,
		Children:   children,
		From:       tx.From,
		Sig:        tx.Sig.Copy(),
		StateURI:   tx.StateURI,
		Patches:    patches,
		Recipients: recipients,
		Checkpoint: tx.Checkpoint,
		Attachment: attachment,
		Status:     tx.Status,
		hash:       tx.hash,
	}
}

func PrivateRootKeyForRecipients(recipients []types.Address) string {
	sort.Slice(recipients, func(i, j int) bool {
		return bytes.Compare(recipients[i][:], recipients[j][:]) < 0
	})

	var bs []byte
	for _, r := range recipients {
		bs = append(bs, r[:]...)
	}
	return "private-" + types.HashBytes(bs).Hex()
}

func (tx Tx) PrivateRootKey() string {
	return PrivateRootKeyForRecipients(tx.Recipients)
}

func (tx Tx) MarshalProto() ([]byte, error) {
	parents := make([][]byte, len(tx.Parents))
	for i, parent := range tx.Parents {
		parents[i] = parent.Bytes()
	}

	children := make([][]byte, len(tx.Children))
	for i, child := range tx.Children {
		children[i] = child.Bytes()
	}

	patches := make([]*pb.Patch, len(tx.Patches))
	for i, patch := range tx.Patches {
		var rng *pb.Range
		if patch.Range != nil {
			rng = &pb.Range{Start: patch.Range.Start, End: patch.Range.End}
		}

		valueBytes, err := json.Marshal(patch.Val)
		if err != nil {
			return nil, err
		}

		patches[i] = &pb.Patch{
			Keypath: []byte(patch.Keypath),
			Range:   rng,
			Value:   &any.Any{Value: valueBytes},
		}
	}

	recipients := make([][]byte, len(tx.Recipients))
	for i, recipient := range tx.Recipients {
		recipients[i] = recipient[:]
	}

	return proto.Marshal(&pb.Tx{
		Id:         tx.ID[:],
		Parents:    parents,
		Children:   children,
		From:       tx.From[:],
		Sig:        tx.Sig,
		StateURI:   tx.StateURI,
		Patches:    patches,
		Recipients: recipients,
		Checkpoint: tx.Checkpoint,
		Attachment: tx.Attachment,
		Status:     string(tx.Status),
	})
}

func (tx *Tx) UnmarshalProto(bs []byte) error {
	var pbtx pb.Tx
	err := proto.Unmarshal(bs, &pbtx)
	if err != nil {
		return err
	}

	tx.ID = types.IDFromBytes(pbtx.Id)

	tx.Parents = make([]types.ID, len(pbtx.Parents))
	for i, parent := range pbtx.Parents {
		tx.Parents[i] = types.IDFromBytes(parent)
	}

	tx.Children = make([]types.ID, len(pbtx.Children))
	for i, child := range pbtx.Children {
		tx.Children[i] = types.IDFromBytes(child)
	}

	tx.From = types.AddressFromBytes(pbtx.From)
	tx.Sig = pbtx.Sig
	tx.StateURI = pbtx.StateURI

	tx.Patches = make([]Patch, len(pbtx.Patches))
	for i, patch := range pbtx.Patches {
		tx.Patches[i].Keypath = tree.Keypath(patch.Keypath)
		if patch.Range != nil {
			tx.Patches[i].Range = &tree.Range{
				Start: patch.Range.Start,
				End:   patch.Range.End,
			}
		}
		err := json.Unmarshal(patch.Value.Value, &tx.Patches[i].Val)
		if err != nil {
			return err
		}
	}

	tx.Recipients = make([]types.Address, len(pbtx.Recipients))
	for i, recipient := range pbtx.Recipients {
		tx.Recipients[i] = types.AddressFromBytes(recipient)
	}

	tx.Checkpoint = pbtx.Checkpoint
	tx.Attachment = pbtx.Attachment
	tx.Status = TxStatus(pbtx.Status)
	return nil
}

type Patch struct {
	Keypath tree.Keypath
	Range   *tree.Range
	Val     interface{}
}

type Range struct {
	Start int64
	End   int64
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
	return Patch{
		Keypath: p.Keypath.Copy(),
		Range:   p.Range.Copy(),
		Val:     DeepCopyJSValue(p.Val), // @@TODO?
	}
}

func (p *Patch) UnmarshalJSON(bs []byte) error {
	var err error
	var s string
	err = json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	*p, err = ParsePatch([]byte(s))
	return err
}

func (p Patch) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (r *Range) Copy() *Range {
	if r == nil {
		return nil
	}
	return &Range{r.Start, r.End}
}
