package main

import (
	"sync"
	"time"
)

type cache[T any] struct {
	mu     sync.RWMutex
	value  T
	expiry time.Time
	ttl    time.Duration
}

func newCache[T any](ttl time.Duration) *cache[T] {
	return &cache[T]{ttl: ttl}
}

func (c *cache[T]) get() (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if time.Now().Before(c.expiry) {
		return c.value, true
	}
	var zero T
	return zero, false
}

func (c *cache[T]) set(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	c.expiry = time.Now().Add(c.ttl)
}
