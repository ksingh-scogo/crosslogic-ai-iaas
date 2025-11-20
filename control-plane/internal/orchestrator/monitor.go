package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/events"
	"go.uber.org/zap"
)

// NodeHealthStatus represents the health state of a node
type NodeHealthStatus string

const (
	NodeHealthy   NodeHealthStatus = "healthy"
	NodeDegraded  NodeHealthStatus = "degraded"
	NodeSuspect   NodeHealthStatus = "suspect"
	NodeDead      NodeHealthStatus = "dead"
	NodeDraining  NodeHealthStatus = "draining"
)

// HealthSignal represents a health check result from one layer
type HealthSignal struct {
	Healthy   bool
	Timestamp time.Time
	Source    string // "heartbeat", "poll", "cloud_api"
	Message   string
}

// TripleSafetyMonitor implements the 3-layer node health monitoring strategy.
// Layer 1: Push (Node Agent Heartbeats) - every 10s
// Layer 2: Pull (Active Polling) - every 30s
// Layer 3: Cloud API Verification - every 60s
type TripleSafetyMonitor struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator
	eventBus     *events.Bus

	// HTTP client for health checks
	httpClient *http.Client

	// Configuration
	heartbeatTimeout   time.Duration
	pollInterval       time.Duration
	cloudCheckInterval time.Duration

	// Cache for health signals
	healthSignals sync.Map // nodeID -> map[string]*HealthSignal
}

// NewTripleSafetyMonitor creates a new safety monitor.
func NewTripleSafetyMonitor(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator, eventBus *events.Bus) *TripleSafetyMonitor {
	return &TripleSafetyMonitor{
		db:           db,
		logger:       logger,
		orchestrator: orch,
		eventBus:     eventBus,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		heartbeatTimeout:   30 * time.Second,
		pollInterval:       30 * time.Second, // More frequent polling
		cloudCheckInterval: 1 * time.Minute,  // More frequent cloud checks
	}
}

// Start begins the background monitoring loops.
func (m *TripleSafetyMonitor) Start(ctx context.Context) {
	m.logger.Info("starting triple safety monitor")

	// Layer 2: Active Polling Loop
	go m.activePollingLoop(ctx)

	// Layer 3: Cloud API Verification Loop
	go m.cloudVerificationLoop(ctx)
}

// RecordHeartbeat processes a heartbeat from a node (Layer 1).
func (m *TripleSafetyMonitor) RecordHeartbeat(ctx context.Context, nodeID string, healthScore float64) error {
	// Update node status and last_heartbeat in DB
	query := `
		UPDATE nodes
		SET last_heartbeat = NOW(), health_score = $1, status = 'active'
		WHERE id = $2
	`
	result, err := m.db.Pool.Exec(ctx, query, healthScore, nodeID)
	if err != nil {
		return fmt.Errorf("failed to record heartbeat: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Store heartbeat signal
	m.storeHealthSignal(nodeID, HealthSignal{
		Healthy:   healthScore > 0.5,
		Timestamp: time.Now(),
		Source:    "heartbeat",
		Message:   fmt.Sprintf("health_score=%.2f", healthScore),
	})

	// Determine overall node health based on all signals
	go m.evaluateNodeHealth(ctx, nodeID)

	return nil
}

// activePollingLoop periodically polls nodes to verify they are reachable (Layer 2).
func (m *TripleSafetyMonitor) activePollingLoop(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollNodes(ctx)
		}
	}
}

func (m *TripleSafetyMonitor) pollNodes(ctx context.Context) {
	// Get all active nodes (or suspect nodes that need verification)
	rows, err := m.db.Pool.Query(ctx, "SELECT id, endpoint FROM nodes WHERE status IN ('active', 'suspect')")
	if err != nil {
		m.logger.Error("failed to fetch nodes for polling", zap.Error(err))
		return
	}
	defer rows.Close()

	var wg sync.WaitGroup
	for rows.Next() {
		var id, endpoint string
		if err := rows.Scan(&id, &endpoint); err != nil {
			continue
		}

		wg.Add(1)
		go func(nodeID, url string) {
			defer wg.Done()
			healthy, message := m.checkNodeHealth(url)

			// Store poll signal
			m.storeHealthSignal(nodeID, HealthSignal{
				Healthy:   healthy,
				Timestamp: time.Now(),
				Source:    "poll",
				Message:   message,
			})

			// Evaluate overall health
			m.evaluateNodeHealth(ctx, nodeID)
		}(id, endpoint)
	}
	wg.Wait()
}

