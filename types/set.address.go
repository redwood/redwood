package types

type AddressSet map[Address]struct{}

func NewAddressSet(vals []Address) AddressSet {
	set := map[Address]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s AddressSet) Add(val Address) AddressSet {
	s[val] = struct{}{}
	return s
}

func (s AddressSet) Remove(val Address) AddressSet {
	delete(s, val)
	return s
}

func (s AddressSet) Contains(val Address) bool {
	_, exists := s[val]
	return exists
}

func (s AddressSet) Any() Address {
	for x := range s {
		return x
	}
	return Address{}
}

func (s AddressSet) Slice() []Address {
	var slice []Address
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s AddressSet) Copy() AddressSet {
	set := map[Address]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
