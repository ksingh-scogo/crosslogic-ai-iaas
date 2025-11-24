package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleListTenantEndpoints lists available inference endpoints for the tenant
// Tenant API - GET /v1/endpoints
// Returns models that have active healthy nodes
func (g *Gateway) handleListTenantEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context (for future tenant-specific filtering if needed)
	_, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	// Get optional type filter
	modelType := r.URL.Query().Get("type")

	// Query models that have active nodes
	query := `
		SELECT DISTINCT m.id, m.name, m.family, m.type, m.context_length,
		       m.price_input_per_million, m.price_output_per_million,
		       m.tokens_per_second_capacity, m.status
		FROM models m
		INNER JOIN nodes n ON n.model_id = m.id
		WHERE m.status = 'active'
		  AND n.status = 'active'
		  AND n.health_score >= 50.0
	`

	args := []interface{}{}
	if modelType != "" {
		query += " AND m.type = $1"
		args = append(args, modelType)
	}

	query += " ORDER BY m.family, m.name"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query endpoints", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query endpoints")
		return
	}
	defer rows.Close()

	var endpoints []map[string]interface{}
	for rows.Next() {
		var modelID uuid.UUID
		var name, family, mType, status string
		var contextLength int
		var priceInput, priceOutput float64
		var tps *int

		if err := rows.Scan(&modelID, &name, &family, &mType, &contextLength,
			&priceInput, &priceOutput, &tps, &status); err != nil {
			g.logger.Warn("failed to scan endpoint row", zap.Error(err))
			continue
		}

		// Get node stats for this model
		var healthyNodes int
		var avgLatency *float64
		err = g.db.Pool.QueryRow(ctx, `
			SELECT COUNT(*), AVG(ur.latency_ms)
			FROM nodes n
			LEFT JOIN usage_records ur ON ur.node_id = n.id
			  AND ur.timestamp > NOW() - INTERVAL '1 hour'
			WHERE n.model_id = $1 AND n.status = 'active'
		`, modelID).Scan(&healthyNodes, &avgLatency)

		capacityTPS := 0
		if tps != nil && healthyNodes > 0 {
			capacityTPS = *tps * healthyNodes
		}

		endpoint := map[string]interface{}{
			"model_id":                  modelID,
			"model_name":                name,
			"family":                    family,
			"type":                      mType,
			"context_length":            contextLength,
			"price_input_per_million":   priceInput,
			"price_output_per_million":  priceOutput,
			"status":                    "available",
			"healthy_nodes":             healthyNodes,
			"capacity_tps":              capacityTPS,
			"description":               generateModelDescription(name, family),
		}

		if avgLatency != nil {
			endpoint["avg_latency_ms"] = *avgLatency
		}

		endpoints = append(endpoints, endpoint)
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": endpoints,
	})
}

// handleGetTenantEndpoint gets details for a specific inference endpoint
// Tenant API - GET /v1/endpoints/{model_id}
func (g *Gateway) handleGetTenantEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tenant_id from context
	_, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "tenant ID not found in context")
		return
	}

	modelIDOrName := chi.URLParam(r, "model_id")

	// Try to parse as UUID first, otherwise treat as name
	var modelID uuid.UUID
	var name, family, mType, status string
	var contextLength int
	var priceInput, priceOutput float64
	var tps *int

	modelUUID, err := uuid.Parse(modelIDOrName)
	if err == nil {
		// Query by UUID
		err = g.db.Pool.QueryRow(ctx, `
			SELECT id, name, family, type, context_length,
			       price_input_per_million, price_output_per_million,
			       tokens_per_second_capacity, status
			FROM models
			WHERE id = $1 AND status = 'active'
		`, modelUUID).Scan(&modelID, &name, &family, &mType, &contextLength,
			&priceInput, &priceOutput, &tps, &status)
	} else {
		// Query by name
		err = g.db.Pool.QueryRow(ctx, `
			SELECT id, name, family, type, context_length,
			       price_input_per_million, price_output_per_million,
			       tokens_per_second_capacity, status
			FROM models
			WHERE name = $1 AND status = 'active'
		`, modelIDOrName).Scan(&modelID, &name, &family, &mType, &contextLength,
			&priceInput, &priceOutput, &tps, &status)
	}

	if err != nil {
		g.logger.Warn("model not found",
			zap.Error(err),
			zap.String("model_id_or_name", modelIDOrName),
		)
		g.writeError(w, http.StatusNotFound, "model not found")
		return
	}

	// Check if model has active nodes
	var healthyNodes int
	var totalNodes int
	err = g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'active' AND health_score >= 50.0) as healthy,
			COUNT(*) as total
		FROM nodes
		WHERE model_id = $1
	`, modelID).Scan(&healthyNodes, &totalNodes)

	if err != nil {
		g.logger.Error("failed to query node stats", zap.Error(err))
	}

	if healthyNodes == 0 {
		g.writeError(w, http.StatusServiceUnavailable, "no healthy nodes available for this model")
		return
	}

	// Get latency stats from recent usage
	var avgLatency, p50, p95, p99 *float64
	err = g.db.Pool.QueryRow(ctx, `
		SELECT
			AVG(latency_ms),
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency_ms),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms),
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms)
		FROM usage_records
		WHERE model_id = $1 AND timestamp > NOW() - INTERVAL '1 hour'
	`, modelID).Scan(&avgLatency, &p50, &p95, &p99)

	capacityTPS := 0
	if tps != nil && healthyNodes > 0 {
		capacityTPS = *tps * healthyNodes
	}

	endpoint := map[string]interface{}{
		"model_id":                  modelID,
		"model_name":                name,
		"family":                    family,
		"type":                      mType,
		"context_length":            contextLength,
		"price_input_per_million":   priceInput,
		"price_output_per_million":  priceOutput,
		"status":                    "available",
		"healthy_nodes":             healthyNodes,
		"total_nodes":               totalNodes,
		"capacity_tps":              capacityTPS,
		"description":               generateModelDescription(name, family),
	}

	if avgLatency != nil {
		endpoint["avg_latency_ms"] = *avgLatency
	}
	if p50 != nil {
		endpoint["p50_latency_ms"] = *p50
	}
	if p95 != nil {
		endpoint["p95_latency_ms"] = *p95
	}
	if p99 != nil {
		endpoint["p99_latency_ms"] = *p99
	}

	g.writeJSON(w, http.StatusOK, endpoint)
}

// generateModelDescription creates a friendly description for a model
func generateModelDescription(name, family string) string {
	descriptions := map[string]string{
		"Llama":   "Meta's Llama series - powerful open-source language models",
		"Mistral": "Mistral AI's efficient and capable language models",
		"Qwen":    "Alibaba's Qwen series - multilingual language models",
		"Gemma":   "Google's Gemma - lightweight open models",
		"GPT":     "OpenAI GPT-compatible models",
	}

	if desc, ok := descriptions[family]; ok {
		return desc
	}

	return "AI language model for " + name
}
