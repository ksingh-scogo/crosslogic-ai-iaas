package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// handleCreateDeployment creates a new model deployment
// Platform Admin Only - POST /admin/deployments
// Deploys a model to vLLM on one or more GPU nodes
func (g *Gateway) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ModelName              string `json:"model_name"`
		NodeCount              int    `json:"node_count"`
		Provider               string `json:"provider"`
		Region                 string `json:"region"`
		InstanceType           string `json:"instance_type"`
		UseSpot                bool   `json:"use_spot"`
		LoadBalancingStrategy  string `json:"load_balancing_strategy"` // round-robin, least-latency, least-connections
		AutoScaling            *struct {
			Enabled          bool `json:"enabled"`
			MinNodes         int  `json:"min_nodes"`
			MaxNodes         int  `json:"max_nodes"`
			TargetLatencyMs  int  `json:"target_latency_ms"`
		} `json:"auto_scaling"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.ModelName == "" || req.Provider == "" || req.Region == "" {
		g.writeError(w, http.StatusBadRequest, "model_name, provider, and region are required")
		return
	}

	if req.NodeCount < 1 {
		req.NodeCount = 1
	}

	if req.LoadBalancingStrategy == "" {
		req.LoadBalancingStrategy = "least-latency"
	}

	// Verify model exists
	var modelID uuid.UUID
	err := g.db.Pool.QueryRow(ctx, `
		SELECT id FROM models WHERE name = $1 AND status = 'active'
	`, req.ModelName).Scan(&modelID)

	if err != nil {
		g.logger.Error("model not found",
			zap.Error(err),
			zap.String("model_name", req.ModelName),
		)
		g.writeError(w, http.StatusBadRequest, "model not found or not active")
		return
	}

	// Create deployment record
	deploymentID := uuid.New()
	minReplicas := req.NodeCount
	maxReplicas := req.NodeCount
	autoScalingEnabled := false

	if req.AutoScaling != nil && req.AutoScaling.Enabled {
		autoScalingEnabled = true
		minReplicas = req.AutoScaling.MinNodes
		maxReplicas = req.AutoScaling.MaxNodes
		if minReplicas < 1 {
			minReplicas = 1
		}
		if maxReplicas < minReplicas {
			maxReplicas = minReplicas
		}
	}

	_, err = g.db.Pool.Exec(ctx, `
		INSERT INTO deployments (
			id, name, model_id, min_replicas, max_replicas,
			current_replicas, strategy, provider, region, gpu_type,
			auto_scaling_enabled, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8, $9, $10, 'launching', NOW(), NOW())
	`, deploymentID, req.ModelName+"-deployment", modelID, minReplicas, maxReplicas,
		req.LoadBalancingStrategy, req.Provider, req.Region, req.InstanceType, autoScalingEnabled)

	if err != nil {
		g.logger.Error("failed to create deployment record",
			zap.Error(err),
			zap.String("deployment_id", deploymentID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	g.logger.Info("deployment created, launching nodes",
		zap.String("deployment_id", deploymentID.String()),
		zap.String("model", req.ModelName),
		zap.Int("node_count", req.NodeCount),
	)

	// Launch nodes asynchronously
	go g.launchDeploymentNodes(context.Background(), deploymentID, req.ModelName, req.NodeCount,
		req.Provider, req.Region, req.InstanceType, req.UseSpot)

	g.writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"deployment_id":   deploymentID,
		"status":          "launching",
		"message":         "Deployment initiated. Launching " + string(rune(req.NodeCount)) + " nodes...",
		"estimated_time":  "5-8 minutes",
		"endpoint_url":    "https://api.crosslogic.ai/inference/" + req.ModelName,
	})
}

// launchDeploymentNodes launches nodes for a deployment in the background
func (g *Gateway) launchDeploymentNodes(ctx context.Context, deploymentID uuid.UUID,
	modelName string, nodeCount int, provider, region, instanceType string, useSpot bool) {

	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	successCount := 0
	for i := 0; i < nodeCount; i++ {
		nodeID := uuid.New().String()

		nodeConfig := orchestrator.NodeConfig{
			NodeID:   nodeID,
			Provider: provider,
			Region:   region,
			Model:    modelName,
			GPU:      instanceType,
			UseSpot:  useSpot,
			DiskSize: 256,
		}

		clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)
		if err != nil {
			g.logger.Error("failed to launch node for deployment",
				zap.Error(err),
				zap.String("deployment_id", deploymentID.String()),
				zap.Int("node_index", i),
			)
			continue
		}

		g.logger.Info("node launched for deployment",
			zap.String("deployment_id", deploymentID.String()),
			zap.String("cluster_name", clusterName),
		)

		successCount++
	}

	// Update deployment status
	status := "active"
	if successCount == 0 {
		status = "failed"
	} else if successCount < nodeCount {
		status = "degraded"
	}

	_, err := g.db.Pool.Exec(ctx, `
		UPDATE deployments SET
			current_replicas = $1,
			status = $2,
			updated_at = NOW()
		WHERE id = $3
	`, successCount, status, deploymentID)

	if err != nil {
		g.logger.Error("failed to update deployment status",
			zap.Error(err),
			zap.String("deployment_id", deploymentID.String()),
		)
	}
}

// handleListDeployments lists all model deployments
// Platform Admin Only - GET /admin/deployments
func (g *Gateway) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	modelFilter := r.URL.Query().Get("model")
	statusFilter := r.URL.Query().Get("status")

	query := `
		SELECT d.id, d.name, m.name as model_name, d.status,
		       d.current_replicas, d.min_replicas, d.max_replicas,
		       d.strategy, d.created_at
		FROM deployments d
		INNER JOIN models m ON m.id = d.model_id
		WHERE 1=1
	`

	args := []interface{}{}
	if modelFilter != "" {
		args = append(args, modelFilter)
		query += " AND m.name = $" + string(rune(len(args)))
	}
	if statusFilter != "" {
		args = append(args, statusFilter)
		query += " AND d.status = $" + string(rune(len(args)))
	}

	query += " ORDER BY d.created_at DESC"

	rows, err := g.db.Pool.Query(ctx, query, args...)
	if err != nil {
		g.logger.Error("failed to query deployments", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to query deployments")
		return
	}
	defer rows.Close()

	var deployments []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, modelName, status, strategy string
		var currentReplicas, minReplicas, maxReplicas int
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &modelName, &status, &currentReplicas,
			&minReplicas, &maxReplicas, &strategy, &createdAt); err != nil {
			g.logger.Warn("failed to scan deployment row", zap.Error(err))
			continue
		}

		// Get nodes for this deployment
		var nodes []map[string]interface{}
		nodeRows, err := g.db.Pool.Query(ctx, `
			SELECT n.cluster_name, n.status, n.health_score
			FROM nodes n
			INNER JOIN models m ON m.id = n.model_id
			WHERE m.name = $1
			ORDER BY n.created_at DESC
			LIMIT $2
		`, modelName, maxReplicas)

		if err == nil {
			defer nodeRows.Close()
			for nodeRows.Next() {
				var clusterName, nodeStatus string
				var healthScore float64
				if err := nodeRows.Scan(&clusterName, &nodeStatus, &healthScore); err == nil {
					nodes = append(nodes, map[string]interface{}{
						"cluster_name": clusterName,
						"status":       nodeStatus,
						"health_score": healthScore,
					})
				}
			}
		}

		deployments = append(deployments, map[string]interface{}{
			"id":                        id,
			"name":                      name,
			"model_name":                modelName,
			"status":                    status,
			"node_count":                currentReplicas,
			"min_replicas":              minReplicas,
			"max_replicas":              maxReplicas,
			"load_balancing_strategy":   strategy,
			"created_at":                createdAt,
			"nodes":                     nodes,
		})
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": deployments,
	})
}

// handleGetDeployment gets details for a specific deployment
// Platform Admin Only - GET /admin/deployments/{id}
func (g *Gateway) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deploymentIDStr := chi.URLParam(r, "id")
	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid deployment ID")
		return
	}

	var name, modelName, status, strategy, provider, region string
	var currentReplicas, minReplicas, maxReplicas int
	var createdAt, updatedAt time.Time

	err = g.db.Pool.QueryRow(ctx, `
		SELECT d.name, m.name, d.status, d.current_replicas,
		       d.min_replicas, d.max_replicas, d.strategy,
		       d.provider, d.region, d.created_at, d.updated_at
		FROM deployments d
		INNER JOIN models m ON m.id = d.model_id
		WHERE d.id = $1
	`, deploymentID).Scan(&name, &modelName, &status, &currentReplicas,
		&minReplicas, &maxReplicas, &strategy, &provider, &region, &createdAt, &updatedAt)

	if err != nil {
		g.logger.Error("deployment not found",
			zap.Error(err),
			zap.String("deployment_id", deploymentID.String()),
		)
		g.writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	// Get nodes
	nodeRows, err := g.db.Pool.Query(ctx, `
		SELECT n.id, n.cluster_name, n.status, n.health_score,
		       n.endpoint_url, n.created_at
		FROM nodes n
		INNER JOIN models m ON m.id = n.model_id
		WHERE m.name = $1
		ORDER BY n.created_at DESC
	`, modelName)

	var nodes []map[string]interface{}
	if err == nil {
		defer nodeRows.Close()
		for nodeRows.Next() {
			var nodeID uuid.UUID
			var clusterName, nodeStatus, endpointURL string
			var healthScore float64
			var nodeCreatedAt time.Time

			if err := nodeRows.Scan(&nodeID, &clusterName, &nodeStatus, &healthScore,
				&endpointURL, &nodeCreatedAt); err == nil {
				nodes = append(nodes, map[string]interface{}{
					"id":            nodeID,
					"cluster_name":  clusterName,
					"status":        nodeStatus,
					"health_score":  healthScore,
					"endpoint_url":  endpointURL,
					"created_at":    nodeCreatedAt,
				})
			}
		}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                      deploymentID,
		"name":                    name,
		"model_name":              modelName,
		"status":                  status,
		"node_count":              currentReplicas,
		"min_replicas":            minReplicas,
		"max_replicas":            maxReplicas,
		"load_balancing_strategy": strategy,
		"provider":                provider,
		"region":                  region,
		"created_at":              createdAt,
		"updated_at":              updatedAt,
		"nodes":                   nodes,
	})
}

// handleScaleDeployment scales a deployment up or down
// Platform Admin Only - PUT /admin/deployments/{id}/scale
func (g *Gateway) handleScaleDeployment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deploymentIDStr := chi.URLParam(r, "id")
	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid deployment ID")
		return
	}

	var req struct {
		TargetNodeCount int    `json:"target_node_count"`
		Strategy        string `json:"strategy"` // gradual, immediate
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetNodeCount < 1 {
		g.writeError(w, http.StatusBadRequest, "target_node_count must be at least 1")
		return
	}

	// Get current deployment info
	var currentReplicas int
	var modelName string
	err = g.db.Pool.QueryRow(ctx, `
		SELECT d.current_replicas, m.name
		FROM deployments d
		INNER JOIN models m ON m.id = d.model_id
		WHERE d.id = $1
	`, deploymentID).Scan(&currentReplicas, &modelName)

	if err != nil {
		g.logger.Error("deployment not found", zap.Error(err))
		g.writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	var message string
	if req.TargetNodeCount > currentReplicas {
		message = "Scaling up: launching additional nodes..."
	} else if req.TargetNodeCount < currentReplicas {
		message = "Scaling down: draining and terminating nodes..."
	} else {
		message = "Already at target node count"
	}

	// Update deployment status
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE deployments SET
			status = 'scaling',
			updated_at = NOW()
		WHERE id = $1
	`, deploymentID)

	if err != nil {
		g.logger.Error("failed to update deployment", zap.Error(err))
		g.writeError(w, http.StatusInternalServerError, "failed to update deployment")
		return
	}

	g.logger.Info("deployment scaling initiated",
		zap.String("deployment_id", deploymentID.String()),
		zap.Int("current", currentReplicas),
		zap.Int("target", req.TargetNodeCount),
	)

	// TODO: Implement actual scaling logic via deployment controller
	// For now, just return success

	g.writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":        "scaling",
		"current_nodes": currentReplicas,
		"target_nodes":  req.TargetNodeCount,
		"message":       message,
	})
}

// handleDeleteDeployment removes a deployment
// Platform Admin Only - DELETE /admin/deployments/{id}
func (g *Gateway) handleDeleteDeployment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deploymentIDStr := chi.URLParam(r, "id")
	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid deployment ID")
		return
	}

	var req struct {
		Graceful bool `json:"graceful"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Graceful {
		req.Graceful = true // Default to graceful
	}

	// Mark deployment as terminating
	_, err = g.db.Pool.Exec(ctx, `
		UPDATE deployments SET
			status = 'terminating',
			updated_at = NOW()
		WHERE id = $1
	`, deploymentID)

	if err != nil {
		g.logger.Error("deployment not found", zap.Error(err))
		g.writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	g.logger.Info("deployment termination initiated",
		zap.String("deployment_id", deploymentID.String()),
		zap.Bool("graceful", req.Graceful),
	)

	// TODO: Implement actual node termination via orchestrator
	// For now, just return success

	g.writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "terminating",
		"message": "Draining nodes and terminating deployment...",
	})
}
