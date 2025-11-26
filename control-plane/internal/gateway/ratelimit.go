package gateway

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/models"
	"go.uber.org/zap"
)

// RateLimitInfo contains rate limit information for response headers
type RateLimitInfo struct {
	// Limit is the maximum number of requests allowed per window
	Limit int64
	// Remaining is the number of requests remaining in the current window
	Remaining int64
	// ResetAt is the Unix timestamp when the window resets
	ResetAt int64
	// RetryAfter is the number of seconds to wait before retrying (only set when limited)
	RetryAfter int64
}

// RateLimiter handles rate limiting
type RateLimiter struct {
	cache  *cache.Cache
	logger *zap.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cache *cache.Cache, logger *zap.Logger) *RateLimiter {
	return &RateLimiter{
		cache:  cache,
		logger: logger,
	}
}

// CheckRateLimit checks if a request should be rate limited
func (rl *RateLimiter) CheckRateLimit(ctx context.Context, key *models.APIKey) (bool, error) {
	now := time.Now()

	// Check multiple layers of rate limits
	// Layer 1: API Key level
	allowed, err := rl.checkKeyRateLimit(ctx, key, now)
	if err != nil {
		return false, err
	}
	if !allowed {
		rl.logger.Warn("key rate limit exceeded",
			zap.String("key_id", key.ID.String()),
		)
		return false, nil
	}

	// Layer 2: Environment level
	allowed, err = rl.checkEnvironmentRateLimit(ctx, key.EnvironmentID, now)
	if err != nil {
		return false, err
	}
	if !allowed {
		rl.logger.Warn("environment rate limit exceeded",
			zap.String("env_id", key.EnvironmentID.String()),
		)
		return false, nil
	}

	// Layer 3: Tenant/Org level
	allowed, err = rl.checkTenantRateLimit(ctx, key.TenantID, now)
	if err != nil {
		return false, err
	}
	if !allowed {
		rl.logger.Warn("tenant rate limit exceeded",
			zap.String("tenant_id", key.TenantID.String()),
		)
		return false, nil
	}

	return true, nil
}

// checkKeyRateLimit checks rate limit for an API key
func (rl *RateLimiter) checkKeyRateLimit(ctx context.Context, key *models.APIKey, now time.Time) (bool, error) {
	// Per-minute request limit
	minuteKey := fmt.Sprintf("ratelimit:key:%s:minute:%s", key.ID.String(), now.Format("2006-01-02T15:04"))

	count, err := rl.cache.Incr(ctx, minuteKey)
	if err != nil {
		return false, err
	}

	// Set expiration on first increment
	if count == 1 {
		rl.cache.Expire(ctx, minuteKey, 65*time.Second) // 65s to handle edge cases
	}

	limit := int64(key.RateLimitRequestsPerMin)
	if limit == 0 {
		limit = 60 // Default: 60 requests per minute
	}

	if count > limit {
		return false, nil
	}

	// Per-second concurrency limit
	concurrencyKey := fmt.Sprintf("ratelimit:key:%s:concurrency", key.ID.String())
	concurrent, err := rl.cache.Incr(ctx, concurrencyKey)
	if err != nil {
		return false, err
	}

	concurrencyLimit := int64(key.ConcurrencyLimit)
	if concurrencyLimit == 0 {
		concurrencyLimit = 10 // Default: 10 concurrent requests
	}

	if concurrent > concurrencyLimit {
		// Decrement since we're rejecting
		rl.cache.IncrBy(ctx, concurrencyKey, -1)
		return false, nil
	}

	// TODO: Decrement concurrency when request completes
	// This requires tracking request completion, which we'll implement later

	return true, nil
}

// checkEnvironmentRateLimit checks rate limit for an environment
func (rl *RateLimiter) checkEnvironmentRateLimit(ctx context.Context, envID interface{}, now time.Time) (bool, error) {
	minuteKey := fmt.Sprintf("ratelimit:env:%v:minute:%s", envID, now.Format("2006-01-02T15:04"))

	count, err := rl.cache.Incr(ctx, minuteKey)
	if err != nil {
		return false, err
	}

	if count == 1 {
		rl.cache.Expire(ctx, minuteKey, 65*time.Second)
	}

	// Default: 10,000 requests per minute per environment
	limit := int64(10000)

	return count <= limit, nil
}

// checkTenantRateLimit checks rate limit for a tenant
func (rl *RateLimiter) checkTenantRateLimit(ctx context.Context, tenantID interface{}, now time.Time) (bool, error) {
	minuteKey := fmt.Sprintf("ratelimit:tenant:%v:minute:%s", tenantID, now.Format("2006-01-02T15:04"))

	count, err := rl.cache.Incr(ctx, minuteKey)
	if err != nil {
		return false, err
	}

	if count == 1 {
		rl.cache.Expire(ctx, minuteKey, 65*time.Second)
	}

	// Default: 50,000 requests per minute per tenant
	limit := int64(50000)

	return count <= limit, nil
}

