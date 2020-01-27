package tree

import (
	"github.com/pkg/errors"
)

var (
	ErrNodeEncoding      = errors.New("corrupted encoding for node")
	ErrInvalidRange      = errors.New("invalid range")
	ErrRangeOverNonSlice = errors.New("range over non-slice")
)

type Range [2]int64

func (rng *Range) Copy() *Range {
	if rng == nil {
		return nil
	}
	return &Range{rng[0], rng[1]}
}

func (rng *Range) Valid() bool {
	if rng[1] < rng[0] {
		return false
	}
	if rng[0] < 0 && rng[1] > 0 {
		return false
	}
	return true
}

func (rng *Range) Size() uint64 {
	if rng[0] < 0 {
		return uint64(-(rng[0] - rng[1]))
	}
	return uint64(rng[1] - rng[0])
}

func (rng *Range) ValidForLength(length uint64) bool {
	if rng[0] < 0 {
		return uint64(-rng[0]) <= length
	}
	return uint64(rng[1]) <= length
}

func (rng *Range) IndicesForLength(length uint64) (uint64, uint64) {
	if rng[0] < 0 {
		return uint64(int64(length) + rng[0]), uint64(int64(length) + rng[1])
	}
	return uint64(rng[0]), uint64(rng[1])
}

type Node interface {
	Close()
	Keypath() Keypath
	AtKeypath(keypath Keypath, rng *Range) Node
	Value(keypath Keypath, rng *Range) (interface{}, bool, error)
	UintValue(keypath Keypath) (uint64, bool, error)
	IntValue(keypath Keypath) (int64, bool, error)
	FloatValue(keypath Keypath) (float64, bool, error)
	StringValue(keypath Keypath) (string, bool, error)
	ContentLength() (int64, error)
	Exists(keypath Keypath) (bool, error)
	Set(keypath Keypath, rng *Range, val interface{}) error
	Delete(keypath Keypath, rng *Range) error
	Diff() *Diff
	ResetDiff()
	CopyToMemory(keypath Keypath, rng *Range) (Node, error)
	DepthFirstIterator(keypath Keypath, prefetchValues bool, prefetchSize int) Iterator
}

type NodeType uint8

const (
	NodeTypeInvalid NodeType = iota
	NodeTypeValue
	NodeTypeMap
	NodeTypeSlice
)

type ValueType uint8

const (
	ValueTypeInvalid ValueType = iota
	ValueTypeString
	ValueTypeUint
	ValueTypeInt
	ValueTypeFloat
	ValueTypeBool
)

type Iterator interface {
	Next() Node
	Close()
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
	if !d.enabled {
		return
	}
	_, exists := d.Added[string(keypath)]
	if !exists {
		d.Added[string(keypath)] = struct{}{}
		d.AddedList = append(d.AddedList, keypath)
	}
}

func (d *Diff) AddMany(keypaths []Keypath) {
	if !d.enabled {
		return
	}
	for _, kp := range keypaths {
		d.Add(kp)
	}
}

func (d *Diff) Remove(keypath Keypath) {
	if !d.enabled {
		return
	}
	_, exists := d.Removed[string(keypath)]
	if !exists {
		d.Removed[string(keypath)] = struct{}{}
		d.RemovedList = append(d.RemovedList, keypath)
	}
}

func (d *Diff) RemoveMany(keypaths []Keypath) {
	if !d.enabled {
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
