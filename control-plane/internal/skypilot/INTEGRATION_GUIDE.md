# SkyPilot HTTP Client Integration Guide

This guide provides step-by-step instructions for integrating the SkyPilot HTTP client into the CrossLogic AI IaaS control plane orchestrator.

## Prerequisites

1. SkyPilot API Server deployed and accessible
2. Service account token generated for the control plane
3. Configuration updated with SkyPilot settings

## Step 1: Environment Configuration

The configuration is already set up in `internal/config/config.go`. Set these environment variables:

```bash
# Required: SkyPilot API Server settings
export SKYPILOT_API_SERVER_URL="https://skypilot-api.crosslogic.ai"
export SKYPILOT_SERVICE_ACCOUNT_TOKEN="sky_sa_xxxxxxxxxxxxx"
export SKYPILOT_USE_API_SERVER="true"

# Optional: Timeouts (defaults shown)
export SKYPILOT_LAUNCH_TIMEOUT="10m"
export SKYPILOT_TERMINATE_TIMEOUT="5m"
export SKYPILOT_STATUS_TIMEOUT="30s"

# Optional: Retry configuration (defaults shown)
export SKYPILOT_MAX_RETRIES="3"
export SKYPILOT_RETRY_BACKOFF="5s"

# Optional: Credential encryption key for storing cloud credentials in DB
export SKYPILOT_CREDENTIAL_ENCRYPTION_KEY="your-32-byte-encryption-key"
```

## Step 2: Update Orchestrator Structure

Edit `internal/orchestrator/skypilot.go` to add the HTTP client:

```go
package orchestrator

import (
    "github.com/crosslogic/control-plane/internal/skypilot"
    // ... other imports
)

type SkyPilotOrchestrator struct {
    db        *database.Database
    logger    *zap.Logger
    eventBus  *events.Bus

    // Add these fields
    skyClient    *skypilot.Client
    useAPIServer bool

    // Existing fields
    controlPlaneURL string
    vllmVersion     string
    torchVersion    string
    r2Config        config.R2Config
}
```

## Step 3: Initialize Client in Constructor

Update `NewSkyPilotOrchestrator`:

```go
func NewSkyPilotOrchestrator(
    db *database.Database,
    logger *zap.Logger,
    controlPlaneURL string,
    vllmVersion string,
    torchVersion string,
    eventBus *events.Bus,
    r2Config config.R2Config,
    skyPilotConfig config.SkyPilotConfig, // Add this parameter
) (*SkyPilotOrchestrator, error) {
    orch := &SkyPilotOrchestrator{
        db:              db,
        logger:          logger,
        eventBus:        eventBus,
        controlPlaneURL: controlPlaneURL,
        vllmVersion:     vllmVersion,
        torchVersion:    torchVersion,
        r2Config:        r2Config,
        useAPIServer:    skyPilotConfig.UseAPIServer,
    }

    // Initialize SkyPilot API client if configured
    if skyPilotConfig.UseAPIServer {
        if skyPilotConfig.APIServerURL == "" {
            return nil, fmt.Errorf("SKYPILOT_API_SERVER_URL is required when UseAPIServer is true")
        }
        if skyPilotConfig.ServiceAccountToken == "" {
            return nil, fmt.Errorf("SKYPILOT_SERVICE_ACCOUNT_TOKEN is required when UseAPIServer is true")
        }

        orch.skyClient = skypilot.NewClient(skypilot.Config{
            BaseURL:       skyPilotConfig.APIServerURL,
            Token:         skyPilotConfig.ServiceAccountToken,
            Timeout:       skyPilotConfig.LaunchTimeout,
            MaxRetries:    skyPilotConfig.MaxRetries,
            RetryDelay:    skyPilotConfig.RetryBackoff,
            RetryMaxDelay: skyPilotConfig.RetryBackoff * 10, // 10x initial delay as max
        }, logger)

        logger.Info("initialized SkyPilot API client",
            zap.String("api_url", skyPilotConfig.APIServerURL),
            zap.Bool("use_api_server", true),
        )

        // Test connectivity
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if _, err := orch.skyClient.Health(ctx); err != nil {
            logger.Warn("SkyPilot API server health check failed",
                zap.Error(err),
            )
            // Don't fail initialization - API might be temporarily down
        } else {
            logger.Info("SkyPilot API server is healthy")
        }
    } else {
        logger.Info("SkyPilot API client not configured, using CLI mode")
    }

    return orch, nil
}
```

## Step 4: Update Main Server Initialization

Edit `cmd/server/main.go` to pass the SkyPilot config:

