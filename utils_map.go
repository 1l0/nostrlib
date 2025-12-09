//go:build !tinygo

package nostr

import "github.com/puzpuzpuz/xsync/v3"

type MapOf[K comparable, V any] = xsync.MapOf[K, V]

func NewMapOf[K comparable, V any]() *MapOf[K, V] {
	return xsync.NewMapOf[K, V]()
}

type Counter = xsync.Counter

func NewCounter() *Counter {
	return xsync.NewCounter()
}
