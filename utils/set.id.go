package utils

import (
	"redwood.dev/types"
)

type IDSet map[types.ID]struct{}

func NewIDSet(vals []types.ID) IDSet {
	set := map[types.ID]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s IDSet) Add(val types.ID) IDSet {
	s[val] = struct{}{}
	return s
}

func (s IDSet) Remove(val types.ID) IDSet {
	delete(s, val)
	return s
}

func (s IDSet) Any() types.ID {
	for x := range s {
		return x
	}
	return types.ID{}
}

func (s IDSet) Slice() []types.ID {
	var slice []types.ID
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s IDSet) Copy() IDSet {
	set := map[types.ID]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
