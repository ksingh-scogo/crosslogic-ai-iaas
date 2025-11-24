package gateway

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleGetPerformanceMetrics returns comprehensive performance metrics
// Tenant API - GET /v1/metrics/performance
func (g *Gateway) handleGetPerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}
	modelFilter := r.URL.Query().Get("model")

	startDate := calculateStartDate(period)
	endDate := time.Now()

	// Build query with optional model filter
	query := `
		SELECT
			m.name as model_name,
			COUNT(*) as total_requests,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(SUM(ur.latency_ms), 0) as total_latency_ms,
			COALESCE(AVG(ur.latency_ms), 0) as avg_latency_ms,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p50_latency_ms,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p95_latency_ms,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p99_latency_ms,
			COALESCE(SUM(ur.cached_tokens), 0) as cached_tokens,
			COUNT(*) FILTER (WHERE ur.billing_failed = false) as successful_requests,
			COUNT(*) FILTER (WHERE ur.billing_failed = true) as failed_requests
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
	`

	args := []interface{}{tenantID, startDate, endDate}
	if modelFilter != "" {
		query += " AND m.name = $4"
		args = append(args, modelFilter)
	}

	query += " GROUP BY m.name ORDER BY total_requests DESC"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query performance metrics",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}
	defer rows.Close()

	var metrics []map[string]interface{}
	for rows.Next() {
		var modelName string
		var totalRequests, totalTokens, totalLatencyMs, cachedTokens, successfulRequests, failedRequests int64
		var avgLatency, p50, p95, p99 float64

		if err := rows.Scan(&modelName, &totalRequests, &totalTokens, &totalLatencyMs,
			&avgLatency, &p50, &p95, &p99, &cachedTokens, &successfulRequests, &failedRequests); err != nil {
			g.logger.Warn("failed to scan performance metrics row", zap.Error(err))
			continue
		}

		// Calculate tokens per second (throughput)
		tokensPerSecond := 0.0
		if totalLatencyMs > 0 {
			tokensPerSecond = float64(totalTokens) / (float64(totalLatencyMs) / 1000.0)
		}

		// Calculate cache hit rate
		cacheHitRate := 0.0
		if totalTokens > 0 {
			cacheHitRate = float64(cachedTokens) / float64(totalTokens) * 100
		}

		// Calculate success rate
		successRate := 0.0
		if totalRequests > 0 {
			successRate = float64(successfulRequests) / float64(totalRequests) * 100
		}

		metrics = append(metrics, map[string]interface{}{
			"model_name":         modelName,
			"total_requests":     totalRequests,
			"successful_requests": successfulRequests,
			"failed_requests":    failedRequests,
			"success_rate_pct":   successRate,
			"total_tokens":       totalTokens,
			"cached_tokens":      cachedTokens,
			"cache_hit_rate_pct": cacheHitRate,
			"tokens_per_second":  tokensPerSecond,
			"latency": map[string]interface{}{
				"avg_ms": avgLatency,
				"p50_ms": p50,
				"p95_ms": p95,
				"p99_ms": p99,
			},
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"period":     period,
		"start_date": startDate,
		"end_date":   endDate,
		"metrics":    metrics,
	})
}

