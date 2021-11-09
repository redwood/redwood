package types

type HashSet map[Hash]struct{}

func NewHashSet(vals []Hash) HashSet {
	set := map[Hash]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s HashSet) Add(val Hash) HashSet {
	s[val] = struct{}{}
	return s
}

func (s HashSet) Remove(val Hash) HashSet {
	delete(s, val)
	return s
}

func (s HashSet) Any() Hash {
	for x := range s {
		return x
	}
	return Hash{}
}

func (s HashSet) Slice() []Hash {
	var slice []Hash
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s HashSet) Copy() HashSet {
	set := map[Hash]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
