package gateway

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// ListR2ModelsHandler lists all models available in R2 bucket
func (g *Gateway) ListR2ModelsHandler(w http.ResponseWriter, r *http.Request) {
	// This endpoint lists models from R2
	// In production, this would query R2 bucket for available models
	
	g.logger.Info("listing models from R2")
	
	// For now, return models from database that are marked as "in_r2"
	// In production, you'd integrate with AWS S3 API to list R2 bucket contents
	
	query := `
		SELECT id, name, family, size, type, context_length, 
		       vram_required_gb, status
		FROM models 
		WHERE status = 'active'
		ORDER BY name
	`
	
	rows, err := g.db.Pool.Query(r.Context(), query)
	if err != nil {
		g.logger.Error("failed to query models", zap.Error(err))
		http.Error(w, "Failed to list models", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	
	type ModelInfo struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		Family         string  `json:"family"`
		Size           *string `json:"size"`
		Type           string  `json:"type"`
		ContextLength  int     `json:"context_length"`
		VRAMRequiredGB int     `json:"vram_required_gb"`
		Status         string  `json:"status"`
	}
	
	models := []ModelInfo{}
	for rows.Next() {
		var m ModelInfo
		if err := rows.Scan(&m.ID, &m.Name, &m.Family, &m.Size, &m.Type, 
			&m.ContextLength, &m.VRAMRequiredGB, &m.Status); err != nil {
			g.logger.Error("failed to scan model", zap.Error(err))
			continue
		}
		models = append(models, m)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// LaunchModelInstanceHandler launches a GPU instance for a specific model
func (g *Gateway) LaunchModelInstanceHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req struct {
		ModelName    string `json:"model_name"`
		Provider     string `json:"provider"`      // aws, azure, gcp
		Region       string `json:"region"`        // us-east-1, eastus, etc
		InstanceType string `json:"instance_type"` // g4dn.xlarge, Standard_NV36ads_A10_v5
		UseSpot      bool   `json:"use_spot"`
		GPU          string `json:"gpu"`       // A10, T4, V100, etc
		GPUCount     int    `json:"gpu_count"` // default: 1
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Validate required fields
	if req.ModelName == "" || req.Provider == "" || req.Region == "" {
		http.Error(w, "model_name, provider, and region are required", http.StatusBadRequest)
		return
	}
	
	// Set defaults
	if req.GPUCount == 0 {
		req.GPUCount = 1
	}
	if req.GPU == "" {
		// Auto-detect GPU based on provider and instance type
		req.GPU = g.detectGPUType(req.Provider, req.InstanceType)
	}
	
	g.logger.Info("launching GPU instance",
		zap.String("model", req.ModelName),
		zap.String("provider", req.Provider),
		zap.String("region", req.Region),
		zap.String("instance_type", req.InstanceType),
	)
	
	// Create launch task using orchestrator
	nodeConfig := struct {
		ModelName    string
		Provider     string
		Region       string
		InstanceType string
		GPU          string
		GPUCount     int
		UseSpot      bool
	}{
		ModelName:    req.ModelName,
		Provider:     req.Provider,
		Region:       req.Region,
		InstanceType: req.InstanceType,
		GPU:          req.GPU,
		GPUCount:     req.GPUCount,
		UseSpot:      req.UseSpot,
	}
	
	// In production, this would:
	// 1. Generate SkyPilot YAML
	// 2. Execute sky launch via subprocess or API
	// 3. Track launch status
	// 4. Return job ID for status polling
	
	// For now, return mock response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "launching",
		"message": "GPU instance launch initiated",
		"details": nodeConfig,
		"job_id":  "launch-" + generateJobID(),
		"estimated_time": "5-10 minutes",
	})
}

// GetLaunchStatusHandler gets the status of a GPU instance launch
func (g *Gateway) GetLaunchStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Get job ID from URL
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id parameter required", http.StatusBadRequest)
		return
	}
	
	g.logger.Info("checking launch status", zap.String("job_id", jobID))
	
	// In production, this would query the actual launch status
	// For now, return mock status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": jobID,
		"status": "in_progress",
		"stage":  "provisioning_instance",
		"progress": 45,
		"stages": []string{
			"✓ Validating configuration",
			"✓ Requesting spot instance",
			"→ Provisioning instance (45%)",
			"  Installing dependencies",
			"  Starting vLLM",
			"  Registering node",
		},
	})
}

// Helper functions
func (g *Gateway) detectGPUType(provider, instanceType string) string {
	// Auto-detect GPU based on instance type
	gpuMap := map[string]string{
		// AWS
		"g4dn":  "T4",
		"g5":    "A10G",
		"p3":    "V100",
		"p4":    "A100",
		// Azure
		"Standard_NC": "K80",
		"Standard_ND": "P40",
		"Standard_NV": "M60",
		"Standard_NV36ads_A10": "A10",
		// GCP
		"n1-standard": "T4",
		"a2-highgpu":  "A100",
	}
	
	for prefix, gpu := range gpuMap {
		if len(instanceType) >= len(prefix) && instanceType[:len(prefix)] == prefix {
			return gpu
		}
	}
	
	return "Unknown"
}

func generateJobID() string {
	// Generate unique job ID
	// In production, use UUID or similar
	return "abc123xyz"
}

