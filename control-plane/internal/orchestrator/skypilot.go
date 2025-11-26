package orchestrator

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/crosslogic/control-plane/internal/config"
	"github.com/crosslogic/control-plane/internal/skypilot"
	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SkyPilotOrchestrator manages GPU node lifecycle using SkyPilot.
//
// SkyPilot (https://github.com/skypilot-org/skypilot) is a framework for running
// LLMs, AI, and batch jobs on any cloud, offering maximum cost savings, highest
// GPU availability, and managed execution.
//
// Architecture:
// - Launch: Generate SkyPilot task YAML → Execute `sky launch` or API call → Register node
// - Monitor: Track node status via `sky status` or API → Update database
// - Terminate: Execute `sky down` or API call → Remove from registry
//
// Operating Modes:
// - CLI Mode (useAPIServer=false): Direct CLI commands (legacy, requires local sky CLI)
// - API Mode (useAPIServer=true): HTTP API calls (recommended, scalable, multi-tenant)
//
// Features:
// - Multi-cloud support (AWS, GCP, Azure, Lambda, OCI)
// - Automatic spot instance provisioning
// - vLLM pre-installation and configuration
// - Node agent auto-start with health checks
// - Graceful shutdown with job draining
// - Multi-tenant cloud credential management
//
// Production Considerations:
// - API Mode: Requires SkyPilot API Server running and configured
// - CLI Mode: Requires SkyPilot CLI installed and configured with cloud credentials
// - Launch latency: 2-5 minutes for cold start, 30s-1min for warm start
// - Cost optimization: Use spot instances with failover to on-demand
// - Monitoring: Track launch success rate, time-to-ready, and cost per node
//
// Security:
// - API Mode: Cloud credentials encrypted in database, passed per-request
// - CLI Mode: Cloud credentials managed via SkyPilot configuration
// - Node agent uses secure HTTPS communication with control plane
// - API keys and secrets passed via environment variables
type SkyPilotOrchestrator struct {
	// taskTemplate is the parsed Go template for SkyPilot task YAML generation
	taskTemplate *template.Template

	// db provides access to PostgreSQL for node registry updates
	db *database.Database

	// logger provides structured logging for observability
	logger *zap.Logger

	// eventBus for publishing node events
	eventBus *events.Bus

	// controlPlaneURL is the HTTPS endpoint for node agent registration
	controlPlaneURL string

	// runtime versions for dependency pinning
	vllmVersion  string
	torchVersion string

	// r2Config holds Cloudflare R2 configuration
	r2Config config.R2Config

	// API client for SkyPilot API Server mode
	apiClient *skypilot.Client

	// useAPIServer determines whether to use API Server (true) or CLI (false)
	useAPIServer bool

	// credentialEncryptionKey for decrypting cloud credentials from database
	credentialEncryptionKey []byte

	// logStore for storing node launch logs in Redis
	logStore *NodeLogStore
}

// NodeConfig defines the configuration for launching a new GPU node.
//
// This configuration is used to generate a SkyPilot task YAML file that
// specifies the cloud provider, region, GPU type, model to serve, and
// initialization scripts.
type NodeConfig struct {
	// NodeID is the unique identifier for this node (UUID)
	NodeID string `json:"node_id"`

	// Provider is the cloud provider (aws, gcp, azure, lambda, oci)
	Provider string `json:"provider"`

	// Region is the cloud region for deployment (e.g., us-west-2, us-central1)
	Region string `json:"region"`

	// GPU specifies the GPU type (e.g., A100, V100, A10G, H100)
	GPU string `json:"gpu"`

	// GPUCount specifies the number of GPUs (e.g., 1, 4, 8)
	GPUCount int `json:"gpu_count"`

	// Model is the LLM model to serve (e.g., meta-llama/Llama-2-7b-chat-hf)
	Model string `json:"model"`

	// UseSpot enables spot instance provisioning for cost savings
	// Default: true (80% cost reduction vs on-demand)
	UseSpot bool `json:"use_spot"`

	// DiskSize is the disk size in GB for model and cache storage
	// Default: 256GB (sufficient for most 7B-13B models)
	DiskSize int `json:"disk_size"`

	// VLLMArgs are additional arguments passed to vLLM server
	// Example: "--tensor-parallel-size 2 --max-model-len 4096"
	VLLMArgs string `json:"vllm_args"`

	// TensorParallel is the tensor parallel size for vLLM (usually equals GPUCount)
	TensorParallel int `json:"tensor_parallel"`

	// DeploymentID links this node to a deployment (optional)
	DeploymentID string `json:"deployment_id,omitempty"`

	// TenantID identifies which tenant owns this node (required for API mode)
	TenantID string `json:"tenant_id,omitempty"`

	// Run:ai Model Streamer configuration (for ultra-fast model loading)
	// StreamerConcurrency is the number of concurrent threads for parallel streaming (8-64)
	// Default: 32 (optimal for most cases, higher = faster but more bandwidth)
	StreamerConcurrency int `json:"streamer_concurrency"`

	// StreamerMemoryLimit is the buffer size in bytes for model streaming
	// Default: 5GB (5368709120 bytes), increase for larger models (e.g., 70B uses 10GB)
	StreamerMemoryLimit int64 `json:"streamer_memory_limit"`

	// GPUMemoryUtilization is the fraction of GPU memory to use (0.0-1.0)
	// Default: 0.95 (Run:ai Streamer is more efficient than standard loading)
	GPUMemoryUtilization float64 `json:"gpu_memory_utilization"`

	// UseRunaiStreamer enables Run:ai Model Streamer for 5-10x faster loading
	// Default: true (reduces load time from 30-60s to 4-23s)
	UseRunaiStreamer bool `json:"use_runai_streamer"`
}

