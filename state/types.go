package state

import (
	"bytes"
	"encoding/hex"

	"redwood.dev/errors"
	"redwood.dev/state/pb"
	"redwood.dev/types"
)

var (
	ErrWrongType         = errors.New("wrong type")
	ErrNodeEncoding      = errors.New("corrupted encoding for node")
	ErrInvalidRange      = errors.New("invalid range")
	ErrRangeOverNonSlice = errors.New("range over non-slice")
	ErrNilKeypath        = errors.New("nil keypath")
)

// A Node is a view over the database at a particular keypath
type Node interface {
	Close()

	Keypath() Keypath
	NodeInfo(keypath Keypath) (NodeType, ValueType, uint64, error)
	Length() (uint64, error)
	Subkeys() []Keypath
	NumSubkeys() uint64
	IndexOfMapSubkey(rootKeypath Keypath, subkey Keypath) (uint64, error)
	// NthMapSubkey(rootKeypath Keypath, n uint64) (Keypath, error)
	Exists(keypath Keypath) (bool, error)
	NodeAt(keypath Keypath, rng *Range) Node
	ParentNodeFor(keypath Keypath) (Node, Keypath)
	DebugPrint(printFn func(inFormat string, args ...any), newlines bool, indentLevel int)

	Value(keypath Keypath, rng *Range) (any, bool, error)
	UintValue(keypath Keypath) (uint64, bool, error)
	IntValue(keypath Keypath) (int64, bool, error)
	FloatValue(keypath Keypath) (float64, bool, error)
	BoolValue(keypath Keypath) (bool, bool, error)
	StringValue(keypath Keypath) (string, bool, error)
	BytesValue(keypath Keypath) ([]byte, bool, error)
	MapValue(keypath Keypath) (map[string]any, bool, error)
	SliceValue(keypath Keypath) ([]any, bool, error)
	Scan(into any) error

	Set(keypath Keypath, rng *Range, val any) error
	Delete(keypath Keypath, rng *Range) error

	Diff() *Diff
	ResetDiff()
	CopyToMemory(keypath Keypath, rng *Range) (Node, error)

	Iterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator
	ChildIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator
	DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator

	innerNode(relKeypath Keypath) Node
}

type NodeRange interface {
	Length() (uint64, error)
	Subkeys() []Keypath
	NumSubkeys() uint64
	Value(keypath Keypath, rng *Range) (any, bool, error)
}

type NodeType uint8

const (
	NodeTypeInvalid NodeType = iota
	NodeTypeValue
	NodeTypeMap
	NodeTypeSlice
	NodeTypeNode
)

func (nt NodeType) String() string {
	switch nt {
	case NodeTypeValue:
		return "Value"
	case NodeTypeMap:
		return "Map"
	case NodeTypeSlice:
		return "Slice"
	case NodeTypeNode:
		return "Node"
	default:
		return "Invalid"
	}
}

type ValueType uint8

const (
	ValueTypeInvalid ValueType = iota
	ValueTypeString
	ValueTypeBytes
	ValueTypeUint
	ValueTypeInt
	ValueTypeFloat
	ValueTypeBool
	ValueTypeNil
)

func (vt ValueType) String() string {
	switch vt {
	case ValueTypeString:
		return "String"
	case ValueTypeBytes:
		return "Bytes"
	case ValueTypeUint:
		return "Uint"
	case ValueTypeInt:
		return "Int"
	case ValueTypeFloat:
		return "Float"
	case ValueTypeBool:
		return "Bool"
	case ValueTypeNil:
		return "Nil"
	default:
		return "Invalid"
	}
}

type Range = pb.Range

type Iterator interface {
	RootKeypath() Keypath
	Rewind()
	SeekTo(relKeypath Keypath)
	Valid() bool
	Next()
	Node() Node
	NodeCopy() Node
	Close()

	seekTo(absKeypath Keypath)
}

type Diff struct {
	Added       map[string]struct{}
	AddedList   []Keypath
	Removed     map[string]struct{}
	RemovedList []Keypath
	enabled     bool
}

func NewDiff() *Diff {
	return &Diff{
		Added:   make(map[string]struct{}),
		Removed: make(map[string]struct{}),
		enabled: true,
	}
}

