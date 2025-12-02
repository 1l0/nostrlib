package cache_memory

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	c := New[string](100)
	key := [32]byte{1}

	c.Set(key, "hello")
	time.Sleep(50 * time.Millisecond)
	val, ok := c.Get(key)
	if !ok {
		t.Fatal("expected to find key")
	}
	if val != "hello" {
		t.Fatalf("expected hello, got %s", val)
	}

	c.Delete(key)
	_, ok = c.Get(key)
	if ok {
		t.Fatal("expected not to find key")
	}
}

func TestTTL(t *testing.T) {
	c := New[string](100)
	key := [32]byte{2}

	c.SetWithTTL(key, "hello", 100*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	val, ok := c.Get(key)
	if !ok {
		t.Fatal("expected to find key")
	}
	if val != "hello" {
		t.Fatalf("expected hello, got %s", val)
	}

	time.Sleep(200 * time.Millisecond)
	_, ok = c.Get(key)
	if ok {
		t.Fatal("expected key to expire")
	}
}

func TestEviction(t *testing.T) {
	max := int64(100)
	c := New[int](max)

	for i := 0; i < 200; i++ {
		k := [32]byte{}
		binary.BigEndian.PutUint64(k[24:], uint64(i))
		c.Set(k, i)
	}

	time.Sleep(200 * time.Millisecond)

	count := 0
	for i := 0; i < 200; i++ {
		k := [32]byte{}
		binary.BigEndian.PutUint64(k[24:], uint64(i))
		if _, ok := c.Get(k); ok {
			count++
		}
	}

	if count > int(max)+10 {
		t.Fatalf("expected around %d items, got %d", max, count)
	}
	if count == 0 {
		t.Fatalf("expected some items, got 0")
	}
}
