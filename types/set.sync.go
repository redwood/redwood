package types

import (
	"sync"
	"gopkg.in/yaml.v3"
)

type SyncSet[T comparable] struct {
    set Set[T]
    mu sync.RWMutex
}

func NewSyncSet[T comparable](vals []T) SyncSet[T] {
    return SyncSet[T]{
        set: NewSet[T](vals),
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

func (s SyncSet[T]) Any() T {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.set.Any()
}

func (s SyncSet[T]) Contains(val T) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.set.Contains(val)
}

func (s SyncSet[T]) Slice() []T {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.set.Slice()
}

func (s SyncSet[T]) Copy() SyncSet[T] {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return SyncSet[T]{set: s.set.Copy()}
}

func (s SyncSet[T]) Equal(other SyncSet[T]) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    other.mu.RLock()
    defer other.mu.RUnlock()
    return s.set.Equal(other.set)
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
