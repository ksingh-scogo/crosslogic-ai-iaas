package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/crosslogic/control-plane/internal/orchestrator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LaunchInstanceRequest represents a request to launch a vLLM instance for PRO tenants
type LaunchInstanceRequest struct {
	Model              string  `json:"model"`
	Provider           string  `json:"provider,omitempty"`            // Optional - uses default credential if not specified
	Region             string  `json:"region"`
	GPU                string  `json:"gpu"`
	GPUCount           int     `json:"gpu_count"`
	IdleMinutesToStop  int     `json:"idle_minutes_to_autostop"`
	CredentialID       *string `json:"credential_id,omitempty"`       // Optional - uses default if not specified
	UseSpot            *bool   `json:"use_spot,omitempty"`            // Optional - defaults to true
	DiskSize           *int    `json:"disk_size,omitempty"`           // Optional - defaults to 256GB
	VLLMArgs           string  `json:"vllm_args,omitempty"`           // Optional additional vLLM arguments
}

// InstanceOutput represents a vLLM instance for tenant viewing
type InstanceOutput struct {
	ID           string     `json:"id"`
	ClusterName  string     `json:"cluster_name"`
	Model        string     `json:"model"`
	Provider     string     `json:"provider"`
	Region       string     `json:"region"`
	GPU          string     `json:"gpu"`
	Status       string     `json:"status"`
	EndpointURL  string     `json:"endpoint_url,omitempty"`
	SpotInstance bool       `json:"spot_instance"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	TerminatedAt *time.Time `json:"terminated_at,omitempty"`
}

// handleLaunchTenantInstance launches a new vLLM instance using tenant's cloud credentials
// POST /v1/instances
func (g *Gateway) handleLaunchTenantInstance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req LaunchInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Model == "" {
		g.writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	if req.Region == "" {
		g.writeError(w, http.StatusBadRequest, "region is required")
		return
	}
	if req.GPU == "" {
		g.writeError(w, http.StatusBadRequest, "gpu is required")
		return
	}
	if req.GPUCount == 0 {
		req.GPUCount = 1
	}

	// Set defaults
	useSpot := true
	if req.UseSpot != nil {
		useSpot = *req.UseSpot
	}

	diskSize := 256
	if req.DiskSize != nil {
		diskSize = *req.DiskSize
	}

	// Determine provider - either from request or credential
	provider := req.Provider
	var credentialID uuid.UUID

	if req.CredentialID != nil {
		// Parse credential ID
		var err error
		credentialID, err = uuid.Parse(*req.CredentialID)
		if err != nil {
			g.writeError(w, http.StatusBadRequest, "invalid credential_id")
			return
		}

		// Get credential to verify ownership and get provider
		credential, err := g.credentialService.GetCredential(ctx, credentialID, tenantID)
		if err != nil {
			g.logger.Error("failed to get tenant credential",
				zap.Error(err),
				zap.String("credential_id", credentialID.String()),
				zap.String("tenant_id", tenantID.String()),
			)
			g.writeError(w, http.StatusNotFound, "credential not found")
			return
		}
		provider = credential.Provider
	} else if provider == "" {
		g.writeError(w, http.StatusBadRequest, "either provider or credential_id must be specified")
		return
	}

	// Generate node ID
	nodeID := uuid.New()

	// Build node configuration
	nodeConfig := orchestrator.NodeConfig{
		NodeID:     nodeID.String(),
		Provider:   provider,
		Region:     req.Region,
		GPU:        req.GPU,
		GPUCount:   req.GPUCount,
		Model:      req.Model,
		UseSpot:    useSpot,
		DiskSize:   diskSize,
		VLLMArgs:   req.VLLMArgs,
		TenantID:   tenantID.String(),
	}

	g.logger.Info("launching tenant instance",
		zap.String("tenant_id", tenantID.String()),
		zap.String("node_id", nodeID.String()),
		zap.String("model", req.Model),
		zap.String("provider", provider),
		zap.String("region", req.Region),
		zap.String("gpu", req.GPU),
	)

	// Launch node using orchestrator
	clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)
	if err != nil {
		g.logger.Error("failed to launch tenant instance",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("node_id", nodeID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to launch instance: "+err.Error())
		return
	}

	// Register instance in database with tenant ownership
	if err := g.registerTenantInstance(ctx, tenantID, nodeID, clusterName, nodeConfig); err != nil {
		g.logger.Error("failed to register tenant instance",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("node_id", nodeID.String()),
		)
		// Instance launched but registration failed - continue anyway
	}

	g.logger.Info("tenant instance launched successfully",
		zap.String("tenant_id", tenantID.String()),
		zap.String("node_id", nodeID.String()),
		zap.String("cluster_name", clusterName),
	)

	g.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"instance_id":  nodeID.String(),
		"cluster_name": clusterName,
		"status":       "launching",
		"message":      "Instance is being launched. This may take 2-5 minutes.",
	})
}

// handleListTenantInstances lists all vLLM instances belonging to the authenticated tenant
// GET /v1/instances
func (g *Gateway) handleListTenantInstances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Query instances for this tenant
	query := `
		SELECT id, cluster_name, model_name, provider, gpu_type,
		       status, endpoint_url, spot_instance, created_at, updated_at, terminated_at
		FROM nodes
		WHERE tenant_id = $1
		  AND status != 'deleted'
		ORDER BY created_at DESC
	`

	rows, err := g.db.Pool.Query(ctx, query, tenantID)
	if err != nil {
		g.logger.Error("failed to list tenant instances",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to list instances")
		return
	}
	defer rows.Close()

	var instances []InstanceOutput
	for rows.Next() {
		var inst InstanceOutput
		var region *string

		err := rows.Scan(
			&inst.ID,
			&inst.ClusterName,
			&inst.Model,
			&inst.Provider,
			&inst.GPU,
			&inst.Status,
			&inst.EndpointURL,
			&inst.SpotInstance,
			&inst.CreatedAt,
			&inst.UpdatedAt,
			&inst.TerminatedAt,
		)
		if err != nil {
			g.logger.Warn("failed to scan instance row", zap.Error(err))
			continue
		}

		if region != nil {
			inst.Region = *region
		}

		instances = append(instances, inst)
	}

	if instances == nil {
		instances = []InstanceOutput{}
	}

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": instances,
	})
}

// handleGetTenantInstance retrieves details for a specific instance
// GET /v1/instances/{id}
func (g *Gateway) handleGetTenantInstance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	instanceIDStr := chi.URLParam(r, "id")
	instanceID, err := uuid.Parse(instanceIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	// Query instance (verify tenant ownership)
	query := `
		SELECT id, cluster_name, model_name, provider, gpu_type,
		       status, endpoint_url, spot_instance, created_at, updated_at, terminated_at
		FROM nodes
		WHERE id = $1 AND tenant_id = $2 AND status != 'deleted'
	`

	var inst InstanceOutput
	var region *string

	err = g.db.Pool.QueryRow(ctx, query, instanceID, tenantID).Scan(
		&inst.ID,
		&inst.ClusterName,
		&inst.Model,
		&inst.Provider,
		&inst.GPU,
		&inst.Status,
		&inst.EndpointURL,
		&inst.SpotInstance,
		&inst.CreatedAt,
		&inst.UpdatedAt,
		&inst.TerminatedAt,
	)

	if err != nil {
		g.logger.Error("failed to get tenant instance",
			zap.Error(err),
			zap.String("instance_id", instanceID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusNotFound, "instance not found")
		return
	}

	if region != nil {
		inst.Region = *region
	}

	g.writeJSON(w, http.StatusOK, inst)
}

// handleTerminateTenantInstance terminates a vLLM instance
// DELETE /v1/instances/{id}
func (g *Gateway) handleTerminateTenantInstance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	instanceIDStr := chi.URLParam(r, "id")
	instanceID, err := uuid.Parse(instanceIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	// Get cluster name and verify tenant ownership
	var clusterName string
	query := `
		SELECT cluster_name
		FROM nodes
		WHERE id = $1 AND tenant_id = $2 AND status NOT IN ('terminated', 'deleted')
	`

	err = g.db.Pool.QueryRow(ctx, query, instanceID, tenantID).Scan(&clusterName)
	if err != nil {
		g.logger.Error("failed to get instance for termination",
			zap.Error(err),
			zap.String("instance_id", instanceID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusNotFound, "instance not found or already terminated")
		return
	}

	g.logger.Info("terminating tenant instance",
		zap.String("tenant_id", tenantID.String()),
		zap.String("instance_id", instanceID.String()),
		zap.String("cluster_name", clusterName),
	)

	// Terminate node using orchestrator
	if err := g.orchestrator.TerminateNode(ctx, clusterName); err != nil {
		g.logger.Error("failed to terminate tenant instance",
			zap.Error(err),
			zap.String("tenant_id", tenantID.String()),
			zap.String("instance_id", instanceID.String()),
		)
		g.writeError(w, http.StatusInternalServerError, "failed to terminate instance: "+err.Error())
		return
	}

	g.logger.Info("tenant instance terminated successfully",
		zap.String("tenant_id", tenantID.String()),
		zap.String("instance_id", instanceID.String()),
	)

	g.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "terminated",
		"message": "instance terminated successfully",
	})
}

// handleStreamTenantInstanceLogs streams logs for a tenant's vLLM instance
// GET /v1/instances/{id}/logs/stream
func (g *Gateway) handleStreamTenantInstanceLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get tenant ID from auth context
	tenantID, ok := ctx.Value("tenant_id").(uuid.UUID)
	if !ok {
		g.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	instanceIDStr := chi.URLParam(r, "id")
	instanceID, err := uuid.Parse(instanceIDStr)
	if err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid instance ID")
		return
	}

	// Get cluster name and verify tenant ownership
	var clusterName string
	query := `
		SELECT cluster_name
		FROM nodes
		WHERE id = $1 AND tenant_id = $2 AND status != 'deleted'
	`

	err = g.db.Pool.QueryRow(ctx, query, instanceID, tenantID).Scan(&clusterName)
	if err != nil {
		g.logger.Error("failed to get instance for logs",
			zap.Error(err),
			zap.String("instance_id", instanceID.String()),
			zap.String("tenant_id", tenantID.String()),
		)
		g.writeError(w, http.StatusNotFound, "instance not found")
		return
	}

	// Set headers for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		g.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	g.logger.Info("streaming tenant instance logs",
		zap.String("tenant_id", tenantID.String()),
		zap.String("instance_id", instanceID.String()),
		zap.String("cluster_name", clusterName),
	)

	// Execute command to get recent logs via orchestrator
	// This is a simplified implementation - in production you'd want to use proper log streaming
	logCommand := "tail -100 /tmp/vllm.log"
	output, err := g.orchestrator.ExecCommand(ctx, clusterName, logCommand)
	if err != nil {
		g.logger.Error("failed to get instance logs",
			zap.Error(err),
			zap.String("cluster_name", clusterName),
		)
		// Send error as SSE
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Stream logs line by line as SSE
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()

		// Check if client disconnected
		select {
		case <-ctx.Done():
			return
		default:
		}
	}

	// Send completion event
	fmt.Fprintf(w, "event: complete\ndata: log streaming complete\n\n")
	flusher.Flush()
}

// registerTenantInstance registers a tenant-owned instance in the database
func (g *Gateway) registerTenantInstance(ctx context.Context, tenantID, instanceID uuid.UUID, clusterName string, config orchestrator.NodeConfig) error {
	query := `
		INSERT INTO nodes (
			id, tenant_id, cluster_name, provider, gpu_type,
			model_name, status, spot_instance, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'launching', $7, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET cluster_name = $3, status = 'launching', updated_at = NOW()
	`

	_, err := g.db.Pool.Exec(ctx, query,
		instanceID,
		tenantID,
		clusterName,
		config.Provider,
		config.GPU,
		config.Model,
		config.UseSpot,
	)

	return err
}
