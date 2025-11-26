# SkyPilot API Server Migration Plan

> **Implementation Status: COMPLETED**
>
> All phases of this migration plan have been implemented. See the following files:
> - `control-plane/internal/skypilot/client.go` - SkyPilot HTTP client with retry logic
> - `control-plane/internal/skypilot/models.go` - Request/response types
> - `control-plane/internal/credentials/` - Credential management service
> - `control-plane/internal/gateway/admin_credentials.go` - Admin API endpoints
> - `database/schemas/03_cloud_credentials.sql` - Database schema
> - `docs/SKYPILOT_API_TESTING_GUIDE.md` - Step-by-step testing guide

## Overview

This document provides a detailed technical implementation plan for migrating CrossLogic AI IaaS from CLI-based cloud authentication (`aws configure`, `az login`) to using the **SkyPilot API Server** with programmatic API key management.

### Goals
1. **No CLI Dependencies**: Eliminate `aws`, `az`, and `gcloud` CLI tools from the control plane
2. **Programmatic Access**: Go control plane makes REST API calls instead of shelling out to CLI
3. **Centralized Credential Management**: Cloud credentials managed in the database as encrypted secrets for each tenant and environment (Kubernetes secrets are not used for this purpose as we do not use kubernetes in this project)
4. **Multi-Cloud Support**: AWS, Azure, GCP, and other providers supported uniformly

---

## Architecture Overview

### Current Architecture (CLI-Based)

```
┌─────────────────────┐
│   Go Control Plane  │
└─────────┬───────────┘
          │ exec()
          ▼
┌─────────────────────┐
│   SkyPilot CLI      │
│   (sky launch/down) │
└─────────┬───────────┘
          │ Reads credentials from
          │ ~/.aws/credentials
          │ ~/.azure/
          │ ~/.config/gcloud/
          ▼
┌─────────────────────┐
│   Cloud Providers   │
│  (AWS/Azure/GCP)    │
└─────────────────────┘
```

**Problems:**
- Requires CLI tools installed on control plane server
- Manual `aws configure` / `az login` / `gcloud auth` required
- Credentials scattered across filesystem
- No centralized credential rotation

### Target Architecture (API Server-Based)

```
┌─────────────────────┐
│   Go Control Plane  │
│   (HTTP Client)     │
└─────────┬───────────┘
          │ REST API (HTTPS)
          │ Service Account Token
          ▼
┌─────────────────────────────────────────────────┐
│          SkyPilot API Server                     │
│  ┌────────────────────────────────────────────┐ │
│  │  Cloud Credentials (K8s Secrets)           │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐      │ │
│  │  │   AWS   │ │  Azure  │ │   GCP   │ ...  │ │
│  │  └─────────┘ └─────────┘ └─────────┘      │ │
│  └────────────────────────────────────────────┘ │
└─────────┬─────────────┬─────────────┬───────────┘
          │             │             │
          ▼             ▼             ▼
       ┌─────┐      ┌─────┐      ┌─────┐
       │ AWS │      │Azure│      │ GCP │
       └─────┘      └─────┘      └─────┘
```

**Benefits:**
- No CLI tools on control plane
- Credentials in the database as encrypted secrets for each tenant and environment (rotatable, auditable)
- Single API endpoint for all cloud operations
- Service account authentication
- Built-in multi-tenancy and RBAC

---

## Phase 1: SkyPilot API Server Deployment

### Launch SkyPilot API Server as a container

```bash
docker run -d --name skypilot-api -p 8080:8080 skypilot/skypilot-api
```
### Configure SkyPilot API Server
- For each tenant dynamically provide skypilot api that tenants credentials to access the cloud providers
- For each environment dynamically provide skypilot api that environment credentials to access the cloud providers
- For each model dynamically provide skypilot api that model credentials to access the cloud providers
- For each user dynamically provide skypilot api that user credentials to access the cloud providers

### Usage
- We want to run one instance of the SkyPilot API Server for all tenants and environments and intelligently supply the credentials to the skypilot api based on the request
---

## Phase 2: Cloud Provider Credentials Configuration




## Phase 3: Service Account for Go Control Plane

### Create Service Account Token