```go
// Initialize SkyPilot orchestrator with event bus and SkyPilot config
orch, err := orchestrator.NewSkyPilotOrchestrator(
    db,
    logger,
    cfg.Server.ControlPlaneURL,
    cfg.Runtime.VLLMVersion,
    cfg.Runtime.TorchVersion,
    eventBus,
    cfg.R2,
    cfg.SkyPilot, // Add this parameter
)
if err != nil {
    logger.Fatal("failed to initialize orchestrator", zap.Error(err))
}
logger.Info("initialized SkyPilot orchestrator",
    zap.Bool("api_mode", cfg.SkyPilot.UseAPIServer),
)
```

## Step 5: Implement API-Based Launch Method

Add this method to `internal/orchestrator/skypilot.go`:

```go
// LaunchNodeViaAPI launches a node using the SkyPilot API Server
func (o *SkyPilotOrchestrator) LaunchNodeViaAPI(
    ctx context.Context,
    clusterName string,
    taskYAML string,
    tenantID uuid.UUID,
) (*database.Node, error) {
    o.logger.Info("launching node via SkyPilot API",
        zap.String("cluster_name", clusterName),
        zap.String("tenant_id", tenantID.String()),
    )

    // Get tenant's cloud credentials from database
    // This enables multi-tenant support where each tenant has their own cloud accounts
    creds, err := o.getTenantCloudCredentials(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("get tenant credentials: %w", err)
    }

    // Launch cluster via API
    resp, err := o.skyClient.Launch(ctx, skypilot.LaunchRequest{
        ClusterName:       clusterName,
        TaskYAML:          taskYAML,
        RetryUntilUp:      true,
        Detach:            true,
        IdleMinutesToStop: 30, // Auto-stop after 30 minutes of idle time
        Envs: map[string]string{
            "CONTROL_PLANE_URL": o.controlPlaneURL,
            "VLLM_VERSION":      o.vllmVersion,
            "TORCH_VERSION":     o.torchVersion,
        },
        CloudCredentials: creds,
    })
    if err != nil {
        o.logger.Error("failed to launch cluster via API",
            zap.String("cluster_name", clusterName),
            zap.Error(err),
        )
        return nil, fmt.Errorf("launch via API: %w", err)
    }

    o.logger.Info("cluster launch initiated via API",
        zap.String("cluster_name", clusterName),
        zap.String("request_id", resp.RequestID),
    )

    // Create node record in database
    node := &database.Node{
        ID:          uuid.New(),
        ClusterName: clusterName,
        TenantID:    tenantID,
        Status:      "launching",
        RequestID:   resp.RequestID, // Store request ID for async tracking
        CreatedAt:   time.Now(),
    }

    if err := o.db.CreateNode(ctx, node); err != nil {
        // Attempt cleanup on DB failure
        o.logger.Error("failed to create node record, attempting cleanup",
            zap.String("cluster_name", clusterName),
            zap.Error(err),
        )

        // Best-effort cleanup
        go func() {
            cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()
            o.skyClient.Terminate(cleanupCtx, clusterName, false)
        }()

        return nil, fmt.Errorf("create node record: %w", err)
    }

    // Publish event
    o.eventBus.Publish(events.Event{
        Type:      events.EventNodeLaunched,
        NodeID:    node.ID.String(),
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "cluster_name": clusterName,
            "request_id":   resp.RequestID,
            "tenant_id":    tenantID.String(),
            "via_api":      true,
        },
    })

    return node, nil
}
```

## Step 6: Add Helper Method for Tenant Credentials

```go
// getTenantCloudCredentials retrieves cloud credentials for a tenant
// This is a placeholder - implement based on your database schema
func (o *SkyPilotOrchestrator) getTenantCloudCredentials(
    ctx context.Context,
    tenantID uuid.UUID,
) (*skypilot.CloudCredentials, error) {
    // TODO: Implement based on your tenant credentials storage
    // This should query the database for encrypted credentials
    // and return them in SkyPilot format

    // Example implementation:
    // 1. Query database for tenant cloud credentials
    // 2. Decrypt credentials (using SKYPILOT_CREDENTIAL_ENCRYPTION_KEY)
    // 3. Convert to skypilot.CloudCredentials format

    o.logger.Debug("retrieving tenant cloud credentials",
        zap.String("tenant_id", tenantID.String()),
    )

    // For now, return nil to use SkyPilot API Server's default credentials
    // In production, you must implement proper credential management
    return nil, nil
}
```

## Step 7: Update Existing Launch Method

Modify the existing `LaunchNode` method to support both API and CLI modes:

