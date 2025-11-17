package gateway

import (
	"errors"
	"sync"
	"time"

	"github.com/crosslogic-ai-iaas/control-plane/pkg/cache"
)

// RateLimiter implements a token bucket per API key.
type RateLimiter struct {
	cache      *cache.LocalCache
	refillRate time.Duration
	mu         sync.Mutex
	buckets    map[string]*bucket
}

type bucket struct {
	tokens        int
	capacity      int
	lastRefreshed time.Time
}

func NewRateLimiter(cache *cache.LocalCache, refillRate time.Duration) *RateLimiter {
	return &RateLimiter{
		cache:      cache,
		refillRate: refillRate,
		buckets:    make(map[string]*bucket),
	}
}

// Allow checks if a request can proceed based on available tokens.
func (r *RateLimiter) Allow(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.buckets[key]
	if !ok {
		b = &bucket{capacity: 10, tokens: 10, lastRefreshed: time.Now()}
		r.buckets[key] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRefreshed)
	if elapsed >= r.refillRate {
		tokensToAdd := int(elapsed / r.refillRate)
		b.tokens = min(b.capacity, b.tokens+tokensToAdd)
		b.lastRefreshed = now
	}

	if b.tokens <= 0 {
		return errors.New("rate limit exceeded")
	}

	b.tokens--
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