```bash
# 1. Create a service account in SkyPilot
# Using the SkyPilot Python SDK or API
python << 'EOF'
import sky

# Connect to API server
sky.api.login("https://skypilot-api.example.com")

# Create service account (if using v0.10+)
# Service accounts are managed through the API server admin interface
EOF

# 2. Alternatively, generate a token via API
curl -X POST https://skypilot-api.example.com/api/v1/auth/service-accounts \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "crosslogic-control-plane",
    "description": "CrossLogic AI Control Plane Service Account"
  }'

# Response:
# {
#   "id": "sa-xxxxx",
#   "token": "sky_sa_XXXXXXXXXXXXXXXX"
# }
```

### Store Token Securely

```bash

# Option 2: Environment variable (for VM deployment)
export SKYPILOT_API_TOKEN=sky_sa_XXXXXXXXXXXXXXXX

```

---

## Phase 4: Go Control Plane Integration

### New SkyPilot HTTP Client

Create `control-plane/internal/skypilot/client.go`:

```go
package skypilot

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// Client is an HTTP client for SkyPilot API Server
type Client struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

// Config holds SkyPilot API client configuration
type Config struct {
    BaseURL string        // e.g., "https://skypilot-api.example.com"
    Token   string        // Service account token
    Timeout time.Duration // HTTP timeout
}

// NewClient creates a new SkyPilot API client
func NewClient(cfg Config) *Client {
    return &Client{
        baseURL: cfg.BaseURL,
        token:   cfg.Token,
        httpClient: &http.Client{
            Timeout: cfg.Timeout,
        },
    }
}

// LaunchRequest represents a cluster launch request
type LaunchRequest struct {
    ClusterName       string            `json:"cluster_name"`
    TaskYAML          string            `json:"task_yaml"`
    RetryUntilUp      bool              `json:"retry_until_up"`
    IdleMinutesToStop int               `json:"idle_minutes_to_autostop,omitempty"`
    Detach            bool              `json:"detach"`
    Envs              map[string]string `json:"envs,omitempty"`
}

// LaunchResponse contains the request ID for async tracking
type LaunchResponse struct {
    RequestID string `json:"request_id"`
}

// ClusterStatus represents cluster state
type ClusterStatus struct {
    Name      string `json:"name"`
    Status    string `json:"status"` // UP, INIT, DOWN, STOPPED
    Provider  string `json:"cloud"`
    Region    string `json:"region"`
    Resources struct {
        Accelerators string `json:"accelerators"`
        CPUs         int    `json:"cpus"`
        Memory       string `json:"memory"`
    } `json:"resources"`
    LaunchedAt time.Time `json:"launched_at"`
    Cost       float64   `json:"cost_per_hour"`
}

// RequestStatus represents async request status
type RequestStatus struct {
    RequestID string `json:"request_id"`
    Status    string `json:"status"` // pending, running, completed, failed
    Result    any    `json:"result,omitempty"`
    Error     string `json:"error,omitempty"`
}

// Launch starts a new cluster (async)
func (c *Client) Launch(ctx context.Context, req LaunchRequest) (*LaunchResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        c.baseURL+"/api/v1/clusters/launch", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    c.setHeaders(httpReq)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("launch failed: status=%d body=%s", resp.StatusCode, body)
    }

    var result LaunchResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}

// Down terminates a cluster (async)
func (c *Client) Down(ctx context.Context, clusterName string) (*LaunchResponse, error) {
    httpReq, err := http.NewRequestWithContext(ctx, "DELETE",
        c.baseURL+"/api/v1/clusters/"+clusterName, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    c.setHeaders(httpReq)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("down failed: status=%d body=%s", resp.StatusCode, body)
    }

    var result LaunchResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}

// Status gets cluster status
func (c *Client) Status(ctx context.Context, clusterName string) (*ClusterStatus, error) {
    httpReq, err := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/api/v1/clusters/"+clusterName, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    c.setHeaders(httpReq)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return nil, fmt.Errorf("cluster not found: %s", clusterName)
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("status failed: status=%d body=%s", resp.StatusCode, body)
    }

    var result ClusterStatus
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}

// ListClusters returns all clusters
func (c *Client) ListClusters(ctx context.Context) ([]ClusterStatus, error) {
    httpReq, err := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/api/v1/clusters", nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    c.setHeaders(httpReq)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("list failed: status=%d body=%s", resp.StatusCode, body)
    }

    var result []ClusterStatus
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return result, nil
}

// GetRequestStatus polls async request status
func (c *Client) GetRequestStatus(ctx context.Context, requestID string) (*RequestStatus, error) {
    httpReq, err := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/api/v1/requests/"+requestID, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    c.setHeaders(httpReq)

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("request status failed: status=%d body=%s", resp.StatusCode, body)
    }

    var result RequestStatus
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}

// WaitForRequest polls until request completes
func (c *Client) WaitForRequest(ctx context.Context, requestID string, pollInterval time.Duration) (*RequestStatus, error) {
    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-ticker.C:
            status, err := c.GetRequestStatus(ctx, requestID)
            if err != nil {
                return nil, err
            }

            switch status.Status {
            case "completed":
                return status, nil
            case "failed":
                return status, fmt.Errorf("request failed: %s", status.Error)
            case "pending", "running":
                // Continue polling
            default:
                return nil, fmt.Errorf("unknown status: %s", status.Status)
            }
        }
    }
}

// Health checks API server health
func (c *Client) Health(ctx context.Context) error {
    httpReq, err := http.NewRequestWithContext(ctx, "GET",
        c.baseURL+"/api/health", nil)
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unhealthy: status=%d", resp.StatusCode)
    }

    return nil
}

func (c *Client) setHeaders(req *http.Request) {
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }
}
```

