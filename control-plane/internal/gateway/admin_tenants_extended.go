package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/crosslogic/control-plane/pkg/events"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleDeleteTenant soft deletes a tenant
// Admin API - DELETE /admin/tenants/{id}
func (g *Gateway) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Check if tenant exists and get current status
	var currentStatus string
	var deletedAt *time.Time
	err = g.db.Pool.QueryRow(ctx, `
		SELECT status, deleted_at FROM tenants WHERE id = $1
	`, tenantID).Scan(&currentStatus, &deletedAt)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query tenant")
		return
	}

	// Return 409 if already deleted
	if currentStatus == "deleted" || deletedAt != nil {
		g.writeError(w, http.StatusConflict, "tenant already deleted")
		return
	}

	// Begin transaction for atomic operations
	tx, err := g.db.Pool.Begin(ctx)
	if err != nil {
		g.logger.Error("failed to begin transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Soft delete tenant
	_, err = tx.Exec(ctx, `
		UPDATE tenants
		SET status = 'deleted', deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to delete tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to delete tenant")
		return
	}

	// Revoke all API keys
	_, err = tx.Exec(ctx, `
		UPDATE api_keys
		SET status = 'revoked', updated_at = NOW()
		WHERE tenant_id = $1 AND status != 'revoked'
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to revoke API keys", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to revoke API keys")
		return
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		g.logger.Error("failed to commit transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	// Publish event
	if g.eventBus != nil {
		evt := events.NewEvent(
			events.EventTenantDeleted,
			tenantID.String(),
			map[string]interface{}{
				"previous_status": currentStatus,
				"deleted_at":      time.Now(),
			},
		)
		if err := g.eventBus.Publish(ctx, evt); err != nil {
			g.logger.Error("failed to publish tenant deleted event", zap.Error(err))
		}
	}

	g.logger.Info("tenant deleted",
		zap.String("tenant_id", tenantID.String()),
		zap.String("previous_status", currentStatus),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "tenant deleted successfully",
	})
}

// handleSuspendTenant suspends a tenant and deactivates API keys
// Admin API - POST /admin/tenants/{id}/suspend
func (g *Gateway) handleSuspendTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Parse request body for suspension reason
	var req struct {
		Reason string `json:"reason"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if tenant exists and get current status
	var currentStatus string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT status FROM tenants WHERE id = $1
	`, tenantID).Scan(&currentStatus)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query tenant")
		return
	}

	// Check if already suspended
	if currentStatus == "suspended" {
		g.writeError(w, http.StatusConflict, "tenant already suspended")
		return
	}

	// Cannot suspend deleted tenants
	if currentStatus == "deleted" {
		g.writeError(w, http.StatusConflict, "cannot suspend deleted tenant")
		return
	}

	// Begin transaction
	tx, err := g.db.Pool.Begin(ctx)
	if err != nil {
		g.logger.Error("failed to begin transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Prepare metadata
	metadata := map[string]interface{}{
		"suspension_reason": req.Reason,
		"suspension_notes":  req.Notes,
		"suspended_at":      time.Now().Format(time.RFC3339),
		"previous_status":   currentStatus,
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Suspend tenant
	_, err = tx.Exec(ctx, `
		UPDATE tenants
		SET status = 'suspended',
		    region_preferences = COALESCE(region_preferences, '{}'::jsonb) || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1
	`, tenantID, metadataJSON)
	if err != nil {
		g.logger.Error("failed to suspend tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to suspend tenant")
		return
	}

	// Temporarily deactivate all API keys (suspend, not revoke)
	_, err = tx.Exec(ctx, `
		UPDATE api_keys
		SET status = 'suspended',
		    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object(
		        'suspended_at', NOW(),
		        'previous_status', status
		    ),
		    updated_at = NOW()
		WHERE tenant_id = $1 AND status = 'active'
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to suspend API keys", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to suspend API keys")
		return
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		g.logger.Error("failed to commit transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	// TODO: Publish event when EventTenantSuspended is added to events package
	// if g.eventBus != nil {
	// 	evt := events.NewEvent(
	// 		events.EventTenantSuspended,
	// 		tenantID.String(),
	// 		map[string]interface{}{
	// 			"reason":          req.Reason,
	// 			"notes":           req.Notes,
	// 			"previous_status": currentStatus,
	// 		},
	// 	)
	// 	if err := g.eventBus.Publish(ctx, evt); err != nil {
	// 		g.logger.Error("failed to publish tenant suspended event", zap.Error(err))
	// 	}
	// }

	g.logger.Info("tenant suspended",
		zap.String("tenant_id", tenantID.String()),
		zap.String("reason", req.Reason),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "suspended",
		"message": "tenant suspended successfully",
		"reason":  req.Reason,
	})
}

// handleActivateTenant activates a suspended tenant and reactivates API keys
// Admin API - POST /admin/tenants/{id}/activate
func (g *Gateway) handleActivateTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Parse request body for activation notes
	var req struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(r.Body).Decode(&req) // Optional, ignore errors

	// Check if tenant exists and get current status
	var currentStatus string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT status FROM tenants WHERE id = $1
	`, tenantID).Scan(&currentStatus)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "tenant not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query tenant")
		return
	}

	// Check if already active
	if currentStatus == "active" {
		g.writeError(w, http.StatusConflict, "tenant already active")
		return
	}

	// Cannot activate deleted tenants
	if currentStatus == "deleted" {
		g.writeError(w, http.StatusConflict, "cannot activate deleted tenant")
		return
	}

	// Begin transaction
	tx, err := g.db.Pool.Begin(ctx)
	if err != nil {
		g.logger.Error("failed to begin transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Activate tenant
	metadata := map[string]interface{}{
		"activation_notes": req.Notes,
		"activated_at":     time.Now().Format(time.RFC3339),
		"previous_status":  currentStatus,
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err = tx.Exec(ctx, `
		UPDATE tenants
		SET status = 'active',
		    region_preferences = COALESCE(region_preferences, '{}'::jsonb) || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1
	`, tenantID, metadataJSON)
	if err != nil {
		g.logger.Error("failed to activate tenant", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to activate tenant")
		return
	}

	// Reactivate API keys that were suspended
	_, err = tx.Exec(ctx, `
		UPDATE api_keys
		SET status = 'active',
		    metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object(
		        'reactivated_at', NOW()
		    ),
		    updated_at = NOW()
		WHERE tenant_id = $1 AND status = 'suspended'
	`, tenantID)
	if err != nil {
		g.logger.Error("failed to reactivate API keys", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to reactivate API keys")
		return
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		g.logger.Error("failed to commit transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	// TODO: Publish event when EventTenantActivated is added to events package
	// if g.eventBus != nil {
	// 	evt := events.NewEvent(
	// 		events.EventTenantActivated,
	// 		tenantID.String(),
	// 		map[string]interface{}{
	// 			"notes":           req.Notes,
	// 			"previous_status": currentStatus,
	// 		},
	// 	)
	// 	if err := g.eventBus.Publish(ctx, evt); err != nil {
	// 		g.logger.Error("failed to publish tenant activated event", zap.Error(err))
	// 	}
	// }

	g.logger.Info("tenant activated",
		zap.String("tenant_id", tenantID.String()),
		zap.String("previous_status", currentStatus),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "active",
		"message": "tenant activated successfully",
	})
}

// handleGetTenantAPIKeys returns all API keys for a tenant
// Admin API - GET /admin/tenants/{id}/api-keys
func (g *Gateway) handleGetTenantAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Parse query parameters
	statusFilter := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if n, err := parseInt(limitStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if offsetStr != "" {
		if n, err := parseInt(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	// Build query
	query := `
		SELECT
			id, key_prefix, name, role, status,
			rate_limit_requests_per_min, concurrency_limit,
			created_at, last_used_at, expires_at
		FROM api_keys
		WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}
	argNum := 2

	if statusFilter != "" {
		query += " AND status = $" + string(rune('0'+argNum))
		args = append(args, statusFilter)
		argNum++
	}

	query += " ORDER BY created_at DESC LIMIT $" + string(rune('0'+argNum)) + " OFFSET $" + string(rune('0'+argNum+1))
	args = append(args, limit, offset)

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query API keys", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query API keys")
		return
	}
	defer rows.Close()

	var apiKeys []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var keyPrefix, name, role, status string
		var rateLimitRPM, concurrencyLimit int
		var createdAt time.Time
		var lastUsedAt, expiresAt *time.Time

		if err := rows.Scan(&id, &keyPrefix, &name, &role, &status,
			&rateLimitRPM, &concurrencyLimit, &createdAt, &lastUsedAt, &expiresAt); err != nil {
			g.logger.Warn("failed to scan API key row", zap.Error(err))
			continue
		}

		keyData := map[string]interface{}{
			"id":                  id,
			"key_prefix":          keyPrefix + "...",
			"name":                name,
			"role":                role,
			"status":              status,
			"rate_limit_rpm":      rateLimitRPM,
			"concurrency_limit":   concurrencyLimit,
			"created_at":          createdAt,
		}

		if lastUsedAt != nil {
			keyData["last_used_at"] = *lastUsedAt
		}
		if expiresAt != nil {
			keyData["expires_at"] = *expiresAt
		}

		apiKeys = append(apiKeys, keyData)
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM api_keys WHERE tenant_id = $1"
	countArgs := []interface{}{tenantID}
	if statusFilter != "" {
		countQuery += " AND status = $2"
		countArgs = append(countArgs, statusFilter)
	}
	g.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": apiKeys,
		"pagination": map[string]interface{}{
			"total":    total,
			"limit":    limit,
			"offset":   offset,
			"has_more": (offset + limit) < total,
		},
	})
}

// handleGetTenantDeployments returns deployment history for a tenant
// Admin API - GET /admin/tenants/{id}/deployments
func (g *Gateway) handleGetTenantDeployments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Parse date range
	startDate, endDate := parseDateRange(r)

	// Query models used by this tenant with usage aggregates
	query := `
		SELECT
			m.id,
			m.name,
			m.family,
			m.type,
			MIN(ur.timestamp) as first_used,
			MAX(ur.timestamp) as last_used,
			COALESCE(SUM(ur.total_tokens), 0) as total_tokens,
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(ur.cost_microdollars), 0) as total_cost_microdollars
		FROM usage_records ur
		INNER JOIN models m ON m.id = ur.model_id
		WHERE ur.tenant_id = $1
		  AND ur.timestamp >= $2
		  AND ur.timestamp <= $3
		GROUP BY m.id, m.name, m.family, m.type
		ORDER BY total_requests DESC
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID, startDate, endDate)
	if err != nil {
		g.logger.Error("failed to query tenant deployments", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query deployments")
		return
	}
	defer rows.Close()

	var deployments []map[string]interface{}
	for rows.Next() {
		var modelID uuid.UUID
		var modelName, family, mType string
		var firstUsed, lastUsed time.Time
		var totalTokens, totalRequests, totalCostMicro int64

		if err := rows.Scan(&modelID, &modelName, &family, &mType,
			&firstUsed, &lastUsed, &totalTokens, &totalRequests, &totalCostMicro); err != nil {
			g.logger.Warn("failed to scan deployment row", zap.Error(err))
			continue
		}

		deployments = append(deployments, map[string]interface{}{
			"model_id":      modelID,
			"model_name":    modelName,
			"family":        family,
			"type":          mType,
			"first_used":    firstUsed,
			"last_used":     lastUsed,
			"total_tokens":  totalTokens,
			"total_requests": totalRequests,
			"total_cost_usd": float64(totalCostMicro) / 1_000_000.0,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":  tenantID,
		"start_date": startDate,
		"end_date":   endDate,
		"data":       deployments,
	})
}

// handleGetTenantDetailedUsage returns detailed usage breakdown for a tenant
// Admin API - GET /admin/tenants/{id}/usage/detailed
func (g *Gateway) handleGetTenantDetailedUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantIDStr := chi.URLParam(r, "id")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	// Parse query parameters
	startDate, endDate := parseDateRange(r)
	groupBy := r.URL.Query().Get("group_by") // model, api_key, region, hour, day
	modelFilter := r.URL.Query().Get("model_id")
	apiKeyFilter := r.URL.Query().Get("api_key_id")
	regionFilter := r.URL.Query().Get("region_id")

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
		g.writeError(w, http.StatusBadRequest, "invalid group_by parameter")
		return
	}

	// Build dynamic query
	var selectClause, joinClause string
	switch groupBy {
	case "model":
		selectClause = "m.id as model_id, m.name as model_name, m.family"
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
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY ur.latency_ms), 0) as p95_latency_ms,
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
			query += " AND ur.model_id = $" + string(rune('0'+argNum))
			args = append(args, modelID)
			argNum++
		}
	}
	if apiKeyFilter != "" {
		apiKeyID, err := uuid.Parse(apiKeyFilter)
		if err == nil {
			query += " AND ur.api_key_id = $" + string(rune('0'+argNum))
			args = append(args, apiKeyID)
			argNum++
		}
	}
	if regionFilter != "" {
		regionID, err := uuid.Parse(regionFilter)
		if err == nil {
			query += " AND ur.region_id = $" + string(rune('0'+argNum))
			args = append(args, regionID)
			argNum++
		}
	}

	query += " GROUP BY " + groupClause + " ORDER BY total_tokens DESC LIMIT 1000"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query detailed usage", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query usage")
		return
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		// Use a flexible scanner based on group_by
		var promptTokens, completionTokens, totalTokens, cachedTokens, totalRequests, totalCostMicro int64
		var avgLatency, p95Latency float64

		switch groupBy {
		case "model":
			var modelID uuid.UUID
			var modelName, family string
			if err := rows.Scan(&modelID, &modelName, &family,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &p95Latency, &totalCostMicro); err != nil {
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
				"prompt_tokens":       promptTokens,
				"completion_tokens":   completionTokens,
				"total_tokens":        totalTokens,
				"cached_tokens":       cachedTokens,
				"cache_hit_rate_pct":  cacheHitRate,
				"total_requests":      totalRequests,
				"avg_latency_ms":      avgLatency,
				"p95_latency_ms":      p95Latency,
				"total_cost_usd":      float64(totalCostMicro) / 1_000_000.0,
			})
		case "api_key":
			var apiKeyID uuid.UUID
			var apiKeyName, keyPrefix string
			if err := rows.Scan(&apiKeyID, &apiKeyName, &keyPrefix,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &p95Latency, &totalCostMicro); err != nil {
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
				"p95_latency_ms":    p95Latency,
				"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			})
		case "region":
			var regionID *uuid.UUID
			var regionName, regionCode *string
			if err := rows.Scan(&regionID, &regionName, &regionCode,
				&promptTokens, &completionTokens, &totalTokens, &cachedTokens,
				&totalRequests, &avgLatency, &p95Latency, &totalCostMicro); err != nil {
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
				"p95_latency_ms":    p95Latency,
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
				&totalRequests, &avgLatency, &p95Latency, &totalCostMicro); err != nil {
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
				"p95_latency_ms":    p95Latency,
				"total_cost_usd":    float64(totalCostMicro) / 1_000_000.0,
			})
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":  tenantID,
		"start_date": startDate,
		"end_date":   endDate,
		"group_by":   groupBy,
		"data":       data,
	})
}

// Helper function to parse integer from string
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
