package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	nodeLoadTTL           = 5 * time.Minute
	nodeLoadKeyTemplate   = "scheduler:nodes:%s:concurrency"
	nodeTokensKeyTemplate = "scheduler:nodes:%s:pending_tokens"
)

// NodeLoadTracker tracks per-node active request counts and pending tokens using Redis.
type NodeLoadTracker struct {
	cache  *cache.Cache
	logger *zap.Logger
}

// NewNodeLoadTracker creates a tracker backed by Redis. Returns nil if cache is nil.
func NewNodeLoadTracker(cache *cache.Cache, logger *zap.Logger) *NodeLoadTracker {
	if cache == nil {
		return nil
	}
	return &NodeLoadTracker{
		cache:  cache,
		logger: logger,
	}
}

// Increment increments the active request counter and pending tokens for a node.
func (t *NodeLoadTracker) Increment(ctx context.Context, nodeID uuid.UUID, estimatedTokens int) {
	if t == nil {
		return
	}

	activeKey := fmt.Sprintf(nodeLoadKeyTemplate, nodeID)
	if _, err := t.cache.Incr(ctx, activeKey); err != nil && t.logger != nil {
		t.logger.Warn("failed to increment node concurrency",
			zap.String("node_id", nodeID.String()),
			zap.Error(err),
		)
	} else {
		_ = t.cache.Expire(ctx, activeKey, nodeLoadTTL)
	}

	if estimatedTokens > 0 {
		tokenKey := fmt.Sprintf(nodeTokensKeyTemplate, nodeID)
		if _, err := t.cache.IncrBy(ctx, tokenKey, int64(estimatedTokens)); err != nil && t.logger != nil {
			t.logger.Warn("failed to increment node pending tokens",
				zap.String("node_id", nodeID.String()),
				zap.Error(err),
			)
		} else {
			_ = t.cache.Expire(ctx, tokenKey, nodeLoadTTL)
		}
	}
}

// Decrement decrements the active request counter and pending tokens for a node.
func (t *NodeLoadTracker) Decrement(ctx context.Context, nodeID uuid.UUID, estimatedTokens int) {
	if t == nil {
		return
	}

	activeKey := fmt.Sprintf(nodeLoadKeyTemplate, nodeID)
	if current, err := t.cache.IncrBy(ctx, activeKey, -1); err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to decrement node concurrency",
				zap.String("node_id", nodeID.String()),
				zap.Error(err),
			)
		}
	} else if current < 0 {
		_ = t.cache.Set(ctx, activeKey, 0, nodeLoadTTL)
	}

	if estimatedTokens > 0 {
		tokenKey := fmt.Sprintf(nodeTokensKeyTemplate, nodeID)
		if current, err := t.cache.IncrBy(ctx, tokenKey, int64(-estimatedTokens)); err != nil {
			if t.logger != nil {
				t.logger.Warn("failed to decrement node pending tokens",
					zap.String("node_id", nodeID.String()),
					zap.Error(err),
				)
			}
		} else if current < 0 {
			_ = t.cache.Set(ctx, tokenKey, 0, nodeLoadTTL)
		}
	}
}

// GetLoad returns the current concurrency and pending tokens for a node.
func (t *NodeLoadTracker) GetLoad(ctx context.Context, nodeID uuid.UUID) (int64, int64, error) {
	if t == nil {
		return 0, 0, nil
	}

	activeKey := fmt.Sprintf(nodeLoadKeyTemplate, nodeID)
	active, _, err := t.cache.GetInt64(ctx, activeKey)
	if err != nil {
		return 0, 0, err
	}

	tokenKey := fmt.Sprintf(nodeTokensKeyTemplate, nodeID)
	pending, _, err := t.cache.GetInt64(ctx, tokenKey)
	if err != nil {
		return 0, 0, err
	}

	return active, pending, nil
}