```go
func (o *SkyPilotOrchestrator) LaunchNode(
    ctx context.Context,
    nodeCfg NodeConfig,
) (*database.Node, error) {
    // Generate cluster name
    clusterName := fmt.Sprintf("%s-%s-%s",
        nodeCfg.TenantID.String()[:8],
        nodeCfg.Model,
        nodeCfg.Provider,
    )

    // Build SkyPilot task YAML
    taskYAML := o.buildTaskYAML(nodeCfg)

    // Route to API or CLI based on configuration
    if o.useAPIServer {
        return o.LaunchNodeViaAPI(ctx, clusterName, taskYAML, nodeCfg.TenantID)
    }

    // Fallback to CLI mode (existing implementation)
    return o.launchNodeViaCLI(ctx, clusterName, taskYAML, nodeCfg)
}
```

## Step 8: Implement API-Based Terminate Method

```go
// TerminateNodeViaAPI terminates a node using the SkyPilot API Server
func (o *SkyPilotOrchestrator) TerminateNodeViaAPI(
    ctx context.Context,
    clusterName string,
) error {
    o.logger.Info("terminating node via SkyPilot API",
        zap.String("cluster_name", clusterName),
    )

    resp, err := o.skyClient.Terminate(ctx, clusterName, false)
    if err != nil {
        // Check if cluster not found (already terminated)
        if apiErr, ok := err.(*skypilot.APIError); ok && apiErr.IsNotFound() {
            o.logger.Info("cluster already terminated",
                zap.String("cluster_name", clusterName),
            )
            return nil
        }

        return fmt.Errorf("terminate via API: %w", err)
    }

    o.logger.Info("cluster termination initiated",
        zap.String("cluster_name", clusterName),
        zap.String("request_id", resp.RequestID),
    )

    // Wait for termination to complete (optional, with timeout)
    waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()

    status, err := o.skyClient.WaitForRequest(waitCtx, resp.RequestID, 5*time.Second)
    if err != nil {
        if err == context.DeadlineExceeded {
            o.logger.Warn("termination still in progress after timeout",
                zap.String("cluster_name", clusterName),
            )
            return nil // Don't fail - termination is still happening
        }
        return fmt.Errorf("wait for termination: %w", err)
    }

    if status.Status != "completed" {
        return fmt.Errorf("termination failed: %s", status.Error)
    }

    o.logger.Info("cluster terminated successfully",
        zap.String("cluster_name", clusterName),
    )

    return nil
}

// Update TerminateNode to route based on configuration
func (o *SkyPilotOrchestrator) TerminateNode(
    ctx context.Context,
    clusterName string,
) error {
    if o.useAPIServer {
        return o.TerminateNodeViaAPI(ctx, clusterName)
    }

    // Fallback to CLI mode
    return o.terminateNodeViaCLI(ctx, clusterName)
}
```

## Step 9: Implement API-Based Status Check

```go
// GetNodeStatusViaAPI retrieves node status using the SkyPilot API Server
func (o *SkyPilotOrchestrator) GetNodeStatusViaAPI(
    ctx context.Context,
    clusterName string,
) (string, error) {
    status, err := o.skyClient.GetStatus(ctx, clusterName)
    if err != nil {
        if apiErr, ok := err.(*skypilot.APIError); ok && apiErr.IsNotFound() {
            return "TERMINATED", nil
        }
        return "UNKNOWN", err
    }

    return status.Status, nil
}

// Update GetNodeStatus to route based on configuration
func (o *SkyPilotOrchestrator) GetNodeStatus(
    ctx context.Context,
    clusterName string,
) (string, error) {
    if o.useAPIServer {
        return o.GetNodeStatusViaAPI(ctx, clusterName)
    }

    // Fallback to CLI mode
    return o.getNodeStatusViaCLI(ctx, clusterName)
}
```

## Step 10: Add Background Task for Async Request Monitoring