// RecordTokenUsage records token usage for quota enforcement
func (rl *RateLimiter) RecordTokenUsage(ctx context.Context, key *models.APIKey, tokens int) error {
	now := time.Now()

	// Per-minute token counter
	minuteKey := fmt.Sprintf("tokens:key:%s:minute:%s", key.ID.String(), now.Format("2006-01-02T15:04"))
	_, err := rl.cache.IncrBy(ctx, minuteKey, int64(tokens))
	if err != nil {
		return err
	}
	rl.cache.Expire(ctx, minuteKey, 65*time.Second)

	// Per-day token counter
	dayKey := fmt.Sprintf("tokens:key:%s:day:%s", key.ID.String(), now.Format("2006-01-02"))
	_, err = rl.cache.IncrBy(ctx, dayKey, int64(tokens))
	if err != nil {
		return err
	}
	rl.cache.Expire(ctx, dayKey, 26*time.Hour)

	return nil
}

// CheckTokenQuota checks if token quota is exceeded
func (rl *RateLimiter) CheckTokenQuota(ctx context.Context, key *models.APIKey, envQuota int64) (bool, error) {
	now := time.Now()

	// Check per-minute quota (if set)
	if key.RateLimitTokensPerMin != nil && *key.RateLimitTokensPerMin > 0 {
		minuteKey := fmt.Sprintf("tokens:key:%s:minute:%s", key.ID.String(), now.Format("2006-01-02T15:04"))
		count, err := rl.cache.Get(ctx, minuteKey)
		if err == nil {
			// Count exists
			var tokenCount int64
			fmt.Sscanf(count, "%d", &tokenCount)
			if tokenCount >= int64(*key.RateLimitTokensPerMin) {
				return false, nil
			}
		}
	}

	// Check per-day quota (environment level)
	if envQuota > 0 {
		dayKey := fmt.Sprintf("tokens:env:%s:day:%s", key.EnvironmentID.String(), now.Format("2006-01-02"))
		count, err := rl.cache.Get(ctx, dayKey)
		if err == nil {
			var tokenCount int64
			fmt.Sscanf(count, "%d", &tokenCount)
			if tokenCount >= envQuota {
				return false, nil
			}
		}
	}

	return true, nil
}

// DecrementConcurrency decrements the concurrency counter
func (rl *RateLimiter) DecrementConcurrency(ctx context.Context, keyID string) error {
	concurrencyKey := fmt.Sprintf("ratelimit:key:%s:concurrency", keyID)
	_, err := rl.cache.IncrBy(ctx, concurrencyKey, -1)
	return err
}

// CheckRateLimitWithInfo checks rate limit and returns info for headers
func (rl *RateLimiter) CheckRateLimitWithInfo(ctx context.Context, key *models.APIKey) (bool, *RateLimitInfo, error) {
	now := time.Now()

	// Calculate window reset time (next minute)
	resetAt := now.Truncate(time.Minute).Add(time.Minute).Unix()

	// Get current count and limit for the key
	minuteKey := fmt.Sprintf("ratelimit:key:%s:minute:%s", key.ID.String(), now.Format("2006-01-02T15:04"))

	// Get current count before increment
	currentCountStr, _ := rl.cache.Get(ctx, minuteKey)
	var currentCount int64 = 0
	if currentCountStr != "" {
		currentCount, _ = strconv.ParseInt(currentCountStr, 10, 64)
	}

	limit := int64(key.RateLimitRequestsPerMin)
	if limit == 0 {
		limit = 60 // Default: 60 requests per minute
	}

	info := &RateLimitInfo{
		Limit:   limit,
		ResetAt: resetAt,
	}

	// Check all layers of rate limits
	allowed, err := rl.CheckRateLimit(ctx, key)
	if err != nil {
		return false, nil, err
	}

	if !allowed {
		info.Remaining = 0
		info.RetryAfter = resetAt - now.Unix()
		if info.RetryAfter < 1 {
			info.RetryAfter = 1
		}
		return false, info, nil
	}

	// After successful check, calculate remaining
	// Note: CheckRateLimit increments, so we need to get the new count
	newCountStr, _ := rl.cache.Get(ctx, minuteKey)
	var newCount int64 = currentCount + 1
	if newCountStr != "" {
		newCount, _ = strconv.ParseInt(newCountStr, 10, 64)
	}

	info.Remaining = limit - newCount
	if info.Remaining < 0 {
		info.Remaining = 0
	}

	return true, info, nil
}

// GetRateLimitHeaders returns HTTP headers for rate limit information
func (info *RateLimitInfo) GetRateLimitHeaders() map[string]string {
	if info == nil {
		return nil
	}

	headers := map[string]string{
		"X-RateLimit-Limit":     strconv.FormatInt(info.Limit, 10),
		"X-RateLimit-Remaining": strconv.FormatInt(info.Remaining, 10),
		"X-RateLimit-Reset":     strconv.FormatInt(info.ResetAt, 10),
	}

	if info.RetryAfter > 0 {
		headers["Retry-After"] = strconv.FormatInt(info.RetryAfter, 10)
	}

	return headers
}
