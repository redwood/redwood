package generics

import (
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

type SyncSet[T comparable] struct {
	set Set[T]
	mu  *sync.RWMutex
}

func NewSyncSet[T comparable](vals []T) SyncSet[T] {
	return SyncSet[T]{
		set: NewSet[T](vals),
		mu:  &sync.RWMutex{},
	}
}

func (s SyncSet[T]) Add(val T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set.Add(val)
}

func (s SyncSet[T]) Remove(val T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set.Remove(val)
}

func (s SyncSet[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set.Clear()
}

func (s SyncSet[T]) Replace(ts []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set.Replace(ts)
}

func (s SyncSet[T]) Any() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.Any()
}

func (s SyncSet[T]) Contains(val T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.Contains(val)
}

func (s SyncSet[T]) Length() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.set)
}

func (s SyncSet[T]) Slice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.Slice()
}

func (s SyncSet[T]) Copy() SyncSet[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SyncSet[T]{set: s.set.Copy(), mu: &sync.RWMutex{}}
}

func (s SyncSet[T]) Unwrap() Set[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.Copy()
}

func (s SyncSet[T]) Equal(other SyncSet[T]) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	return s.set.Equal(other.set)
}

func (s SyncSet[T]) ForEach(fn func(t T)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for x := range s.set {
		fn(x)
	}
}

func (s SyncSet[T]) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.set)
}

func (s *SyncSet[T]) UnmarshalJSON(bs []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(bs, &s.set)
}

func (s SyncSet[T]) MarshalYAML() (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.set.MarshalYAML()
}

func (s *SyncSet[T]) UnmarshalYAML(node *yaml.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set.UnmarshalYAML(node)
}