### Update Orchestrator to Use HTTP Client

Modify `control-plane/internal/orchestrator/skypilot.go`:

```go
// Add to imports
import (
    "github.com/crosslogic-ai/iaas/control-plane/internal/skypilot"
)

// Update SkyPilotOrchestrator struct
type SkyPilotOrchestrator struct {
    // ... existing fields ...

    // New: SkyPilot API client (replaces CLI)
    apiClient     *skypilot.Client
    useAPIServer  bool  // Feature flag for gradual migration
}

// NewSkyPilotOrchestrator - add API client initialization
func NewSkyPilotOrchestrator(cfg *config.Config, db *database.Database, eventBus *events.Bus) (*SkyPilotOrchestrator, error) {
    orch := &SkyPilotOrchestrator{
        // ... existing initialization ...
    }

    // Initialize API client if configured
    if cfg.SkyPilot.APIServerURL != "" {
        orch.apiClient = skypilot.NewClient(skypilot.Config{
            BaseURL: cfg.SkyPilot.APIServerURL,
            Token:   cfg.SkyPilot.ServiceAccountToken,
            Timeout: 5 * time.Minute,
        })
        orch.useAPIServer = true
    }

    return orch, nil
}

// LaunchNode - update to use API client
func (o *SkyPilotOrchestrator) LaunchNode(ctx context.Context, nodeCfg NodeConfig) (*database.Node, error) {
    // ... validation and YAML generation (unchanged) ...

    if o.useAPIServer {
        return o.launchNodeViaAPI(ctx, nodeCfg, clusterName, taskYAML)
    }

    // Fallback to CLI (for gradual migration)
    return o.launchNodeViaCLI(ctx, nodeCfg, clusterName, taskYAML)
}

// launchNodeViaAPI - new method using HTTP client
func (o *SkyPilotOrchestrator) launchNodeViaAPI(ctx context.Context, nodeCfg NodeConfig, clusterName, taskYAML string) (*database.Node, error) {
    // Launch via API (async)
    resp, err := o.apiClient.Launch(ctx, skypilot.LaunchRequest{
        ClusterName:  clusterName,
        TaskYAML:     taskYAML,
        RetryUntilUp: true,
        Detach:       true,
        Envs: map[string]string{
            "CONTROL_PLANE_URL": o.controlPlaneURL,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("launch via API: %w", err)
    }

    // Register node in database
    node := &database.Node{
        ID:           uuid.New(),
        ClusterName:  clusterName,
        DeploymentID: nodeCfg.DeploymentID,
        ModelName:    nodeCfg.Model,
        Provider:     nodeCfg.Provider,
        Region:       nodeCfg.Region,
        GPU:          nodeCfg.GPU,
        GPUCount:     nodeCfg.GPUCount,
        Status:       "launching",
        RequestID:    resp.RequestID, // Track async request
        CreatedAt:    time.Now(),
    }

    if err := o.db.CreateNode(ctx, node); err != nil {
        // Attempt cleanup on DB failure
        go o.apiClient.Down(context.Background(), clusterName)
        return nil, fmt.Errorf("create node record: %w", err)
    }

    // Publish event
    o.eventBus.Publish(events.Event{
        Type:      events.EventNodeLaunched,
        NodeID:    node.ID.String(),
        Timestamp: time.Now(),
        Data: map[string]any{
            "cluster_name": clusterName,
            "request_id":   resp.RequestID,
            "provider":     nodeCfg.Provider,
        },
    })

    return node, nil
}

// TerminateNode - update to use API client
func (o *SkyPilotOrchestrator) TerminateNode(ctx context.Context, clusterName string) error {
    if o.useAPIServer {
        resp, err := o.apiClient.Down(ctx, clusterName)
        if err != nil {
            return fmt.Errorf("terminate via API: %w", err)
        }

        // Optionally wait for completion
        _, err = o.apiClient.WaitForRequest(ctx, resp.RequestID, 5*time.Second)
        return err
    }

    // Fallback to CLI
    return o.terminateNodeViaCLI(ctx, clusterName)
}

// GetNodeStatus - update to use API client
func (o *SkyPilotOrchestrator) GetNodeStatus(ctx context.Context, clusterName string) (string, error) {
    if o.useAPIServer {
        status, err := o.apiClient.Status(ctx, clusterName)
        if err != nil {
            return "UNKNOWN", err
        }
        return status.Status, nil
    }

    // Fallback to CLI
    return o.getNodeStatusViaCLI(ctx, clusterName)
}
```