func (d *Diff) SetEnabled(enabled bool) {
	d.enabled = enabled
}

func (d *Diff) Enabled() bool {
	return d.enabled
}

func (d *Diff) Add(keypath Keypath) {
	if d == nil || !d.enabled {
		return
	}
	_, exists := d.Added[string(keypath)]
	if !exists {
		d.Added[string(keypath)] = struct{}{}
		d.AddedList = append(d.AddedList, keypath)
	}
}

func (d *Diff) AddMany(keypaths []Keypath) {
	if d == nil || !d.enabled {
		return
	}
	for _, kp := range keypaths {
		d.Add(kp)
	}
}

func (d *Diff) Remove(keypath Keypath) {
	if d == nil || !d.enabled {
		return
	}
	_, exists := d.Removed[string(keypath)]
	if !exists {
		d.Removed[string(keypath)] = struct{}{}
		d.RemovedList = append(d.RemovedList, keypath)
	}
}

func (d *Diff) RemoveMany(keypaths []Keypath) {
	if d == nil || !d.enabled {
		return
	}
	for _, kp := range keypaths {
		d.Remove(kp)
	}
}

func (d *Diff) Copy() *Diff {
	if d == nil {
		return NewDiff()
	}
	d2 := &Diff{
		Added:       make(map[string]struct{}, len(d.Added)),
		AddedList:   make([]Keypath, len(d.AddedList)),
		Removed:     make(map[string]struct{}, len(d.Removed)),
		RemovedList: make([]Keypath, len(d.RemovedList)),
	}
	for i, x := range d.AddedList {
		d2.AddedList[i] = x.Copy()
	}
	for i, x := range d.RemovedList {
		d2.RemovedList[i] = x.Copy()
	}
	for kp := range d.Added {
		d2.Added[kp] = struct{}{}
	}
	for kp := range d.Removed {
		d2.Removed[kp] = struct{}{}
	}
	return d2
}

type Version [32]byte

func RandomVersion() Version {
	bs := types.RandomBytes(len(Version{}))
	var v Version
	copy(v[:], bs)
	return v
}

func VersionFromHex(h string) (Version, error) {
	bs, err := hex.DecodeString(h)
	if err != nil {
		return Version{}, errors.WithStack(err)
	}
	var id Version
	copy(id[:], bs)
	return id, nil
}

func VersionFromString(s string) Version {
	var id Version
	copy(id[:], []byte(s))
	return id
}

func VersionFromBytes(bs []byte) Version {
	var id Version
	copy(id[:], bs)
	return id
}

func (id Version) String() string {
	return id.Hex()
}

func (id Version) Bytes() []byte {
	return id[:]
}

func (id Version) Pretty() string {
	return id.String()[:6]
}

func (id Version) Hex() string {
	return hex.EncodeToString(id[:])
}

func (id Version) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(id[:])), nil
}

func (id *Version) UnmarshalText(text []byte) error {
	if id == nil {
		id = &Version{}
	}
	bs, err := hex.DecodeString(string(text))
	if err != nil {
		return errors.WithStack(err)
	}
	copy((*id)[:], bs)
	return nil
}

func (id Version) Marshal() ([]byte, error) { return id[:], nil }

func (id *Version) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*id)[:])
	return len(data), nil
}

func (id *Version) Unmarshal(data []byte) error {
	*id = Version{}
	copy((*id)[:], data)
	return nil
}

func (id *Version) Size() int { return len(*id) }
func (id Version) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.Hex() + `"`), nil
}
func (id *Version) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		*id = Version{}
		return nil
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*id = VersionFromBytes(bs)
	return err
}
func (id Version) Compare(other Version) int { return bytes.Compare(id[:], other[:]) }
func (id Version) Equal(other Version) bool  { return bytes.Equal(id[:], other[:]) }

func NewPopulatedVersion(_ gogoprotobufTest) *Version {
	var id Version
	copy(id[:], types.RandomBytes(32))
	return &id
}

func (id Version) MapKey() ([]byte, error) {
	return []byte(id.Hex()), nil
}

func (id *Version) ScanMapKey(keypath []byte) error {
	bs, err := hex.DecodeString(string(keypath))
	if err != nil {
		return err
	}
	copy((*id)[:], bs)
	return nil
}
