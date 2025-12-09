//go:build tinygo

package nostr

import (
	"sync"
	"sync/atomic"
)

type MapOf[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]V
}

func NewMapOf[K comparable, V any]() *MapOf[K, V] {
	return &MapOf[K, V]{
		m: make(map[K]V),
	}
}

func (m *MapOf[K, V]) Load(key K) (value V, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok = m.m[key]
	return
}

func (m *MapOf[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *MapOf[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	actual, loaded = m.m[key]
	if loaded {
		return actual, true
	}
	m.m[key] = value
	return value, false
}

func (m *MapOf[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

func (m *MapOf[K, V]) Range(yield func(key K, value V) bool) {
	m.mu.Lock()
	// Copy keys to avoid holding lock during iteration or modification issues
	// Since we can't easily copy map without knowing size, and we want to be safe.
	// A full copy is safest.
	snapshot := make(map[K]V, len(m.m))
	for k, v := range m.m {
		snapshot[k] = v
	}
	m.mu.Unlock()

	for k, v := range snapshot {
		if !yield(k, v) {
			return
		}
	}
}

func (m *MapOf[K, V]) Compute(key K, computeFunc func(oldValue V, loaded bool) (newValue V, delete bool)) (actual V, deleted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldValue, loaded := m.m[key]
	newValue, shouldDelete := computeFunc(oldValue, loaded)
	if shouldDelete {
		delete(m.m, key)
		return oldValue, true
	}
	m.m[key] = newValue
	return newValue, false
}

type Counter struct {
	v atomic.Int64
}

func NewCounter() *Counter {
	return &Counter{}
}

func (c *Counter) Add(delta int64) {
	c.v.Add(delta)
}

func (c *Counter) Dec() {
	c.v.Add(-1)
}

func (c *Counter) Value() int64 {
	return c.v.Load()
}
