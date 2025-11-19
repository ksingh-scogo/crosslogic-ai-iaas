package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	"go.uber.org/zap"
)

// EndpointStats tracks metrics for a specific endpoint.
type EndpointStats struct {
	Latency      time.Duration
	RequestCount int64
	ErrorCount   int64
	LastUpdated  time.Time
}

// IntelligentLoadBalancer distributes traffic across healthy nodes.
type IntelligentLoadBalancer struct {
	db     *database.Database
	logger *zap.Logger
	stats  map[string]*EndpointStats // Key: endpoint URL
	mu     sync.RWMutex
}

// NewIntelligentLoadBalancer creates a new load balancer.
func NewIntelligentLoadBalancer(db *database.Database, logger *zap.Logger) *IntelligentLoadBalancer {
	return &IntelligentLoadBalancer{
		db:     db,
		logger: logger,
		stats:  make(map[string]*EndpointStats),
	}
}

// SelectEndpoint chooses the best available endpoint for a model.
//
// Strategy: Weighted Round Robin (simplified for now)
// - Filters for healthy nodes serving the model
// - Prefers nodes with lower latency and error rates
func (lb *IntelligentLoadBalancer) SelectEndpoint(ctx context.Context, modelName string) (string, error) {
	// Get active nodes for model
	nodes, err := lb.getHealthyNodes(ctx, modelName)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		return "", nil // No nodes available
	}

	// Simple selection: Pick the one with lowest latency
	// TODO: Implement full weighted round robin
	bestNode := nodes[0]
	minLatency := time.Hour // Start high

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	for _, node := range nodes {
		stats, ok := lb.stats[node]
		if !ok {
			// No stats yet, treat as good candidate
			return node, nil
		}

		if stats.Latency < minLatency {
			minLatency = stats.Latency
			bestNode = node
		}
	}

	return bestNode, nil
}

// RecordRequest updates stats for an endpoint after a request.
func (lb *IntelligentLoadBalancer) RecordRequest(endpoint string, latency time.Duration, isError bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats, ok := lb.stats[endpoint]
	if !ok {
		stats = &EndpointStats{}
		lb.stats[endpoint] = stats
	}

	// Exponential moving average for latency
	if stats.Latency == 0 {
		stats.Latency = latency
	} else {
		stats.Latency = time.Duration(float64(stats.Latency)*0.8 + float64(latency)*0.2)
	}

	stats.RequestCount++
	if isError {
		stats.ErrorCount++
	}
	stats.LastUpdated = time.Now()
}

func (lb *IntelligentLoadBalancer) getHealthyNodes(ctx context.Context, modelName string) ([]string, error) {
	query := `
		SELECT endpoint FROM nodes
		WHERE model_name = $1 AND status = 'active' AND endpoint != ''
	`
	rows, err := lb.db.Pool.Query(ctx, query, modelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []string
	for rows.Next() {
		var endpoint string
		if err := rows.Scan(&endpoint); err != nil {
			continue
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}