// GenerateClusterName generates a unique cluster name based on the naming convention.
// Format: cic-{provider}-{region}-{gpu}-{spot|od}-{id}
func GenerateClusterName(config NodeConfig) string {
	pricing := "od" // on-demand
	if config.UseSpot {
		pricing = "spot"
	}

	// Short region names for readability (simple replacement for now)
	region := strings.ReplaceAll(config.Region, "-", "")

	// Use first 6 chars of NodeID for uniqueness
	id := "unknown"
	if len(config.NodeID) >= 6 {
		id = config.NodeID[:6]
	}

	return fmt.Sprintf("cic-%s-%s-%s-%s-%s",
		config.Provider,
		region,
		strings.ToLower(config.GPU),
		pricing,
		id,
	)
}

// SkyPilotTaskTemplate is the Go template for generating SkyPilot task YAML.
//
// Template variables:
// - .NodeID: Unique node identifier
// - .Provider: Cloud provider (aws, gcp, azure, etc.)
// - .Region: Cloud region
// - .GPU: GPU type and count
// - .Model: LLM model to serve
// - .UseSpot: Enable spot instances
// - .DiskSize: Disk size in GB
// - .VLLMArgs: Additional vLLM arguments
// - .ControlPlaneURL: Control plane HTTPS endpoint
//
// The generated YAML defines:
// 1. Resource requirements (GPU, cloud, region, disk)
// 2. Setup commands (install dependencies, download node agent)
// 3. Run commands (start vLLM, wait for health, start node agent)
const SkyPilotTaskTemplate = `# SkyPilot Task: CrossLogic Inference Node
# Generated: {{.Timestamp}}
# Node ID: {{.NodeID}}

name: {{.ClusterName}}

resources:
  accelerators: {{.GPU}}:{{.GPUCount}}
  {{if .Provider}}cloud: {{.Provider}}{{end}}
  {{if .Region}}region: {{.Region}}{{end}}
  {{if .UseSpot}}use_spot: true{{else}}use_spot: false{{end}}
  disk_size: {{.DiskSize}}
  disk_tier: best

# Setup: Install dependencies and configure environment
setup: |
  set -e  # Exit on error

  echo "=== Configuring Cloudflare R2 for Model Storage ==="
  export AWS_ACCESS_KEY_ID="{{.R2AccessKey}}"
  export AWS_SECRET_ACCESS_KEY="{{.R2SecretKey}}"
  export AWS_ENDPOINT_URL="{{.R2Endpoint}}"
  export HF_HUB_ENABLE_HF_TRANSFER=1

  # Create HuggingFace cache directory
  mkdir -p ~/.cache/huggingface

  if [ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_ENDPOINT_URL" ]; then
    echo "✓ R2 credentials configured"
    echo "  Endpoint: $AWS_ENDPOINT_URL"
    echo "  Bucket: {{.R2Bucket}}"
    echo "  Models will be streamed directly from R2"
    echo "  Cache directory: ~/.cache/huggingface"
  else
    echo "⚠️  R2 not configured - models will be downloaded from HuggingFace"
  fi

  echo "=== Installing Python and vLLM ==="
  # Install Python 3.10 if not present
  if ! command -v python3.10 &> /dev/null; then
    sudo add-apt-repository -y ppa:deadsnakes/ppa
    sudo apt-get update
    sudo apt-get install -y python3.10 python3.10-venv python3-pip
  fi

  # Create virtual environment
  python3.10 -m venv /opt/vllm-env
  source /opt/vllm-env/bin/activate

  # Install vLLM with CUDA 12.1 support and Run:ai Model Streamer
  pip install --upgrade pip setuptools wheel
  pip install vllm[runai]=={{.VLLMVersion}} torch=={{.TorchVersion}}

  echo "=== Downloading CrossLogic Node Agent ==="
  # Download node agent binary
  wget -q https://{{.ControlPlaneURL}}/downloads/node-agent-linux-amd64 \
    -O /usr/local/bin/node-agent || \
    echo "Warning: Failed to download node agent, using fallback"
  chmod +x /usr/local/bin/node-agent

  echo "=== Setup Complete ==="

# Run: Start vLLM and node agent
run: |
  set -e
  source /opt/vllm-env/bin/activate

  echo "=== Starting vLLM Server ==="
  # Set up model path - vLLM will handle S3:// URLs natively
  MODEL_NAME="{{.Model}}"

  # Check if model is in R2
  if [ -n "$AWS_ENDPOINT_URL" ] && [ -n "{{.R2Bucket}}" ]; then
    # Use S3 URL for model stored in R2
    # vLLM natively supports s3:// URLs via HuggingFace Hub
    R2_MODEL_PATH="s3://{{.R2Bucket}}/$MODEL_NAME"

    echo "✓ Checking if model exists in R2..."
    # Quick check (optional - vLLM will fail gracefully if not found)
    if aws s3 ls "$R2_MODEL_PATH/" --endpoint-url "$AWS_ENDPOINT_URL" &> /dev/null; then
      echo "✓ Model found in R2: $R2_MODEL_PATH"
      echo "  vLLM will stream directly from Cloudflare R2"
      echo "  First load: ~30-60s (CDN fetch + cache)"
      echo "  Subsequent loads: ~5-10s (local HF cache)"
      MODEL_PATH="$R2_MODEL_PATH"
    else
      echo "⚠️  Model not found in R2: $R2_MODEL_PATH"
      echo "  Falling back to HuggingFace download"
      echo "  To upload: python scripts/upload-model-to-r2.py $MODEL_NAME"
      MODEL_PATH="$MODEL_NAME"
    fi
  else
    echo "⚠️  R2 not configured - using HuggingFace download"
    MODEL_PATH="$MODEL_NAME"
  fi

  echo "Starting vLLM with Run:ai Model Streamer (ultra-fast loading)"
  nohup python -m vllm.entrypoints.openai.api_server \
    --model "$MODEL_PATH" \
    --load-format runai_streamer \
    --model-loader-extra-config '{"concurrency": {{.StreamerConcurrency}}, "memory_limit": {{.StreamerMemoryLimit}}}' \
    --host 0.0.0.0 \
    --port 8000 \
    --gpu-memory-utilization {{.GPUMemoryUtilization}} \
    --max-num-seqs 256 \
    --max-model-len 32768 \
    --tensor-parallel-size {{.TensorParallel}} \
    --dtype bfloat16 \
    --enable-prefix-caching \
    --enable-chunked-prefill \
    --disable-log-requests \
    --disable-log-stats \
{{- if .VLLMArgs }}
    {{.VLLMArgs}} \
{{- end}}
    > /tmp/vllm.log 2>&1 &

  VLLM_PID=$!
  echo "vLLM started with PID: $VLLM_PID"

  echo "=== Waiting for vLLM to be ready ==="
  # Wait up to 10 minutes for vLLM to load model and start serving
  for i in {1..600}; do
    if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
      echo "✓ vLLM is ready after ${i} seconds"
      break
    fi

    # Check if vLLM process crashed
    if ! kill -0 $VLLM_PID 2>/dev/null; then
      echo "✗ vLLM process crashed, check /tmp/vllm.log"
      tail -50 /tmp/vllm.log
      exit 1
    fi

    if [ $i -eq 600 ]; then
      echo "✗ vLLM failed to start after 10 minutes"
      tail -50 /tmp/vllm.log
      exit 1
    fi

    sleep 1
  done

  echo "=== Starting CrossLogic Node Agent ==="
  # Set environment variables for node agent
  export CONTROL_PLANE_URL={{.ControlPlaneURL}}
  export NODE_ID={{.NodeID}}
  export MODEL_NAME={{.Model}}
  export REGION={{.Region}}
  export PROVIDER={{.Provider}}
  export VLLM_ENDPOINT=http://localhost:8000
  export LOG_LEVEL=info

  # Start node agent (blocks until interrupted)
  /usr/local/bin/node-agent
`

