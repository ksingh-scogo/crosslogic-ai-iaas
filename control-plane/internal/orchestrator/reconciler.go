package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// StateReconciler ensures the database state matches the actual cloud state.
// This is the third layer of the triple safety monitoring system.
type StateReconciler struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator
	monitor      *TripleSafetyMonitor
	interval     time.Duration

	// Configuration
	autoTerminateOrphans bool // Automatically terminate orphan clusters
	orphanGracePeriod    time.Duration // Grace period before terminating orphans
}

// NewStateReconciler creates a new state reconciler.
func NewStateReconciler(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator, monitor *TripleSafetyMonitor) *StateReconciler {
	return &StateReconciler{
		db:                   db,
		logger:               logger,
		orchestrator:         orch,
		monitor:              monitor,
		interval:             1 * time.Minute, // More frequent reconciliation
		autoTerminateOrphans: true,
		orphanGracePeriod:    10 * time.Minute, // 10 minute grace period
	}
}

// Start begins the reconciliation loop.
func (r *StateReconciler) Start(ctx context.Context) {
	r.logger.Info("starting state reconciler")
	go r.reconciliationLoop(ctx)
}

func (r *StateReconciler) reconciliationLoop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	r.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

type SkyPilotCluster struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Region string `json:"region"`
	HeadIP string `json:"head_ip"`
}

func (r *StateReconciler) reconcile(ctx context.Context) {
	r.logger.Debug("running state reconciliation")

	// 1. Get actual state from SkyPilot
	skyClusters, err := r.getSkyPilotClusters(ctx)
	if err != nil {
		r.logger.Error("failed to get skypilot clusters", zap.Error(err))
		return
	}

	// 2. Get expected state from Database
	dbNodes, err := r.getDBNodes(ctx)
	if err != nil {
		r.logger.Error("failed to get db nodes", zap.Error(err))
		return
	}

	// 3. Compare and Reconcile
	r.detectOrphans(ctx, skyClusters, dbNodes)
	r.detectGhosts(ctx, skyClusters, dbNodes)
	r.syncStatus(ctx, skyClusters, dbNodes)
}

func (r *StateReconciler) getSkyPilotClusters(ctx context.Context) (map[string]SkyPilotCluster, error) {
	cmd := exec.CommandContext(ctx, "sky", "status", "--refresh", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sky status failed: %w", err)
	}

	var clusters []SkyPilotCluster
	if err := json.Unmarshal(output, &clusters); err != nil {
		return nil, fmt.Errorf("failed to parse sky status json: %w", err)
	}

	clusterMap := make(map[string]SkyPilotCluster)
	for _, c := range clusters {
		// Filter for our clusters only (cic- prefix)
		if strings.HasPrefix(c.Name, "cic-") {
			clusterMap[c.Name] = c
		}
	}
	return clusterMap, nil
}