### Update Configuration

Add to `control-plane/internal/config/config.go`:

```go
type SkyPilotConfig struct {
    // API Server Configuration (new)
    APIServerURL        string `env:"SKYPILOT_API_SERVER_URL"`
    ServiceAccountToken string `env:"SKYPILOT_SERVICE_ACCOUNT_TOKEN"`

    // Feature flags
    UseAPIServer bool `env:"SKYPILOT_USE_API_SERVER" envDefault:"false"`

    // Existing CLI configuration (kept for fallback)
    // ... existing fields ...
}
```

Update `config/env.template`:

```bash
# SkyPilot API Server Configuration (New)
SKYPILOT_API_SERVER_URL=https://skypilot-api.example.com
SKYPILOT_SERVICE_ACCOUNT_TOKEN=sky_sa_XXXXXXXX
SKYPILOT_USE_API_SERVER=true
```

---

## Phase 5: Testing and Migration

### Testing Checklist

```bash
# 1. Test API Server connectivity
curl -H "Authorization: Bearer $SKYPILOT_SERVICE_ACCOUNT_TOKEN" \
  https://skypilot-api.example.com/api/health

# 2. Test cluster listing
curl -H "Authorization: Bearer $SKYPILOT_SERVICE_ACCOUNT_TOKEN" \
  https://skypilot-api.example.com/api/v1/clusters

# 3. Test launch (with test task)
curl -X POST \
  -H "Authorization: Bearer $SKYPILOT_SERVICE_ACCOUNT_TOKEN" \
  -H "Content-Type: application/json" \
  https://skypilot-api.example.com/api/v1/clusters/launch \
  -d '{
    "cluster_name": "test-cluster",
    "task_yaml": "resources:\n  cloud: aws\n  instance_type: t3.micro\nrun: echo hello",
    "detach": true
  }'

# 4. Test each cloud provider
# AWS
sky launch -c aws-test --infra aws -y --detach echo hello
# Azure
sky launch -c azure-test --infra azure -y --detach echo hello
# GCP
sky launch -c gcp-test --infra gcp -y --detach echo hello
```

### Gradual Migration Strategy

1. **1**: Deploy SkyPilot API Server alongside existing CLI setup
2. **2**: Enable API Server for non-production workloads (`SKYPILOT_USE_API_SERVER=true` in staging)
3. **3**: Monitor and collect metrics, fix any issues
4. **4**: Enable for production with CLI fallback
5. **5**: Remove CLI dependencies after validation

### Rollback Plan

```bash
# If issues occur, disable API Server mode
SKYPILOT_USE_API_SERVER=false

# The Go control plane will automatically fall back to CLI mode
```

---

## Summary

### What Changes

| Component | Before | After |
|-----------|--------|-------|
| Authentication | `aws configure`, `az login`, `gcloud auth` | K8s Secrets + API Server |
| Go Control Plane | Executes CLI commands | HTTP REST API calls |
| Credentials Storage | Filesystem (`~/.aws/`, etc.) | Kubernetes Secrets |
| Multi-tenancy | N/A | Workspaces in SkyPilot |
| Audit Trail | None | API Server logs |

### Dependencies Removed

- `aws` CLI tool
- `az` CLI tool
- `gcloud` CLI tool
- Shell access from Go control plane

### New Dependencies

- SkyPilot API Server (Kubernetes or VM)
- Kubernetes secrets for credentials
- Service account tokens

