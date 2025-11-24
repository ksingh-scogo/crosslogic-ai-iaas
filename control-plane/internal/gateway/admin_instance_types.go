package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// handleCreateInstanceType creates a new instance type
// Admin API - POST /admin/instance-types
func (g *Gateway) handleCreateInstanceType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Provider             string  `json:"provider"`
		InstanceType         string  `json:"instance_type"`
		InstanceName         string  `json:"instance_name"`
		VCPUCount            int     `json:"vcpu_count"`
		MemoryGB             float64 `json:"memory_gb"`
		GPUCount             int     `json:"gpu_count"`
		GPUMemoryGB          float64 `json:"gpu_memory_gb"`
		GPUModel             string  `json:"gpu_model"`
		GPUComputeCapability string  `json:"gpu_compute_capability"`
		PricePerHour         float64 `json:"price_per_hour"`
		SpotPricePerHour     float64 `json:"spot_price_per_hour"`
		Available            bool    `json:"available"`
		SupportsSpot         bool    `json:"supports_spot"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Provider == "" || req.InstanceType == "" || req.GPUModel == "" {
		g.writeError(w, http.StatusBadRequest, "provider, instance_type, and gpu_model are required")
		return
	}

	if req.VCPUCount <= 0 || req.MemoryGB <= 0 || req.GPUCount <= 0 || req.GPUMemoryGB <= 0 {
		g.writeError(w, http.StatusBadRequest, "vcpu_count, memory_gb, gpu_count, and gpu_memory_gb must be positive")
		return
	}

	// Validate provider
	validProviders := map[string]bool{"aws": true, "azure": true, "gcp": true, "oci": true}
	if !validProviders[req.Provider] {
		g.writeError(w, http.StatusBadRequest, "invalid provider. Valid values: aws, azure, gcp, oci")
		return
	}

	// Check if instance type already exists for this provider
	var existingID int
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id FROM instance_types
		WHERE provider = $1 AND instance_type = $2
	`, req.Provider, req.InstanceType).Scan(&existingID)

	if err == nil {
		// Instance type already exists
		g.writeError(w, http.StatusConflict, "instance type already exists for this provider")
		return
	}

	// Set defaults
	if req.SupportsSpot && req.SpotPricePerHour == 0 {
		req.SpotPricePerHour = req.PricePerHour * 0.3 // Default 30% of on-demand
	}

	// Insert instance type
	var instanceTypeID int
	err = g.db.Pool.QueryRow(ctx, `
		INSERT INTO instance_types (
			provider, instance_type, instance_name,
			vcpu_count, memory_gb, gpu_count, gpu_memory_gb,
			gpu_model, gpu_compute_capability,
			price_per_hour, spot_price_per_hour,
			is_available, supports_spot
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`, req.Provider, req.InstanceType, req.InstanceName,
		req.VCPUCount, req.MemoryGB, req.GPUCount, req.GPUMemoryGB,
		req.GPUModel, req.GPUComputeCapability,
		req.PricePerHour, req.SpotPricePerHour,
		req.Available, req.SupportsSpot).Scan(&instanceTypeID)

	if err != nil {
		g.logger.Error("failed to create instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to create instance type")
		return
	}

	g.logger.Info("instance type created",
		zap.Int("instance_type_id", instanceTypeID),
		zap.String("provider", req.Provider),
		zap.String("instance_type", req.InstanceType),
	)

	g.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                     instanceTypeID,
		"provider":               req.Provider,
		"instance_type":          req.InstanceType,
		"instance_name":          req.InstanceName,
		"gpu_model":              req.GPUModel,
		"gpu_count":              req.GPUCount,
		"gpu_memory_gb":          req.GPUMemoryGB,
		"vcpu_count":             req.VCPUCount,
		"memory_gb":              req.MemoryGB,
		"price_per_hour":         req.PricePerHour,
		"spot_price_per_hour":    req.SpotPricePerHour,
		"available":              req.Available,
		"supports_spot":          req.SupportsSpot,
	})
}

