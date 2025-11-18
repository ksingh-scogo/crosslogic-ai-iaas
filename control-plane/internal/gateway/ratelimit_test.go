package gateway

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/crosslogic/control-plane/internal/config"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func setupLimiterCache(t *testing.T) (*cache.Cache, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	cfg := config.RedisConfig{
		Host: mr.Host(),
		Port: func() int {
			port, _ := strconv.Atoi(mr.Port())
			return port
		}(),
		DB: 0,
	}
	c, err := cache.NewCache(cfg)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to init cache: %v", err)
	}
	return c, func() {
		c.Close()
		mr.Close()
	}
}

func TestRateLimiterConcurrencyWindow(t *testing.T) {
	cacheClient, cleanup := setupLimiterCache(t)
	defer cleanup()

	rl := NewRateLimiter(cacheClient, zap.NewNop())
	apiKey := &models.APIKey{
		ID:                      uuid.New(),
		TenantID:                uuid.New(),
		EnvironmentID:           uuid.New(),
		RateLimitRequestsPerMin: 100,
		ConcurrencyLimit:        2,
		Status:                  "active",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	allowed, err := rl.CheckRateLimit(ctx, apiKey)
	if err != nil || !allowed {
		t.Fatalf("first request should be allowed: %v", err)
	}

	allowed, err = rl.CheckRateLimit(ctx, apiKey)
	if err != nil || !allowed {
		t.Fatalf("second request should be allowed: %v", err)
	}

	allowed, err = rl.CheckRateLimit(ctx, apiKey)
	if err != nil {
		t.Fatalf("third request error: %v", err)
	}
	if allowed {
		t.Fatal("concurrency limit should reject third simultaneous request")
	}

	if err := rl.DecrementConcurrency(context.Background(), apiKey.ID.String()); err != nil {
		t.Fatalf("failed to decrement concurrency: %v", err)
	}

	allowed, err = rl.CheckRateLimit(ctx, apiKey)
	if err != nil || !allowed {
		t.Fatalf("request after decrement should be allowed: %v", err)
	}
}
