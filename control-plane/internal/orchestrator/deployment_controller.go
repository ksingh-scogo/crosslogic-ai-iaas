package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LoadBalancer interface to avoid import cycle with gateway
type LoadBalancer interface {
	GetAverageLatency(ctx context.Context, modelName string) (time.Duration, error)
}

// Deployment represents a managed set of GPU nodes serving a model.
type Deployment struct {
	ID              string
	Name            string
	ModelName       string
	MinReplicas     int
	MaxReplicas     int
	CurrentReplicas int
	Strategy        string
	Provider        string
	Region          string
	GPUType         string
}

// DeploymentController manages the lifecycle of deployments and auto-scaling.
type DeploymentController struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator
	loadBalancer LoadBalancer
	ticker       *time.Ticker
	stopChan     chan struct{}
}

// NewDeploymentController creates a new deployment controller.
func NewDeploymentController(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator, lb LoadBalancer) *DeploymentController {
	return &DeploymentController{
		db:           db,
		logger:       logger,
		orchestrator: orch,
		loadBalancer: lb,
		stopChan:     make(chan struct{}),
	}
}

// Start begins the reconciliation loop.
func (c *DeploymentController) Start(ctx context.Context) {
	c.logger.Info("starting deployment controller")
	c.ticker = time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.Stop()
				return
			case <-c.stopChan:
				return
			case <-c.ticker.C:
				if err := c.reconcile(ctx); err != nil {
					c.logger.Error("deployment reconciliation failed", zap.Error(err))
				}
			}
		}
	}()
}

// Stop stops the reconciliation loop.
func (c *DeploymentController) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	close(c.stopChan)
	c.logger.Info("stopped deployment controller")
}

// reconcile checks all deployments and scales them if necessary.
func (c *DeploymentController) reconcile(ctx context.Context) error {
	deployments, err := c.getAllDeployments(ctx)
	if err != nil {
		return err
	}

	for _, d := range deployments {
		if err := c.reconcileDeployment(ctx, d); err != nil {
			c.logger.Error("failed to reconcile deployment",
				zap.String("deployment_id", d.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (c *DeploymentController) getAllDeployments(ctx context.Context) ([]Deployment, error) {
	query := `
		SELECT id, name, model_name, min_replicas, max_replicas, current_replicas, strategy, provider, region, gpu_type
		FROM deployments
	`
	rows, err := c.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query deployments: %w", err)
	}
	defer rows.Close()

	var deployments []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(
			&d.ID, &d.Name, &d.ModelName, &d.MinReplicas, &d.MaxReplicas,
			&d.CurrentReplicas, &d.Strategy, &d.Provider, &d.Region, &d.GPUType,
		); err != nil {
			c.logger.Error("failed to scan deployment", zap.Error(err))
			continue
		}
		deployments = append(deployments, d)
	}
	return deployments, nil
}

func (c *DeploymentController) reconcileDeployment(ctx context.Context, d Deployment) error {
	// Count active nodes for this deployment
	activeNodes, err := c.countActiveNodes(ctx, d.ID)
	if err != nil {
		return err
	}

	c.logger.Debug("reconciling deployment",
		zap.String("name", d.Name),
		zap.Int("active_nodes", activeNodes),
		zap.Int("min", d.MinReplicas),
		zap.Int("max", d.MaxReplicas),
	)

	// Update current_replicas in DB
	if activeNodes != d.CurrentReplicas {
		if err := c.updateCurrentReplicas(ctx, d.ID, activeNodes); err != nil {
			c.logger.Warn("failed to update current replicas", zap.Error(err))
		}
	}

	// Scale Up
	if activeNodes < d.MinReplicas {
		needed := d.MinReplicas - activeNodes
		c.logger.Info("scaling up deployment",
			zap.String("name", d.Name),
			zap.Int("needed", needed),
		)
		return c.scaleUp(ctx, d, needed)
	}

	// Scale Down
	if activeNodes > d.MaxReplicas {
		excess := activeNodes - d.MaxReplicas
		c.logger.Info("scaling down deployment",
			zap.String("name", d.Name),
			zap.Int("excess", excess),
		)
		return c.scaleDown(ctx, d, excess)
	}

	// Scale Up based on metrics (Latency)
	if err := c.checkScalingMetrics(ctx, d, activeNodes); err != nil {
		c.logger.Error("failed to check scaling metrics", zap.Error(err))
	}

	return nil
}

func (c *DeploymentController) checkScalingMetrics(ctx context.Context, d Deployment, activeNodes int) error {
	// Don't scale if we are already at max replicas
	if activeNodes >= d.MaxReplicas {
		return nil
	}

	// Get average latency from load balancer
	avgLatency, err := c.loadBalancer.GetAverageLatency(ctx, d.ModelName)
	if err != nil {
		return err
	}

	// Threshold: 200ms (from plan)
	if avgLatency > 200*time.Millisecond {
		c.logger.Info("high latency detected, scaling up",
			zap.String("deployment", d.Name),
			zap.Duration("avg_latency", avgLatency),
		)
		// Scale up by 1
		return c.scaleUp(ctx, d, 1)
	}

	// TODO: Scale down logic based on low latency (optional for now)
	return nil
}

func (c *DeploymentController) countActiveNodes(ctx context.Context, deploymentID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM nodes
		WHERE deployment_id = $1 AND status IN ('initializing', 'active', 'ready')
	`
	var count int
	err := c.db.Pool.QueryRow(ctx, query, deploymentID).Scan(&count)
	return count, err
}

func (c *DeploymentController) updateCurrentReplicas(ctx context.Context, deploymentID string, count int) error {
	query := `UPDATE deployments SET current_replicas = $1 WHERE id = $2`
	_, err := c.db.Pool.Exec(ctx, query, count, deploymentID)
	return err
}

func (c *DeploymentController) scaleUp(ctx context.Context, d Deployment, count int) error {
	// Generate optimal config if GPU type is "auto"
	gpuType := d.GPUType
	gpuCount := 1
	if gpuType == "auto" || gpuType == "" {
		generator := NewModelConfigGenerator()
		gpuType, gpuCount, _ = generator.GetOptimalConfig(d.ModelName)
	}

	// Launch nodes
	for i := 0; i < count; i++ {
		config := NodeConfig{
			NodeID:       uuid.New().String(),
			Provider:     d.Provider,
			Region:       d.Region,
			GPU:          gpuType,
			GPUCount:     gpuCount,
			Model:        d.ModelName,
			UseSpot:      true, // Default to spot for cost savings
			DeploymentID: d.ID,
		}

		// Launch asynchronously to avoid blocking
		go func(cfg NodeConfig) {
			if _, err := c.orchestrator.LaunchNode(context.Background(), cfg); err != nil {
				c.logger.Error("failed to launch scaled node",
					zap.String("deployment", d.Name),
					zap.Error(err),
				)
			}
		}(config)
	}
	return nil
}

func (c *DeploymentController) scaleDown(ctx context.Context, d Deployment, count int) error {
	// Find nodes to terminate (oldest first)
	query := `
		SELECT cluster_name FROM nodes
		WHERE deployment_id = $1 AND status IN ('active', 'ready')
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := c.db.Pool.Query(ctx, query, d.ID, count)
	if err != nil {
		return err
	}
	defer rows.Close()

	var clusters []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		clusters = append(clusters, name)
	}

	for _, cluster := range clusters {
		go func(name string) {
			if err := c.orchestrator.TerminateNode(context.Background(), name); err != nil {
				c.logger.Error("failed to terminate scaled node",
					zap.String("cluster", name),
					zap.Error(err),
				)
			}
		}(cluster)
	}

	return nil
}
