package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/internal/config"
	"github.com/go-redis/redis/v8"
)

// Cache wraps the Redis client
type Cache struct {
	Client *redis.Client
}

// NewCache creates a new Redis cache client
func NewCache(cfg config.RedisConfig) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.PoolSize / 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("unable to connect to Redis: %w", err)
	}

	return &Cache{Client: client}, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	return c.Client.Close()
}

// Health checks cache health
func (c *Cache) Health(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}

// Set sets a key-value pair with expiration
func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.Client.Get(ctx, key).Result()
}

// Delete deletes a key
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	return c.Client.Del(ctx, keys...).Err()
}

// Incr increments a counter
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	return c.Client.Incr(ctx, key).Result()
}

// IncrBy increments a counter by a specific amount
func (c *Cache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.Client.IncrBy(ctx, key, value).Result()
}

// Expire sets expiration on a key
func (c *Cache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.Client.Expire(ctx, key, expiration).Err()
}

// Exists checks if a key exists
func (c *Cache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.Client.Exists(ctx, keys...).Result()
}
