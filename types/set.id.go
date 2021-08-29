package types

type IDSet map[ID]struct{}

func NewIDSet(vals []ID) IDSet {
	set := map[ID]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s IDSet) Len() int {
	return len(s)
}

func (s IDSet) Add(val ID) IDSet {
	s[val] = struct{}{}
	return s
}

func (s IDSet) Remove(val ID) IDSet {
	delete(s, val)
	return s
}

func (s IDSet) Any() ID {
	for x := range s {
		return x
	}
	return ID{}
}

func (s IDSet) Slice() []ID {
	var slice []ID
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s IDSet) HexSlice() []string {
	var slice []string
	for x := range s {
		slice = append(slice, x.Hex())
	}
	return slice
}

func (s IDSet) Copy() IDSet {
	set := map[ID]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
