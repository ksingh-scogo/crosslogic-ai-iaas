package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleListRoutes lists all inference routes/endpoints
// Platform Admin Only - GET /admin/routes
func (g *Gateway) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query models with active nodes
	rows, err := g.db.Pool.Query(ctx, `
		SELECT
			m.id,
			m.name,
			COUNT(n.id) FILTER (WHERE n.status = 'active' AND n.health_score >= 50) as healthy_nodes,
			COUNT(n.id) as total_nodes,
			AVG(ur.latency_ms) FILTER (WHERE ur.timestamp > NOW() - INTERVAL '1 hour') as avg_latency
		FROM models m
		LEFT JOIN nodes n ON n.model_id = m.id
		LEFT JOIN usage_records ur ON ur.model_id = m.id
		WHERE m.status = 'active'
		GROUP BY m.id, m.name
		HAVING COUNT(n.id) > 0
		ORDER BY m.name
	`)

	if err != nil {
		g.logger.Error("failed to query routes", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query routes")
		return
	}
	defer rows.Close()

	var routes []map[string]interface{}
	for rows.Next() {
		var modelID uuid.UUID
		var modelName string
		var healthyNodes, totalNodes int
		var avgLatency *float64

		if err := rows.Scan(&modelID, &modelName, &healthyNodes, &totalNodes, &avgLatency); err != nil {
			g.logger.Warn("failed to scan route row", zap.Error(err))
			continue
		}

		// Get strategy from deployment or default
		var strategy string
		err = g.db.Pool.QueryRow(ctx, `
			SELECT strategy FROM deployments d
			WHERE d.model_id = $1 AND d.status IN ('active', 'scaling')
			ORDER BY d.created_at DESC LIMIT 1
		`, modelID).Scan(&strategy)

		if err != nil || strategy == "" {
			strategy = "least-latency" // Default
		}

		// Calculate requests per second from recent usage
		var rps *float64
		g.db.Pool.QueryRow(ctx, `
			SELECT COUNT(*)::float / 60.0
			FROM usage_records
			WHERE model_id = $1
			  AND timestamp > NOW() - INTERVAL '1 minute'
		`, modelID).Scan(&rps)

		route := map[string]interface{}{
			"model_id":           modelID,
			"model_name":         modelName,
			"endpoint_url":       "https://api.crosslogic.ai/inference/" + modelName,
			"strategy":           strategy,
			"healthy_nodes":      healthyNodes,
			"total_nodes":        totalNodes,
		}

		if avgLatency != nil {
			route["avg_latency_ms"] = *avgLatency
		}

		if rps != nil {
			route["requests_per_second"] = *rps
		}

		routes = append(routes, route)
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": routes,
	})
}

