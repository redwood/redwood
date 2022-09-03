package generics

import (
	"sync"
)

type SyncMap[K comparable, V any] struct {
	m  map[K]V
	mu *sync.RWMutex
}

func NewSyncMap[K comparable, V any]() SyncMap[K, V] {
	return SyncMap[K, V]{
		m:  make(map[K]V),
		mu: &sync.RWMutex{},
	}
}

func (m SyncMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, exists := m.m[key]
	return val, exists
}

func (m SyncMap[K, V]) Set(key K, value V) (exists bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists = m.m[key]
	m.m[key] = value
	return exists
}

func (m SyncMap[K, V]) Delete(key K) (v V, exists bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, exists = m.m[key]
	delete(m.m, key)
	return v, exists
}

func (m SyncMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]K, len(m.m))
	i := 0
	for k := range m.m {
		keys[i] = k
	}
	return keys
}

func (m SyncMap[K, V]) Zip() []Tuple2[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zipped := make([]Tuple2[K, V], len(m.m))
	i := 0
	for k, v := range m.m {
		zipped[i] = Tuple2[K, V]{k, v}
		i++
	}
	return zipped
}

func (m SyncMap[K, V]) Range(fn func(key K, value V) (keepGoing bool)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, v := range m.m {
		keepGoing := fn(k, v)
		if !keepGoing {
			break
		}
	}
}
