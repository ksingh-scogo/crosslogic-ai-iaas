package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status", "tenant_id"},
	)

	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of currently active HTTP connections",
		},
	)

	vllmProxyErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vllm_proxy_errors_total",
			Help: "Total number of vLLM proxy errors",
		},
		[]string{"node_id", "error_type"},
	)

	dependencyUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dependency_up",
			Help: "Status of dependencies (1 = up, 0 = down)",
		},
		[]string{"service"},
	)

	// Phase 3: Cost Metrics
	tenantCostTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_total_usd",
			Help: "Total cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	tenantCostCompute = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_compute_usd",
			Help: "Compute cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	tenantCostToken = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_token_usd",
			Help: "Token cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	tenantSavings = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_savings_usd",
			Help: "Savings from spot instances per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	spotUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spot_usage_percent",
			Help: "Percentage of compute running on spot instances",
		},
		[]string{"tenant_id"},
	)

	// Phase 3: Performance Metrics
	modelLoadingTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "model_loading_time_seconds",
			Help:    "Time taken to load a model",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300}, // 1s to 5min
		},
		[]string{"model_name", "cache_hit"},
	)

	cacheHitRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "juicefs_cache_hit_rate",
			Help: "JuiceFS cache hit rate (0-1)",
		},
		[]string{"model_name"},
	)

	queueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vllm_queue_depth",
			Help: "Number of requests waiting in vLLM queue",
		},
		[]string{"node_id", "model_name"},
	)

	activeRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vllm_active_requests",
			Help: "Number of requests currently being processed",
		},
		[]string{"node_id", "model_name"},
	)

	inferenceLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "inference_latency_seconds",
			Help:    "End-to-end inference latency",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30}, // 100ms to 30s
		},
		[]string{"model_name", "tenant_id"},
	)

	tokensPerSecond = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tokens_per_second",
			Help:    "Token generation throughput",
			Buckets: []float64{10, 25, 50, 100, 200, 500, 1000},
		},
		[]string{"model_name", "tenant_id"},
	)

	// Phase 3: GPU Metrics
	gpuUtilization = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_utilization_percent",
			Help: "GPU utilization percentage",
		},
		[]string{"node_id", "gpu_type"},
	)

	gpuMemoryUsed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_memory_used_bytes",
			Help: "GPU memory used in bytes",
		},
		[]string{"node_id", "gpu_type"},
	)

	// Phase 3: Node Health Metrics
	nodeHealthScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_health_score",
			Help: "Node health score (0-100)",
		},
		[]string{"node_id", "model_name"},
	)

	nodeStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_status",
			Help: "Node status (1=active, 0=inactive)",
		},
		[]string{"node_id", "status"},
	)

	// Phase 3: Load Balancer Metrics
	loadBalancerScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "load_balancer_score",
			Help: "Load balancer selection score for a node",
		},
		[]string{"node_id", "model_name"},
	)
)

// metricsMiddleware returns a middleware that records HTTP metrics
func (g *Gateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		activeConnections.Inc()
		defer activeConnections.Dec()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())
		
		// Extract tenant ID from context if available, otherwise "anonymous"
		tenantID := "anonymous"
		if tid, ok := r.Context().Value("tenant_id").(string); ok {
			tenantID = tid
		} else if tidUUID, ok := r.Context().Value("tenant_id").(interface{ String() string }); ok {
			tenantID = tidUUID.String()
		}

		// Use low cardinality path (e.g. /v1/chat/completions instead of /v1/chat/completions/123)
		// Chi router context can provide the route pattern
		routePath := r.URL.Path
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if pattern := rctx.RoutePattern(); pattern != "" {
				routePath = pattern
			}
		}

		httpRequestsTotal.WithLabelValues(r.Method, routePath, status, tenantID).Inc()
		httpRequestDuration.WithLabelValues(r.Method, routePath, status, tenantID).Observe(duration)
	})
}

// registerMetrics registers the /metrics endpoint
func (g *Gateway) registerMetrics() {
	g.router.Handle("/metrics", promhttp.Handler())
}

// Phase 3: Metric Helper Functions

// UpdateCostMetrics updates cost metrics for a tenant
func UpdateCostMetrics(tenantID string, totalCost, computeCost, tokenCost, savings, spotPercent float64) {
	tenantCostTotal.WithLabelValues(tenantID).Set(totalCost)
	tenantCostCompute.WithLabelValues(tenantID).Set(computeCost)
	tenantCostToken.WithLabelValues(tenantID).Set(tokenCost)
	tenantSavings.WithLabelValues(tenantID).Set(savings)
	spotUsagePercent.WithLabelValues(tenantID).Set(spotPercent)
}

// RecordModelLoadingTime records how long it took to load a model
func RecordModelLoadingTime(modelName string, duration float64, cacheHit bool) {
	cacheStatus := "false"
	if cacheHit {
		cacheStatus = "true"
	}
	modelLoadingTime.WithLabelValues(modelName, cacheStatus).Observe(duration)
}

// UpdateCacheHitRate updates the cache hit rate for a model
func UpdateCacheHitRate(modelName string, hitRate float64) {
	cacheHitRate.WithLabelValues(modelName).Set(hitRate)
}

// UpdateQueueMetrics updates queue depth and active requests
func UpdateQueueMetrics(nodeID, modelName string, depth, active int64) {
	queueDepth.WithLabelValues(nodeID, modelName).Set(float64(depth))
	activeRequests.WithLabelValues(nodeID, modelName).Set(float64(active))
}

// RecordInferenceLatency records end-to-end inference latency
func RecordInferenceLatency(modelName, tenantID string, latencySeconds float64) {
	inferenceLatency.WithLabelValues(modelName, tenantID).Observe(latencySeconds)
}

// RecordTokenThroughput records token generation throughput
func RecordTokenThroughput(modelName, tenantID string, tokensPerSec float64) {
	tokensPerSecond.WithLabelValues(modelName, tenantID).Observe(tokensPerSec)
}

// UpdateGPUMetrics updates GPU utilization and memory metrics
func UpdateGPUMetrics(nodeID, gpuType string, utilizationPercent float64, memoryUsedBytes int64) {
	gpuUtilization.WithLabelValues(nodeID, gpuType).Set(utilizationPercent)
	gpuMemoryUsed.WithLabelValues(nodeID, gpuType).Set(float64(memoryUsedBytes))
}

// UpdateNodeHealthMetrics updates node health score
func UpdateNodeHealthMetrics(nodeID, modelName string, healthScore float64) {
	nodeHealthScore.WithLabelValues(nodeID, modelName).Set(healthScore)
}

// UpdateNodeStatus updates node status (1=active, 0=inactive)
func UpdateNodeStatus(nodeID, status string) {
	// Reset all status labels for this node
	nodeStatus.WithLabelValues(nodeID, "active").Set(0)
	nodeStatus.WithLabelValues(nodeID, "inactive").Set(0)
	nodeStatus.WithLabelValues(nodeID, "provisioning").Set(0)
	nodeStatus.WithLabelValues(nodeID, "dead").Set(0)
	nodeStatus.WithLabelValues(nodeID, "draining").Set(0)
	nodeStatus.WithLabelValues(nodeID, "suspect").Set(0)
	nodeStatus.WithLabelValues(nodeID, "degraded").Set(0)

	// Set the current status to 1
	nodeStatus.WithLabelValues(nodeID, status).Set(1)
}

// UpdateLoadBalancerScore updates the load balancer selection score for a node
func UpdateLoadBalancerScore(nodeID, modelName string, score float64) {
	loadBalancerScore.WithLabelValues(nodeID, modelName).Set(score)
}