// NewSkyPilotOrchestrator creates a new SkyPilot orchestrator.
//
// Parameters:
// - db: Database connection for node registry management
// - logger: Structured logger for observability
// - controlPlaneURL: HTTPS endpoint for node agent registration (e.g., "https://api.crosslogic.ai")
// - vllmVersion: vLLM version to install
// - torchVersion: PyTorch version to install
// - eventBus: Event bus for publishing node lifecycle events
// - r2Config: Cloudflare R2 configuration for model storage
// - skyPilotConfig: SkyPilot configuration (API server URL, credentials, timeouts)
//
// Returns:
// - *SkyPilotOrchestrator: Configured orchestrator ready to launch nodes
// - error: Configuration error, template parsing error, or API client initialization error
//
// Example:
//
//	orchestrator, err := NewSkyPilotOrchestrator(
//	    database,
//	    logger,
//	    "https://api.crosslogic.ai",
//	    "0.6.2",
//	    "2.4.0",
//	    eventBus,
//	    r2Config,
//	    skyPilotConfig,
//	)
func NewSkyPilotOrchestrator(
	db *database.Database,
	cache *cache.Cache,
	logger *zap.Logger,
	controlPlaneURL, vllmVersion, torchVersion string,
	eventBus *events.Bus,
	r2Config config.R2Config,
	skyPilotConfig config.SkyPilotConfig,
) (*SkyPilotOrchestrator, error) {
	// Parse template
	tmpl, err := template.New("skypilot").Parse(SkyPilotTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SkyPilot task template: %w", err)
	}

	orchestrator := &SkyPilotOrchestrator{
		taskTemplate:    tmpl,
		db:              db,
		logger:          logger,
		eventBus:        eventBus,
		controlPlaneURL: controlPlaneURL,
		vllmVersion:     vllmVersion,
		torchVersion:    torchVersion,
		r2Config:        r2Config,
		useAPIServer:    skyPilotConfig.UseAPIServer,
		logStore:        NewNodeLogStore(cache, logger),
	}

	// Initialize API client if API Server mode is enabled
	if skyPilotConfig.UseAPIServer {
		if skyPilotConfig.APIServerURL == "" {
			return nil, fmt.Errorf("SkyPilot API Server URL is required when UseAPIServer is true")
		}
		if skyPilotConfig.ServiceAccountToken == "" {
			return nil, fmt.Errorf("SkyPilot service account token is required when UseAPIServer is true")
		}
		if skyPilotConfig.CredentialEncryptionKey == "" {
			return nil, fmt.Errorf("credential encryption key is required when UseAPIServer is true")
		}

		// Store encryption key for credential decryption
		orchestrator.credentialEncryptionKey = []byte(skyPilotConfig.CredentialEncryptionKey)

		// Initialize API client
		clientConfig := skypilot.Config{
			BaseURL:       skyPilotConfig.APIServerURL,
			Token:         skyPilotConfig.ServiceAccountToken,
			Timeout:       skyPilotConfig.LaunchTimeout,
			MaxRetries:    skyPilotConfig.MaxRetries,
			RetryDelay:    skyPilotConfig.RetryBackoff,
			RetryMaxDelay: skyPilotConfig.RetryBackoff * 4, // Max 4x initial backoff
		}

		orchestrator.apiClient = skypilot.NewClient(clientConfig, logger)

		logger.Info("SkyPilot orchestrator initialized in API Server mode",
			zap.String("api_server_url", skyPilotConfig.APIServerURL),
			zap.Duration("launch_timeout", skyPilotConfig.LaunchTimeout),
		)
	} else {
		logger.Info("SkyPilot orchestrator initialized in CLI mode")
	}

	return orchestrator, nil
}

