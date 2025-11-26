package skypilot

// This file provides example integration patterns for the SkyPilot HTTP client
// in the CrossLogic AI IaaS control plane.
//
// NOTE: This is example code for reference. The actual integration should be
// implemented in the orchestrator package.

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ExampleClusterManager demonstrates how to use the SkyPilot client
// in a production orchestration system
type ExampleClusterManager struct {
	client *Client
	logger *zap.Logger
}

// NewExampleClusterManager creates a new cluster manager
func NewExampleClusterManager(apiURL, token string, logger *zap.Logger) *ExampleClusterManager {
	client := NewClient(Config{
		BaseURL:       apiURL,
		Token:         token,
		Timeout:       10 * time.Minute,
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		RetryMaxDelay: 30 * time.Second,
	}, logger)

	return &ExampleClusterManager{
		client: client,
		logger: logger,
	}
}

// LaunchVLLMCluster launches a vLLM cluster with the specified configuration
func (m *ExampleClusterManager) LaunchVLLMCluster(
	ctx context.Context,
	clusterName string,
	modelName string,
	gpuType string,
	gpuCount int,
	cloud string,
	region string,
	tenantCredentials *CloudCredentials,
) (*ClusterStatus, error) {
	m.logger.Info("launching vLLM cluster",
		zap.String("cluster_name", clusterName),
		zap.String("model", modelName),
		zap.String("gpu", gpuType),
		zap.Int("gpu_count", gpuCount),
		zap.String("cloud", cloud),
		zap.String("region", region),
	)

	// Build SkyPilot task YAML
	taskYAML := m.buildVLLMTaskYAML(modelName, gpuType, gpuCount, cloud, region)

	// Launch the cluster
	resp, err := m.client.Launch(ctx, LaunchRequest{
		ClusterName:       clusterName,
		TaskYAML:          taskYAML,
		RetryUntilUp:      true,
		Detach:            true,
		IdleMinutesToStop: 30, // Auto-stop after 30 min of idle
		Envs: map[string]string{
			"MODEL_NAME": modelName,
			"VLLM_PORT":  "8000",
		},
		CloudCredentials: tenantCredentials, // Multi-tenant: inject tenant's credentials
	})
	if err != nil {
		return nil, fmt.Errorf("launch cluster: %w", err)
	}

	m.logger.Info("cluster launch initiated, waiting for completion",
		zap.String("request_id", resp.RequestID),
	)

	// Wait for the launch to complete
	status, err := m.client.WaitForRequest(ctx, resp.RequestID, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for launch: %w", err)
	}

	if status.Status != "completed" {
		return nil, fmt.Errorf("launch failed: %s", status.Error)
	}

	// Get the final cluster status
	clusterStatus, err := m.client.GetStatus(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("get cluster status: %w", err)
	}

	m.logger.Info("vLLM cluster launched successfully",
		zap.String("cluster_name", clusterName),
		zap.String("status", clusterStatus.Status),
		zap.String("endpoint", clusterStatus.Endpoints.Custom["vllm"]),
		zap.Float64("cost_per_hour", clusterStatus.CostPerHour),
	)

	return clusterStatus, nil
}

// buildVLLMTaskYAML generates a SkyPilot task YAML for vLLM deployment
func (m *ExampleClusterManager) buildVLLMTaskYAML(
	modelName string,
	gpuType string,
	gpuCount int,
	cloud string,
	region string,
) string {
	// This is a simplified example. In production, use a proper YAML builder
	// or template engine (e.g., text/template or a YAML library).
	return fmt.Sprintf(`
resources:
  cloud: %s
  region: %s
  accelerators: %s:%d
  disk_size: 512
  disk_tier: high

setup: |
  # Install vLLM and dependencies
  conda create -n vllm python=3.10 -y
  conda activate vllm
  pip install vllm==0.6.2 torch==2.4.0

  # Download model weights (example, adjust based on model source)
  echo "Downloading model: %s"

run: |
  conda activate vllm

  # Start vLLM server
  python -m vllm.entrypoints.openai.api_server \
    --model %s \
    --port 8000 \
    --host 0.0.0.0 \
    --tensor-parallel-size %d \
    --max-model-len 4096 \
    --gpu-memory-utilization 0.95
`,
		cloud,
		region,
		gpuType,
		gpuCount,
		modelName,
		modelName,
		gpuCount,
	)
}

