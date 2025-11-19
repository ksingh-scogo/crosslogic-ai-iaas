package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// ModelCacheWarmer handles pre-warming of model weights in JuiceFS cache.
type ModelCacheWarmer struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator
}

// NewModelCacheWarmer creates a new cache warmer.
func NewModelCacheWarmer(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator) *ModelCacheWarmer {
	return &ModelCacheWarmer{
		db:           db,
		logger:       logger,
		orchestrator: orch,
	}
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
