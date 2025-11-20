package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/pkg/database"
	pkgmetrics "github.com/crosslogic/control-plane/pkg/metrics"
	"go.uber.org/zap"
)

// EndpointStats tracks metrics for a specific endpoint.
type EndpointStats struct {
	Latency      time.Duration
	RequestCount int64
	ErrorCount   int64
	QueueDepth   int64 // Number of requests waiting in vLLM queue
	ActiveRequests int64 // Number of requests currently being processed
	LastUpdated  time.Time
}

// VLLMMetrics represents metrics from vLLM's metrics endpoint
type VLLMMetrics struct {
	NumRequestsRunning int64 `json:"num_requests_running"`
	NumRequestsWaiting int64 `json:"num_requests_waiting"`
}

// IntelligentLoadBalancer distributes traffic across healthy nodes.
type IntelligentLoadBalancer struct {
	db         *database.Database
	logger     *zap.Logger
	stats      map[string]*EndpointStats // Key: endpoint URL
	mu         sync.RWMutex
	httpClient *http.Client
	stopChan   chan struct{}
}

// NewIntelligentLoadBalancer creates a new load balancer.
func NewIntelligentLoadBalancer(db *database.Database, logger *zap.Logger) *IntelligentLoadBalancer {
	return &IntelligentLoadBalancer{
		db:     db,
		logger: logger,
		stats:  make(map[string]*EndpointStats),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		stopChan: make(chan struct{}),
	}
}

// StartQueueMonitoring begins background queue depth monitoring
func (lb *IntelligentLoadBalancer) StartQueueMonitoring(ctx context.Context) {
	lb.logger.Info("starting queue depth monitoring")
	go lb.queueMonitoringLoop(ctx)
}

// Stop gracefully stops the load balancer
func (lb *IntelligentLoadBalancer) Stop() {
	close(lb.stopChan)
}

// queueMonitoringLoop periodically polls vLLM endpoints for queue depth
func (lb *IntelligentLoadBalancer) queueMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lb.stopChan:
			return
		case <-ticker.C:
			lb.updateAllQueueDepths(ctx)
		}
	}
}

// updateAllQueueDepths updates queue depth for all active nodes
func (lb *IntelligentLoadBalancer) updateAllQueueDepths(ctx context.Context) {
	// Get all active nodes
	query := `SELECT endpoint FROM nodes WHERE status = 'active' AND endpoint != ''`
	rows, err := lb.db.Pool.Query(ctx, query)
	if err != nil {
		lb.logger.Error("failed to fetch active nodes for queue monitoring", zap.Error(err))
		return
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

	// Poll each endpoint for metrics
	var wg sync.WaitGroup
	for _, endpoint := range endpoints {
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			lb.updateQueueDepth(ep)
		}(endpoint)
	}
	wg.Wait()
}