// ScaleCluster scales a cluster to the specified number of nodes
func (m *ExampleClusterManager) ScaleCluster(
	ctx context.Context,
	clusterName string,
	targetNodes int,
) error {
	m.logger.Info("scaling cluster",
		zap.String("cluster_name", clusterName),
		zap.Int("target_nodes", targetNodes),
	)

	// Get current status
	status, err := m.client.GetStatus(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("get cluster status: %w", err)
	}

	if status.Status != "UP" {
		return fmt.Errorf("cluster not ready for scaling: status=%s", status.Status)
	}

	// Execute scaling command (example - actual command depends on cluster setup)
	result, err := m.client.Execute(ctx, ExecuteRequest{
		ClusterName: clusterName,
		Command:     fmt.Sprintf("scale-workers.sh %d", targetNodes),
		Timeout:     300, // 5 minutes
	})
	if err != nil {
		return fmt.Errorf("execute scaling command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("scaling failed: %s", result.Stderr)
	}

	m.logger.Info("cluster scaled successfully",
		zap.String("cluster_name", clusterName),
		zap.Int("nodes", targetNodes),
	)

	return nil
}

// MonitorClusterHealth continuously monitors cluster health
func (m *ExampleClusterManager) MonitorClusterHealth(
	ctx context.Context,
	clusterName string,
	interval time.Duration,
	healthCheckFn func(*ClusterStatus) error,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("stopping cluster health monitoring",
				zap.String("cluster_name", clusterName),
			)
			return ctx.Err()

		case <-ticker.C:
			status, err := m.client.GetStatus(ctx, clusterName)
			if err != nil {
				m.logger.Error("failed to get cluster status",
					zap.String("cluster_name", clusterName),
					zap.Error(err),
				)
				continue
			}

			// Run custom health check
			if err := healthCheckFn(status); err != nil {
				m.logger.Error("cluster health check failed",
					zap.String("cluster_name", clusterName),
					zap.String("status", status.Status),
					zap.Error(err),
				)
				// In production, trigger alerts or auto-remediation here
			} else {
				m.logger.Debug("cluster health check passed",
					zap.String("cluster_name", clusterName),
					zap.String("status", status.Status),
				)
			}
		}
	}
}

// GracefulShutdown terminates a cluster gracefully
func (m *ExampleClusterManager) GracefulShutdown(
	ctx context.Context,
	clusterName string,
	drainTimeout time.Duration,
) error {
	m.logger.Info("initiating graceful shutdown",
		zap.String("cluster_name", clusterName),
		zap.Duration("drain_timeout", drainTimeout),
	)

	// Step 1: Drain traffic (example - actual implementation depends on load balancer)
	m.logger.Info("draining traffic from cluster",
		zap.String("cluster_name", clusterName),
	)
	_, err := m.client.Execute(ctx, ExecuteRequest{
		ClusterName: clusterName,
		Command:     "drain-traffic.sh",
		Timeout:     int(drainTimeout.Seconds()),
	})
	if err != nil {
		m.logger.Warn("failed to drain traffic gracefully",
			zap.String("cluster_name", clusterName),
			zap.Error(err),
		)
	}

	// Step 2: Wait for drain timeout
	time.Sleep(drainTimeout)

	// Step 3: Terminate the cluster
	resp, err := m.client.Terminate(ctx, clusterName, false)
	if err != nil {
		return fmt.Errorf("terminate cluster: %w", err)
	}

	// Wait for termination to complete
	status, err := m.client.WaitForRequest(ctx, resp.RequestID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("wait for termination: %w", err)
	}

	if status.Status != "completed" {
		return fmt.Errorf("termination failed: %s", status.Error)
	}

	m.logger.Info("cluster terminated successfully",
		zap.String("cluster_name", clusterName),
	)

	return nil
}

