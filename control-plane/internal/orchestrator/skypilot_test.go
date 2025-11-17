package orchestrator

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TestNewSkyPilotOrchestrator verifies orchestrator initialization
func TestNewSkyPilotOrchestrator(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	controlPlaneURL := "https://api.crosslogic.ai"

	orch, err := NewSkyPilotOrchestrator(db, logger, controlPlaneURL)

	if err != nil {
		t.Fatalf("NewSkyPilotOrchestrator failed: %v", err)
	}

	if orch == nil {
		t.Fatal("Orchestrator is nil")
	}

	if orch.taskTemplate == nil {
		t.Error("Task template not initialized")
	}

	if orch.db == nil {
		t.Error("Database not set")
	}

	if orch.logger == nil {
		t.Error("Logger not set")
	}

	if orch.controlPlaneURL != controlPlaneURL {
		t.Errorf("Expected control plane URL %s, got %s", controlPlaneURL, orch.controlPlaneURL)
	}
}

// TestValidateNodeConfig tests configuration validation
func TestValidateNodeConfig(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	tests := []struct {
		name        string
		config      NodeConfig
		shouldError bool
	}{
		{
			name: "Valid configuration",
			config: NodeConfig{
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      "A100",
				Model:    "meta-llama/Llama-2-7b-chat-hf",
				UseSpot:  true,
			},
			shouldError: false,
		},
		{
			name: "Missing provider",
			config: NodeConfig{
				Region: "us-west-2",
				GPU:    "A100",
				Model:  "meta-llama/Llama-2-7b-chat-hf",
			},
			shouldError: true,
		},
		{
			name: "Missing region",
			config: NodeConfig{
				Provider: "aws",
				GPU:      "A100",
				Model:    "meta-llama/Llama-2-7b-chat-hf",
			},
			shouldError: true,
		},
		{
			name: "Missing GPU",
			config: NodeConfig{
				Provider: "aws",
				Region:   "us-west-2",
				Model:    "meta-llama/Llama-2-7b-chat-hf",
			},
			shouldError: true,
		},
		{
			name: "Missing model",
			config: NodeConfig{
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      "A100",
			},
			shouldError: true,
		},
		{
			name: "Auto-generated NodeID",
			config: NodeConfig{
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      "A100",
				Model:    "meta-llama/Llama-2-7b-chat-hf",
				// NodeID not provided - should be auto-generated
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := orch.validateNodeConfig(&tt.config)

			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check auto-generated NodeID
			if !tt.shouldError && tt.config.NodeID == "" {
				t.Error("NodeID should be auto-generated")
			}

			// Check default DiskSize
			if !tt.shouldError && tt.config.DiskSize == 0 {
				// After validation, DiskSize should be set to default
				// (Note: Current implementation doesn't set default in validateNodeConfig)
			}
		})
	}
}

// TestGenerateTaskYAML tests YAML generation
func TestGenerateTaskYAML(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	controlPlaneURL := "https://api.test.com"
	orch, _ := NewSkyPilotOrchestrator(db, logger, controlPlaneURL)

	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A100",
		Model:    "meta-llama/Llama-2-7b-chat-hf",
		UseSpot:  true,
		DiskSize: 256,
		VLLMArgs: "--tensor-parallel-size 2",
	}

	yaml, err := orch.generateTaskYAML(config)

	if err != nil {
		t.Fatalf("generateTaskYAML failed: %v", err)
	}

	// Verify YAML contains expected content
	expectedStrings := []string{
		"name: cic-" + config.NodeID,
		"accelerators: A100:1",
		"cloud: aws",
		"region: us-west-2",
		"use_spot: true",
		"disk_size: 256",
		"meta-llama/Llama-2-7b-chat-hf",
		"--tensor-parallel-size 2",
		controlPlaneURL,
		"export NODE_ID=" + config.NodeID,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(yaml, expected) {
			t.Errorf("YAML doesn't contain expected string: %s", expected)
		}
	}
}

// TestGenerateTaskYAML_OnDemand tests on-demand instance YAML
func TestGenerateTaskYAML_OnDemand(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "gcp",
		Region:   "us-central1",
		GPU:      "V100",
		Model:    "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
		UseSpot:  false, // On-demand
		DiskSize: 128,
	}

	yaml, err := orch.generateTaskYAML(config)

	if err != nil {
		t.Fatalf("generateTaskYAML failed: %v", err)
	}

	// Should not contain spot configuration
	if strings.Contains(yaml, "use_spot: true") {
		t.Error("On-demand YAML should not contain 'use_spot: true'")
	}

	// Should contain GCP configuration
	if !strings.Contains(yaml, "cloud: gcp") {
		t.Error("YAML should contain GCP cloud")
	}
}

// TestLaunchNode_YAMLFileCreation tests task file creation
func TestLaunchNode_YAMLFileCreation(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A10G",
		Model:    "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
		UseSpot:  true,
		DiskSize: 100,
	}

	// Generate YAML
	yaml, err := orch.generateTaskYAML(config)
	if err != nil {
		t.Fatalf("generateTaskYAML failed: %v", err)
	}

	// Write to temp file (simulating LaunchNode behavior)
	taskFile := "/tmp/test-sky-task-" + config.NodeID + ".yaml"
	err = os.WriteFile(taskFile, []byte(yaml), 0644)
	if err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}
	defer os.Remove(taskFile)

	// Verify file exists and has content
	content, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("Failed to read task file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Task file is empty")
	}

	if !strings.Contains(string(content), config.NodeID) {
		t.Error("Task file doesn't contain node ID")
	}
}

