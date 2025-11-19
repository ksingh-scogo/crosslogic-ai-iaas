package scheduler

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

func setupTestCache(t *testing.T) (*cache.Cache, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	port, _ := strconv.Atoi(mr.Port())
	cfg := config.RedisConfig{
		Host: mr.Host(),
		Port: port,
		DB:   0,
	}

	cacheClient, err := cache.NewCache(cfg)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to create cache: %v", err)
	}

	cleanup := func() {
		cacheClient.Close()
		mr.Close()
	}

	return cacheClient, cleanup
}

func TestLeastLoadedStrategyUsesNodeMetrics(t *testing.T) {
	cacheClient, cleanup := setupTestCache(t)
	defer cleanup()

	logger := zap.NewNop()
	tracker := NewNodeLoadTracker(cacheClient, logger)
	strategy := NewLeastLoadedStrategy(tracker, logger)

	nodeA := &models.Node{ID: uuid.New(), HealthScore: 90}
	nodeB := &models.Node{ID: uuid.New(), HealthScore: 80}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	tracker.Increment(ctx, nodeA.ID, 500)
	tracker.Increment(ctx, nodeA.ID, 500)
	tracker.Increment(ctx, nodeB.ID, 500)

	selected, err := strategy.SelectNode(context.Background(), []*models.Node{nodeA, nodeB})
	if err != nil {
		t.Fatalf("SelectNode failed: %v", err)
	}

	if selected.ID != nodeB.ID {
		t.Fatalf("Expected nodeB (lower concurrency), got %s", selected.ID)
	}
}