// EstimateDeploymentCost estimates the cost of a deployment before launching
func (m *ExampleClusterManager) EstimateDeploymentCost(
	ctx context.Context,
	modelName string,
	gpuType string,
	gpuCount int,
	cloud string,
	region string,
	durationHours int,
) (*EstimateCostResponse, error) {
	// Build task YAML for cost estimation
	taskYAML := m.buildVLLMTaskYAML(modelName, gpuType, gpuCount, cloud, region)

	estimate, err := m.client.EstimateCost(ctx, EstimateCostRequest{
		TaskYAML: taskYAML,
		Hours:    durationHours,
	})
	if err != nil {
		return nil, fmt.Errorf("estimate cost: %w", err)
	}

	m.logger.Info("cost estimated",
		zap.String("model", modelName),
		zap.String("gpu", gpuType),
		zap.Int("gpu_count", gpuCount),
		zap.String("cloud", cloud),
		zap.String("region", region),
		zap.Int("hours", durationHours),
		zap.Float64("total_cost", estimate.EstimatedCost),
		zap.Float64("hourly_cost", estimate.CostPerHour),
		zap.String("currency", estimate.Currency),
	)

	return estimate, nil
}

// ListActiveClusters returns all active clusters
func (m *ExampleClusterManager) ListActiveClusters(ctx context.Context) ([]ClusterStatus, error) {
	list, err := m.client.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	// Filter only active clusters
	var active []ClusterStatus
	for _, cluster := range list.Clusters {
		if cluster.Status == "UP" || cluster.Status == "INIT" {
			active = append(active, cluster)
		}
	}

	m.logger.Info("listed active clusters",
		zap.Int("total", list.Total),
		zap.Int("active", len(active)),
	)

	return active, nil
}

// GetClusterLogs retrieves recent logs from a cluster
func (m *ExampleClusterManager) GetClusterLogs(
	ctx context.Context,
	clusterName string,
	lines int,
) (string, error) {
	logs, err := m.client.GetLogs(ctx, LogsRequest{
		ClusterName: clusterName,
		TailLines:   lines,
	})
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}

	return logs.Logs, nil
}

// ExampleUsage demonstrates how to use the cluster manager
func ExampleUsage() {
	// This is example code showing integration patterns
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize cluster manager
	manager := NewExampleClusterManager(
		"https://skypilot-api.crosslogic.ai",
		"sky_sa_your_token_here",
		logger,
	)

	ctx := context.Background()

	// Example 1: Launch a vLLM cluster
	tenantCreds := &CloudCredentials{
		AWS: &AWSCredentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Region:          "us-east-1",
		},
	}

	cluster, err := manager.LaunchVLLMCluster(
		ctx,
		"llama3-8b-tenant-abc",
		"NousResearch/Meta-Llama-3-8B-Instruct",
		"A100",
		4,
		"aws",
		"us-east-1",
		tenantCreds,
	)
	if err != nil {
		logger.Fatal("failed to launch cluster", zap.Error(err))
	}

	logger.Info("cluster ready",
		zap.String("name", cluster.Name),
		zap.String("endpoint", cluster.Endpoints.Custom["vllm"]),
	)

	// Example 2: Estimate cost before launching
	estimate, err := manager.EstimateDeploymentCost(
		ctx,
		"NousResearch/Meta-Llama-3-70B-Instruct",
		"A100",
		8,
		"aws",
		"us-east-1",
		24, // 24 hours
	)
	if err != nil {
		logger.Fatal("failed to estimate cost", zap.Error(err))
	}

	logger.Info("deployment cost estimate",
		zap.Float64("total_24h", estimate.EstimatedCost),
		zap.Float64("hourly", estimate.CostPerHour),
		zap.String("instance", estimate.InstanceType),
	)

	// Example 3: Monitor cluster health
	go manager.MonitorClusterHealth(
		ctx,
		"llama3-8b-tenant-abc",
		30*time.Second,
		func(status *ClusterStatus) error {
			// Custom health check logic
			if status.Status != "UP" {
				return fmt.Errorf("cluster not healthy: %s", status.Status)
			}
			return nil
		},
	)

	// Example 4: List all active clusters
	active, err := manager.ListActiveClusters(ctx)
	if err != nil {
		logger.Fatal("failed to list clusters", zap.Error(err))
	}

	for _, cluster := range active {
		logger.Info("active cluster",
			zap.String("name", cluster.Name),
			zap.String("status", cluster.Status),
			zap.Float64("cost_per_hour", cluster.CostPerHour),
		)
	}

	// Example 5: Graceful shutdown
	if err := manager.GracefulShutdown(ctx, "llama3-8b-tenant-abc", 2*time.Minute); err != nil {
		logger.Fatal("failed to shutdown cluster", zap.Error(err))
	}
}