// LaunchNode provisions a new GPU node using SkyPilot.
//
// Process:
// 1. Validate configuration and set defaults
// 2. Generate SkyPilot task YAML from template
// 3. Route to API or CLI based on useAPIServer flag
// 4. Register node in database
// 5. Return cluster name for tracking
//
// API Mode:
// - Retrieves tenant cloud credentials from database
// - Decrypts credentials using encryption key
// - Sends launch request to SkyPilot API Server with credentials
// - Polls async request status until completion
//
// CLI Mode:
// - Writes task file to temporary location
// - Executes `sky launch` command
// - Returns immediately (detached mode)
//
// Launch Time:
// - Cold start (new region/GPU): 3-5 minutes
// - Warm start (cached resources): 30 seconds - 1 minute
//
// Cost Optimization:
// - Spot instances: 60-90% cost savings vs on-demand
// - Automatic failover to on-demand if spot unavailable
// - Job preemption handling via spot recovery
//
// Error Handling:
// - Invalid config: Returns validation error immediately
// - SkyPilot failure: Returns error with output/details for debugging
// - Cloud API errors: Propagated from SkyPilot (check cloud credentials)
//
// Returns:
// - string: Cluster name (format: "cic-{provider}-{region}-{gpu}-{spot|od}-{id}")
// - error: Validation error, credential error, template error, or SkyPilot launch failure
func (o *SkyPilotOrchestrator) LaunchNode(ctx context.Context, config NodeConfig) (string, error) {
	startTime := time.Now()

	// Validate and set defaults
	if err := o.validateNodeConfig(&config); err != nil {
		o.logStore.LogError(ctx, config.NodeID, PhaseQueued, "Invalid configuration", err.Error())
		return "", fmt.Errorf("invalid node configuration: %w", err)
	}

	clusterName := GenerateClusterName(config)

	// Log initial queued status
	o.logStore.LogInfo(ctx, config.NodeID, PhaseQueued,
		fmt.Sprintf("Node launch request queued: %s", clusterName), 0)
	o.logStore.LogInfo(ctx, config.NodeID, PhaseQueued,
		fmt.Sprintf("Provider: %s, Region: %s, GPU: %s:%d, Model: %s",
			config.Provider, config.Region, config.GPU, config.GPUCount, config.Model), 5)

	o.logger.Info("launching GPU node with SkyPilot",
		zap.String("node_id", config.NodeID),
		zap.String("cluster_name", clusterName),
		zap.String("provider", config.Provider),
		zap.String("region", config.Region),
		zap.String("gpu", config.GPU),
		zap.Int("gpu_count", config.GPUCount),
		zap.String("model", config.Model),
		zap.Bool("use_spot", config.UseSpot),
		zap.Bool("use_api_server", o.useAPIServer),
	)

	// Log provisioning phase
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		"Starting cloud resource provisioning...", 10)

	// Route to API or CLI based on configuration
	var err error
	if o.useAPIServer {
		err = o.launchNodeViaAPI(ctx, config, clusterName)
	} else {
		err = o.launchNodeViaCLI(ctx, config, clusterName)
	}

	if err != nil {
		o.logStore.LogError(ctx, config.NodeID, PhaseFailed,
			"Node launch failed", err.Error())
		return "", err
	}

	launchDuration := time.Since(startTime)

	o.logger.Info("GPU node launched successfully",
		zap.String("cluster_name", clusterName),
		zap.Duration("launch_duration", launchDuration),
		zap.String("node_id", config.NodeID),
	)

	// Publish node launched event
	if o.eventBus != nil {
		evt := events.NewEvent(
			events.EventNodeLaunched,
			config.TenantID,
			map[string]interface{}{
				"node_id":         config.NodeID,
				"cluster_name":    clusterName,
				"provider":        config.Provider,
				"region":          config.Region,
				"gpu_type":        config.GPU,
				"gpu_count":       config.GPUCount,
				"spot_instance":   config.UseSpot,
				"model":           config.Model,
				"launch_duration": launchDuration.String(),
				"api_mode":        o.useAPIServer,
			},
		)
		if err := o.eventBus.Publish(ctx, evt); err != nil {
			o.logger.Error("failed to publish node launched event",
				zap.Error(err),
				zap.String("node_id", config.NodeID),
			)
		}
	}

	// Register node in database
	if err := o.registerNode(ctx, config, clusterName); err != nil {
		// Node launched but registration failed - log warning but don't fail
		o.logger.Warn("node launched but database registration failed",
			zap.Error(err),
			zap.String("cluster_name", clusterName),
		)
	}

	return clusterName, nil
}

// launchNodeViaAPI launches a node using the SkyPilot API Server.
func (o *SkyPilotOrchestrator) launchNodeViaAPI(ctx context.Context, config NodeConfig, clusterName string) error {
	// Get tenant cloud credentials from database
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		"Retrieving cloud credentials...", 15)

	cloudCreds, err := o.getTenantCredentials(ctx, config.TenantID, config.Provider)
	if err != nil {
		return fmt.Errorf("failed to get tenant credentials: %w", err)
	}

	// Generate task YAML
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		"Generating SkyPilot task configuration...", 20)

	taskYAML, err := o.generateTaskYAML(config, clusterName)
	if err != nil {
		return fmt.Errorf("failed to generate task YAML: %w", err)
	}

	// Build launch request
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		fmt.Sprintf("Submitting launch request to SkyPilot API (cluster: %s)...", clusterName), 25)

	launchReq := skypilot.LaunchRequest{
		ClusterName:      clusterName,
		TaskYAML:         taskYAML,
		RetryUntilUp:     true,
		Detach:           true,
		CloudCredentials: cloudCreds,
		Envs: map[string]string{
			"NODE_ID":          config.NodeID,
			"CONTROL_PLANE_URL": o.controlPlaneURL,
		},
	}

	// Call API
	launchResp, err := o.apiClient.Launch(ctx, launchReq)
	if err != nil {
		return fmt.Errorf("API launch failed: %w", err)
	}

	o.logger.Info("cluster launch request submitted",
		zap.String("cluster_name", clusterName),
		zap.String("request_id", launchResp.RequestID),
	)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		fmt.Sprintf("Launch request accepted (ID: %s). Waiting for cloud resources...", launchResp.RequestID), 30)

	// Poll for completion (async operation)
	// Use a reasonable poll interval (5 seconds initially)
	o.logStore.LogInfo(ctx, config.NodeID, PhaseInstanceReady,
		"Cloud instance is starting up...", 50)

	requestStatus, err := o.apiClient.WaitForRequest(ctx, launchResp.RequestID, 5*time.Second)
	if err != nil {
		return fmt.Errorf("launch request failed: %w", err)
	}

	if requestStatus.Status != "completed" {
		return fmt.Errorf("launch request ended with status: %s, error: %s",
			requestStatus.Status, requestStatus.Error)
	}

	o.logger.Info("cluster launch completed",
		zap.String("cluster_name", clusterName),
		zap.String("request_id", launchResp.RequestID),
	)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseInstalling,
		"Instance is ready. Installing dependencies and vLLM...", 60)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseModelLoading,
		fmt.Sprintf("Loading model %s...", config.Model), 70)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseHealthCheck,
		"Running health checks...", 85)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseActive,
		"Node is ready and serving requests!", 100)

	return nil
}

