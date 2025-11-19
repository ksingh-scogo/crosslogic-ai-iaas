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

// GetAverageLatency returns the average latency for a model across all healthy nodes.
func (lb *IntelligentLoadBalancer) GetAverageLatency(ctx context.Context, modelName string) (time.Duration, error) {
	nodes, err := lb.getHealthyNodes(ctx, modelName)
	if err != nil {
		return 0, err
	}

	if len(nodes) == 0 {
		return 0, nil
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var totalLatency time.Duration
	var count int

	for _, node := range nodes {
		if stats, ok := lb.stats[node]; ok && stats.Latency > 0 {
			totalLatency += stats.Latency
			count++
		}
	}

	if count == 0 {
		return 0, nil
	}

	return time.Duration(int64(totalLatency) / int64(count)), nil
}

// SelectEndpoint chooses the best available endpoint for a model.
//
// Strategy: Weighted Score (Latency + Reliability)
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

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Calculate scores
	type nodeScore struct {
		node  string
		score float64
	}
	var scores []nodeScore

	for _, node := range nodes {
		stats, ok := lb.stats[node]
		if !ok {
			// No stats yet, give it a high default score to encourage exploration
			// Score = 1.0 (equivalent to 0ms latency and 0 errors in our formula roughly)
			scores = append(scores, nodeScore{node: node, score: 2.0})
			continue
		}

		// Latency score: 1.0 / (latency_ms + 1)
		// Example: 100ms -> 0.0099
		// Example: 10ms -> 0.09
		latencyMs := float64(stats.Latency.Milliseconds())
		latencyScore := 1.0 / (latencyMs + 1.0)

		// Error score: 1.0 / (error_count + 1)
		errorScore := 1.0 / (float64(stats.ErrorCount) + 1.0)

		// Combined score (weighted)
		// 60% Latency, 40% Reliability
		finalScore := (latencyScore * 0.6) + (errorScore * 0.4)

		scores = append(scores, nodeScore{node: node, score: finalScore})
	}

	// Pick the highest score
	var bestNode string
	var maxScore float64 = -1.0

	for _, ns := range scores {
		if ns.score > maxScore {
			maxScore = ns.score
			bestNode = ns.node
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
