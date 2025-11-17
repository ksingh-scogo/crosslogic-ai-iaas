package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodePool manages the pool of GPU nodes
type NodePool struct {
	db     *database.Database
	logger *zap.Logger
	nodes  sync.Map // map[uuid.UUID]*models.Node
	mu     sync.RWMutex
}

// NewNodePool creates a new node pool
func NewNodePool(db *database.Database, logger *zap.Logger) *NodePool {
	np := &NodePool{
		db:     db,
		logger: logger,
	}

	// Start background refresh
	go np.refreshLoop()

	return np
}

// RegisterNode registers a new node in the pool
func (np *NodePool) RegisterNode(ctx context.Context, node *models.Node) error {
	// Insert or update node in database
	_, err := np.db.Pool.Exec(ctx, `
		INSERT INTO nodes (
			id, node_id_external, provider, region_id, instance_type,
			gpu_type, vram_total_gb, vram_free_gb, model_id,
			endpoint_url, internal_ip, spot_instance, spot_price,
			throughput_tokens_per_sec, status, health_score,
			last_heartbeat_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17
		)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			health_score = EXCLUDED.health_score,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			vram_free_gb = EXCLUDED.vram_free_gb,
			throughput_tokens_per_sec = EXCLUDED.throughput_tokens_per_sec,
			updated_at = CURRENT_TIMESTAMP
	`,
		node.ID, node.NodeIDExternal, node.Provider, node.RegionID,
		node.InstanceType, node.GPUType, node.VRAMTotalGB, node.VRAMFreeGB,
		node.ModelID, node.EndpointURL, node.InternalIP, node.SpotInstance,
		node.SpotPrice, node.ThroughputTokensPerSec, node.Status,
		node.HealthScore, node.LastHeartbeatAt,
	)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Add to in-memory map
	np.nodes.Store(node.ID, node)

	np.logger.Info("registered node",
		zap.String("node_id", node.ID.String()),
		zap.String("provider", node.Provider),
		zap.String("endpoint", node.EndpointURL),
	)

	return nil
}

// GetNode retrieves a node by ID
func (np *NodePool) GetNode(nodeID uuid.UUID) (*models.Node, bool) {
	value, ok := np.nodes.Load(nodeID)
	if !ok {
		return nil, false
	}
	return value.(*models.Node), true
}