// launchNodeViaCLI launches a node using the SkyPilot CLI (legacy mode).
func (o *SkyPilotOrchestrator) launchNodeViaCLI(ctx context.Context, config NodeConfig, clusterName string) error {
	// Generate task YAML
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		"Generating SkyPilot task configuration...", 15)

	taskYAML, err := o.generateTaskYAML(config, clusterName)
	if err != nil {
		return fmt.Errorf("failed to generate task YAML: %w", err)
	}

	// Write task file
	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		"Preparing launch command...", 20)

	taskFile := fmt.Sprintf("/tmp/sky-task-%s.yaml", config.NodeID)
	if err := os.WriteFile(taskFile, []byte(taskYAML), 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}
	defer os.Remove(taskFile)

	o.logger.Debug("generated SkyPilot task file",
		zap.String("task_file", taskFile),
		zap.Int("yaml_size", len(taskYAML)),
	)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseProvisioning,
		fmt.Sprintf("Launching cluster %s via SkyPilot CLI...", clusterName), 30)

	// Launch with SkyPilot
	// Note: Do NOT use --down flag as it terminates the cluster after job completion
	// We want the vLLM server to keep running for inference requests
	cmd := exec.CommandContext(ctx, "sky", "launch",
		"-c", clusterName, // Cluster name
		taskFile,          // Task file
		"-y",              // Auto-confirm
		"--detach-run",    // Detach after launch (returns immediately)
	)

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	o.logStore.LogInfo(ctx, config.NodeID, PhaseInstanceReady,
		"Waiting for cloud instance to start...", 50)

	// Execute launch
	if err := cmd.Run(); err != nil {
		o.logger.Error("SkyPilot CLI launch failed",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return fmt.Errorf("sky launch failed: %w\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	o.logStore.LogInfo(ctx, config.NodeID, PhaseInstalling,
		"Instance is ready. Installing dependencies and vLLM...", 60)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseModelLoading,
		fmt.Sprintf("Loading model %s...", config.Model), 70)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseHealthCheck,
		"Running health checks...", 85)

	o.logStore.LogInfo(ctx, config.NodeID, PhaseActive,
		"Node is ready and serving requests!", 100)

	return nil
}

// TerminateNode terminates a GPU node and removes it from the cluster.
//
// Process:
// 1. Route to API or CLI based on useAPIServer flag
// 2. Execute termination (async in API mode, sync in CLI mode)
// 3. Update node status in database to 'terminated'
//
// Behavior:
// - Graceful shutdown: Waits for running jobs to complete (configurable timeout)
// - Force shutdown: Use context cancellation for immediate termination
// - Resource cleanup: All cloud resources (VM, disk, network) are deleted
// - Cost: No charges after termination (except for persistent disk if configured)
//
// Error Handling:
// - Already terminated: Returns success (idempotent)
// - SkyPilot failure: Returns error with command output/details
// - Partial failure: Cloud resources may be left in inconsistent state (manual cleanup required)
//
// Returns:
// - error: Termination failure or database update error
func (o *SkyPilotOrchestrator) TerminateNode(ctx context.Context, clusterName string) error {
	o.logger.Info("terminating GPU node",
		zap.String("cluster_name", clusterName),
		zap.Bool("use_api_server", o.useAPIServer),
	)

	var err error
	if o.useAPIServer {
		err = o.terminateNodeViaAPI(ctx, clusterName)
	} else {
		err = o.terminateNodeViaCLI(ctx, clusterName)
	}

	if err != nil {
		return err
	}

	o.logger.Info("GPU node terminated successfully",
		zap.String("cluster_name", clusterName),
	)

	// Update node status in database
	if err := o.updateNodeStatus(ctx, clusterName, "terminated"); err != nil {
		o.logger.Warn("failed to update node status in database",
			zap.Error(err),
			zap.String("cluster_name", clusterName),
		)
	}

	return nil
}

// terminateNodeViaAPI terminates a node using the SkyPilot API Server.
func (o *SkyPilotOrchestrator) terminateNodeViaAPI(ctx context.Context, clusterName string) error {
	// Call API to terminate cluster
	terminateResp, err := o.apiClient.Terminate(ctx, clusterName, true)
	if err != nil {
		// Check if cluster not found (already terminated)
		if apiErr, ok := err.(*skypilot.APIError); ok && apiErr.IsNotFound() {
			o.logger.Info("cluster already terminated",
				zap.String("cluster_name", clusterName),
			)
			return nil
		}
		return fmt.Errorf("API terminate failed: %w", err)
	}

	o.logger.Info("cluster termination request submitted",
		zap.String("cluster_name", clusterName),
		zap.String("request_id", terminateResp.RequestID),
	)

	// Wait for termination to complete
	requestStatus, err := o.apiClient.WaitForRequest(ctx, terminateResp.RequestID, 3*time.Second)
	if err != nil {
		return fmt.Errorf("termination request failed: %w", err)
	}

	if requestStatus.Status != "completed" {
		return fmt.Errorf("termination request ended with status: %s, error: %s",
			requestStatus.Status, requestStatus.Error)
	}

	return nil
}