// updateQueueDepth polls a single endpoint for queue depth metrics
func (lb *IntelligentLoadBalancer) updateQueueDepth(endpoint string) {
	// vLLM exposes metrics at /metrics or /v1/metrics
	metricsURL := endpoint + "/metrics"

	req, err := http.NewRequest("GET", metricsURL, nil)
	if err != nil {
		lb.logger.Debug("failed to create metrics request",
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		return
	}

	resp, err := lb.httpClient.Do(req)
	if err != nil {
		// Endpoint might not be reachable, skip silently
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var metrics VLLMMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		// Metrics might be in Prometheus format, try parsing that
		// For now, skip if not JSON
		return
	}

	// Update stats with queue depth
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats, ok := lb.stats[endpoint]
	if !ok {
		stats = &EndpointStats{}
		lb.stats[endpoint] = stats
	}

	stats.QueueDepth = metrics.NumRequestsWaiting
	stats.ActiveRequests = metrics.NumRequestsRunning
	stats.LastUpdated = time.Now()

	// Update Prometheus metrics
	// Get model name for this endpoint
	modelName := lb.getModelNameForEndpoint(endpoint)
	nodeID := lb.getNodeIDForEndpoint(endpoint)

	if modelName != "" && nodeID != "" {
		pkgmetrics.UpdateQueueMetrics(nodeID, modelName, stats.QueueDepth, stats.ActiveRequests)
	}
}

// getModelNameForEndpoint retrieves the model name for an endpoint
func (lb *IntelligentLoadBalancer) getModelNameForEndpoint(endpoint string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var modelName string
	err := lb.db.Pool.QueryRow(ctx,
		"SELECT model_name FROM nodes WHERE endpoint = $1 LIMIT 1",
		endpoint,
	).Scan(&modelName)

	if err != nil {
		return ""
	}
	return modelName
}

// getNodeIDForEndpoint retrieves the node ID for an endpoint
func (lb *IntelligentLoadBalancer) getNodeIDForEndpoint(endpoint string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var nodeID string
	err := lb.db.Pool.QueryRow(ctx,
		"SELECT id FROM nodes WHERE endpoint = $1 LIMIT 1",
		endpoint,
	).Scan(&nodeID)

	if err != nil {
		return ""
	}
	return nodeID
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
// Strategy: Weighted Score (Latency + Reliability + Queue Depth)
// - Filters for healthy nodes serving the model
// - Prefers nodes with lower latency, error rates, and queue depth
// - Weights: 40% Latency, 30% Queue Depth, 30% Reliability
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
		node         string
		score        float64
		queueDepth   int64
		latencyMs    float64
		errorRate    float64
	}
	var scores []nodeScore

	for _, node := range nodes {
		stats, ok := lb.stats[node]
		if !ok {
			// No stats yet, give it a high default score to encourage exploration
			scores = append(scores, nodeScore{
				node:       node,
				score:      2.0,
				queueDepth: 0,
				latencyMs:  0,
				errorRate:  0,
			})
			continue
		}

		// Latency score: 1.0 / (latency_ms + 1)
		// Lower latency = higher score
		// Example: 100ms -> 0.0099, 10ms -> 0.09, 1ms -> 0.5
		latencyMs := float64(stats.Latency.Milliseconds())
		latencyScore := 1.0 / (latencyMs + 1.0)

		// Queue depth score: 1.0 / (queue_depth + 1)
		// Lower queue = higher score
		// Example: 0 waiting -> 1.0, 10 waiting -> 0.09, 100 waiting -> 0.0099
		queueScore := 1.0 / (float64(stats.QueueDepth) + 1.0)

		// Error rate score: 1.0 / (error_count + 1)
		// Lower errors = higher score
		errorScore := 1.0 / (float64(stats.ErrorCount) + 1.0)

		// Calculate error rate for logging
		errorRate := 0.0
		if stats.RequestCount > 0 {
			errorRate = float64(stats.ErrorCount) / float64(stats.RequestCount) * 100
		}

		// Combined score (weighted)
		// 40% Latency - Response time matters most for user experience
		// 30% Queue Depth - Avoid overloaded nodes to prevent cascading delays
		// 30% Reliability - Prefer nodes with fewer errors
		finalScore := (latencyScore * 0.4) + (queueScore * 0.3) + (errorScore * 0.3)

		scores = append(scores, nodeScore{
			node:       node,
			score:      finalScore,
			queueDepth: stats.QueueDepth,
			latencyMs:  latencyMs,
			errorRate:  errorRate,
		})
	}

	// Pick the highest score
	var bestNode string
	var maxScore float64 = -1.0
	var bestStats nodeScore

	for _, ns := range scores {
		if ns.score > maxScore {
			maxScore = ns.score
			bestNode = ns.node
			bestStats = ns
		}
	}

	// Log selection for observability
	lb.logger.Debug("selected endpoint",
		zap.String("model", modelName),
		zap.String("endpoint", bestNode),
		zap.Float64("score", bestStats.score),
		zap.Int64("queue_depth", bestStats.queueDepth),
		zap.Float64("latency_ms", bestStats.latencyMs),
		zap.Float64("error_rate", bestStats.errorRate),
	)

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
