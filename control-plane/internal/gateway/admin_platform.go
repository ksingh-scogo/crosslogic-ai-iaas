package gateway

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// handlePlatformHealth returns overall platform health status
// Platform Admin Only - GET /admin/platform/health
func (g *Gateway) handlePlatformHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check control plane health
	controlPlaneStatus := "healthy"

	// Check database health
	dbStatus := "healthy"
	if err := g.db.Health(ctx); err != nil {
		dbStatus = "unhealthy"
		controlPlaneStatus = "degraded"
		g.logger.Error("database health check failed", zap.Error(err))
	}

	// Check cache health
	cacheStatus := "healthy"
	if err := g.cache.Health(ctx); err != nil {
		cacheStatus = "unhealthy"
		controlPlaneStatus = "degraded"
		g.logger.Error("cache health check failed", zap.Error(err))
	}

	// Check GPU nodes health
	var totalNodes, healthyNodes, unhealthyNodes int
	err := g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'active' AND health_score >= 70) as healthy,
			COUNT(*) FILTER (WHERE status IN ('unhealthy', 'dead') OR health_score < 50) as unhealthy
		FROM nodes
	`).Scan(&totalNodes, &healthyNodes, &unhealthyNodes)

	gpuNodesStatus := "healthy"
	if err != nil {
		g.logger.Error("failed to query node health", zap.Error(err))
		gpuNodesStatus = "unknown"
	} else if totalNodes == 0 {
		gpuNodesStatus = "no_nodes"
	} else if unhealthyNodes > 0 {
		gpuNodesStatus = "degraded"
		if unhealthyNodes > totalNodes/2 {
			gpuNodesStatus = "unhealthy"
		}
	}

	// Calculate total capacity
	var totalCapacityTPS *int
	g.db.Pool.QueryRow(ctx, `
		SELECT SUM(m.tokens_per_second_capacity)
		FROM nodes n
		INNER JOIN models m ON m.id = n.model_id
		WHERE n.status = 'active' AND n.health_score >= 70
	`).Scan(&totalCapacityTPS)

	// Calculate current RPS
	var currentRPS *float64
	g.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::float / 60.0
		FROM usage_records
		WHERE timestamp > NOW() - INTERVAL '1 minute'
	`).Scan(&currentRPS)

	// Calculate average latency
	var avgLatency *float64
	g.db.Pool.QueryRow(ctx, `
		SELECT AVG(latency_ms)
		FROM usage_records
		WHERE timestamp > NOW() - INTERVAL '5 minutes'
	`).Scan(&avgLatency)

	// Determine overall status
	overallStatus := "healthy"
	if controlPlaneStatus != "healthy" || gpuNodesStatus == "unhealthy" {
		overallStatus = "unhealthy"
	} else if controlPlaneStatus == "degraded" || gpuNodesStatus == "degraded" {
		overallStatus = "degraded"
	}

	healthResponse := map[string]interface{}{
		"status": overallStatus,
		"components": map[string]string{
			"control_plane": controlPlaneStatus,
			"database":      dbStatus,
			"cache":         cacheStatus,
			"gpu_nodes":     gpuNodesStatus,
		},
		"metrics": map[string]interface{}{
			"total_nodes":      totalNodes,
			"healthy_nodes":    healthyNodes,
			"unhealthy_nodes":  unhealthyNodes,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if totalCapacityTPS != nil {
		healthResponse["metrics"].(map[string]interface{})["total_capacity_tps"] = *totalCapacityTPS
	}

	if currentRPS != nil {
		healthResponse["metrics"].(map[string]interface{})["current_rps"] = *currentRPS
	}

	if avgLatency != nil {
		healthResponse["metrics"].(map[string]interface{})["avg_latency_ms"] = *avgLatency
	}

	g.writeJSON(w, http.StatusOK, healthResponse)
}

// handlePlatformMetrics returns platform-wide metrics
// Platform Admin Only - GET /admin/platform/metrics
func (g *Gateway) handlePlatformMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	startDate := calculateStartDate(period)
	endDate := time.Now()

	// Request metrics
	var totalRequests int64
	var successRequests, errorRequests int64
	g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE NOT billing_failed),
			COUNT(*) FILTER (WHERE billing_failed)
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
	`, startDate, endDate).Scan(&totalRequests, &successRequests, &errorRequests)

	// Token metrics
	var totalTokens, promptTokens, completionTokens int64
	g.db.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0)
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
	`, startDate, endDate).Scan(&totalTokens, &promptTokens, &completionTokens)

	// Latency metrics
	var avgLatency, p50, p95, p99 *float64
	g.db.Pool.QueryRow(ctx, `
		SELECT
			AVG(latency_ms),
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency_ms),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms),
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms)
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
	`, startDate, endDate).Scan(&avgLatency, &p50, &p95, &p99)

	// Cost metrics
	var totalCostMicrodollars int64
	g.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_microdollars), 0)
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
	`, startDate, endDate).Scan(&totalCostMicrodollars)

	// Active tenants
	var activeTenants int64
	g.db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT tenant_id)
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
	`, startDate, endDate).Scan(&activeTenants)

	// Models usage
	modelRows, err := g.db.Pool.Query(ctx, `
		SELECT
			m.name,
			COUNT(*) as requests,
			SUM(ur.total_tokens) as tokens,
			AVG(ur.latency_ms) as avg_latency
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.timestamp >= $1 AND ur.timestamp <= $2
		GROUP BY m.name
		ORDER BY requests DESC
		LIMIT 10
	`, startDate, endDate)

	var modelStats []map[string]interface{}
	if err == nil {
		defer modelRows.Close()
		for modelRows.Next() {
			var modelName string
			var requests, tokens int64
			var avgLatency float64

			if err := modelRows.Scan(&modelName, &requests, &tokens, &avgLatency); err == nil {
				modelStats = append(modelStats, map[string]interface{}{
					"model_name":     modelName,
					"requests":       requests,
					"tokens":         tokens,
					"avg_latency_ms": avgLatency,
				})
			}
		}
	}

	// Resource utilization
	var activeNodes, drainingNodes, unhealthyNodes int
	g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'draining'),
			COUNT(*) FILTER (WHERE status IN ('unhealthy', 'dead'))
		FROM nodes
	`).Scan(&activeNodes, &drainingNodes, &unhealthyNodes)

	// Time series data for requests
	timeSeriesRows, err := g.db.Pool.Query(ctx, `
		SELECT
			DATE_TRUNC('hour', timestamp) as hour,
			COUNT(*) as requests,
			SUM(total_tokens) as tokens
		FROM usage_records
		WHERE timestamp >= $1 AND timestamp <= $2
		GROUP BY hour
		ORDER BY hour
	`, startDate, endDate)

	var timeSeries []map[string]interface{}
	if err == nil {
		defer timeSeriesRows.Close()
		for timeSeriesRows.Next() {
			var hour time.Time
			var requests, tokens int64

			if err := timeSeriesRows.Scan(&hour, &requests, &tokens); err == nil {
				timeSeries = append(timeSeries, map[string]interface{}{
					"timestamp": hour,
					"requests":  requests,
					"tokens":    tokens,
				})
			}
		}
	}

	metrics := map[string]interface{}{
		"period":        period,
		"start_date":    startDate,
		"end_date":      endDate,
		"requests": map[string]interface{}{
			"total":   totalRequests,
			"success": successRequests,
			"errors":  errorRequests,
		},
		"tokens": map[string]interface{}{
			"total":      totalTokens,
			"prompt":     promptTokens,
			"completion": completionTokens,
		},
		"latency": map[string]interface{}{},
		"cost": map[string]interface{}{
			"total_usd": float64(totalCostMicrodollars) / 1_000_000.0,
		},
		"tenants": map[string]interface{}{
			"active": activeTenants,
		},
		"resources": map[string]interface{}{
			"active_nodes":    activeNodes,
			"draining_nodes":  drainingNodes,
			"unhealthy_nodes": unhealthyNodes,
		},
		"top_models":  modelStats,
		"time_series": timeSeries,
	}

	if avgLatency != nil {
		metrics["latency"].(map[string]interface{})["avg_ms"] = *avgLatency
	}
	if p50 != nil {
		metrics["latency"].(map[string]interface{})["p50_ms"] = *p50
	}
	if p95 != nil {
		metrics["latency"].(map[string]interface{})["p95_ms"] = *p95
	}
	if p99 != nil {
		metrics["latency"].(map[string]interface{})["p99_ms"] = *p99
	}

	g.writeJSON(w, http.StatusOK, metrics)
}

// handleListTenants lists all tenants (admin view)
// Platform Admin Only - GET /admin/tenants
func (g *Gateway) handleListTenants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	statusFilter := r.URL.Query().Get("status")
	limit := 50
	offset := 0

	query := `
		SELECT
			t.id,
			t.name,
			t.email,
			t.status,
			t.billing_plan,
			t.created_at,
			COUNT(DISTINCT ak.id) as api_keys_count
		FROM tenants t
		LEFT JOIN api_keys ak ON ak.tenant_id = t.id AND ak.status = 'active'
		WHERE 1=1
	`

	args := []interface{}{}
	argNum := 1
	if statusFilter != "" {
		args = append(args, statusFilter)
		query += " AND t.status = $" + string(rune('0'+argNum))
		argNum++
	}

	query += `
		GROUP BY t.id, t.name, t.email, t.status, t.billing_plan, t.created_at
		ORDER BY t.created_at DESC
		LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))

	args = append(args, limit, offset)

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query tenants", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query tenants")
		return
	}
	defer rows.Close()

	var tenants []map[string]interface{}
	for rows.Next() {
		var id string
		var name, email, status, billingPlan string
		var createdAt time.Time
		var apiKeysCount int

		if err := rows.Scan(&id, &name, &email, &status, &billingPlan, &createdAt, &apiKeysCount); err != nil {
			g.logger.Warn("failed to scan tenant row", zap.Error(err))
			continue
		}

		// Get usage stats for tenant
		var totalSpendMicrodollars int64
		g.db.Pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(cost_microdollars), 0)
			FROM usage_records
			WHERE tenant_id = $1
		`, id).Scan(&totalSpendMicrodollars)

		tenants = append(tenants, map[string]interface{}{
			"id":              id,
			"name":            name,
			"email":           email,
			"status":          status,
			"tier":            billingPlan,
			"created_at":      createdAt,
			"total_spend_usd": float64(totalSpendMicrodollars) / 1_000_000.0,
			"api_keys_count":  apiKeysCount,
		})
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM tenants WHERE 1=1"
	if statusFilter != "" {
		countQuery += " AND status = $1"
		g.db.Pool.QueryRow(ctx, countQuery, statusFilter).Scan(&total)
	} else {
		g.db.Pool.QueryRow(ctx, countQuery).Scan(&total)
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": tenants,
		"pagination": map[string]interface{}{
			"total":    total,
			"limit":    limit,
			"offset":   offset,
			"has_more": (offset + limit) < total,
		},
	})
}

// handleGetTenantUsage gets usage data for a specific tenant (admin view)
// Platform Admin Only - GET /admin/tenants/{id}/usage
func (g *Gateway) handleGetTenantUsage(w http.ResponseWriter, r *http.Request) {
	// Delegate to existing handler but with admin context
	// This is a placeholder - actual implementation would extract tenant_id from URL
	g.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Admin tenant usage view - to be implemented",
	})
}

// handleUpdateTenant updates tenant configuration
// Platform Admin Only - PUT /admin/tenants/{id}
func (g *Gateway) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	// Implementation placeholder
	g.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Tenant update - to be implemented",
	})
}
