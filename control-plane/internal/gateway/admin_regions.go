package gateway

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleCreateRegion creates a new region
// Admin API - POST /admin/regions
func (g *Gateway) handleCreateRegion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Code              string                 `json:"code"`
		Name              string                 `json:"name"`
		Provider          string                 `json:"provider"`
		Country           string                 `json:"country"`
		City              string                 `json:"city"`
		Available         bool                   `json:"available"`
		PricingMultiplier float64                `json:"pricing_multiplier"`
		Metadata          map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Code == "" || req.Name == "" || req.Provider == "" {
		g.writeError(w, http.StatusBadRequest, "code, name, and provider are required")
		return
	}

	// Validate provider
	validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true, "oci": true}
	if !validProviders[req.Provider] {
		g.writeError(w, http.StatusBadRequest, "invalid provider. Valid values: aws, azure, gcp, oci")
		return
	}

	// Set defaults
	if req.PricingMultiplier == 0 {
		req.PricingMultiplier = 1.0
	}

	// Check if region code already exists
	var existingID uuid.UUID
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id FROM regions WHERE code = $1
	`, req.Code).Scan(&existingID)

	if err == nil {
		// Region already exists
		g.writeError(w, http.StatusConflict, "region with this code already exists")
		return
	}

	// Prepare metadata
	metadataJSON := []byte("{}")
	if req.Metadata != nil {
		metadataJSON, _ = json.Marshal(req.Metadata)
	}

	// Set status based on available flag
	status := "offline"
	if req.Available {
		status = "active"
	}

	// Insert region
	var regionID uuid.UUID
	err = g.db.Pool.QueryRow(ctx, `
		INSERT INTO regions (
			code, name, city, country, provider, status, cost_multiplier, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, req.Code, req.Name, req.City, req.Country, req.Provider, status, req.PricingMultiplier, metadataJSON).Scan(&regionID)

	if err != nil {
		g.logger.Error("failed to create region", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create region")
		return
	}

	g.logger.Info("region created",
		zap.String("region_id", regionID.String()),
		zap.String("code", req.Code),
		zap.String("provider", req.Provider),
	)

	g.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                 regionID,
		"code":               req.Code,
		"name":               req.Name,
		"provider":           req.Provider,
		"country":            req.Country,
		"city":               req.City,
		"status":             status,
		"pricing_multiplier": req.PricingMultiplier,
	})
}

