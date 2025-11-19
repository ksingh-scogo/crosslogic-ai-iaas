package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Phase 3: Cost Metrics
	TenantCostTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_total_usd",
			Help: "Total cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	TenantCostCompute = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_compute_usd",
			Help: "Compute cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	TenantCostToken = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_cost_token_usd",
			Help: "Token cost per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	TenantSavings = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tenant_savings_usd",
			Help: "Savings from spot instances per tenant in USD",
		},
		[]string{"tenant_id"},
	)

	SpotUsagePercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "spot_usage_percent",
			Help: "Percentage of compute running on spot instances",
		},
		[]string{"tenant_id"},
	)

	// Phase 3: Performance Metrics
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vllm_queue_depth",
			Help: "Number of requests waiting in vLLM queue",
		},
		[]string{"node_id", "model_name"},
	)

	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vllm_active_requests",
			Help: "Number of requests currently being processed",
		},
		[]string{"node_id", "model_name"},
	)
)

// UpdateCostMetrics updates cost metrics for a tenant
func UpdateCostMetrics(tenantID string, totalCost, computeCost, tokenCost, savings, spotPercent float64) {
	TenantCostTotal.WithLabelValues(tenantID).Set(totalCost)
	TenantCostCompute.WithLabelValues(tenantID).Set(computeCost)
	TenantCostToken.WithLabelValues(tenantID).Set(tokenCost)
	TenantSavings.WithLabelValues(tenantID).Set(savings)
	SpotUsagePercent.WithLabelValues(tenantID).Set(spotPercent)
}

// UpdateQueueMetrics updates queue depth and active requests
func UpdateQueueMetrics(nodeID, modelName string, depth, active int64) {
	QueueDepth.WithLabelValues(nodeID, modelName).Set(float64(depth))
	ActiveRequests.WithLabelValues(nodeID, modelName).Set(float64(active))
}
