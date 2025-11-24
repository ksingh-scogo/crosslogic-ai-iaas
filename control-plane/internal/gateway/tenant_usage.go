package gateway

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleGetUsage returns overall usage summary for the tenant
// Tenant API - GET /v1/usage
func (g *Gateway) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse date range
	startDate, endDate := parseDateRange(r)

	// Query overall usage summary
	var totalTokens, totalRequests int64
	var totalCostMicrodollars int64
	err := g.db.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(1), 0),
			COALESCE(SUM(cost_microdollars), 0)
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
	`, tenantID, startDate, endDate).Scan(&totalTokens, &totalRequests, &totalCostMicrodollars)

	if err != nil {
		g.logger.Error("failed to query usage summary",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}

	// Query usage by model
	rows, err := g.db.Pool.Query(ctx, `
		SELECT
			m.name,
			COALESCE(SUM(ur.total_tokens), 0) as tokens,
			COALESCE(COUNT(*), 0) as requests,
			COALESCE(SUM(ur.cost_microdollars), 0) as cost_microdollars
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
		GROUP BY m.name
		ORDER BY tokens DESC
	`, tenantID, startDate, endDate)

	if err != nil {
		g.logger.Error("failed to query usage by model", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var byModel []map[string]interface{}
	for rows.Next() {
		var modelName string
		var tokens, requests, costMicro int64

		if err := rows.Scan(&modelName, &tokens, &requests, &costMicro); err != nil {
			g.logger.Warn("failed to scan usage row", zap.Error(err))
			continue
		}

		byModel = append(byModel, map[string]interface{}{
			"model_name": modelName,
			"tokens":     tokens,
			"requests":   requests,
			"cost_usd":   float64(costMicro) / 1_000_000.0,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":      startDate,
		"end_date":        endDate,
		"total_tokens":    totalTokens,
		"total_requests":  totalRequests,
		"total_cost_usd":  float64(totalCostMicrodollars) / 1_000_000.0,
		"by_model":        byModel,
	})
}

// handleGetUsageByModel returns usage breakdown by model
// Tenant API - GET /v1/usage/by-model
func (g *Gateway) handleGetUsageByModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	startDate, endDate := parseDateRange(r)

	rows, err := g.db.Pool.Query(ctx, `
		SELECT
			m.id,
			m.name,
			m.family,
			m.type,
			COALESCE(SUM(ur.prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(ur.completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(COUNT(*), 0) as requests,
			COALESCE(AVG(ur.latency_ms), 0) as avg_latency_ms,
			COALESCE(SUM(ur.cost_microdollars), 0) as cost_microdollars
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
		GROUP BY m.id, m.name, m.family, m.type
		ORDER BY total_tokens DESC
	`, tenantID, startDate, endDate)

	if err != nil {
		g.logger.Error("failed to query usage by model",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var modelID uuid.UUID
		var modelName, family, mType string
		var promptTokens, completionTokens, totalTokens, requests, costMicro int64
		var avgLatency float64

		if err := rows.Scan(&modelID, &modelName, &family, &mType, &promptTokens,
			&completionTokens, &totalTokens, &requests, &avgLatency, &costMicro); err != nil {
			g.logger.Warn("failed to scan model usage row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"model_id":          modelID,
			"model_name":        modelName,
			"family":            family,
			"type":              mType,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      totalTokens,
			"requests":          requests,
			"avg_latency_ms":    avgLatency,
			"cost_usd":          float64(costMicro) / 1_000_000.0,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}

// handleGetUsageByKey returns usage breakdown by API key
// Tenant API - GET /v1/usage/by-key
func (g *Gateway) handleGetUsageByKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	startDate, endDate := parseDateRange(r)

	rows, err := g.db.Pool.Query(ctx, `
		SELECT
			ak.id,
			ak.name,
			ak.key_prefix,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(COUNT(*), 0) as requests,
			COALESCE(SUM(ur.cost_microdollars), 0) as cost_microdollars,
			MAX(ur.timestamp) as last_used
		FROM api_keys ak
		LEFT JOIN usage_records ur ON ur.api_key_id = ak.id
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
		WHERE ak.tenant_id = $1
		GROUP BY ak.id, ak.name, ak.key_prefix
		ORDER BY total_tokens DESC
	`, tenantID, startDate, endDate)

	if err != nil {
		g.logger.Error("failed to query usage by key",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var keyID uuid.UUID
		var name, prefix string
		var totalTokens, requests, costMicro int64
		var lastUsed *time.Time

		if err := rows.Scan(&keyID, &name, &prefix, &totalTokens, &requests, &costMicro, &lastUsed); err != nil {
			g.logger.Warn("failed to scan key usage row", zap.Error(err))
			continue
		}

		keyData := map[string]interface{}{
			"key_id":       keyID,
			"key_name":     name,
			"key_prefix":   prefix + "...",
			"total_tokens": totalTokens,
			"requests":     requests,
			"cost_usd":     float64(costMicro) / 1_000_000.0,
		}

		if lastUsed != nil {
			keyData["last_used"] = *lastUsed
		}

		data = append(data, keyData)
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}

// handleGetUsageByDate returns time-series usage data
// Tenant API - GET /v1/usage/by-date
func (g *Gateway) handleGetUsageByDate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	startDate, endDate := parseDateRange(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	// Determine time truncation based on granularity
	var truncFunc string
	switch granularity {
	case "hour":
		truncFunc = "DATE_TRUNC('hour', timestamp)"
	case "month":
		truncFunc = "DATE_TRUNC('month', timestamp)"
	default: // day
		truncFunc = "DATE_TRUNC('day', timestamp)"
	}

	query := `
		SELECT
			` + truncFunc + ` as period,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(COUNT(*), 0) as requests,
			COALESCE(SUM(cost_microdollars), 0) as cost_microdollars
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY period
		ORDER BY period
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query usage by date",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var period time.Time
		var totalTokens, requests, costMicro int64

		if err := rows.Scan(&period, &totalTokens, &requests, &costMicro); err != nil {
			g.logger.Warn("failed to scan time series row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"period":       period,
			"total_tokens": totalTokens,
			"requests":     requests,
			"cost_usd":     float64(costMicro) / 1_000_000.0,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":  startDate,
		"end_date":    endDate,
		"granularity": granularity,
		"data":        data,
	})
}

// handleGetLatencyMetrics returns latency performance metrics
// Tenant API - GET /v1/metrics/latency
func (g *Gateway) handleGetLatencyMetrics(w http.ResponseWriter, r *http.Request) {
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

	modelFilter := r.URL.Query().Get("model")

	// Calculate time range based on period
	startDate := calculateStartDate(period)
	endDate := time.Now()

	query := `
		SELECT
			m.name,
			COALESCE(AVG(ur.latency_ms), 0) as avg_latency,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p50,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p95,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p99,
			COUNT(*) as total_requests
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

	query += " GROUP BY m.name ORDER BY avg_latency"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query latency metrics",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var modelName string
		var avgLatency, p50, p95, p99 float64
		var totalRequests int64

		if err := rows.Scan(&modelName, &avgLatency, &p50, &p95, &p99, &totalRequests); err != nil {
			g.logger.Warn("failed to scan latency row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"model_name":      modelName,
			"avg_latency_ms":  avgLatency,
			"p50_latency_ms":  p50,
			"p95_latency_ms":  p95,
			"p99_latency_ms":  p99,
			"total_requests":  totalRequests,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"period": period,
		"data":   data,
	})
}

// handleGetTokenMetrics returns token usage metrics
// Tenant API - GET /v1/metrics/tokens
func (g *Gateway) handleGetTokenMetrics(w http.ResponseWriter, r *http.Request) {
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

	modelFilter := r.URL.Query().Get("model")
	startDate := calculateStartDate(period)
	endDate := time.Now()

	query := `
		SELECT
			m.name,
			COALESCE(SUM(ur.prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(ur.completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(AVG(ur.prompt_tokens), 0) as avg_prompt_tokens,
			COALESCE(AVG(ur.completion_tokens), 0) as avg_completion_tokens,
			COUNT(*) as total_requests
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

	query += " GROUP BY m.name ORDER BY total_tokens DESC"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query token metrics",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query metrics")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var modelName string
		var promptTokens, completionTokens, totalTokens, totalRequests int64
		var avgPrompt, avgCompletion float64

		if err := rows.Scan(&modelName, &promptTokens, &completionTokens, &totalTokens,
			&avgPrompt, &avgCompletion, &totalRequests); err != nil {
			g.logger.Warn("failed to scan token metrics row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"model_name":              modelName,
			"prompt_tokens":           promptTokens,
			"completion_tokens":       completionTokens,
			"total_tokens":            totalTokens,
			"avg_prompt_tokens":       avgPrompt,
			"avg_completion_tokens":   avgCompletion,
			"total_requests":          totalRequests,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"period": period,
		"data":   data,
	})
}

// Helper functions

// parseDateRange parses start_date and end_date from query params
func parseDateRange(r *http.Request) (time.Time, time.Time) {
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")

	var startDate, endDate time.Time

	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startDate = t
		}
	}

	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endDate = t
		}
	}

	// Default to last 30 days if not specified
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, 0, -30)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	return startDate, endDate
}

// calculateStartDate calculates start date based on period string
func calculateStartDate(period string) time.Time {
	now := time.Now()

	switch period {
	case "1h":
		return now.Add(-1 * time.Hour)
	case "24h":
		return now.AddDate(0, 0, -1)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	default:
		return now.AddDate(0, 0, -1) // Default to 24h
	}
}