// handleUpdateRegion updates an existing region
// Admin API - PUT /admin/regions/{id}
func (g *Gateway) handleUpdateRegion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	regionIDStr := chi.URLParam(r, "id")

	regionID, err := uuid.Parse(regionIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid region ID")
		return
	}

	var req struct {
		Name              *string                `json:"name"`
		Available         *bool                  `json:"available"`
		PricingMultiplier *float64               `json:"pricing_multiplier"`
		Metadata          map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if region exists
	var existingCode, existingProvider string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT code, provider FROM regions WHERE id = $1
	`, regionID).Scan(&existingCode, &existingProvider)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "region not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query region", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query region")
		return
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+string(rune('0'+argNum)))
		args = append(args, *req.Name)
		argNum++
	}

	if req.Available != nil {
		status := "offline"
		if *req.Available {
			status = "active"
		}
		updates = append(updates, "status = $"+string(rune('0'+argNum)))
		args = append(args, status)
		argNum++
	}

	if req.PricingMultiplier != nil {
		updates = append(updates, "cost_multiplier = $"+string(rune('0'+argNum)))
		args = append(args, *req.PricingMultiplier)
		argNum++
	}

	if req.Metadata != nil {
		metadataJSON, _ := json.Marshal(req.Metadata)
		updates = append(updates, "metadata = $"+string(rune('0'+argNum)))
		args = append(args, metadataJSON)
		argNum++
	}

	if len(updates) == 0 {
		g.writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	// Add updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add WHERE clause
	args = append(args, regionID)
	query := "UPDATE regions SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = $" + string(rune('0'+argNum))

	_, err = g.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to update region", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to update region")
		return
	}

	g.logger.Info("region updated",
		zap.String("region_id", regionID.String()),
		zap.String("code", existingCode),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"message": "region updated successfully",
	})
}

// handleDeleteRegion deletes a region if no active nodes exist
// Admin API - DELETE /admin/regions/{id}
func (g *Gateway) handleDeleteRegion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	regionIDStr := chi.URLParam(r, "id")

	regionID, err := uuid.Parse(regionIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid region ID")
		return
	}

	// Check if any active nodes exist in this region
	var activeNodeCount int
	err = g.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM nodes
		WHERE region_id = $1 AND status IN ('active', 'initializing', 'draining')
	`, regionID).Scan(&activeNodeCount)

	if err != nil {
		g.logger.Error("failed to query nodes", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query nodes")
		return
	}

	if activeNodeCount > 0 {
		g.writeError(w, http.StatusConflict, "cannot delete region with active nodes")
		return
	}

	// Delete the region (or soft delete by setting status to 'offline')
	// Using soft delete here
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE regions
		SET status = 'offline', updated_at = NOW()
		WHERE id = $1
	`, regionID)

	if err != nil {
		g.logger.Error("failed to delete region", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to delete region")
		return
	}

	g.logger.Info("region deleted (soft)",
		zap.String("region_id", regionID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "region deleted successfully",
	})
}

// handleGetRegionAvailability returns availability information for a region
// Admin API - GET /admin/regions/{id}/availability
func (g *Gateway) handleGetRegionAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	regionIDStr := chi.URLParam(r, "id")

	regionID, err := uuid.Parse(regionIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid region ID")
		return
	}

	// Get region details
	var regionCode, regionName, provider, status string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT code, name, provider, status
		FROM regions
		WHERE id = $1
	`, regionID).Scan(&regionCode, &regionName, &provider, &status)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "region not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query region", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query region")
		return
	}

	// Get available instance types in this region
	instanceQuery := `
		SELECT
			it.id,
			it.instance_type,
			it.instance_name,
			it.gpu_model,
			it.gpu_count,
			it.gpu_memory_gb,
			it.vcpu_count,
			it.memory_gb,
			it.price_per_hour,
			it.spot_price_per_hour,
			it.supports_spot,
			ria.is_available,
			ria.stock_status
		FROM instance_types it
		INNER JOIN region_instance_availability ria ON ria.instance_type_id = it.id
		WHERE ria.region_code = $1
		  AND it.is_available = true
		ORDER BY it.gpu_model, it.gpu_count
	`

	rows, err := g.db.Pool.Query(ctx, instanceQuery, regionCode)
	if err != nil {
		g.logger.Error("failed to query instance types", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query instance types")
		return
	}
	defer rows.Close()

	var instanceTypes []map[string]interface{}
	for rows.Next() {
		var id int
		var instanceType, instanceName, gpuModel, stockStatus *string
		var gpuCount int
		var gpuMemoryGB, vcpuCount, memoryGB, pricePerHour, spotPricePerHour *float64
		var supportsSpot, isAvailable bool

		if err := rows.Scan(&id, &instanceType, &instanceName, &gpuModel, &gpuCount,
			&gpuMemoryGB, &vcpuCount, &memoryGB, &pricePerHour, &spotPricePerHour,
			&supportsSpot, &isAvailable, &stockStatus); err != nil {
			g.logger.Warn("failed to scan instance type", zap.Error(err))
			continue
		}

		instanceData := map[string]interface{}{
			"id":            id,
			"gpu_count":     gpuCount,
			"supports_spot": supportsSpot,
			"is_available":  isAvailable,
		}

		if instanceType != nil {
			instanceData["instance_type"] = *instanceType
		}
		if instanceName != nil {
			instanceData["instance_name"] = *instanceName
		}
		if gpuModel != nil {
			instanceData["gpu_model"] = *gpuModel
		}
		if gpuMemoryGB != nil {
			instanceData["gpu_memory_gb"] = *gpuMemoryGB
		}
		if vcpuCount != nil {
			instanceData["vcpu_count"] = *vcpuCount
		}
		if memoryGB != nil {
			instanceData["memory_gb"] = *memoryGB
		}
		if pricePerHour != nil {
			instanceData["price_per_hour"] = *pricePerHour
		}
		if spotPricePerHour != nil {
			instanceData["spot_price_per_hour"] = *spotPricePerHour
		}
		if stockStatus != nil {
			instanceData["stock_status"] = *stockStatus
		}

		instanceTypes = append(instanceTypes, instanceData)
	}

	// Get current node count in region
	var currentNodes int
	g.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM nodes
		WHERE region_id = $1 AND status IN ('active', 'initializing')
	`, regionID).Scan(&currentNodes)

	// Mock quota limits (in production, would query cloud provider API)
	quotaLimits := map[string]interface{}{
		"max_nodes":     100,
		"current_nodes": currentNodes,
		"available_quota": 100 - currentNodes,
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"region_id":          regionID,
		"region_code":        regionCode,
		"region_name":        regionName,
		"provider":           provider,
		"status":             status,
		"available_instances": instanceTypes,
		"quota_limits":       quotaLimits,
		"estimated_launch_time_seconds": 180, // Mock value
	})
}
