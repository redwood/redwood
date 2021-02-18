package utils

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type StringSet map[string]struct{}

func NewStringSet(vals []string) StringSet {
	set := map[string]struct{}{}
	for _, val := range vals {
		set[val] = struct{}{}
	}
	return set
}

func (s StringSet) Add(val string) StringSet {
	if s == nil {
		s = StringSet(make(map[string]struct{}))
	}
	s[val] = struct{}{}
	return s
}

func (s StringSet) Remove(val string) StringSet {
	if s == nil {
		return nil
	}
	delete(s, val)
	return s
}

func (s StringSet) Any() string {
	for x := range s {
		if strings.Contains(x, "dns4") {
			return x
		}
	}
	for x := range s {
		return x
	}
	return ""
}

func (s StringSet) Contains(str string) bool {
	_, ok := s[str]
	return ok
}

func (s StringSet) Slice() []string {
	var slice []string
	for x := range s {
		slice = append(slice, x)
	}
	return slice
}

func (s StringSet) Copy() StringSet {
	set := map[string]struct{}{}
	for val := range s {
		set[val] = struct{}{}
	}
	return set
}

func (s StringSet) MarshalYAML() (interface{}, error) {
	return s.Slice(), nil
}

func (s *StringSet) UnmarshalYAML(node *yaml.Node) error {
	var slice []string
	if err := node.Decode(&slice); err != nil {
		return err
	}
	*s = NewStringSet(slice)
	return nil
}

type SortedStringSet struct {
	values map[string]struct{}
	order  []string
}

func NewSortedStringSet(vals []string) *SortedStringSet {
	values := map[string]struct{}{}
	order := []string{}
	for _, val := range vals {
		values[val] = struct{}{}
		order = append(order, val)
	}
	return &SortedStringSet{values, order}
}

func (s SortedStringSet) Len() int {
	return len(s.order)
}

func (s SortedStringSet) Contains(val string) bool {
	_, exists := s.values[val]
	return exists
}

func (s SortedStringSet) ForEach(fn func(val string) bool) {
	for _, val := range s.order {
		ok := fn(val)
		if !ok {
			break
		}
	}
}

func (s SortedStringSet) Add(val string) SortedStringSet {
	s.values[val] = struct{}{}
	s.order = append(s.order, val)
	return s
}

func (s SortedStringSet) Remove(val string) SortedStringSet {
	delete(s.values, val)
	idx := -1
	for i, id := range s.order {
		if id == val {
			idx = i
			break
		}
	}
	if idx > -1 {
		if idx < len(s.order)-1 {
			s.order = append(s.order[:idx], s.order[idx+1:]...)
		} else {
			s.order = s.order[:idx]
		}
	}
	return s
}

func (s SortedStringSet) Pop() string {
	id := s.order[len(s.order)-1]
	delete(s.values, id)
	s.order = s.order[:len(s.order)-1]
	return id
}

func (s SortedStringSet) Any() string {
	var id string
	for x := range s.values {
		id = x
		break
	}
	s.Remove(id)
	return id
}

func (s SortedStringSet) Slice() []string {
	slice := make([]string, len(s.order))
	copy(slice, s.order)
	return slice
}

func (s SortedStringSet) Copy() *SortedStringSet {
	set := map[string]struct{}{}
	for val := range s.values {
		set[val] = struct{}{}
	}
	return &SortedStringSet{
		values: set,
		order:  s.Slice(),
	}
}
