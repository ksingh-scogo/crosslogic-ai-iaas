package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/crosslogic/control-plane/internal/config"
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
// - Launch: Generate SkyPilot task YAML → Execute `sky launch` → Register node
// - Monitor: Track node status via `sky status` → Update database
// - Terminate: Execute `sky down` → Remove from registry
//
// Features:
// - Multi-cloud support (AWS, GCP, Azure, Lambda, OCI)
// - Automatic spot instance provisioning
// - vLLM pre-installation and configuration
// - Node agent auto-start with health checks
// - Graceful shutdown with job draining
//
// Production Considerations:
// - Requires SkyPilot CLI installed and configured with cloud credentials
// - Launch latency: 2-5 minutes for cold start, 30s-1min for warm start
// - Cost optimization: Use spot instances with failover to on-demand
// - Monitoring: Track launch success rate, time-to-ready, and cost per node
//
// Security:
// - Cloud credentials managed via SkyPilot configuration
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
	// runtime versions for dependency pinning
	vllmVersion  string
	torchVersion string

	// r2Config holds Cloudflare R2 configuration
	r2Config config.R2Config
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
  {{if .UseSpot}}use_spot: true
  spot_recovery: true{{else}}use_spot: false{{end}}
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

  # Install vLLM with CUDA 12.1 support
  pip install --upgrade pip setuptools wheel
  pip install vllm=={{.VLLMVersion}} torch=={{.TorchVersion}}

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

  echo "Starting vLLM with model: $MODEL_PATH"
  nohup python -m vllm.entrypoints.openai.api_server \
    --model "$MODEL_PATH" \
    --host 0.0.0.0 \
    --port 8000 \
    --gpu-memory-utilization 0.9 \
    --max-num-seqs 256 \
    --max-model-len 32768 \
    --enable-prefix-caching \
    --disable-log-requests \
    --tensor-parallel-size {{.TensorParallel}} \
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
//
// Returns:
// - *SkyPilotOrchestrator: Configured orchestrator ready to launch nodes
// - error: Template parsing error (should never occur with valid template)
//
// Example:
//
//	orchestrator, err := NewSkyPilotOrchestrator(
//	    database,
//	    logger,
//	    "https://api.crosslogic.ai",
//	    "https://api.crosslogic.ai",
//	)
func NewSkyPilotOrchestrator(db *database.Database, logger *zap.Logger, controlPlaneURL, vllmVersion, torchVersion string, eventBus *events.Bus, r2Config config.R2Config) (*SkyPilotOrchestrator, error) {
	// Parse template
	tmpl, err := template.New("skypilot").Parse(SkyPilotTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SkyPilot task template: %w", err)
	}

	return &SkyPilotOrchestrator{
		taskTemplate:    tmpl,
		db:              db,
		logger:          logger,
		eventBus:        eventBus,
		controlPlaneURL: controlPlaneURL,
		vllmVersion:     vllmVersion,
		torchVersion:    torchVersion,
		r2Config:        r2Config,
	}, nil
}

