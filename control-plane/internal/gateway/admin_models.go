package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/crosslogic/control-plane/internal/orchestrator"
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
	
	// Check if orchestrator is available for real launches
	if g.orchestrator != nil {
		// REAL LAUNCH using SkyPilot orchestrator
		nodeID := uuid.New().String()
		
		// Create node configuration for SkyPilot
		nodeConfig := orchestrator.NodeConfig{
			NodeID:       nodeID,
			Provider:     req.Provider,
			Region:       req.Region,
			GPU:          req.GPU,
			GPUCount:     req.GPUCount,
			Model:        req.ModelName,
			UseSpot:      req.UseSpot,
			DiskSize:     256, // Default 256GB
			VLLMArgs:     "",  // Optional custom args
		}
		
		// Launch node asynchronously
		jobID := "launch-" + nodeID[:8]
		
		// Create job tracker for UI status
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
				"  Provisioning cloud resources",
				"  Installing dependencies",
				"  Loading model from R2",
				"  Starting vLLM",
				"  Registering node",
			},
		}
		
		jobsMutex.Lock()
		launchJobs[jobID] = job
		jobsMutex.Unlock()
		
		// Launch in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			
			clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)
			
			jobsMutex.Lock()
			defer jobsMutex.Unlock()
			
			if err != nil {
				g.logger.Error("failed to launch node",
					zap.Error(err),
					zap.String("job_id", jobID),
				)

				if job, exists := launchJobs[jobID]; exists {
					job.Status = "failed"
					job.Stage = "error"

					// Parse SkyPilot error for better user feedback
					errorMsg := err.Error()
					stages := []string{"✗ Launch failed"}

					// Check for common error patterns
					if containsString(errorMsg, "Failed to acquire resources in all zones") {
						stages = append(stages,
							"  → SkyPilot tried all availability zones in " + nodeConfig.Region,
							"  → No spot capacity available in any zone",
							"",
							"💡 Suggestions:",
							"  • Try a different region (westus2, centralindia, southindia)",
							"  • Use on-demand instead of spot (uncheck 'Use Spot')",
							"  • Wait 10-15 minutes and retry (capacity changes frequently)",
						)
					} else if containsString(errorMsg, "ResourcesUnavailableError") {
						stages = append(stages,
							"  → Cloud provider has no capacity for this GPU type",
							"  → Region: " + nodeConfig.Region,
							"  → GPU: " + nodeConfig.GPU,
							"",
							"💡 Try different region or GPU type",
						)
					} else {
						// Generic error - show full message
						stages = append(stages, "  → " + errorMsg)
					}

					job.Stages = stages
				}
				return
			}
			
			g.logger.Info("node launched successfully",
				zap.String("cluster_name", clusterName),
				zap.String("job_id", jobID),
			)
			
			// Update job to completed
			if job, exists := launchJobs[jobID]; exists {
				job.Status = "completed"
				job.Progress = 100
				job.Stage = "ready"
				job.Stages = []string{
					"✓ Validated configuration",
					"✓ Provisioned cloud resources",
					"✓ Installed dependencies",
					"✓ Loaded model from R2",
					"✓ Started vLLM",
					"✓ Node registered: " + clusterName,
				}
			}
		}()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "launching",
			"message":        "Real GPU instance launch initiated via SkyPilot",
			"job_id":         jobID,
			"node_id":        nodeID,
			"model":          req.ModelName,
			"provider":       req.Provider,
			"region":         req.Region,
			"estimated_time": "3-5 minutes",
		})
		return
	}
	
	// FALLBACK: Mock launch for testing when orchestrator not available
	g.logger.Warn("orchestrator not available, using mock launch simulation")
	
	jobID := "launch-" + uuid.New().String()[:8]
	
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
	
	jobsMutex.Lock()
	launchJobs[jobID] = job
	jobsMutex.Unlock()
	
	go simulateLaunchProgress(jobID, g.logger)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "launching",
		"message":         "GPU instance launch initiated (SIMULATION)",
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
	// IMPORTANT: Check longer prefixes first to avoid false matches
	// (e.g., "Standard_NV36ads_A10" must be checked before "Standard_NV")

	// Azure - order matters! Longer prefixes first
	azurePrefixes := []struct {
		prefix string
		gpu    string
	}{
		{"Standard_NV36ads_A10", "A10"},  // Check this before "Standard_NV"
		{"Standard_NV72ads_A10", "A10"},  // A10 variant with 2 GPUs
		{"Standard_NC", "K80"},
		{"Standard_ND", "P40"},
		{"Standard_NV", "M60"},  // Generic NV series (last)
	}

	// AWS
	awsPrefixes := []struct {
		prefix string
		gpu    string
	}{
		{"g5", "A10G"},
		{"g4dn", "T4"},
		{"p4", "A100"},
		{"p3", "V100"},
	}

	// GCP
	gcpPrefixes := []struct {
		prefix string
		gpu    string
	}{
		{"a2-highgpu", "A100"},
		{"n1-standard", "T4"},
	}

	// Select prefix list based on provider
	var prefixes []struct {
		prefix string
		gpu    string
	}

	switch provider {
	case "azure":
		prefixes = azurePrefixes
	case "aws":
		prefixes = awsPrefixes
	case "gcp":
		prefixes = gcpPrefixes
	default:
		// Try all if provider unknown
		prefixes = append(azurePrefixes, awsPrefixes...)
		prefixes = append(prefixes, gcpPrefixes...)
	}

	// Check prefixes in order (longer ones first for Azure)
	for _, p := range prefixes {
		if len(instanceType) >= len(p.prefix) && instanceType[:len(p.prefix)] == p.prefix {
			g.logger.Debug("detected GPU type",
				zap.String("instance_type", instanceType),
				zap.String("prefix_matched", p.prefix),
				zap.String("gpu", p.gpu),
			)
			return p.gpu
		}
	}

	g.logger.Warn("unknown GPU type for instance",
		zap.String("instance_type", instanceType),
		zap.String("provider", provider),
	)
	return "Unknown"
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
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

