package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crosslogic/node-agent/internal/agent"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("starting CrossLogic Node Agent")

	// Load configuration from environment
	config := &agent.Config{
		ControlPlaneURL: getEnv("CONTROL_PLANE_URL", "http://localhost:8080"),
		NodeID:          getEnv("NODE_ID", ""),
		Provider:        getEnv("PROVIDER", "aws"),
		Region:          getEnv("REGION", "us-east-1"),
		ModelName:       getEnv("MODEL_NAME", "llama-3-8b"),
		VLLMEndpoint:    getEnv("VLLM_ENDPOINT", "http://localhost:8000"),
		GPUType:         getEnv("GPU_TYPE", "unknown"),
		InstanceType:    getEnv("INSTANCE_TYPE", "unknown"),
		SpotInstance:    getEnv("SPOT_INSTANCE", "false") == "true",
		HeartbeatInterval: 10 * time.Second,
	}

	// Create and start agent
	nodeAgent, err := agent.NewAgent(config, logger)
	if err != nil {
		logger.Fatal("failed to create agent", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start agent
	if err := nodeAgent.Start(ctx); err != nil {
		logger.Fatal("failed to start agent", zap.Error(err))
	}

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down agent...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := nodeAgent.Stop(shutdownCtx); err != nil {
		logger.Error("failed to stop agent gracefully", zap.Error(err))
	}

	logger.Info("agent stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