// handleUpdateInstanceType updates an existing instance type
// Admin API - PUT /admin/instance-types/{id}
func (g *Gateway) handleUpdateInstanceType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instanceTypeIDStr := chi.URLParam(r, "id")

	var instanceTypeID int
	if _, err := fmt.Sscanf(instanceTypeIDStr, "%d", &instanceTypeID); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance type ID")
		return
	}

	var req struct {
		PricePerHour     *float64               `json:"price_per_hour"`
		SpotPricePerHour *float64               `json:"spot_price_per_hour"`
		Available        *bool                  `json:"available"`
		Metadata         map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if instance type exists
	var existingProvider, existingInstanceType string
	err := g.db.Pool.QueryRow(ctx, `
		SELECT provider, instance_type
		FROM instance_types
		WHERE id = $1
	`, instanceTypeID).Scan(&existingProvider, &existingInstanceType)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "instance type not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query instance type")
		return
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.PricePerHour != nil {
		updates = append(updates, fmt.Sprintf("price_per_hour = $%d", argNum))
		args = append(args, *req.PricePerHour)
		argNum++
	}

	if req.SpotPricePerHour != nil {
		updates = append(updates, fmt.Sprintf("spot_price_per_hour = $%d", argNum))
		args = append(args, *req.SpotPricePerHour)
		argNum++
	}

	if req.Available != nil {
		updates = append(updates, fmt.Sprintf("is_available = $%d", argNum))
		args = append(args, *req.Available)
		argNum++
	}

	if len(updates) == 0 {
		g.writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	// Add updated_at
	updates = append(updates, "updated_at = NOW()")

	// Build and execute query
	args = append(args, instanceTypeID)
	query := "UPDATE instance_types SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += fmt.Sprintf(" WHERE id = $%d", argNum)

	_, err = g.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to update instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to update instance type")
		return
	}

	g.logger.Info("instance type updated",
		zap.Int("instance_type_id", instanceTypeID),
		zap.String("provider", existingProvider),
		zap.String("instance_type", existingInstanceType),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"message": "instance type updated successfully",
	})
}

// handleDeleteInstanceType deletes an instance type if not in use
// Admin API - DELETE /admin/instance-types/{id}
func (g *Gateway) handleDeleteInstanceType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instanceTypeIDStr := chi.URLParam(r, "id")

	var instanceTypeID int
	if _, err := fmt.Sscanf(instanceTypeIDStr, "%d", &instanceTypeID); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance type ID")
		return
	}

	// Get instance type name for checking usage
	var instanceTypeName string
	err := g.db.Pool.QueryRow(ctx, `
		SELECT instance_type FROM instance_types WHERE id = $1
	`, instanceTypeID).Scan(&instanceTypeName)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "instance type not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query instance type")
		return
	}

	// Check if any active nodes use this instance type
	var activeNodeCount int
	err = g.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM nodes
		WHERE instance_type = $1 AND status IN ('active', 'initializing', 'draining')
	`, instanceTypeName).Scan(&activeNodeCount)

	if err != nil {
		g.logger.Error("failed to query nodes", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query nodes")
		return
	}

	if activeNodeCount > 0 {
		g.writeError(w, http.StatusConflict, fmt.Sprintf("cannot delete instance type with %d active nodes", activeNodeCount))
		return
	}

	// Soft delete by marking as unavailable
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE instance_types
		SET is_available = false, updated_at = NOW()
		WHERE id = $1
	`, instanceTypeID)

	if err != nil {
		g.logger.Error("failed to delete instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to delete instance type")
		return
	}

	g.logger.Info("instance type deleted (soft)",
		zap.Int("instance_type_id", instanceTypeID),
		zap.String("instance_type", instanceTypeName),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"message": "instance type deleted successfully",
	})
}