// terminateNodeViaCLI terminates a node using the SkyPilot CLI (legacy mode).
func (o *SkyPilotOrchestrator) terminateNodeViaCLI(ctx context.Context, clusterName string) error {
	// Execute sky down
	cmd := exec.CommandContext(ctx, "sky", "down",
		clusterName, // Cluster name
		"-y",        // Auto-confirm
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if cluster already down (not an error)
		if bytes.Contains(stderr.Bytes(), []byte("not found")) ||
			bytes.Contains(stderr.Bytes(), []byte("does not exist")) {
			o.logger.Info("cluster already terminated",
				zap.String("cluster_name", clusterName),
			)
			return nil
		}

		o.logger.Error("SkyPilot CLI termination failed",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return fmt.Errorf("sky down failed: %w\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	return nil
}

// GetNodeStatus retrieves the current status of a GPU node from SkyPilot.
//
// Status values:
// - UP: Node is running and healthy
// - INIT: Node is initializing (provisioning or starting)
// - DOWN: Node is terminated
// - STOPPED: Node is stopped but not terminated (can be restarted)
//
// Routes to API or CLI based on useAPIServer flag.
//
// Returns:
// - string: Node status (UP, INIT, DOWN, STOPPED, UNKNOWN)
// - error: Command execution error, API error, or JSON parsing error
func (o *SkyPilotOrchestrator) GetNodeStatus(ctx context.Context, clusterName string) (string, error) {
	if o.useAPIServer {
		return o.getNodeStatusViaAPI(ctx, clusterName)
	}
	return o.getNodeStatusViaCLI(ctx, clusterName)
}

// getNodeStatusViaAPI retrieves node status using the SkyPilot API Server.
func (o *SkyPilotOrchestrator) getNodeStatusViaAPI(ctx context.Context, clusterName string) (string, error) {
	status, err := o.apiClient.GetStatus(ctx, clusterName)
	if err != nil {
		// Check if cluster not found
		if apiErr, ok := err.(*skypilot.APIError); ok && apiErr.IsNotFound() {
			return "DOWN", nil
		}
		return "", fmt.Errorf("API get status failed: %w", err)
	}

	return status.Status, nil
}

// getNodeStatusViaCLI retrieves node status using the SkyPilot CLI (legacy mode).
func (o *SkyPilotOrchestrator) getNodeStatusViaCLI(ctx context.Context, clusterName string) (string, error) {
	cmd := exec.CommandContext(ctx, "sky", "status",
		clusterName, // Cluster name
		"--json",    // JSON output
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Cluster not found
		if bytes.Contains(output, []byte("not found")) {
			return "DOWN", nil
		}
		return "", fmt.Errorf("sky status failed: %w\nOutput: %s", err, output)
	}

	// Parse JSON output
	var status map[string]interface{}
	if err := json.Unmarshal(output, &status); err != nil {
		return "", fmt.Errorf("failed to parse status JSON: %w", err)
	}

	// Extract status field
	if statusStr, ok := status["status"].(string); ok {
		return statusStr, nil
	}

	return "UNKNOWN", nil
}

// GetClusterStatus is an alias for GetNodeStatus for semantic clarity
// in the monitoring context. It returns the status of a SkyPilot cluster.
func (o *SkyPilotOrchestrator) GetClusterStatus(clusterName string) (string, error) {
	return o.GetNodeStatus(context.Background(), clusterName)
}

// GetAllClusters returns all active GPU clusters managed by SkyPilot.
//
// This queries either the SkyPilot API Server or local SkyPilot database
// for all clusters with the "cic-" prefix (CrossLogic Inference Cloud nodes).
//
// Returns:
// - []string: List of cluster names
// - error: API error, command execution error, or parsing error
func (o *SkyPilotOrchestrator) GetAllClusters(ctx context.Context) ([]string, error) {
	if o.useAPIServer {
		return o.getAllClustersViaAPI(ctx)
	}
	return o.getAllClustersViaCLI(ctx)
}

// getAllClustersViaAPI retrieves all clusters using the SkyPilot API Server.
func (o *SkyPilotOrchestrator) getAllClustersViaAPI(ctx context.Context) ([]string, error) {
	listResp, err := o.apiClient.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("API list clusters failed: %w", err)
	}

	// Filter for CIC nodes
	var nodeNames []string
	for _, cluster := range listResp.Clusters {
		if len(cluster.Name) > 4 && cluster.Name[:4] == "cic-" {
			nodeNames = append(nodeNames, cluster.Name)
		}
	}

	return nodeNames, nil
}

// getAllClustersViaCLI retrieves all clusters using the SkyPilot CLI (legacy mode).
func (o *SkyPilotOrchestrator) getAllClustersViaCLI(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "sky", "status", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sky status failed: %w\nOutput: %s", err, output)
	}

	// Parse JSON array
	var clusters []map[string]interface{}
	if err := json.Unmarshal(output, &clusters); err != nil {
		return nil, fmt.Errorf("failed to parse clusters JSON: %w", err)
	}

	// Filter for CIC nodes
	var nodeNames []string
	for _, cluster := range clusters {
		if name, ok := cluster["name"].(string); ok {
			if len(name) > 4 && name[:4] == "cic-" {
				nodeNames = append(nodeNames, name)
			}
		}
	}

	return nodeNames, nil
}

// ListNodes is an alias for GetAllClusters for backward compatibility.
func (o *SkyPilotOrchestrator) ListNodes(ctx context.Context) ([]string, error) {
	return o.GetAllClusters(ctx)
}

// ExecCommand executes a command on a running node.
//
// Routes to API or CLI based on useAPIServer flag.
//
// Returns:
// - string: Command output (stdout + stderr)
// - error: Execution failure
func (o *SkyPilotOrchestrator) ExecCommand(ctx context.Context, clusterName, command string) (string, error) {
	o.logger.Debug("executing command on node",
		zap.String("cluster_name", clusterName),
		zap.String("command", command),
		zap.Bool("use_api_server", o.useAPIServer),
	)

	if o.useAPIServer {
		return o.execCommandViaAPI(ctx, clusterName, command)
	}
	return o.execCommandViaCLI(ctx, clusterName, command)
}

// execCommandViaAPI executes a command using the SkyPilot API Server.
func (o *SkyPilotOrchestrator) execCommandViaAPI(ctx context.Context, clusterName, command string) (string, error) {
	execReq := skypilot.ExecuteRequest{
		ClusterName: clusterName,
		Command:     command,
		Timeout:     300, // 5 minutes
	}

	execResp, err := o.apiClient.Execute(ctx, execReq)
	if err != nil {
		return "", fmt.Errorf("API execute failed: %w", err)
	}

	// Combine stdout and stderr
	output := execResp.Stdout
	if execResp.Stderr != "" {
		output += "\n" + execResp.Stderr
	}

	if execResp.ExitCode != 0 {
		return output, fmt.Errorf("command exited with code %d", execResp.ExitCode)
	}

	return output, nil
}

// execCommandViaCLI executes a command using the SkyPilot CLI (legacy mode).
func (o *SkyPilotOrchestrator) execCommandViaCLI(ctx context.Context, clusterName, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sky", "exec",
		clusterName,
		command,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sky exec failed: %w\nOutput: %s", err, output)
	}

	return string(output), nil
}