func (m *TripleSafetyMonitor) checkNodeHealth(endpoint string) (bool, string) {
	// Check vLLM health endpoint
	healthURL := endpoint + "/health"

	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return false, fmt.Sprintf("failed to create request: %v", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false, fmt.Sprintf("http error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, "health check passed"
	}

	return false, fmt.Sprintf("health check failed: status=%d", resp.StatusCode)
}

// cloudVerificationLoop periodically verifies node status with cloud provider (Layer 3).
func (m *TripleSafetyMonitor) cloudVerificationLoop(ctx context.Context) {
	ticker := time.NewTicker(m.cloudCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.verifyCloudStatus(ctx)
		}
	}
}

func (m *TripleSafetyMonitor) verifyCloudStatus(ctx context.Context) {
	// Get all nodes with cluster names
	rows, err := m.db.Pool.Query(ctx, "SELECT id, cluster_name FROM nodes WHERE cluster_name IS NOT NULL AND status IN ('active', 'suspect')")
	if err != nil {
		m.logger.Error("failed to fetch nodes for cloud verification", zap.Error(err))
		return
	}
	defer rows.Close()

	var wg sync.WaitGroup
	for rows.Next() {
		var id, clusterName string
		if err := rows.Scan(&id, &clusterName); err != nil {
			continue
		}

		wg.Add(1)
		go func(nodeID, cluster string) {
			defer wg.Done()

			// Check cluster status via SkyPilot
			running, message := m.checkClusterStatus(cluster)

			// Store cloud verification signal
			m.storeHealthSignal(nodeID, HealthSignal{
				Healthy:   running,
				Timestamp: time.Now(),
				Source:    "cloud_api",
				Message:   message,
			})

			// Evaluate overall health
			m.evaluateNodeHealth(ctx, nodeID)
		}(id, clusterName)
	}
	wg.Wait()
}

func (m *TripleSafetyMonitor) checkClusterStatus(clusterName string) (bool, string) {
	if m.orchestrator == nil {
		return true, "orchestrator not available"
	}

	// Use SkyPilot to check cluster status
	status, err := m.orchestrator.GetClusterStatus(clusterName)
	if err != nil {
		return false, fmt.Sprintf("failed to get cluster status: %v", err)
	}

	running := status == "UP" || status == "RUNNING"
	return running, fmt.Sprintf("cluster_status=%s", status)
}

// storeHealthSignal stores a health signal for a node
func (m *TripleSafetyMonitor) storeHealthSignal(nodeID string, signal HealthSignal) {
	// Get or create signals map for this node
	value, _ := m.healthSignals.LoadOrStore(nodeID, &sync.Map{})
	signals := value.(*sync.Map)

	// Store signal by source
	signals.Store(signal.Source, &signal)
}

// getHealthSignals retrieves all health signals for a node
func (m *TripleSafetyMonitor) getHealthSignals(nodeID string) map[string]*HealthSignal {
	value, ok := m.healthSignals.Load(nodeID)
	if !ok {
		return nil
	}

	signals := value.(*sync.Map)
	result := make(map[string]*HealthSignal)

	signals.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*HealthSignal)
		return true
	})

	return result
}

// evaluateNodeHealth uses truth table logic to determine node health status
func (m *TripleSafetyMonitor) evaluateNodeHealth(ctx context.Context, nodeID string) {
	signals := m.getHealthSignals(nodeID)
	if signals == nil {
		return
	}

	// Get signals from each layer
	heartbeat := signals["heartbeat"]
	poll := signals["poll"]
	cloudAPI := signals["cloud_api"]

	// Apply truth table logic
	status := m.determineNodeHealth(heartbeat, poll, cloudAPI)

	// Update database and publish event
	m.updateNodeStatus(ctx, nodeID, status, signals)
}