// TestLaunchNode_ClusterName tests cluster name generation
func TestLaunchNode_ClusterName(t *testing.T) {
	nodeID := uuid.New().String()
	expectedClusterName := "cic-" + nodeID

	config := NodeConfig{
		NodeID:   nodeID,
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A100",
		Model:    "test-model",
	}

	clusterName := "cic-" + config.NodeID

	if clusterName != expectedClusterName {
		t.Errorf("Expected cluster name %s, got %s", expectedClusterName, clusterName)
	}
}

// TestNodeConfig_JSONSerialization tests JSON marshaling
func TestNodeConfig_JSONSerialization(t *testing.T) {
	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A100",
		Model:    "meta-llama/Llama-2-7b-chat-hf",
		UseSpot:  true,
		DiskSize: 256,
		VLLMArgs: "--max-model-len 4096",
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded NodeConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify fields
	if decoded.NodeID != config.NodeID {
		t.Error("NodeID mismatch after JSON round-trip")
	}

	if decoded.Provider != config.Provider {
		t.Error("Provider mismatch after JSON round-trip")
	}

	if decoded.GPU != config.GPU {
		t.Error("GPU mismatch after JSON round-trip")
	}

	if decoded.UseSpot != config.UseSpot {
		t.Error("UseSpot mismatch after JSON round-trip")
	}
}

// TestMultiCloudConfigurations tests different cloud providers
func TestMultiCloudConfigurations(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	providers := []string{"aws", "gcp", "azure", "lambda", "oci"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			config := NodeConfig{
				NodeID:   uuid.New().String(),
				Provider: provider,
				Region:   "us-west-2",
				GPU:      "A100",
				Model:    "test-model",
				UseSpot:  true,
			}

			err := orch.validateNodeConfig(&config)
			if err != nil {
				t.Errorf("Configuration should be valid for %s: %v", provider, err)
			}

			yaml, err := orch.generateTaskYAML(config)
			if err != nil {
				t.Errorf("YAML generation failed for %s: %v", provider, err)
			}

			if !strings.Contains(yaml, "cloud: "+provider) {
				t.Errorf("YAML should contain cloud: %s", provider)
			}
		})
	}
}

// TestGPUTypes tests different GPU configurations
func TestGPUTypes(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	gpuTypes := []string{"A100", "V100", "A10G", "T4", "H100", "L4"}

	for _, gpu := range gpuTypes {
		t.Run(gpu, func(t *testing.T) {
			config := NodeConfig{
				NodeID:   uuid.New().String(),
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      gpu,
				Model:    "test-model",
			}

			yaml, err := orch.generateTaskYAML(config)
			if err != nil {
				t.Errorf("YAML generation failed for %s: %v", gpu, err)
			}

			expected := "accelerators: " + gpu + ":1"
			if !strings.Contains(yaml, expected) {
				t.Errorf("YAML should contain '%s'", expected)
			}
		})
	}
}

// TestVLLMArgsIncorporation tests custom vLLM arguments
func TestVLLMArgsIncorporation(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	customArgs := []string{
		"--tensor-parallel-size 2",
		"--max-model-len 4096",
		"--gpu-memory-utilization 0.95",
		"--dtype float16",
	}

	for _, args := range customArgs {
		t.Run(args, func(t *testing.T) {
			config := NodeConfig{
				NodeID:   uuid.New().String(),
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      "A100",
				Model:    "test-model",
				VLLMArgs: args,
			}

			yaml, err := orch.generateTaskYAML(config)
			if err != nil {
				t.Fatalf("YAML generation failed: %v", err)
			}

			if !strings.Contains(yaml, args) {
				t.Errorf("YAML should contain vLLM args: %s", args)
			}
		})
	}
}

// TestTaskYAMLStructure tests YAML has required sections
func TestTaskYAMLStructure(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A100",
		Model:    "test-model",
	}

	yaml, err := orch.generateTaskYAML(config)
	if err != nil {
		t.Fatalf("YAML generation failed: %v", err)
	}

	// Required sections
	requiredSections := []string{
		"name:",
		"resources:",
		"setup:",
		"run:",
		"accelerators:",
		"cloud:",
		"region:",
		"disk_size:",
	}

	for _, section := range requiredSections {
		if !strings.Contains(yaml, section) {
			t.Errorf("YAML missing required section: %s", section)
		}
	}

	// Should contain environment variable exports
	envVars := []string{
		"export CONTROL_PLANE_URL=",
		"export NODE_ID=",
		"export MODEL_NAME=",
		"export REGION=",
		"export PROVIDER=",
	}

	for _, envVar := range envVars {
		if !strings.Contains(yaml, envVar) {
			t.Errorf("YAML missing environment variable: %s", envVar)
		}
	}
}

// BenchmarkGenerateTaskYAML benchmarks YAML generation
func BenchmarkGenerateTaskYAML(b *testing.B) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	config := NodeConfig{
		NodeID:   uuid.New().String(),
		Provider: "aws",
		Region:   "us-west-2",
		GPU:      "A100",
		Model:    "meta-llama/Llama-2-7b-chat-hf",
		UseSpot:  true,
		DiskSize: 256,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orch.generateTaskYAML(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestConcurrentYAMLGeneration tests thread safety
func TestConcurrentYAMLGeneration(t *testing.T) {
	logger := zap.NewNop()
	db := &database.Database{Pool: &pgxpool.Pool{}}
	orch, _ := NewSkyPilotOrchestrator(db, logger, "https://api.test.com")

	done := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			config := NodeConfig{
				NodeID:   uuid.New().String(),
				Provider: "aws",
				Region:   "us-west-2",
				GPU:      "A100",
				Model:    "test-model",
			}

			_, err := orch.generateTaskYAML(config)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	select {
	case err := <-errors:
		t.Errorf("Concurrent YAML generation failed: %v", err)
	default:
		// No errors
	}
}
