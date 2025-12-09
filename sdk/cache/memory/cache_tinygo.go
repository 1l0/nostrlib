//go:build tinygo

package cache_memory

import (
	"sync"
	"time"
)

type RistrettoCache[V any] struct {
	backend *simpleCache[V]
}

type item[V any] struct {
	value      V
	expiration time.Time
}

type simpleCache[V any] struct {
	mu      sync.Mutex
	items   map[[32]byte]item[V]
	maxSize int64
}

func New[V any](max int64) *RistrettoCache[V] {
	return &RistrettoCache[V]{
		backend: &simpleCache[V]{
			items:   make(map[[32]byte]item[V]),
			maxSize: max,
		},
	}
}

func (s RistrettoCache[V]) Get(k [32]byte) (v V, ok bool) {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()

	it, found := s.backend.items[k]
	if !found {
		return v, false
	}

	if !it.expiration.IsZero() && time.Now().After(it.expiration) {
		delete(s.backend.items, k)
		return v, false
	}

	return it.value, true
}

func (s RistrettoCache[V]) Delete(k [32]byte) {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	delete(s.backend.items, k)
}

func (s RistrettoCache[V]) Set(k [32]byte, v V) bool {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()

	if int64(len(s.backend.items)) >= s.backend.maxSize {
		// Evict one random item
		for k := range s.backend.items {
			delete(s.backend.items, k)
			break
		}
	}

	s.backend.items[k] = item[V]{value: v}
	return true
}

func (s RistrettoCache[V]) SetWithTTL(k [32]byte, v V, d time.Duration) bool {
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()

	if int64(len(s.backend.items)) >= s.backend.maxSize {
		for k := range s.backend.items {
			delete(s.backend.items, k)
			break
		}
	}

	s.backend.items[k] = item[V]{
		value:      v,
		expiration: time.Now().Add(d),
	}
	return true
}