// handleGetThroughputMetrics returns throughput and capacity metrics
// Tenant API - GET /v1/metrics/throughput
func (g *Gateway) handleGetThroughputMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	startDate := calculateStartDate(period)
	endDate := time.Now()

	// Overall throughput metrics
	var totalRequests, totalTokens, cachedTokens int64
	var avgTokensPerRequest float64
	err := g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cached_tokens), 0) as cached_tokens,
			COALESCE(AVG(total_tokens), 0) as avg_tokens_per_request
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
	`, tenantID, startDate, endDate).Scan(&totalRequests, &totalTokens, &cachedTokens, &avgTokensPerRequest)

	if err != nil {
		g.logger.Error("failed to query throughput metrics", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}

	// Calculate peak RPS by finding the minute with the most requests
	var peakRPS float64
	var peakTimestamp time.Time
	g.db.Pool.QueryRow(ctx, `
		SELECT
			DATE_TRUNC('minute', timestamp) as minute,
			COUNT(*)::float / 60.0 as rps
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY minute
		ORDER BY rps DESC
		LIMIT 1
	`, tenantID, startDate, endDate).Scan(&peakTimestamp, &peakRPS)

	// Calculate average RPS
	durationSeconds := endDate.Sub(startDate).Seconds()
	avgRPS := 0.0
	if durationSeconds > 0 {
		avgRPS = float64(totalRequests) / durationSeconds
	}

	// Calculate cache distribution
	uncachedTokens := totalTokens - cachedTokens
	cachedPct := 0.0
	uncachedPct := 0.0
	if totalTokens > 0 {
		cachedPct = float64(cachedTokens) / float64(totalTokens) * 100
		uncachedPct = float64(uncachedTokens) / float64(totalTokens) * 100
	}

	// Time series data by minute for recent period
	timeSeriesQuery := `
		SELECT
			DATE_TRUNC('minute', timestamp) as minute,
			COUNT(*) as requests,
			COALESCE(SUM(total_tokens), 0) as tokens
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY minute
		ORDER BY minute DESC
		LIMIT 60
	`

	rows, err := g.db.Pool.Query(ctx, timeSeriesQuery, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query time series", zap.Error(err))
		// Continue without time series data
	}

	var timeSeries []map[string]interface{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var minute time.Time
			var requests, tokens int64

			if err := rows.Scan(&minute, &requests, &tokens); err == nil {
				timeSeries = append(timeSeries, map[string]interface{}{
					"timestamp": minute,
					"requests":  requests,
					"tokens":    tokens,
					"rps":       float64(requests) / 60.0,
				})
			}
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"period":     period,
		"start_date": startDate,
		"end_date":   endDate,
		"throughput": map[string]interface{}{
			"total_requests":       totalRequests,
			"total_tokens":         totalTokens,
			"avg_tokens_per_request": avgTokensPerRequest,
			"avg_rps":              avgRPS,
			"peak_rps":             peakRPS,
			"peak_timestamp":       peakTimestamp,
		},
		"cache_distribution": map[string]interface{}{
			"cached_tokens":      cachedTokens,
			"uncached_tokens":    uncachedTokens,
			"cached_pct":         cachedPct,
			"uncached_pct":       uncachedPct,
		},
		"time_series": timeSeries,
	})
}

// handleGetModelMetrics returns per-model usage and performance metrics
// Tenant API - GET /v1/metrics/by-model
func (g *Gateway) handleGetModelMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	startDate := calculateStartDate(period)
	endDate := time.Now()

	// Get total tokens for percentage calculation
	var totalTokensAllModels int64
	g.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_tokens), 0)
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
	`, tenantID, startDate, endDate).Scan(&totalTokensAllModels)

	// Query per-model metrics
	query := `
		SELECT
			m.id,
			m.name,
			m.family,
			m.type,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(SUM(ur.prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(ur.completion_tokens), 0) as completion_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(ur.cost_microdollars), 0) as total_cost_microdollars,
			COALESCE(AVG(ur.latency_ms), 0) as avg_latency_ms,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p95_latency_ms
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
		GROUP BY m.id, m.name, m.family, m.type
		ORDER BY total_tokens DESC
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query model metrics",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}
	defer rows.Close()

	var modelMetrics []map[string]interface{}
	for rows.Next() {
		var modelID uuid.UUID
		var modelName, family, mType string
		var totalTokens, promptTokens, completionTokens, totalRequests, totalCostMicro int64
		var avgLatency, p95Latency float64

		if err := rows.Scan(&modelID, &modelName, &family, &mType,
			&totalTokens, &promptTokens, &completionTokens, &totalRequests,
			&totalCostMicro, &avgLatency, &p95Latency); err != nil {
			g.logger.Warn("failed to scan model metrics row", zap.Error(err))
			continue
		}

		// Calculate usage percentage
		usagePct := 0.0
		if totalTokensAllModels > 0 {
			usagePct = float64(totalTokens) / float64(totalTokensAllModels) * 100
		}

		// Calculate average tokens per request
		avgTokensPerRequest := 0.0
		if totalRequests > 0 {
			avgTokensPerRequest = float64(totalTokens) / float64(totalRequests)
		}

		modelMetrics = append(modelMetrics, map[string]interface{}{
			"model_id":              modelID,
			"model_name":            modelName,
			"family":                family,
			"type":                  mType,
			"total_tokens":          totalTokens,
			"prompt_tokens":         promptTokens,
			"completion_tokens":     completionTokens,
			"total_requests":        totalRequests,
			"avg_tokens_per_request": avgTokensPerRequest,
			"usage_percentage":      usagePct,
			"total_cost_usd":        float64(totalCostMicro) / 1_000_000.0,
			"avg_latency_ms":        avgLatency,
			"p95_latency_ms":        p95Latency,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"period":     period,
		"start_date": startDate,
		"end_date":   endDate,
		"models":     modelMetrics,
	})
}
