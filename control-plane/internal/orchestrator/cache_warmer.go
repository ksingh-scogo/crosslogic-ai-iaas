package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// CacheWarmingStrategy defines how to warm the cache
type CacheWarmingStrategy string

const (
	// StrategyFull warms the entire model (default)
	StrategyFull CacheWarmingStrategy = "full"
	// StrategyPartial warms only frequently accessed parts
	StrategyPartial CacheWarmingStrategy = "partial"
	// StrategyPredictive warms based on usage predictions
	StrategyPredictive CacheWarmingStrategy = "predictive"
)

// ModelAccessPattern tracks model access patterns for intelligent warming
type ModelAccessPattern struct {
	ModelName      string
	AccessCount    int64
	LastAccessTime time.Time
	AvgLatency     time.Duration
	CacheHitRate   float64
}

// ModelCacheWarmer handles pre-warming of model weights in JuiceFS cache.
type ModelCacheWarmer struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator

	// Configuration
	autoWarmOnLaunch    bool
	predictiveEnabled   bool
	warmupInterval      time.Duration

	// Tracking
	accessPatterns sync.Map // modelName -> *ModelAccessPattern
	stopChan       chan struct{}
}

// NewModelCacheWarmer creates a new cache warmer.
func NewModelCacheWarmer(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator) *ModelCacheWarmer {
	return &ModelCacheWarmer{
		db:                db,
		logger:            logger,
		orchestrator:      orch,
		autoWarmOnLaunch:  true,  // Enable auto-warm by default
		predictiveEnabled: true,  // Enable predictive warming
		warmupInterval:    30 * time.Minute, // Check every 30 minutes
		stopChan:          make(chan struct{}),
	}
}

// Start begins background predictive warming
func (w *ModelCacheWarmer) Start(ctx context.Context) {
	if !w.predictiveEnabled {
		w.logger.Info("predictive cache warming disabled")
		return
	}

	w.logger.Info("starting predictive cache warming",
		zap.Duration("interval", w.warmupInterval),
	)

	go w.predictiveWarmingLoop(ctx)
}

// Stop stops the cache warmer
func (w *ModelCacheWarmer) Stop() {
	close(w.stopChan)
}

// predictiveWarmingLoop periodically warms caches based on usage patterns
func (w *ModelCacheWarmer) predictiveWarmingLoop(ctx context.Context) {
	ticker := time.NewTicker(w.warmupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.predictiveWarmup(ctx)
		}
	}
}

// predictiveWarmup analyzes usage patterns and warms frequently accessed models
func (w *ModelCacheWarmer) predictiveWarmup(ctx context.Context) {
	w.logger.Info("running predictive cache warmup")

	// Get models sorted by access frequency
	models, err := w.getTopAccessedModels(ctx, 10) // Top 10 models
	if err != nil {
		w.logger.Error("failed to get top accessed models", zap.Error(err))
		return
	}

	for _, modelName := range models {
		// Check if model needs warming
		if w.shouldWarmModel(modelName) {
			w.logger.Info("predictive warming for high-traffic model",
				zap.String("model", modelName),
			)

			if err := w.Prewarm(ctx, modelName); err != nil {
				w.logger.Error("predictive warmup failed",
					zap.String("model", modelName),
					zap.Error(err),
				)
			}
		}
	}
}

// getTopAccessedModels retrieves the most frequently accessed models
func (w *ModelCacheWarmer) getTopAccessedModels(ctx context.Context, limit int) ([]string, error) {
	query := `
		SELECT model_name, COUNT(*) as access_count
		FROM usage_records
		WHERE created_at > NOW() - INTERVAL '1 hour'
		GROUP BY model_name
		ORDER BY access_count DESC
		LIMIT $1
	`

	rows, err := w.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []string
	for rows.Next() {
		var modelName string
		var accessCount int64
		if err := rows.Scan(&modelName, &accessCount); err != nil {
			continue
		}
		models = append(models, modelName)

		// Update access patterns
		w.accessPatterns.Store(modelName, &ModelAccessPattern{
			ModelName:      modelName,
			AccessCount:    accessCount,
			LastAccessTime: time.Now(),
		})
	}

	return models, nil
}

// shouldWarmModel determines if a model should be warmed based on patterns
func (w *ModelCacheWarmer) shouldWarmModel(modelName string) bool {
	val, ok := w.accessPatterns.Load(modelName)
	if !ok {
		return false // No pattern data, skip
	}

	pattern := val.(*ModelAccessPattern)

	// Warm if:
	// 1. High access count (>100 in last hour)
	// 2. Recent access (within last 5 minutes)
	// 3. Low cache hit rate (<80%)
	highAccess := pattern.AccessCount > 100
	recentAccess := time.Since(pattern.LastAccessTime) < 5*time.Minute
	lowCacheHit := pattern.CacheHitRate < 0.8

	return highAccess && recentAccess && lowCacheHit
}

