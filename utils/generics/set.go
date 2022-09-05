package generics

import (
	"gopkg.in/yaml.v3"
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](vals []T) Set[T] {
	set := make(map[T]struct{}, len(vals))
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s Set[T]) Add(val T) bool {
	if s == nil {
		s = NewSet[T](nil)
	}
	_, exists := s[val]
	s[val] = struct{}{}
	return exists
}

func (s Set[T]) AddAll(vals ...T) {
	for _, val := range vals {
		s.Add(val)
	}
}

func (s Set[T]) Remove(val T) bool {
	if s == nil {
		return false
	}
	_, exists := s[val]
	delete(s, val)
	return exists
}

func (s Set[T]) Clear() {
	if s == nil {
		return
	}
	for x := range s {
		delete(s, x)
	}
}

func (s Set[T]) Replace(ts []T) {
	s.Clear()
	s.AddAll(ts...)
}

func (s Set[T]) Any() (T, bool) {
	if s != nil {
		for x := range s {
			return x, true
		}
	}
	var x T
	return x, false
}

func (s Set[T]) Contains(val T) bool {
	if s == nil {
		return false
	}
	_, ok := s[val]
	return ok
}

func (s Set[T]) Length() int {
	return len(s)
}

func (s Set[T]) Intersection(other Set[T]) Set[T] {
	intersection := NewSet[T](nil)
	for item := range s {
		if other.Contains(item) {
			intersection.Add(item)
		}
	}
	return intersection
}

func (s Set[T]) Union(other Set[T]) Set[T] {
	union := make(Set[T], len(s)+len(other))
	for item := range s {
		union[item] = struct{}{}
	}
	for item := range other {
		union[item] = struct{}{}
	}
	return union
}

func (s Set[T]) Slice() []T {
	var slice []T
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s Set[T]) Copy() Set[T] {
	set := map[T]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}

func (s Set[T]) Equal(other Set[T]) bool {
	if len(s) != len(other) {
		return false
	}
	for x := range s {
		if !other.Contains(x) {
			return false
		}
	}
	return true
}

func (s Set[T]) MarshalYAML() (interface{}, error) {
	return s.Slice(), nil
}

func (s *Set[T]) UnmarshalYAML(node *yaml.Node) error {
	var slice []T
	if err := node.Decode(&slice); err != nil {
		return err
	}
	*s = NewSet(slice)
	return nil
}