// determineNodeHealth implements the truth table decision logic
func (m *TripleSafetyMonitor) determineNodeHealth(heartbeat, poll, cloudAPI *HealthSignal) NodeHealthStatus {
	now := time.Now()

	// Check if signals are stale
	heartbeatHealthy := heartbeat != nil && heartbeat.Healthy && now.Sub(heartbeat.Timestamp) < m.heartbeatTimeout
	pollHealthy := poll != nil && poll.Healthy && now.Sub(poll.Timestamp) < 2*m.pollInterval
	cloudHealthy := cloudAPI != nil && cloudAPI.Healthy && now.Sub(cloudAPI.Timestamp) < 2*m.cloudCheckInterval

	// Truth Table Logic:
	// All agree: healthy -> NodeHealthy
	if heartbeatHealthy && pollHealthy && cloudHealthy {
		return NodeHealthy
	}

	// All agree: unhealthy -> NodeDead
	if !heartbeatHealthy && !pollHealthy && !cloudHealthy {
		return NodeDead
	}

	// Disagreement cases:
	// Cloud says running but others fail -> likely network issue or node-agent crash
	if cloudHealthy && (!heartbeatHealthy || !pollHealthy) {
		return NodeDegraded // Cloud running but service issues
	}

	// Heartbeat and poll healthy but cloud says down -> suspect (investigate)
	if heartbeatHealthy && pollHealthy && !cloudHealthy {
		return NodeSuspect // Data plane healthy but control plane doesn't see it
	}

	// Mixed signals -> suspect
	return NodeSuspect
}

// updateNodeStatus updates the node status in database and publishes event
func (m *TripleSafetyMonitor) updateNodeStatus(ctx context.Context, nodeID string, status NodeHealthStatus, signals map[string]*HealthSignal) {
	// Build status message from signals
	var messages []string
	for source, signal := range signals {
		messages = append(messages, fmt.Sprintf("%s: %s", source, signal.Message))
	}
	statusMessage := fmt.Sprintf("%s | %s", status, messages)

	// Map NodeHealthStatus to database status
	dbStatus := "active"
	switch status {
	case NodeHealthy:
		dbStatus = "active"
	case NodeDegraded:
		dbStatus = "degraded"
	case NodeSuspect:
		dbStatus = "suspect"
	case NodeDead:
		dbStatus = "dead"
	case NodeDraining:
		dbStatus = "draining"
	}

	// Update database
	query := `UPDATE nodes SET status = $1, status_message = $2, updated_at = NOW() WHERE id = $3`
	_, err := m.db.Pool.Exec(ctx, query, dbStatus, statusMessage, nodeID)
	if err != nil {
		m.logger.Error("failed to update node status",
			zap.String("node_id", nodeID),
			zap.String("status", dbStatus),
			zap.Error(err),
		)
		return
	}

	// Publish event
	if m.eventBus != nil {
		event := events.NewEvent(
			events.EventNodeHealthChanged,
			"", // System event, no specific tenant
			map[string]interface{}{
				"node_id": nodeID,
				"status":  dbStatus,
				"message": statusMessage,
				"signals": signals,
			},
		)
		m.eventBus.Publish(context.Background(), event)
	}

	m.logger.Info("node health status updated",
		zap.String("node_id", nodeID),
		zap.String("status", dbStatus),
	)
}

func (m *TripleSafetyMonitor) markNodeSuspect(ctx context.Context, nodeID, reason string) {
	m.logger.Warn("marking node as suspect",
		zap.String("node_id", nodeID),
		zap.String("reason", reason),
	)

	query := `UPDATE nodes SET status = 'suspect', status_message = $1 WHERE id = $2`
	m.db.Pool.Exec(ctx, query, reason, nodeID)
}
