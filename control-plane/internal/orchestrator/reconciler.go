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
type StateReconciler struct {
	db           *database.Database
	logger       *zap.Logger
	orchestrator *SkyPilotOrchestrator
	interval     time.Duration
}

// NewStateReconciler creates a new state reconciler.
func NewStateReconciler(db *database.Database, logger *zap.Logger, orch *SkyPilotOrchestrator) *StateReconciler {
	return &StateReconciler{
		db:           db,
		logger:       logger,
		orchestrator: orch,
		interval:     5 * time.Minute,
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
	for name := range skyClusters {
		if _, exists := dbNodes[name]; !exists {
			r.logger.Warn("found orphan cluster", zap.String("cluster_name", name))
			// Action: Terminate orphan to save cost
			// In production, might want to check age or manual override
			// For now, log and maybe terminate
			// r.orchestrator.TerminateNode(ctx, name)
		}
	}
}

// detectGhosts: Clusters in DB (active) but not in SkyPilot
func (r *StateReconciler) detectGhosts(ctx context.Context, skyClusters map[string]SkyPilotCluster, dbNodes map[string]string) {
	for name, status := range dbNodes {
		if _, exists := skyClusters[name]; !exists {
			// If DB says active/provisioning but SkyPilot doesn't have it
			if status == "active" || status == "provisioning" {
				r.logger.Warn("found ghost cluster", zap.String("cluster_name", name), zap.String("db_status", status))
				// Action: Mark as terminated or failed in DB
				r.updateDBStatus(ctx, name, "failed", "ghost_detected")
			}
		}
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
			switch skyStatus {
			case "UP":
				newStatus = "active"
			case "INIT", "PROVISIONING":
				newStatus = "provisioning"
			case "STOPPED", "AUTOSTOPPED":
				newStatus = "stopped"
			default:
				newStatus = "unknown"
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

func (r *StateReconciler) updateDBStatus(ctx context.Context, clusterName, status, message string) {
	query := `UPDATE nodes SET status = $1, status_message = COALESCE(NULLIF($2, ''), status_message), updated_at = NOW() WHERE cluster_name = $3`
	_, err := r.db.Pool.Exec(ctx, query, status, message, clusterName)
	if err != nil {
		r.logger.Error("failed to update db status", zap.Error(err))
	}
}