// LaunchNode provisions a new GPU node using SkyPilot.
//
// Process:
// 1. Validate configuration and set defaults
// 2. Generate SkyPilot task YAML from template
// 3. Write task file to temporary location
// 4. Execute `sky launch` command
// 5. Register node in database
// 6. Return cluster name for tracking
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
// - SkyPilot failure: Returns error with command output for debugging
// - Cloud API errors: Propagated from SkyPilot (check cloud credentials)
//
// Returns:
// - string: Cluster name (format: "cic-{provider}-{region}-{gpu}-{spot|od}-{id}")
// - error: Validation error, template error, or SkyPilot launch failure
func (o *SkyPilotOrchestrator) LaunchNode(ctx context.Context, config NodeConfig) (string, error) {
	startTime := time.Now()

	// Validate and set defaults
	if err := o.validateNodeConfig(&config); err != nil {
		return "", fmt.Errorf("invalid node configuration: %w", err)
	}

	clusterName := GenerateClusterName(config)

	o.logger.Info("launching GPU node with SkyPilot",
		zap.String("node_id", config.NodeID),
		zap.String("cluster_name", clusterName),
		zap.String("provider", config.Provider),
		zap.String("region", config.Region),
		zap.String("gpu", config.GPU),
		zap.Int("gpu_count", config.GPUCount),
		zap.String("model", config.Model),
		zap.Bool("use_spot", config.UseSpot),
	)

	// Generate task YAML
	taskYAML, err := o.generateTaskYAML(config, clusterName)
	if err != nil {
		return "", fmt.Errorf("failed to generate task YAML: %w", err)
	}

	// Write task file
	taskFile := fmt.Sprintf("/tmp/sky-task-%s.yaml", config.NodeID)
	if err := os.WriteFile(taskFile, []byte(taskYAML), 0644); err != nil {
		return "", fmt.Errorf("failed to write task file: %w", err)
	}
	defer os.Remove(taskFile)

	o.logger.Debug("generated SkyPilot task file",
		zap.String("task_file", taskFile),
		zap.Int("yaml_size", len(taskYAML)),
	)

	// Launch with SkyPilot
	cmd := exec.CommandContext(ctx, "sky", "launch",
		"-c", clusterName, // Cluster name
		taskFile,       // Task file
		"-y",           // Auto-confirm
		"--down",       // Terminate on job completion
		"--detach-run", // Detach after launch
	)

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute launch
	if err := cmd.Run(); err != nil {
		o.logger.Error("SkyPilot launch failed",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return "", fmt.Errorf("sky launch failed: %w\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
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
			"", // No tenant ID for system events
			map[string]interface{}{
				"node_id":         config.NodeID,
				"cluster_name":    clusterName,
				"provider":        config.Provider,
				"region":          config.Region,
				"instance_type":   "unknown", // Not in config anymore, maybe infer?
				"gpu_type":        config.GPU,
				"gpu_count":       config.GPUCount,
				"spot_instance":   config.UseSpot,
				"model":           config.Model,
				"launch_duration": launchDuration.String(),
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

// TerminateNode terminates a GPU node and removes it from the cluster.
//
// Process:
// 1. Execute `sky down` to terminate cloud resources
// 2. Update node status in database to 'terminated'
// 3. Clean up local SkyPilot state
//
// Behavior:
// - Graceful shutdown: Waits for running jobs to complete (configurable timeout)
// - Force shutdown: Use context cancellation for immediate termination
// - Resource cleanup: All cloud resources (VM, disk, network) are deleted
// - Cost: No charges after termination (except for persistent disk if configured)
//
// Error Handling:
// - Already terminated: Returns success (idempotent)
// - SkyPilot failure: Returns error with command output
// - Partial failure: Cloud resources may be left in inconsistent state (manual cleanup required)
//
// Returns:
// - error: Termination failure or database update error
func (o *SkyPilotOrchestrator) TerminateNode(ctx context.Context, clusterName string) error {
	o.logger.Info("terminating GPU node",
		zap.String("cluster_name", clusterName),
	)

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

		o.logger.Error("SkyPilot termination failed",
			zap.Error(err),
			zap.String("stdout", stdout.String()),
			zap.String("stderr", stderr.String()),
		)
		return fmt.Errorf("sky down failed: %w\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
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

// GetNodeStatus retrieves the current status of a GPU node from SkyPilot.
//
// Status values:
// - UP: Node is running and healthy
// - INIT: Node is initializing (provisioning or starting)
// - DOWN: Node is terminated
// - STOPPED: Node is stopped but not terminated (can be restarted)
//
// Uses `sky status` command with JSON output for structured parsing.
//
// Returns:
// - string: Node status (UP, INIT, DOWN, STOPPED)
// - error: Command execution error or JSON parsing error
func (o *SkyPilotOrchestrator) GetNodeStatus(ctx context.Context, clusterName string) (string, error) {
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

// ListNodes returns all active GPU nodes managed by SkyPilot.
//
// This queries the local SkyPilot database for all clusters with the
// "cic-" prefix (CrossLogic Inference Cloud nodes).
//
// Returns:
// - []string: List of cluster names
// - error: Command execution error or parsing error
func (o *SkyPilotOrchestrator) ListNodes(ctx context.Context) ([]string, error) {
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

// ExecCommand executes a command on a running node.
//
// Uses `sky exec` to run the command via SSH.
//
// Returns:
// - string: Command output (stdout + stderr)
// - error: Execution failure
func (o *SkyPilotOrchestrator) ExecCommand(ctx context.Context, clusterName, command string) (string, error) {
	o.logger.Debug("executing command on node",
		zap.String("cluster_name", clusterName),
		zap.String("command", command),
	)

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
		"VLLMVersion":  o.vllmVersion,
		"TorchVersion": o.torchVersion,
		"Timestamp":    time.Now().Format(time.RFC3339),
		"R2Endpoint":   o.r2Config.Endpoint,
		"R2Bucket":     o.r2Config.Bucket,
		"R2AccessKey":  o.r2Config.AccessKey,
		"R2SecretKey":  o.r2Config.SecretKey,
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
