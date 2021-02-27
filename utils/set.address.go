package utils

import (
	"redwood.dev/types"
)

type AddressSet map[types.Address]struct{}

func NewAddressSet(vals []types.Address) AddressSet {
	set := map[types.Address]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s AddressSet) Add(val types.Address) AddressSet {
	s[val] = struct{}{}
	return s
}

func (s AddressSet) Remove(val types.Address) AddressSet {
	delete(s, val)
	return s
}

func (s AddressSet) Any() types.Address {
	for x := range s {
		return x
	}
	return types.Address{}
}

func (s AddressSet) Slice() []types.Address {
	var slice []types.Address
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s AddressSet) Copy() AddressSet {
	set := map[types.Address]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