// handleAssociateInstanceTypeRegions associates instance type with multiple regions
// Admin API - POST /admin/instance-types/{id}/regions
func (g *Gateway) handleAssociateInstanceTypeRegions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instanceTypeIDStr := chi.URLParam(r, "id")

	var instanceTypeID int
	if _, err := fmt.Sscanf(instanceTypeIDStr, "%d", &instanceTypeID); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance type ID")
		return
	}

	var req struct {
		RegionCodes []string `json:"region_codes"`
		IsAvailable bool     `json:"is_available"`
		StockStatus string   `json:"stock_status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.RegionCodes) == 0 {
		g.writeError(w, http.StatusBadRequest, "region_codes is required")
		return
	}

	// Set defaults
	if req.StockStatus == "" {
		req.StockStatus = "available"
	}

	// Validate stock status
	validStockStatus := map[string]bool{"available": true, "limited": true, "out_of_stock": true}
	if !validStockStatus[req.StockStatus] {
		g.writeError(w, http.StatusBadRequest, "invalid stock_status. Valid values: available, limited, out_of_stock")
		return
	}

	// Check if instance type exists
	var exists bool
	err := g.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM instance_types WHERE id = $1)
	`, instanceTypeID).Scan(&exists)

	if err != nil || !exists {
		g.writeError(w, http.StatusNotFound, "instance type not found")
		return
	}

	// Begin transaction for bulk insert
	tx, err := g.db.Pool.Begin(ctx)
	if err != nil {
		g.logger.Error("failed to begin transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Insert or update associations
	insertedCount := 0
	updatedCount := 0

	for _, regionCode := range req.RegionCodes {
		// Try insert, on conflict update
		result, err := tx.Exec(ctx, `
			INSERT INTO region_instance_availability (
				region_code, instance_type_id, is_available, stock_status
			) VALUES ($1, $2, $3, $4)
			ON CONFLICT (region_code, instance_type_id)
			DO UPDATE SET
				is_available = EXCLUDED.is_available,
				stock_status = EXCLUDED.stock_status,
				updated_at = NOW()
		`, regionCode, instanceTypeID, req.IsAvailable, req.StockStatus)

		if err != nil {
			g.logger.Warn("failed to associate region",
				zap.Error(err),
				zap.String("region_code", regionCode),
			)
			continue
		}

		rowsAffected := result.RowsAffected()
		if rowsAffected > 0 {
			insertedCount++
		} else {
			updatedCount++
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		g.logger.Error("failed to commit transaction", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	g.logger.Info("instance type regions associated",
		zap.Int("instance_type_id", instanceTypeID),
		zap.Int("inserted", insertedCount),
		zap.Int("updated", updatedCount),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"message":  "regions associated successfully",
		"inserted": insertedCount,
		"updated":  updatedCount,
	})
}

// handleGetInstanceTypePricing returns pricing information across all regions
// Admin API - GET /admin/instance-types/{id}/pricing
func (g *Gateway) handleGetInstanceTypePricing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instanceTypeIDStr := chi.URLParam(r, "id")

	var instanceTypeID int
	if _, err := fmt.Sscanf(instanceTypeIDStr, "%d", &instanceTypeID); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance type ID")
		return
	}

	// Get instance type details
	var provider, instanceType, instanceName, gpuModel string
	var gpuCount int
	var basePricePerHour, baseSpotPricePerHour float64
	var supportsSpot bool

	err := g.db.Pool.QueryRow(ctx, `
		SELECT
			provider, instance_type, instance_name, gpu_model, gpu_count,
			price_per_hour, spot_price_per_hour, supports_spot
		FROM instance_types
		WHERE id = $1
	`, instanceTypeID).Scan(&provider, &instanceType, &instanceName, &gpuModel,
		&gpuCount, &basePricePerHour, &baseSpotPricePerHour, &supportsSpot)

	if err == sql.ErrNoRows {
		g.writeError(w, http.StatusNotFound, "instance type not found")
		return
	}
	if err != nil {
		g.logger.Error("failed to query instance type", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query instance type")
		return
	}

	// Query pricing across regions
	query := `
		SELECT
			r.code,
			r.name,
			r.city,
			r.country,
			r.cost_multiplier,
			ria.is_available,
			ria.stock_status
		FROM region_instance_availability ria
		INNER JOIN regions r ON r.code = ria.region_code
		WHERE ria.instance_type_id = $1
		  AND r.status = 'active'
		ORDER BY r.name
	`

	rows, err := g.db.Pool.Query(ctx, query, instanceTypeID)
	if err != nil {
		g.logger.Error("failed to query regional pricing", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query pricing")
		return
	}
	defer rows.Close()

	var regionalPricing []map[string]interface{}
	for rows.Next() {
		var regionCode, regionName string
		var city, country *string
		var costMultiplier float64
		var isAvailable bool
		var stockStatus string

		if err := rows.Scan(&regionCode, &regionName, &city, &country,
			&costMultiplier, &isAvailable, &stockStatus); err != nil {
			g.logger.Warn("failed to scan regional pricing", zap.Error(err))
			continue
		}

		// Calculate regional pricing
		regionalOnDemand := basePricePerHour * costMultiplier
		regionalSpot := baseSpotPricePerHour * costMultiplier

		location := ""
		if city != nil && *city != "" {
			location = *city
			if country != nil && *country != "" {
				location += ", " + *country
			}
		} else if country != nil && *country != "" {
			location = *country
		}

		regionalPricing = append(regionalPricing, map[string]interface{}{
			"region_code":         regionCode,
			"region_name":         regionName,
			"location":            location,
			"is_available":        isAvailable,
			"stock_status":        stockStatus,
			"pricing_multiplier":  costMultiplier,
			"on_demand_price":     regionalOnDemand,
			"spot_price":          regionalSpot,
			"spot_savings_pct":    ((regionalOnDemand - regionalSpot) / regionalOnDemand) * 100,
		})
	}

	// Calculate average and range
	var minOnDemand, maxOnDemand, avgOnDemand float64
	var minSpot, maxSpot, avgSpot float64
	if len(regionalPricing) > 0 {
		minOnDemand = regionalPricing[0]["on_demand_price"].(float64)
		maxOnDemand = minOnDemand
		minSpot = regionalPricing[0]["spot_price"].(float64)
		maxSpot = minSpot

		totalOnDemand := 0.0
		totalSpot := 0.0

		for _, rp := range regionalPricing {
			onDemand := rp["on_demand_price"].(float64)
			spot := rp["spot_price"].(float64)

			if onDemand < minOnDemand {
				minOnDemand = onDemand
			}
			if onDemand > maxOnDemand {
				maxOnDemand = onDemand
			}
			if spot < minSpot {
				minSpot = spot
			}
			if spot > maxSpot {
				maxSpot = spot
			}

			totalOnDemand += onDemand
			totalSpot += spot
		}

		avgOnDemand = totalOnDemand / float64(len(regionalPricing))
		avgSpot = totalSpot / float64(len(regionalPricing))
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"instance_type_id": instanceTypeID,
		"provider":         provider,
		"instance_type":    instanceType,
		"instance_name":    instanceName,
		"gpu_model":        gpuModel,
		"gpu_count":        gpuCount,
		"supports_spot":    supportsSpot,
		"base_pricing": map[string]interface{}{
			"on_demand_price": basePricePerHour,
			"spot_price":      baseSpotPricePerHour,
		},
		"pricing_range": map[string]interface{}{
			"on_demand": map[string]interface{}{
				"min": minOnDemand,
				"max": maxOnDemand,
				"avg": avgOnDemand,
			},
			"spot": map[string]interface{}{
				"min": minSpot,
				"max": maxSpot,
				"avg": avgSpot,
			},
		},
		"regional_pricing": regionalPricing,
	})
}
