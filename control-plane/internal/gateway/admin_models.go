package gateway

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Mock launch job tracker for demo purposes
type LaunchJob struct {
	JobID       string
	Status      string
	Progress    int
	Stage       string
	Stages      []string
	StartTime   time.Time
	ModelName   string
	Provider    string
	Region      string
}

var (
	launchJobs = make(map[string]*LaunchJob)
	jobsMutex  sync.RWMutex
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
	
	// In production, this would:
	// 1. Generate SkyPilot YAML
	// 2. Execute sky launch via subprocess or API
	// 3. Track launch status
	// 4. Return job ID for status polling
	
	// For now, simulate launch with mock job tracking
	jobID := "launch-" + uuid.New().String()[:8]
	
	// Create mock job
	job := &LaunchJob{
		JobID:     jobID,
		Status:    "in_progress",
		Progress:  0,
		Stage:     "validating",
		StartTime: time.Now(),
		ModelName: req.ModelName,
		Provider:  req.Provider,
		Region:    req.Region,
		Stages: []string{
			"→ Validating configuration",
			"  Requesting spot instance",
			"  Provisioning instance",
			"  Installing dependencies",
			"  Loading model from R2",
			"  Starting vLLM",
			"  Registering node",
		},
	}
	
	// Store job
	jobsMutex.Lock()
	launchJobs[jobID] = job
	jobsMutex.Unlock()
	
	// Start simulated launch progress in background
	go simulateLaunchProgress(jobID, g.logger)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "launching",
		"message":         "GPU instance launch initiated",
		"job_id":          jobID,
		"model":           req.ModelName,
		"estimated_time":  "2-3 minutes (simulated)",
		"note":            "This is a simulated launch for UI testing. Real SkyPilot integration needed for production.",
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
	
	// Get job from mock tracker
	jobsMutex.RLock()
	job, exists := launchJobs[jobID]
	jobsMutex.RUnlock()
	
	if !exists {
		// Job not found - might be old or invalid
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":  jobID,
			"status":  "not_found",
			"message": "Job not found. It may have expired or never existed.",
		})
		return
	}
	
	// Return current job status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":   job.JobID,
		"status":   job.Status,
		"stage":    job.Stage,
		"progress": job.Progress,
		"stages":   job.Stages,
		"model":    job.ModelName,
		"elapsed":  time.Since(job.StartTime).Seconds(),
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

// simulateLaunchProgress simulates a GPU instance launch for UI testing
// In production, this would be replaced with real SkyPilot orchestration
func simulateLaunchProgress(jobID string, logger *zap.Logger) {
	stages := []struct {
		name     string
		duration time.Duration
		progress int
	}{
		{"validating", 2 * time.Second, 10},
		{"requesting", 5 * time.Second, 25},
		{"provisioning", 15 * time.Second, 45},
		{"installing", 20 * time.Second, 65},
		{"loading_model", 25 * time.Second, 80},
		{"starting_vllm", 10 * time.Second, 90},
		{"registering", 5 * time.Second, 100},
	}
	
	for i, stage := range stages {
		time.Sleep(stage.duration)
		
		jobsMutex.Lock()
		job, exists := launchJobs[jobID]
		if !exists {
			jobsMutex.Unlock()
			return
		}
		
		job.Progress = stage.progress
		job.Stage = stage.name
		
		// Update stages display
		stageNames := []string{
			"Validating configuration",
			"Requesting spot instance",
			"Provisioning instance",
			"Installing dependencies",
			"Loading model from R2",
			"Starting vLLM",
			"Registering node",
		}
		
		updatedStages := make([]string, len(stageNames))
		for j, name := range stageNames {
			if j < i {
				updatedStages[j] = "✓ " + name
			} else if j == i {
				updatedStages[j] = "→ " + name
			} else {
				updatedStages[j] = "  " + name
			}
		}
		job.Stages = updatedStages
		
		if stage.progress >= 100 {
			job.Status = "completed"
			job.Stage = "ready"
			logger.Info("simulated launch completed", 
				zap.String("job_id", jobID),
				zap.String("model", job.ModelName),
			)
		}
		
		jobsMutex.Unlock()
	}
	
	// Clean up job after 5 minutes
	time.AfterFunc(5*time.Minute, func() {
		jobsMutex.Lock()
		delete(launchJobs, jobID)
		jobsMutex.Unlock()
		logger.Debug("cleaned up launch job", zap.String("job_id", jobID))
	})
}


