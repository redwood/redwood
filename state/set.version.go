package state

type VersionSet map[Version]struct{}

func NewVersionSet(vals []Version) VersionSet {
	set := map[Version]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s VersionSet) Add(val Version) VersionSet {
	s[val] = struct{}{}
	return s
}

func (s VersionSet) Remove(val Version) VersionSet {
	delete(s, val)
	return s
}

func (s VersionSet) Any() Version {
	for x := range s {
		return x
	}
	return Version{}
}

func (s VersionSet) Slice() []Version {
	var slice []Version
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s VersionSet) Copy() VersionSet {
	set := map[Version]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}