// GetActiveNodes returns all active nodes
func (np *NodePool) GetActiveNodes() []*models.Node {
	var nodes []*models.Node
	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)
		if node.Status == "active" {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// GetNodesByModel returns all active nodes for a specific model
func (np *NodePool) GetNodesByModel(modelID uuid.UUID) []*models.Node {
	var nodes []*models.Node
	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)
		if node.Status == "active" && node.ModelID != nil && *node.ModelID == modelID {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// GetNodesByRegion returns all active nodes in a specific region
func (np *NodePool) GetNodesByRegion(regionID uuid.UUID) []*models.Node {
	var nodes []*models.Node
	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)
		if node.Status == "active" && node.RegionID != nil && *node.RegionID == regionID {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// GetNodesByModelAndRegion returns all active nodes for a model in a region
func (np *NodePool) GetNodesByModelAndRegion(modelID, regionID uuid.UUID) []*models.Node {
	var nodes []*models.Node
	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)
		if node.Status == "active" &&
			node.ModelID != nil && *node.ModelID == modelID &&
			node.RegionID != nil && *node.RegionID == regionID {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// UpdateNodeStatus updates the status of a node
func (np *NodePool) UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error {
	_, err := np.db.Pool.Exec(ctx, `
		UPDATE nodes
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, status, nodeID)
	if err != nil {
		return err
	}

	// Update in-memory
	if value, ok := np.nodes.Load(nodeID); ok {
		node := value.(*models.Node)
		node.Status = status
		np.nodes.Store(nodeID, node)
	}

	np.logger.Info("updated node status",
		zap.String("node_id", nodeID.String()),
		zap.String("status", status),
	)

	return nil
}

// RecordHeartbeat records a heartbeat from a node
func (np *NodePool) RecordHeartbeat(ctx context.Context, nodeID uuid.UUID, healthScore float64) error {
	now := time.Now()

	_, err := np.db.Pool.Exec(ctx, `
		UPDATE nodes
		SET
			last_heartbeat_at = $1,
			health_score = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, now, healthScore, nodeID)
	if err != nil {
		return err
	}

	// Update in-memory
	if value, ok := np.nodes.Load(nodeID); ok {
		node := value.(*models.Node)
		node.LastHeartbeatAt = &now
		node.HealthScore = healthScore
		np.nodes.Store(nodeID, node)
	}

	return nil
}

// MarkDraining marks a node as draining (preparing for shutdown)
func (np *NodePool) MarkDraining(ctx context.Context, nodeID uuid.UUID) error {
	return np.UpdateNodeStatus(ctx, nodeID, "draining")
}

// MarkUnhealthy marks a node as unhealthy
func (np *NodePool) MarkUnhealthy(ctx context.Context, nodeID uuid.UUID) error {
	return np.UpdateNodeStatus(ctx, nodeID, "unhealthy")
}

// RemoveNode removes a node from the pool
func (np *NodePool) RemoveNode(ctx context.Context, nodeID uuid.UUID) error {
	now := time.Now()

	_, err := np.db.Pool.Exec(ctx, `
		UPDATE nodes
		SET
			status = 'dead',
			terminated_at = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, now, nodeID)
	if err != nil {
		return err
	}

	// Remove from in-memory map
	np.nodes.Delete(nodeID)

	np.logger.Info("removed node",
		zap.String("node_id", nodeID.String()),
	)

	return nil
}

// refreshLoop periodically refreshes the node pool from database
func (np *NodePool) refreshLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		np.refresh()
	}
}

// refresh loads active nodes from database
func (np *NodePool) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := np.db.Pool.Query(ctx, `
		SELECT
			id, node_id_external, provider, region_id, instance_type,
			gpu_type, vram_total_gb, vram_free_gb, model_id,
			endpoint_url, internal_ip, spot_instance, spot_price,
			throughput_tokens_per_sec, status, health_score,
			last_heartbeat_at, created_at, updated_at
		FROM nodes
		WHERE status IN ('active', 'draining', 'initializing')
		AND (terminated_at IS NULL OR terminated_at > CURRENT_TIMESTAMP - INTERVAL '1 hour')
	`)
	if err != nil {
		np.logger.Error("failed to refresh node pool", zap.Error(err))
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var node models.Node
		err := rows.Scan(
			&node.ID, &node.NodeIDExternal, &node.Provider, &node.RegionID,
			&node.InstanceType, &node.GPUType, &node.VRAMTotalGB,
			&node.VRAMFreeGB, &node.ModelID, &node.EndpointURL,
			&node.InternalIP, &node.SpotInstance, &node.SpotPrice,
			&node.ThroughputTokensPerSec, &node.Status, &node.HealthScore,
			&node.LastHeartbeatAt, &node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			np.logger.Warn("failed to scan node", zap.Error(err))
			continue
		}

		np.nodes.Store(node.ID, &node)
		count++
	}

	np.logger.Debug("refreshed node pool", zap.Int("count", count))
}

// CheckStaleNodes checks for nodes that haven't sent heartbeat recently
func (np *NodePool) CheckStaleNodes(ctx context.Context) {
	threshold := time.Now().Add(-2 * time.Minute)

	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)

		if node.LastHeartbeatAt != nil && node.LastHeartbeatAt.Before(threshold) {
			np.logger.Warn("stale node detected",
				zap.String("node_id", node.ID.String()),
				zap.Time("last_heartbeat", *node.LastHeartbeatAt),
			)

			// Mark as unhealthy
			np.MarkUnhealthy(ctx, node.ID)
		}

		return true
	})
}

// GetNodeCount returns the count of nodes by status
func (np *NodePool) GetNodeCount() map[string]int {
	counts := make(map[string]int)

	np.nodes.Range(func(key, value interface{}) bool {
		node := value.(*models.Node)
		counts[node.Status]++
		return true
	})

	return counts
}
