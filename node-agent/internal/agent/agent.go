package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Config holds agent configuration
type Config struct {
	ControlPlaneURL   string
	NodeID            string
	Provider          string
	Region            string
	ModelName         string
	VLLMEndpoint      string
	GPUType           string
	InstanceType      string
	SpotInstance      bool
	HeartbeatInterval time.Duration
}

// Agent represents a node agent
type Agent struct {
	config     *Config
	logger     *zap.Logger
	httpClient *http.Client
	nodeID     string
	stopChan   chan struct{}
}

// NewAgent creates a new node agent
func NewAgent(config *Config, logger *zap.Logger) (*Agent, error) {
	return &Agent{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopChan: make(chan struct{}),
	}, nil
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("starting node agent",
		zap.String("provider", a.config.Provider),
		zap.String("region", a.config.Region),
		zap.String("model", a.config.ModelName),
	)

	// Register with control plane
	if err := a.register(ctx); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	// Start heartbeat loop
	go a.heartbeatLoop(ctx)

	// Start health monitoring
	go a.healthMonitorLoop(ctx)

	return nil
}

// Stop stops the agent
func (a *Agent) Stop(ctx context.Context) error {
	a.logger.Info("stopping node agent")

	// Signal to stop loops
	close(a.stopChan)

	// Deregister from control plane
	return a.deregister(ctx)
}

// register registers the node with the control plane
func (a *Agent) register(ctx context.Context) error {
	payload := map[string]interface{}{
		"provider":      a.config.Provider,
		"region":        a.config.Region,
		"model_name":    a.config.ModelName,
		"endpoint_url":  a.config.VLLMEndpoint,
		"gpu_type":      a.config.GPUType,
		"instance_type": a.config.InstanceType,
		"spot_instance": a.config.SpotInstance,
		"status":        "active",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/admin/nodes/register", a.config.ControlPlaneURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	// Parse response to get node ID
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if nodeID, ok := result["node_id"].(string); ok {
		a.nodeID = nodeID
		a.logger.Info("registered successfully", zap.String("node_id", nodeID))
	}

	return nil
}

// deregister deregisters the node from the control plane
func (a *Agent) deregister(ctx context.Context) error {
	if a.nodeID == "" {
		return nil
	}

	url := fmt.Sprintf("%s/admin/nodes/%s/deregister", a.config.ControlPlaneURL, a.nodeID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	a.logger.Info("deregistered from control plane")
	return nil
}

// heartbeatLoop sends periodic heartbeats
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopChan:
			return
		case <-ticker.C:
			if err := a.sendHeartbeat(ctx); err != nil {
				a.logger.Error("heartbeat failed", zap.Error(err))
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the control plane
func (a *Agent) sendHeartbeat(ctx context.Context) error {
	if a.nodeID == "" {
		return fmt.Errorf("node not registered")
	}

	// Get health metrics
	healthScore := a.calculateHealthScore(ctx)

	payload := map[string]interface{}{
		"node_id":      a.nodeID,
		"health_score": healthScore,
		"timestamp":    time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/admin/nodes/%s/heartbeat", a.config.ControlPlaneURL, a.nodeID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status %d", resp.StatusCode)
	}

	a.logger.Debug("heartbeat sent", zap.Float64("health_score", healthScore))
	return nil
}

// calculateHealthScore calculates the node's health score
func (a *Agent) calculateHealthScore(ctx context.Context) float64 {
	// Check vLLM health
	vllmHealthy := a.checkVLLMHealth(ctx)
	if !vllmHealthy {
		return 0.0
	}

	// Base health score
	healthScore := 100.0

	// TODO: Add more health checks:
	// - GPU temperature
	// - VRAM usage
	// - CPU usage
	// - Network latency

	return healthScore
}

// checkVLLMHealth checks if vLLM is healthy
func (a *Agent) checkVLLMHealth(ctx context.Context) bool {
	url := fmt.Sprintf("%s/health", a.config.VLLMEndpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// healthMonitorLoop monitors node health
func (a *Agent) healthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopChan:
			return
		case <-ticker.C:
			a.monitorHealth(ctx)
		}
	}
}

// monitorHealth monitors node health and reports issues
func (a *Agent) monitorHealth(ctx context.Context) {
	if !a.checkVLLMHealth(ctx) {
		a.logger.Warn("vLLM health check failed")
		// TODO: Report to control plane
	}
}
