package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/crosslogic/control-plane/pkg/cache"
	"github.com/crosslogic/control-plane/pkg/database"
	"github.com/crosslogic/control-plane/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Scheduler handles request scheduling to GPU nodes
type Scheduler struct {
	db       *database.Database
	logger   *zap.Logger
	nodePool *NodePool
	strategy SchedulingStrategy
	load     *NodeLoadTracker
}

// SchedulingStrategy defines how nodes are selected
type SchedulingStrategy interface {
	SelectNode(ctx context.Context, nodes []*models.Node) (*models.Node, error)
}

// NewScheduler creates a new scheduler
func NewScheduler(db *database.Database, cache *cache.Cache, logger *zap.Logger) *Scheduler {
	loadTracker := NewNodeLoadTracker(cache, logger)
	return &Scheduler{
		db:       db,
		logger:   logger,
		nodePool: NewNodePool(db, logger),
		strategy: NewLeastLoadedStrategy(loadTracker, logger),
		load:     loadTracker,
	}
}

// ScheduleRequest schedules a request to an appropriate node
func (s *Scheduler) ScheduleRequest(ctx context.Context, req *ScheduleRequest) (*models.Node, error) {
	// Find model ID by name
	var modelID uuid.UUID
	err := s.db.Pool.QueryRow(ctx, `
		SELECT id FROM models WHERE name = $1 AND status = 'active'
	`, req.Model).Scan(&modelID)
	if err != nil {
		return nil, fmt.Errorf("model not found: %s", req.Model)
	}

	// Find region ID by code (if specified)
	var regionID *uuid.UUID
	if req.Region != "" {
		var rid uuid.UUID
		err := s.db.Pool.QueryRow(ctx, `
			SELECT id FROM regions WHERE code = $1 AND status = 'active'
		`, req.Region).Scan(&rid)
		if err != nil {
			s.logger.Warn("region not found, will use any available",
				zap.String("region", req.Region),
			)
		} else {
			regionID = &rid
		}
	}

	// Get candidate nodes
	var nodes []*models.Node
	if regionID != nil {
		nodes = s.nodePool.GetNodesByModelAndRegion(modelID, *regionID)
	} else {
		nodes = s.nodePool.GetNodesByModel(modelID)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available for model %s in region %s", req.Model, req.Region)
	}

	// Filter healthy nodes
	healthyNodes := filterHealthyNodes(nodes)
	if len(healthyNodes) == 0 {
		return nil, fmt.Errorf("no healthy nodes available")
	}

	// Select best node using strategy
	selectedNode, err := s.strategy.SelectNode(ctx, healthyNodes)
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	s.logger.Info("scheduled request",
		zap.String("model", req.Model),
		zap.String("node_id", selectedNode.ID.String()),
		zap.String("endpoint", selectedNode.EndpointURL),
	)

	return selectedNode, nil
}

// TrackRequestStart increments the in-flight counters for the target node.
func (s *Scheduler) TrackRequestStart(nodeID uuid.UUID, estimatedTokens int) {
	if s.load == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.load.Increment(ctx, nodeID, estimatedTokens)
}

// TrackRequestEnd decrements the in-flight counters for the target node.
func (s *Scheduler) TrackRequestEnd(nodeID uuid.UUID, estimatedTokens int) {
	if s.load == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.load.Decrement(ctx, nodeID, estimatedTokens)
}

// GetNodePool returns the node pool
func (s *Scheduler) GetNodePool() *NodePool {
	return s.nodePool
}

// ScheduleRequest represents a scheduling request
type ScheduleRequest struct {
	Model    string
	Region   string
	TenantID uuid.UUID
	EnvID    uuid.UUID
	Reserved bool
}

// filterHealthyNodes filters nodes with good health scores
func filterHealthyNodes(nodes []*models.Node) []*models.Node {
	var healthy []*models.Node
	for _, node := range nodes {
		if node.HealthScore >= 80.0 { // At least 80% health
			healthy = append(healthy, node)
		}
	}
	return healthy
}

// ========== Scheduling Strategies ==========

// LeastLoadedStrategy selects the node with the lowest concurrency/pending tokens
type LeastLoadedStrategy struct {
	tracker *NodeLoadTracker
	logger  *zap.Logger
}

// NewLeastLoadedStrategy constructs the strategy with an optional load tracker.
func NewLeastLoadedStrategy(tracker *NodeLoadTracker, logger *zap.Logger) *LeastLoadedStrategy {
	return &LeastLoadedStrategy{
		tracker: tracker,
		logger:  logger,
	}
}

func (s *LeastLoadedStrategy) SelectNode(ctx context.Context, nodes []*models.Node) (*models.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	if s.tracker == nil {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].HealthScore > nodes[j].HealthScore
		})
		return nodes[0], nil
	}

	type candidate struct {
		node    *models.Node
		active  int64
		pending int64
	}

	candidates := make([]candidate, 0, len(nodes))
	for _, node := range nodes {
		active, pending, err := s.tracker.GetLoad(ctx, node.ID)
		if err != nil && s.logger != nil {
			s.logger.Warn("failed to read node load stats",
				zap.Error(err),
				zap.String("node_id", node.ID.String()),
			)
		}
		candidates = append(candidates, candidate{
			node:    node,
			active:  active,
			pending: pending,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].active == candidates[j].active {
			if candidates[i].pending == candidates[j].pending {
				return candidates[i].node.HealthScore > candidates[j].node.HealthScore
			}
			return candidates[i].pending < candidates[j].pending
		}
		return candidates[i].active < candidates[j].active
	})

	return candidates[0].node, nil
}

// RoundRobinStrategy selects nodes in round-robin fashion
type RoundRobinStrategy struct {
	counter int
}

func (s *RoundRobinStrategy) SelectNode(_ context.Context, nodes []*models.Node) (*models.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	node := nodes[s.counter%len(nodes)]
	s.counter++
	return node, nil
}

// RandomStrategy selects a random node
type RandomStrategy struct{}

func (s *RandomStrategy) SelectNode(_ context.Context, nodes []*models.Node) (*models.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	return nodes[rand.Intn(len(nodes))], nil
}

// WeightedStrategy selects nodes based on health score weighting
type WeightedStrategy struct{}

func (s *WeightedStrategy) SelectNode(_ context.Context, nodes []*models.Node) (*models.Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, node := range nodes {
		totalWeight += node.HealthScore
	}

	// Random selection weighted by health score
	r := rand.Float64() * totalWeight
	cumulative := 0.0

	for _, node := range nodes {
		cumulative += node.HealthScore
		if r <= cumulative {
			return node, nil
		}
	}

	// Fallback to first node
	return nodes[0], nil
}