func (r *StateReconciler) getDBNodes(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.Pool.Query(ctx, "SELECT cluster_name, status FROM nodes WHERE status != 'terminated'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodeMap := make(map[string]string)
	for rows.Next() {
		var name, status string
		if err := rows.Scan(&name, &status); err != nil {
			continue
		}
		nodeMap[name] = status
	}
	return nodeMap, nil
}

// detectOrphans: Clusters in SkyPilot but not in DB (or terminated in DB)
func (r *StateReconciler) detectOrphans(ctx context.Context, skyClusters map[string]SkyPilotCluster, dbNodes map[string]string) {
	for name, cluster := range skyClusters {
		if _, exists := dbNodes[name]; !exists {
			r.logger.Warn("found orphan cluster",
				zap.String("cluster_name", name),
				zap.String("status", cluster.Status),
				zap.String("region", cluster.Region),
			)

			// Automatically terminate orphans to prevent cost leakage
			if r.autoTerminateOrphans {
				// Check if cluster has existed beyond grace period
				// For now, terminate immediately as we can't get creation time easily
				// In production, you might want to tag clusters with creation time

				r.logger.Info("terminating orphan cluster to prevent cost leakage",
					zap.String("cluster_name", name),
				)

				if err := r.orchestrator.TerminateNode(ctx, name); err != nil {
					r.logger.Error("failed to terminate orphan cluster",
						zap.String("cluster_name", name),
						zap.Error(err),
					)
				} else {
					r.logger.Info("successfully terminated orphan cluster",
						zap.String("cluster_name", name),
					)
				}
			}
		}
	}
}

// detectGhosts: Clusters in DB (active) but not in SkyPilot
func (r *StateReconciler) detectGhosts(ctx context.Context, skyClusters map[string]SkyPilotCluster, dbNodes map[string]string) {
	for name, status := range dbNodes {
		if _, exists := skyClusters[name]; !exists {
			// If DB says active/provisioning but SkyPilot doesn't have it
			if status == "active" || status == "provisioning" || status == "suspect" || status == "degraded" {
				r.logger.Warn("found ghost cluster",
					zap.String("cluster_name", name),
					zap.String("db_status", status),
				)

				// Ghost clusters should be marked as dead
				// This feeds back into the triple safety monitor
				r.updateDBStatus(ctx, name, "dead", "ghost_detected_by_reconciler")

				// Find node ID and trigger health evaluation
				if r.monitor != nil {
					go r.markGhostNodeForInvestigation(ctx, name)
				}
			}
		}
	}
}

// markGhostNodeForInvestigation finds the node ID and triggers health signal update
func (r *StateReconciler) markGhostNodeForInvestigation(ctx context.Context, clusterName string) {
	// Get node ID from cluster name
	var nodeID string
	err := r.db.Pool.QueryRow(ctx,
		"SELECT id FROM nodes WHERE cluster_name = $1",
		clusterName,
	).Scan(&nodeID)

	if err != nil {
		r.logger.Error("failed to find node for ghost cluster",
			zap.String("cluster_name", clusterName),
			zap.Error(err),
		)
		return
	}

	// Store a cloud API signal indicating the cluster is not found
	if r.monitor != nil {
		r.monitor.storeHealthSignal(nodeID, HealthSignal{
			Healthy:   false,
			Timestamp: time.Now(),
			Source:    "cloud_api",
			Message:   "cluster_not_found_by_reconciler",
		})

		// Trigger health evaluation
		r.monitor.evaluateNodeHealth(ctx, nodeID)
	}
}

// syncStatus: Update DB status if different from SkyPilot
func (r *StateReconciler) syncStatus(ctx context.Context, skyClusters map[string]SkyPilotCluster, dbNodes map[string]string) {
	for name, dbStatus := range dbNodes {
		if skyCluster, exists := skyClusters[name]; exists {
			// Map SkyPilot status to our status
			// SkyPilot statuses: INIT, PROVISIONING, UP, STOPPED, AUTOSTOPPED
			skyStatus := strings.ToUpper(skyCluster.Status)

			var newStatus string
			healthy := false
			switch skyStatus {
			case "UP":
				newStatus = "active"
				healthy = true
			case "INIT", "PROVISIONING":
				newStatus = "provisioning"
				healthy = true // Provisioning is healthy
			case "STOPPED", "AUTOSTOPPED":
				newStatus = "stopped"
				healthy = false
			default:
				newStatus = "unknown"
				healthy = false
			}

			// Update triple safety monitor with cloud API signal
			if r.monitor != nil {
				go r.updateMonitorSignal(ctx, name, healthy, fmt.Sprintf("cloud_status=%s", skyStatus))
			}

			if newStatus != dbStatus && newStatus != "unknown" {
				r.logger.Info("syncing node status",
					zap.String("cluster_name", name),
					zap.String("old_status", dbStatus),
					zap.String("new_status", newStatus),
				)
				r.updateDBStatus(ctx, name, newStatus, "")
			}
		}
	}
}

// updateMonitorSignal sends a cloud API health signal to the triple safety monitor
func (r *StateReconciler) updateMonitorSignal(ctx context.Context, clusterName string, healthy bool, message string) {
	// Get node ID from cluster name
	var nodeID string
	err := r.db.Pool.QueryRow(ctx,
		"SELECT id FROM nodes WHERE cluster_name = $1",
		clusterName,
	).Scan(&nodeID)

	if err != nil {
		// Node might not exist yet, ignore
		return
	}

	// Store cloud API signal
	r.monitor.storeHealthSignal(nodeID, HealthSignal{
		Healthy:   healthy,
		Timestamp: time.Now(),
		Source:    "cloud_api",
		Message:   message,
	})

	// Trigger health evaluation
	r.monitor.evaluateNodeHealth(ctx, nodeID)
}

func (r *StateReconciler) updateDBStatus(ctx context.Context, clusterName, status, message string) {
	query := `UPDATE nodes SET status = $1, status_message = COALESCE(NULLIF($2, ''), status_message), updated_at = NOW() WHERE cluster_name = $3`
	_, err := r.db.Pool.Exec(ctx, query, status, message, clusterName)
	if err != nil {
		r.logger.Error("failed to update db status", zap.Error(err))
	}
}
