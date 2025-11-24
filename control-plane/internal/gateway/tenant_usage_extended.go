package gateway

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleGetUsageDetailed returns detailed usage records with advanced filtering
// Tenant API - GET /v1/usage/detailed
func (g *Gateway) handleGetUsageDetailed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse query parameters
	startDate, endDate := parseDateRange(r)
	modelFilter := r.URL.Query().Get("model_id")
	apiKeyFilter := r.URL.Query().Get("api_key_id")
	groupBy := r.URL.Query().Get("group_by") // model, api_key, region, hour, day
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Parse pagination
	limit := 100
	offset := 0
	if limitStr != "" {
		if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if offsetStr != "" {
		if n, err := parseInt(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	// Default group by
	if groupBy == "" {
		groupBy = "model"
	}

	// Validate group_by
	validGroupBy := map[string]string{
		"model":   "m.id, m.name",
		"api_key": "ak.id, ak.name, ak.key_prefix",
		"region":  "r.id, r.name, r.code",
		"hour":    "DATE_TRUNC('hour', ur.timestamp)",
		"day":     "DATE_TRUNC('day', ur.timestamp)",
	}

	groupClause, ok := validGroupBy[groupBy]
	if !ok {
		g.writeError(w, http.StatusBadRequest, "invalid group_by parameter. Valid values: model, api_key, region, hour, day")
		return
	}

	// Build dynamic query
	var selectClause, joinClause string
	switch groupBy {
	case "model":
		selectClause = "m.id as model_id, m.name as model_name, m.family, m.type"
		joinClause = "INNER JOIN models m ON m.id = ur.model_id"
	case "api_key":
		selectClause = "ak.id as api_key_id, ak.name as api_key_name, ak.key_prefix"
		joinClause = "INNER JOIN api_keys ak ON ak.id = ur.api_key_id"
	case "region":
		selectClause = "r.id as region_id, r.name as region_name, r.code as region_code"
		joinClause = "LEFT JOIN regions r ON r.id = ur.region_id"
	case "hour", "day":
		selectClause = "DATE_TRUNC('" + groupBy + "', ur.timestamp) as period"
		joinClause = ""
	}

	query := `
		SELECT
			` + selectClause + `,
			COALESCE(SUM(ur.prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(ur.completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(SUM(ur.cached_tokens), 0) as cached_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(AVG(ur.latency_ms), 0) as avg_latency_ms,
			COALESCE(MIN(ur.latency_ms), 0) as min_latency_ms,
			COALESCE(MAX(ur.latency_ms), 0) as max_latency_ms,
			COALESCE(SUM(ur.cost_microdollars), 0) as total_cost_microdollars
		FROM usage_records ur
		` + joinClause + `
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
	`

	args := []interface{}{tenantID, startDate, endDate}
	argNum := 4

	// Add filters
	if modelFilter != "" {
		modelID, err := uuid.Parse(modelFilter)
		if err == nil {
			query += fmt.Sprintf(" AND ur.model_id = $%d", argNum)
			args = append(args, modelID)
			argNum++
		}
	}
	if apiKeyFilter != "" {
		apiKeyID, err := uuid.Parse(apiKeyFilter)
		if err == nil {
			query += fmt.Sprintf(" AND ur.api_key_id = $%d", argNum)
			args = append(args, apiKeyID)
			argNum++
		}
	}

	query += " GROUP BY " + groupClause + " ORDER BY total_tokens DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, limit, offset)

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query detailed usage",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var promptTokens, completionTokens, totalTokens, cachedTokens, totalRequests, totalCostMicro int64
		var avgLatency, minLatency, maxLatency float64

		switch groupBy {
		case "model":
			var modelID uuid.UUID
			var modelName, family, mType string
			if err := rows.Scan(&modelID, &modelName, &family, &mType,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &minLatency, &maxLatency, &totalCostMicro); err != nil {
				g.logger.Warn("failed to scan row", zap.Error(err))
				continue
			}
			cacheHitRate := 0.0
			if totalTokens > 0 {
				cacheHitRate = float64(cachedTokens) / float64(totalTokens) * 100
			}
			data = append(data, map[string]interface{}{
				"model_id":            modelID,
				"model_name":          modelName,
				"family":              family,
				"type":                mType,
				"prompt_tokens":       promptTokens,
				"completion_tokens":   completionTokens,
				"total_tokens":        totalTokens,
				"cached_tokens":       cachedTokens,
				"cache_hit_rate_pct":  cacheHitRate,
				"total_requests":      totalRequests,
				"avg_latency_ms":      avgLatency,
				"min_latency_ms":      minLatency,
				"max_latency_ms":      maxLatency,
				"total_cost_usd":      float64(totalCostMicro) / 1_000_000.0,
			})
		case "api_key":
			var apiKeyID uuid.UUID
			var apiKeyName, keyPrefix string
			if err := rows.Scan(&apiKeyID, &apiKeyName, &keyPrefix,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &minLatency, &maxLatency, &totalCostMicro); err != nil {
				g.logger.Warn("failed to scan row", zap.Error(err))
				continue
			}
			data = append(data, map[string]interface{}{
				"api_key_id":        apiKeyID,
				"api_key_name":      apiKeyName,
				"key_prefix":        keyPrefix + "...",
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      totalTokens,
				"cached_tokens":     cachedTokens,
				"total_requests":    totalRequests,
				"avg_latency_ms":    avgLatency,
				"min_latency_ms":    minLatency,
				"max_latency_ms":    maxLatency,
				"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			})
		case "region":
			var regionID *uuid.UUID
			var regionName, regionCode *string
			if err := rows.Scan(&regionID, &regionName, &regionCode,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &minLatency, &maxLatency, &totalCostMicro); err != nil {
				g.logger.Warn("failed to scan row", zap.Error(err))
				continue
			}
			regionData := map[string]interface{}{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      totalTokens,
				"cached_tokens":     cachedTokens,
				"total_requests":    totalRequests,
				"avg_latency_ms":    avgLatency,
				"min_latency_ms":    minLatency,
				"max_latency_ms":    maxLatency,
				"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			}
			if regionID != nil {
				regionData["region_id"] = *regionID
			}
			if regionName != nil {
				regionData["region_name"] = *regionName
			}
			if regionCode != nil {
				regionData["region_code"] = *regionCode
			}
			data = append(data, regionData)
		case "hour", "day":
			var period time.Time
			if err := rows.Scan(&period,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &minLatency, &maxLatency, &totalCostMicro); err != nil {
				g.logger.Warn("failed to scan row", zap.Error(err))
				continue
			}
			data = append(data, map[string]interface{}{
				"period":            period,
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      totalTokens,
				"cached_tokens":     cachedTokens,
				"total_requests":    totalRequests,
				"avg_latency_ms":    avgLatency,
				"min_latency_ms":    minLatency,
				"max_latency_ms":    maxLatency,
				"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			})
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
		"group_by":   groupBy,
		"data":       data,
		"pagination": map[string]interface{}{
			"limit":  limit,
			"offset": offset,
		},
	})
}

// handleGetUsageByHour returns usage aggregated by hour
// Tenant API - GET /v1/usage/by-hour
func (g *Gateway) handleGetUsageByHour(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse hours parameter (default 24, max 168 = 7 days)
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if n, err := parseInt(hoursStr); err == nil && n > 0 && n <= 168 {
			hours = n
		}
	}

	startDate := time.Now().Add(-time.Duration(hours) * time.Hour)
	endDate := time.Now()

	query := `
		SELECT
			DATE_TRUNC('hour', timestamp) as hour,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(cost_microdollars), 0) as total_cost_microdollars,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY hour
		ORDER BY hour
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query hourly usage",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var hour time.Time
		var totalTokens, totalRequests, totalCostMicro int64
		var avgLatency float64

		if err := rows.Scan(&hour, &totalTokens, &totalRequests, &totalCostMicro, &avgLatency); err != nil {
			g.logger.Warn("failed to scan row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"hour":           hour,
			"total_tokens":   totalTokens,
			"total_requests": totalRequests,
			"total_cost_usd": float64(totalCostMicro) / 1_000_000.0,
			"avg_latency_ms": avgLatency,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"hours":      hours,
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}

// handleGetUsageByDay returns usage aggregated by day
// Tenant API - GET /v1/usage/by-day
func (g *Gateway) handleGetUsageByDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse days parameter (default 30, max 90)
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if n, err := parseInt(daysStr); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}

	startDate := time.Now().AddDate(0, 0, -days)
	endDate := time.Now()

	query := `
		SELECT
			DATE_TRUNC('day', timestamp) as day,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(cost_microdollars), 0) as total_cost_microdollars,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY day
		ORDER BY day
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query daily usage",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var day time.Time
		var totalTokens, promptTokens, completionTokens, totalRequests, totalCostMicro int64
		var avgLatency float64

		if err := rows.Scan(&day, &totalTokens, &promptTokens, &completionTokens,
			&totalRequests, &totalCostMicro, &avgLatency); err != nil {
			g.logger.Warn("failed to scan row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"day":               day,
			"total_tokens":      totalTokens,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_requests":    totalRequests,
			"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			"avg_latency_ms":    avgLatency,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"days":       days,
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}

// handleGetUsageByWeek returns usage aggregated by week
// Tenant API - GET /v1/usage/by-week
func (g *Gateway) handleGetUsageByWeek(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse weeks parameter (default 12, max 52)
	weeksStr := r.URL.Query().Get("weeks")
	weeks := 12
	if weeksStr != "" {
		if n, err := parseInt(weeksStr); err == nil && n > 0 && n <= 52 {
			weeks = n
		}
	}

	startDate := time.Now().AddDate(0, 0, -weeks*7)
	endDate := time.Now()

	query := `
		SELECT
			DATE_TRUNC('week', timestamp) as week,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(cost_microdollars), 0) as total_cost_microdollars,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY week
		ORDER BY week
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query weekly usage",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var week time.Time
		var totalTokens, promptTokens, completionTokens, totalRequests, totalCostMicro int64
		var avgLatency float64

		if err := rows.Scan(&week, &totalTokens, &promptTokens, &completionTokens,
			&totalRequests, &totalCostMicro, &avgLatency); err != nil {
			g.logger.Warn("failed to scan row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"week_start":        week,
			"total_tokens":      totalTokens,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_requests":    totalRequests,
			"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			"avg_latency_ms":    avgLatency,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"weeks":      weeks,
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}

// handleGetUsageByMonth returns usage aggregated by month
// Tenant API - GET /v1/usage/by-month
func (g *Gateway) handleGetUsageByMonth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Parse months parameter (default 12, max 24)
	monthsStr := r.URL.Query().Get("months")
	months := 12
	if monthsStr != "" {
		if n, err := parseInt(monthsStr); err == nil && n > 0 && n <= 24 {
			months = n
		}
	}

	startDate := time.Now().AddDate(0, -months, 0)
	endDate := time.Now()

	query := `
		SELECT
			DATE_TRUNC('month', timestamp) as month,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(cost_microdollars), 0) as total_cost_microdollars,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE tenant_id = $1
		  AND timestamp >= $2
		  AND timestamp <= $3
		GROUP BY month
		ORDER BY month
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query monthly usage",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var month time.Time
		var totalTokens, promptTokens, completionTokens, totalRequests, totalCostMicro int64
		var avgLatency float64

		if err := rows.Scan(&month, &totalTokens, &promptTokens, &completionTokens,
			&totalRequests, &totalCostMicro, &avgLatency); err != nil {
			g.logger.Warn("failed to scan row", zap.Error(err))
			continue
		}

		data = append(data, map[string]interface{}{
			"month":             month,
			"total_tokens":      totalTokens,
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_requests":    totalRequests,
			"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			"avg_latency_ms":    avgLatency,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"months":     months,
		"start_date": startDate,
		"end_date":   endDate,
		"data":       data,
	})
}
