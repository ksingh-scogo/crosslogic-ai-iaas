package cache

import (
	"sync"
	"time"
)

// LocalCache simulates Redis-like TTL caching for development without dependencies.
type LocalCache struct {
	mu    sync.RWMutex
	items map[string]cachedItem
}

type cachedItem struct {
	value      any
	expiration time.Time
}

func NewLocalCache() *LocalCache {
	return &LocalCache{items: make(map[string]cachedItem)}
}

// Set stores a value with TTL semantics.
func (c *LocalCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cachedItem{value: value, expiration: time.Now().Add(ttl)}
}

// Get returns a cached value if it exists and is fresh.
func (c *LocalCache) Get(key string) (any, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	return item.value, true
}