// ListRegionsHandler lists all available regions for a cloud provider
func (g *Gateway) ListRegionsHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "provider parameter required", http.StatusBadRequest)
		return
	}

	g.logger.Info("listing regions", zap.String("provider", provider))

	// Using existing schema: id (uuid), code, name, city, country, provider
	query := `
		SELECT id, code, name, city, country
		FROM regions
		WHERE provider = $1 AND status = 'active'
		ORDER BY name
	`

	rows, err := g.db.Pool.Query(r.Context(), query, provider)
	if err != nil {
		g.logger.Error("failed to query regions", zap.Error(err))
		http.Error(w, "Failed to list regions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Region struct {
		ID          string  `json:"id"`
		Provider    string  `json:"provider"`
		RegionCode  string  `json:"region_code"`
		RegionName  string  `json:"region_name"`
		Location    string  `json:"location"`
		IsAvailable bool    `json:"is_available"`
	}

	regions := []Region{}
	for rows.Next() {
		var id, code, name string
		var city, country *string
		if err := rows.Scan(&id, &code, &name, &city, &country); err != nil {
			g.logger.Error("failed to scan region", zap.Error(err))
			continue
		}

		location := ""
		if city != nil && *city != "" {
			location = *city
		}
		if country != nil && *country != "" {
			if location != "" {
				location += ", " + *country
			} else {
				location = *country
			}
		}

		regions = append(regions, Region{
			ID:          id,
			Provider:    provider,
			RegionCode:  code,
			RegionName:  name,
			Location:    location,
			IsAvailable: true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(regions)
}

// ListInstanceTypesHandler lists all available instance types for a provider and region
func (g *Gateway) ListInstanceTypesHandler(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	regionCode := r.URL.Query().Get("region")

	if provider == "" {
		http.Error(w, "provider parameter required", http.StatusBadRequest)
		return
	}

	g.logger.Info("listing instance types",
		zap.String("provider", provider),
		zap.String("region", regionCode),
	)

	var query string
	var rows interface{ Next() bool; Scan(dest ...interface{}) error; Close() }
	var err error

	if regionCode != "" {
		// Get instances available in specific region (using region_code for join)
		query = `
			SELECT DISTINCT i.id, i.provider, i.instance_type, i.instance_name,
			       i.vcpu_count, i.memory_gb, i.gpu_count, i.gpu_memory_gb,
			       i.gpu_model, i.gpu_compute_capability, i.price_per_hour,
			       i.spot_price_per_hour, i.is_available, i.supports_spot
			FROM instance_types i
			JOIN region_instance_availability ria ON ria.instance_type_id = i.id
			WHERE i.provider = $1 AND ria.region_code = $2 AND i.is_available = true AND ria.is_available = true
			ORDER BY i.gpu_model, i.gpu_count, i.vcpu_count
		`
		rows, err = g.db.Pool.Query(r.Context(), query, provider, regionCode)
	} else {
		// Get all instances for provider
		query = `
			SELECT id, provider, instance_type, instance_name,
			       vcpu_count, memory_gb, gpu_count, gpu_memory_gb,
			       gpu_model, gpu_compute_capability, price_per_hour,
			       spot_price_per_hour, is_available, supports_spot
			FROM instance_types
			WHERE provider = $1 AND is_available = true
			ORDER BY gpu_model, gpu_count, vcpu_count
		`
		rows, err = g.db.Pool.Query(r.Context(), query, provider)
	}

	if err != nil {
		g.logger.Error("failed to query instance types", zap.Error(err))
		http.Error(w, "Failed to list instance types", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type InstanceType struct {
		ID                    int     `json:"id"`
		Provider              string  `json:"provider"`
		InstanceType          string  `json:"instance_type"`
		InstanceName          *string `json:"instance_name"`
		VCPUCount             int     `json:"vcpu_count"`
		MemoryGB              float64 `json:"memory_gb"`
		GPUCount              int     `json:"gpu_count"`
		GPUMemoryGB           float64 `json:"gpu_memory_gb"`
		GPUModel              string  `json:"gpu_model"`
		GPUComputeCapability  *string `json:"gpu_compute_capability"`
		PricePerHour          *float64 `json:"price_per_hour"`
		SpotPricePerHour      *float64 `json:"spot_price_per_hour"`
		IsAvailable           bool    `json:"is_available"`
		SupportsSpot          bool    `json:"supports_spot"`
	}

	instanceTypes := []InstanceType{}
	for rows.Next() {
		var it InstanceType
		if err := rows.Scan(
			&it.ID, &it.Provider, &it.InstanceType, &it.InstanceName,
			&it.VCPUCount, &it.MemoryGB, &it.GPUCount, &it.GPUMemoryGB,
			&it.GPUModel, &it.GPUComputeCapability, &it.PricePerHour,
			&it.SpotPricePerHour, &it.IsAvailable, &it.SupportsSpot,
		); err != nil {
			g.logger.Error("failed to scan instance type", zap.Error(err))
			continue
		}
		instanceTypes = append(instanceTypes, it)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instanceTypes)
}


