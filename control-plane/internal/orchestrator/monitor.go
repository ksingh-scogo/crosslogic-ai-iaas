package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// TripleSafetyMonitor implements the 3-layer node health monitoring strategy.
// Layer 1: Push (Node Agent Heartbeats)
// Layer 2: Pull (Active Polling)
// Layer 3: Cloud API Verification
type TripleSafetyMonitor struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator

	// Configuration
	heartbeatTimeout   time.Duration
	pollInterval       time.Duration
	cloudCheckInterval time.Duration
}

// NewTripleSafetyMonitor creates a new safety monitor.
func NewTripleSafetyMonitor(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator) *TripleSafetyMonitor {
	return &TripleSafetyMonitor{
		db:                 db,
		logger:             logger,
		orchestrator:       orch,
		heartbeatTimeout:   30 * time.Second,
		pollInterval:       1 * time.Minute,
		cloudCheckInterval: 5 * time.Minute,
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
	// We use a direct query here for speed, or could use a cache
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
	// Get all active nodes
	rows, err := m.db.Pool.Query(ctx, "SELECT id, endpoint FROM nodes WHERE status = 'active'")
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
			if !m.checkNodeHealth(url) {
				m.logger.Warn("node active poll failed", zap.String("node_id", nodeID))
				// Mark as suspect or trigger verification
				m.markNodeSuspect(ctx, nodeID, "poll_failed")
			}
		}(id, endpoint)
	}
	wg.Wait()
}

func (m *TripleSafetyMonitor) checkNodeHealth(endpoint string) bool {
	// Simple HTTP check to vLLM health endpoint
	// Implementation would use http.Client with short timeout
	// For now, just a placeholder logic
	return true
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
	// Get all nodes that are supposed to be active
	// Run `sky status` or cloud SDK calls to verify they exist and are running
	// This is expensive, so done less frequently

	// For now, we can rely on SkyPilot's status
	// In a real implementation, we'd call SkyPilot CLI or library
}

func (m *TripleSafetyMonitor) markNodeSuspect(ctx context.Context, nodeID, reason string) {
	m.logger.Warn("marking node as suspect",
		zap.String("node_id", nodeID),
		zap.String("reason", reason),
	)

	query := `UPDATE nodes SET status = 'suspect', status_message = $1 WHERE id = $2`
	m.db.Pool.Exec(ctx, query, reason, nodeID)
}