```go
// StartAsyncRequestMonitor monitors pending async requests
func (o *SkyPilotOrchestrator) StartAsyncRequestMonitor(ctx context.Context) {
    if !o.useAPIServer {
        return // Only needed in API mode
    }

    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                o.checkPendingRequests(ctx)
            }
        }
    }()

    o.logger.Info("started async request monitor")
}

func (o *SkyPilotOrchestrator) checkPendingRequests(ctx context.Context) {
    // Get nodes with pending requests from database
    nodes, err := o.db.GetNodesByStatus(ctx, "launching")
    if err != nil {
        o.logger.Error("failed to get pending nodes", zap.Error(err))
        return
    }

    for _, node := range nodes {
        if node.RequestID == "" {
            continue // Skip nodes without request ID
        }

        // Check request status
        status, err := o.skyClient.GetRequestStatus(ctx, node.RequestID)
        if err != nil {
            o.logger.Warn("failed to get request status",
                zap.String("node_id", node.ID.String()),
                zap.String("request_id", node.RequestID),
                zap.Error(err),
            )
            continue
        }

        // Update node based on request status
        switch status.Status {
        case "completed":
            node.Status = "running"
            if err := o.db.UpdateNode(ctx, node); err != nil {
                o.logger.Error("failed to update node status",
                    zap.String("node_id", node.ID.String()),
                    zap.Error(err),
                )
            }

            o.eventBus.Publish(events.Event{
                Type:      events.EventNodeReady,
                NodeID:    node.ID.String(),
                Timestamp: time.Now(),
            })

        case "failed":
            node.Status = "failed"
            node.Error = status.Error
            if err := o.db.UpdateNode(ctx, node); err != nil {
                o.logger.Error("failed to update node status",
                    zap.String("node_id", node.ID.String()),
                    zap.Error(err),
                )
            }

            o.eventBus.Publish(events.Event{
                Type:      events.EventNodeFailed,
                NodeID:    node.ID.String(),
                Timestamp: time.Now(),
                Data:      map[string]interface{}{"error": status.Error},
            })
        }
    }
}
```

## Step 11: Update Main Server to Start Monitor

In `cmd/server/main.go`, after starting other background services:

```go
// Start async request monitor (for SkyPilot API mode)
if cfg.SkyPilot.UseAPIServer {
    orch.StartAsyncRequestMonitor(ctx)
    logger.Info("started SkyPilot async request monitor")
}
```

## Testing the Integration

### 1. Test Health Check

```bash
# In Go code or via debug endpoint
health, err := skyClient.Health(ctx)
if err != nil {
    log.Fatal(err)
}
log.Printf("SkyPilot API Status: %s, Version: %s", health.Status, health.Version)
```

### 2. Test Launch (Development)

```bash
# Enable API mode
export SKYPILOT_USE_API_SERVER=true
export SKYPILOT_API_SERVER_URL=https://skypilot-api.example.com
export SKYPILOT_SERVICE_ACCOUNT_TOKEN=sky_sa_xxxxx

# Start control plane
./control-plane

# Monitor logs for:
# - "initialized SkyPilot API client"
# - "SkyPilot API server is healthy"
# - "launching node via SkyPilot API"
```

### 3. Test with Real Deployment

```bash
# Create a deployment via API
curl -X POST http://localhost:8080/api/v1/tenants/YOUR_TENANT_ID/deployments \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "NousResearch/Meta-Llama-3-8B-Instruct",
    "replicas": 1,
    "provider": "aws",
    "region": "us-east-1",
    "gpu_type": "A100",
    "gpu_count": 1
  }'

# Monitor logs for API interactions
# Check database for node records with request IDs
```

## Gradual Migration Strategy

### Phase 1: Parallel Running (Week 1-2)
- Set `SKYPILOT_USE_API_SERVER=false` (CLI mode)
- Deploy and monitor in production
- Deploy SkyPilot API Server in parallel

### Phase 2: Staging Testing (Week 3-4)
- Set `SKYPILOT_USE_API_SERVER=true` in staging
- Test all operations (launch, terminate, status)
- Monitor error rates and latency
- Validate multi-tenant credentials

### Phase 3: Production Rollout (Week 5-6)
- Enable for 10% of traffic
- Monitor metrics and errors
- Gradually increase to 100%

### Phase 4: CLI Deprecation (Week 7-8)
- Remove CLI code paths
- Update documentation
- Clean up dependencies

## Rollback Plan

If issues occur:

```bash
# Immediate rollback
export SKYPILOT_USE_API_SERVER=false

# Restart control plane
systemctl restart control-plane
```

The orchestrator will automatically fall back to CLI mode.

## Monitoring

Add these metrics to track API usage:

```go
skypilot_api_requests_total{operation, status}
skypilot_api_request_duration_seconds{operation}
skypilot_api_errors_total{operation, error_type}
skypilot_api_retries_total{operation}
```

## Troubleshooting

See `README.md` for detailed troubleshooting steps.

## Next Steps

1. Implement tenant credential storage and encryption
2. Add Prometheus metrics
3. Set up alerting for API failures
4. Create dashboard for API health monitoring
5. Document credential rotation procedures

## Support

For issues or questions:
- Check `README.md` for common problems
- Review logs with `LOG_LEVEL=debug`
- Check SkyPilot API Server health
- Verify network connectivity