// handleGetRoute gets routing configuration for a specific model
// Platform Admin Only - GET /admin/routes/{model_id}
func (g *Gateway) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	modelIDStr := chi.URLParam(r, "model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID")
		return
	}

	// Get model info
	var modelName string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT name FROM models WHERE id = $1 AND status = 'active'
	`, modelID).Scan(&modelName)

	if err != nil {
		g.logger.Error("model not found", zap.Error(err))
		g.writeError(w, http.StatusNotFound, "model not found")
		return
	}

	// Get deployment config
	var strategy string
	var autoScalingEnabled bool
	err = g.db.Pool.QueryRow(ctx, `
		SELECT strategy, auto_scaling_enabled
		FROM deployments
		WHERE model_id = $1 AND status IN ('active', 'scaling')
		ORDER BY created_at DESC LIMIT 1
	`, modelID).Scan(&strategy, &autoScalingEnabled)

	if err != nil {
		// No deployment exists, use defaults
		strategy = "least-latency"
		autoScalingEnabled = false
	}

	// Get node stats
	var healthyNodes, totalNodes int
	g.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'active' AND health_score >= 50),
			COUNT(*)
		FROM nodes
		WHERE model_id = $1
	`, modelID).Scan(&healthyNodes, &totalNodes)

	// Get node details with weights
	nodeRows, err := g.db.Pool.Query(ctx, `
		SELECT
			n.id,
			n.cluster_name,
			n.endpoint_url,
			n.health_score,
			n.status,
			COALESCE(AVG(ur.latency_ms) FILTER (WHERE ur.timestamp > NOW() - INTERVAL '10 minutes'), 0) as avg_latency
		FROM nodes n
		LEFT JOIN usage_records ur ON ur.node_id = n.id
		WHERE n.model_id = $1
		GROUP BY n.id, n.cluster_name, n.endpoint_url, n.health_score, n.status
		ORDER BY n.created_at
	`, modelID)

	var nodes []map[string]interface{}
	if err == nil {
		defer nodeRows.Close()
		for nodeRows.Next() {
			var nodeID uuid.UUID
			var clusterName, endpointURL, status string
			var healthScore, avgLatency float64

			if err := nodeRows.Scan(&nodeID, &clusterName, &endpointURL, &healthScore, &status, &avgLatency); err == nil {
				weight := 1.0
				if strategy == "weighted" {
					// Calculate weight based on health score and latency
					weight = healthScore / 100.0
					if avgLatency > 0 {
						weight *= (100.0 / avgLatency)
					}
				}

				nodes = append(nodes, map[string]interface{}{
					"node_id":        nodeID,
					"cluster_name":   clusterName,
					"endpoint_url":   endpointURL,
					"health_score":   healthScore,
					"status":         status,
					"avg_latency_ms": avgLatency,
					"weight":         weight,
				})
			}
		}
	}

	config := map[string]interface{}{
		"model_id":    modelID,
		"model_name":  modelName,
		"strategy":    strategy,
		"health_check": map[string]interface{}{
			"interval_seconds":     10,
			"timeout_seconds":      5,
			"unhealthy_threshold":  3,
			"healthy_threshold":    2,
		},
		"sticky_sessions":        false,
		"connection_timeout_ms":  30000,
		"auto_scaling_enabled":   autoScalingEnabled,
		"healthy_nodes":          healthyNodes,
		"total_nodes":            totalNodes,
		"nodes":                  nodes,
	}

	g.writeJSON(w, http.StatusOK, config)
}

// handleUpdateRoute updates routing strategy for a model
// Platform Admin Only - PUT /admin/routes/{model_id}
func (g *Gateway) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	modelIDStr := chi.URLParam(r, "model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid model ID")
		return
	}

	var req struct {
		Strategy    string `json:"strategy"` // round-robin, least-latency, least-connections, weighted
		HealthCheck *struct {
			IntervalSeconds    int `json:"interval_seconds"`
			TimeoutSeconds     int `json:"timeout_seconds"`
			UnhealthyThreshold int `json:"unhealthy_threshold"`
			HealthyThreshold   int `json:"healthy_threshold"`
		} `json:"health_check"`
		StickySessions       bool `json:"sticky_sessions"`
		ConnectionTimeoutMs  int  `json:"connection_timeout_ms"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate strategy
	validStrategies := map[string]bool{
		"round-robin":      true,
		"least-latency":    true,
		"least-connections": true,
		"weighted":         true,
	}

	if req.Strategy != "" && !validStrategies[req.Strategy] {
		g.writeError(w, http.StatusBadRequest, "invalid strategy. Must be one of: round-robin, least-latency, least-connections, weighted")
		return
	}

	// Verify model exists
	var modelName string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT name FROM models WHERE id = $1 AND status = 'active'
	`, modelID).Scan(&modelName)

	if err != nil {
		g.logger.Error("model not found", zap.Error(err))
		g.writeError(w, http.StatusNotFound, "model not found")
		return
	}

	// Update deployment strategy if exists
	result, err := g.db.Pool.Exec(ctx, `
		UPDATE deployments
		SET strategy = $1, updated_at = NOW()
		WHERE model_id = $2 AND status IN ('active', 'scaling')
	`, req.Strategy, modelID)

	if err != nil {
		g.logger.Error("failed to update routing strategy", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to update routing strategy")
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// No deployment exists, create routing config entry
		_, err = g.db.Pool.Exec(ctx, `
			INSERT INTO routing_configs (model_id, strategy, created_at, updated_at)
			VALUES ($1, $2, NOW(), NOW())
			ON CONFLICT (model_id) DO UPDATE SET
				strategy = EXCLUDED.strategy,
				updated_at = NOW()
		`, modelID, req.Strategy)

		if err != nil {
			g.logger.Error("failed to create routing config", zap.Error(err))
			g.writeError(w, http.StatusInternalServerError, "failed to update routing config")
			return
		}
	}

	g.logger.Info("routing strategy updated",
		zap.String("model_id", modelID.String()),
		zap.String("model_name", modelName),
		zap.String("strategy", req.Strategy),
	)

	// Return updated config
	g.handleGetRoute(w, r)
}