// RecordModelAccess records model access for predictive warming
func (w *ModelCacheWarmer) RecordModelAccess(modelName string, latency time.Duration, cacheHit bool) {
	val, _ := w.accessPatterns.LoadOrStore(modelName, &ModelAccessPattern{
		ModelName: modelName,
	})

	pattern := val.(*ModelAccessPattern)
	pattern.AccessCount++
	pattern.LastAccessTime = time.Now()

	// Update cache hit rate (exponential moving average)
	hitValue := 0.0
	if cacheHit {
		hitValue = 1.0
	}

	if pattern.CacheHitRate == 0 {
		pattern.CacheHitRate = hitValue
	} else {
		pattern.CacheHitRate = pattern.CacheHitRate*0.9 + hitValue*0.1
	}

	// Update average latency
	if pattern.AvgLatency == 0 {
		pattern.AvgLatency = latency
	} else {
		pattern.AvgLatency = time.Duration(float64(pattern.AvgLatency)*0.9 + float64(latency)*0.1)
	}
}

// WarmOnLaunch automatically warms cache when a new node is launched
func (w *ModelCacheWarmer) WarmOnLaunch(ctx context.Context, clusterName, modelName string) error {
	if !w.autoWarmOnLaunch {
		return nil // Auto-warm disabled
	}

	w.logger.Info("auto-warming cache on node launch",
		zap.String("cluster", clusterName),
		zap.String("model", modelName),
	)

	// Give the node a few seconds to fully initialize
	time.Sleep(5 * time.Second)

	return w.warmupNode(ctx, clusterName, modelName)
}

// PrewarmWithStrategy triggers cache warming with a specific strategy
func (w *ModelCacheWarmer) PrewarmWithStrategy(ctx context.Context, modelName string, strategy CacheWarmingStrategy) error {
	switch strategy {
	case StrategyFull:
		return w.Prewarm(ctx, modelName)
	case StrategyPartial:
		return w.prewarmPartial(ctx, modelName)
	case StrategyPredictive:
		// Use predictive logic to determine what to warm
		if w.shouldWarmModel(modelName) {
			return w.Prewarm(ctx, modelName)
		}
		return nil
	default:
		return w.Prewarm(ctx, modelName)
	}
}

// prewarmPartial warms only frequently accessed parts of the model
func (w *ModelCacheWarmer) prewarmPartial(ctx context.Context, modelName string) error {
	w.logger.Info("partial cache warming", zap.String("model", modelName))

	// Find active nodes serving this model
	nodes, err := w.getNodesForModel(ctx, modelName)
	if err != nil {
		return fmt.Errorf("failed to get nodes for model: %w", err)
	}

	if len(nodes) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(nodes))

	// Warm only the most frequently accessed files
	// For transformers models, this is typically:
	// - model.safetensors or pytorch_model.bin (weights)
	// - config.json, tokenizer files
	for _, clusterName := range nodes {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()

			// Warm only critical files
			cmd := fmt.Sprintf(`juicefs warmup --files-only --max-size 10GB /mnt/models/%s`, modelName)
			ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			_, err := w.orchestrator.ExecCommand(ctx, cluster, cmd)
			if err != nil {
				errChan <- fmt.Errorf("node %s: %w", cluster, err)
			}
		}(clusterName)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		w.logger.Error("partial warmup failed", zap.Error(err))
	}

	return nil
}

// Prewarm triggers cache warming for a specific model on all active nodes serving it.
//
// This executes `juicefs warmup` on the nodes, which pulls data from S3 into the
// local NVMe cache. This is useful when scaling up or after a cold start.
func (w *ModelCacheWarmer) Prewarm(ctx context.Context, modelName string) error {
	w.logger.Info("starting model cache warming", zap.String("model", modelName))

	// Find active nodes serving this model
	nodes, err := w.getNodesForModel(ctx, modelName)
	if err != nil {
		return fmt.Errorf("failed to get nodes for model: %w", err)
	}

	if len(nodes) == 0 {
		w.logger.Info("no active nodes found for model, skipping warmup", zap.String("model", modelName))
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(nodes))

	// Trigger warmup on each node in parallel
	for _, clusterName := range nodes {
		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			if err := w.warmupNode(ctx, cluster, modelName); err != nil {
				w.logger.Error("failed to warmup node",
					zap.String("cluster", cluster),
					zap.Error(err),
				)
				errChan <- fmt.Errorf("node %s: %w", cluster, err)
			}
		}(clusterName)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("warmup failed on %d/%d nodes", len(errs), len(nodes))
	}

	w.logger.Info("model cache warming completed successfully",
		zap.String("model", modelName),
		zap.Int("nodes_count", len(nodes)),
	)

	return nil
}

func (w *ModelCacheWarmer) getNodesForModel(ctx context.Context, modelName string) ([]string, error) {
	query := `SELECT cluster_name FROM nodes WHERE model_name = $1 AND status = 'active'`
	rows, err := w.db.Pool.Query(ctx, query, modelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		nodes = append(nodes, name)
	}
	return nodes, nil
}

func (w *ModelCacheWarmer) warmupNode(ctx context.Context, clusterName, modelName string) error {
	// Construct warmup command
	// Assuming model is mounted at /mnt/models/{modelName}
	cmd := fmt.Sprintf("juicefs warmup /mnt/models/%s", modelName)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	output, err := w.orchestrator.ExecCommand(ctx, clusterName, cmd)
	if err != nil {
		return err
	}

	w.logger.Debug("warmup output",
		zap.String("cluster", clusterName),
		zap.String("output", output),
	)

	return nil
}
