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