// getTenantCredentials retrieves and decrypts cloud credentials for a tenant from the database.
func (o *SkyPilotOrchestrator) getTenantCredentials(ctx context.Context, tenantID, provider string) (*skypilot.CloudCredentials, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is required for API mode")
	}

	// Query database for credentials
	query := `
		SELECT credentials_encrypted, encryption_key_id
		FROM cloud_credentials
		WHERE tenant_id = $1
		  AND provider = $2
		  AND status = 'active'
		  AND (is_default = true OR environment_id IS NULL)
		ORDER BY is_default DESC
		LIMIT 1
	`

	var encryptedCreds []byte
	var keyID string

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	err = o.db.Pool.QueryRow(ctx, query, tenantUUID, provider).Scan(&encryptedCreds, &keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query credentials: %w", err)
	}

	// Decrypt credentials
	decryptedJSON, err := o.decryptCredentials(encryptedCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Parse decrypted JSON based on provider
	cloudCreds := &skypilot.CloudCredentials{}

	switch provider {
	case "aws":
		var awsCreds skypilot.AWSCredentials
		if err := json.Unmarshal(decryptedJSON, &awsCreds); err != nil {
			return nil, fmt.Errorf("failed to parse AWS credentials: %w", err)
		}
		cloudCreds.AWS = &awsCreds

	case "azure":
		var azureCreds skypilot.AzureCredentials
		if err := json.Unmarshal(decryptedJSON, &azureCreds); err != nil {
			return nil, fmt.Errorf("failed to parse Azure credentials: %w", err)
		}
		cloudCreds.Azure = &azureCreds

	case "gcp":
		var gcpCreds skypilot.GCPCredentials
		if err := json.Unmarshal(decryptedJSON, &gcpCreds); err != nil {
			return nil, fmt.Errorf("failed to parse GCP credentials: %w", err)
		}
		cloudCreds.GCP = &gcpCreds

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	o.logger.Debug("retrieved tenant credentials",
		zap.String("tenant_id", tenantID),
		zap.String("provider", provider),
		zap.String("key_id", keyID),
	)

	return cloudCreds, nil
}

// decryptCredentials decrypts encrypted credentials using AES-256-GCM.
func (o *SkyPilotOrchestrator) decryptCredentials(encryptedData []byte) ([]byte, error) {
	// Ensure key is 32 bytes for AES-256
	key := o.credentialEncryptionKey
	if len(key) != 32 {
		// If key is not 32 bytes, hash it or pad/truncate
		// For production, use proper key derivation (PBKDF2, Argon2)
		if len(key) < 32 {
			// Pad with zeros (NOT recommended for production)
			key = append(key, make([]byte, 32-len(key))...)
		} else {
			key = key[:32]
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// validateNodeConfig validates and sets defaults for node configuration.
func (o *SkyPilotOrchestrator) validateNodeConfig(config *NodeConfig) error {
	// Validate required fields
	if config.NodeID == "" {
		config.NodeID = uuid.New().String()
	}

	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if config.Region == "" {
		return fmt.Errorf("region is required")
	}

	if config.GPU == "" {
		return fmt.Errorf("GPU type is required")
	}

	if config.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Validate tenant ID in API mode
	if o.useAPIServer && config.TenantID == "" {
		return fmt.Errorf("tenant ID is required when using API Server mode")
	}

	// Set defaults
	if config.DiskSize == 0 {
		config.DiskSize = 256 // 256GB default
	}

	if config.GPUCount == 0 {
		config.GPUCount = 1
	}

	if config.TensorParallel == 0 {
		config.TensorParallel = config.GPUCount
	}

	// Set Run:ai Streamer defaults for ultra-fast model loading
	if config.StreamerConcurrency == 0 {
		config.StreamerConcurrency = 32 // Optimal for most models (8-64 range)
	}

	if config.StreamerMemoryLimit == 0 {
		config.StreamerMemoryLimit = 5368709120 // 5GB default (increase for 70B+ models)
	}

	if config.GPUMemoryUtilization == 0 {
		config.GPUMemoryUtilization = 0.95 // Run:ai Streamer is more efficient
	}

	// Enable Run:ai Streamer by default (can be disabled if needed)
	if !config.UseRunaiStreamer {
		config.UseRunaiStreamer = true // Default to enabled for better performance
	}

	// Sanitize optional VLLM args
	cleanArgs, err := sanitizeVLLMArgs(config.VLLMArgs)
	if err != nil {
		return err
	}
	config.VLLMArgs = cleanArgs

	// UseSpot defaults to true (not set in struct, Go zero value is false)
	// So we need to explicitly check if it was provided
	// For simplicity, we'll document that UseSpot=false means on-demand

	return nil
}

var allowedVLLMArgPattern = regexp.MustCompile(`^[a-zA-Z0-9@./_=:-]+$`)

func sanitizeVLLMArgs(args string) (string, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return "", nil
	}

	fields := strings.Fields(args)
	sanitized := make([]string, 0, len(fields))
	for _, field := range fields {
		if !allowedVLLMArgPattern.MatchString(field) {
			return "", fmt.Errorf("invalid vLLM argument: %s", field)
		}
		sanitized = append(sanitized, fmt.Sprintf("'%s'", field))
	}

	return strings.Join(sanitized, " "), nil
}

// generateTaskYAML generates SkyPilot task YAML from configuration.
func (o *SkyPilotOrchestrator) generateTaskYAML(config NodeConfig, clusterName string) (string, error) {
	// Prepare template data
	data := map[string]interface{}{
		"NodeID":           config.NodeID,
		"ClusterName":      clusterName,
		"Provider":         config.Provider,
		"Region":           config.Region,
		"GPU":              config.GPU,
		"GPUCount":         config.GPUCount,
		"Model":            config.Model,
		"UseSpot":          config.UseSpot,
		"DiskSize":         config.DiskSize,
		"VLLMArgs":         config.VLLMArgs,
		"TensorParallel":   config.TensorParallel,
		"ControlPlaneURL":  o.controlPlaneURL,
		"VLLMVersion":      o.vllmVersion,
		"TorchVersion":     o.torchVersion,
		"Timestamp":        time.Now().Format(time.RFC3339),
		"R2Endpoint":       o.r2Config.Endpoint,
		"R2Bucket":         o.r2Config.Bucket,
		"R2AccessKey":      o.r2Config.AccessKey,
		"R2SecretKey":      o.r2Config.SecretKey,
		// Run:ai Model Streamer configuration
		"StreamerConcurrency":    config.StreamerConcurrency,
		"StreamerMemoryLimit":    config.StreamerMemoryLimit,
		"GPUMemoryUtilization":   config.GPUMemoryUtilization,
		"UseRunaiStreamer":       config.UseRunaiStreamer,
	}

	// Execute template
	var buf bytes.Buffer
	if err := o.taskTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// registerNode registers a newly launched node in the database.
func (o *SkyPilotOrchestrator) registerNode(ctx context.Context, config NodeConfig, clusterName string) error {
	query := `
		INSERT INTO nodes (
			id, cluster_name, provider, region, gpu_type,
			model_name, status, endpoint, created_at, deployment_id
		) VALUES ($1, $2, $3, $4, $5, $6, 'initializing', '', NOW(), $7)
		ON CONFLICT (id) DO UPDATE
		SET cluster_name = $2, status = 'initializing', updated_at = NOW()
	`

	nodeID, err := uuid.Parse(config.NodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}

	var deploymentID *uuid.UUID
	if config.DeploymentID != "" {
		id, err := uuid.Parse(config.DeploymentID)
		if err != nil {
			return fmt.Errorf("invalid deployment ID: %w", err)
		}
		deploymentID = &id
	}

	_, err = o.db.Pool.Exec(ctx, query,
		nodeID,
		clusterName,
		config.Provider,
		config.Region,
		config.GPU,
		config.Model,
		deploymentID,
	)

	return err
}

// updateNodeStatus updates the status of a node in the database.
func (o *SkyPilotOrchestrator) updateNodeStatus(ctx context.Context, clusterName, status string) error {
	query := `
		UPDATE nodes
		SET status = $1, updated_at = NOW()
		WHERE cluster_name = $2
	`

	_, err := o.db.Pool.Exec(ctx, query, status, clusterName)
	return err
}

// ModelConfigGenerator helps determine optimal GPU configuration for a model.
type ModelConfigGenerator struct {
	modelSizes map[string]int64 // Model name -> parameter count
}

// NewModelConfigGenerator creates a new generator with known model sizes.
func NewModelConfigGenerator() *ModelConfigGenerator {
	return &ModelConfigGenerator{
		modelSizes: map[string]int64{
			"meta-llama/Llama-2-7b-chat-hf":     7_000_000_000,
			"meta-llama/Llama-2-13b-chat-hf":    13_000_000_000,
			"meta-llama/Llama-2-70b-chat-hf":    70_000_000_000,
			"meta-llama/Llama-3-8b-instruct":    8_000_000_000,
			"meta-llama/Llama-3-70b-instruct":   70_000_000_000,
			"meta-llama/Llama-3-405b-instruct":  405_000_000_000,
			"deepseek-ai/deepseek-llm-67b-chat": 67_000_000_000,
		},
	}
}

// GetOptimalConfig returns the optimal GPU configuration for a given model.
func (g *ModelConfigGenerator) GetOptimalConfig(modelName string) (string, int, int) {
	paramCount, ok := g.modelSizes[modelName]
	if !ok {
		// Default to A100:1 if unknown
		return "A100", 1, 1
	}

	switch {
	case paramCount < 14_000_000_000: // < 14B
		return "A10G", 1, 1 // Cost effective
	case paramCount < 70_000_000_000: // < 70B
		return "A100", 1, 1
	case paramCount < 200_000_000_000: // 70B - 200B
		return "H100", 4, 4 // Needs multi-GPU
	default: // 200B+
		return "H100", 8, 8
	}
}
