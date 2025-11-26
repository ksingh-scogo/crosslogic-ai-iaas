# SkyPilot HTTP Client

A production-ready HTTP client for the SkyPilot API Server, designed for the CrossLogic AI IaaS control plane.

## Features

- **Complete API Coverage**: All SkyPilot API operations (launch, terminate, status, etc.)
- **Production-Ready**: Retry logic, connection pooling, timeout handling
- **Multi-Tenant Support**: Dynamic credential injection per request
- **Comprehensive Logging**: Integration with zap logger for debugging
- **Error Handling**: Detailed error types with retry categorization
- **Context Support**: Proper context cancellation and timeout handling
- **Exponential Backoff**: Smart retry strategy for transient failures

## Installation

The client is part of the control plane's internal packages and is already available.

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/crosslogic/control-plane/internal/skypilot"
    "go.uber.org/zap"
)

func main() {
    // Initialize logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Create client
    client := skypilot.NewClient(skypilot.Config{
        BaseURL: "https://skypilot-api.example.com",
        Token:   "sky_sa_your_service_account_token",
        Timeout: 5 * time.Minute,
    }, logger)
    defer client.Close()

    // Check API health
    ctx := context.Background()
    health, err := client.Health(ctx)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("SkyPilot API is %s, version %s", health.Status, health.Version)
}
```

## Common Operations

### 1. Launch a Cluster

```go
// Define cluster configuration via SkyPilot YAML
taskYAML := `
resources:
  cloud: aws
  region: us-east-1
  instance_type: g5.4xlarge
  accelerators: A10G:1

setup: |
  conda create -n vllm python=3.10 -y
  conda activate vllm
  pip install vllm==0.6.2

run: |
  conda activate vllm
  python -m vllm.entrypoints.openai.api_server \
    --model NousResearch/Meta-Llama-3-8B-Instruct \
    --port 8000 \
    --host 0.0.0.0
`

// Launch the cluster
resp, err := client.Launch(ctx, skypilot.LaunchRequest{
    ClusterName:       "llama3-8b-prod",
    TaskYAML:          taskYAML,
    RetryUntilUp:      true,
    Detach:            true,
    IdleMinutesToStop: 30,
    Envs: map[string]string{
        "VLLM_PORT": "8000",
        "MODEL_NAME": "NousResearch/Meta-Llama-3-8B-Instruct",
    },
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Launch initiated, request ID: %s", resp.RequestID)
```

### 2. Launch with Dynamic Cloud Credentials (Multi-Tenant)

```go
// For multi-tenant scenarios, inject tenant-specific credentials
resp, err := client.Launch(ctx, skypilot.LaunchRequest{
    ClusterName:  "tenant-abc-cluster",
    TaskYAML:     taskYAML,
    RetryUntilUp: true,
    Detach:       true,

    // Dynamically provide tenant's cloud credentials
    CloudCredentials: &skypilot.CloudCredentials{
        AWS: &skypilot.AWSCredentials{
            AccessKeyID:     tenantAWSAccessKey,
            SecretAccessKey: tenantAWSSecretKey,
            Region:          "us-west-2",
        },
    },
})
```

### 3. Wait for Cluster Launch Completion

```go
// Wait for the async launch to complete
status, err := client.WaitForRequest(ctx, resp.RequestID, 10*time.Second)
if err != nil {
    log.Fatalf("Launch failed: %v", err)
}

if status.Status == "completed" {
    log.Printf("Cluster launched successfully!")
} else {
    log.Fatalf("Launch failed: %s", status.Error)
}
```

### 4. Get Cluster Status

```go
status, err := client.GetStatus(ctx, "llama3-8b-prod")
if err != nil {
    if apiErr, ok := err.(*skypilot.APIError); ok && apiErr.IsNotFound() {
        log.Println("Cluster not found")
        return
    }
    log.Fatal(err)
}

log.Printf("Cluster: %s", status.Name)
log.Printf("Status: %s", status.Status)
log.Printf("Provider: %s", status.Provider)
log.Printf("Region: %s", status.Region)
log.Printf("GPUs: %s", status.Resources.Accelerators)
log.Printf("Cost/hour: $%.2f", status.CostPerHour)
```

### 5. List All Clusters

```go
list, err := client.ListClusters(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Total clusters: %d", list.Total)
for _, cluster := range list.Clusters {
    log.Printf("  - %s: %s (%s, %s)",
        cluster.Name,
        cluster.Status,
        cluster.Provider,
        cluster.Region)
}
```

### 6. Execute Command on Cluster

```go
result, err := client.Execute(ctx, skypilot.ExecuteRequest{
    ClusterName: "llama3-8b-prod",
    Command:     "nvidia-smi",
    Timeout:     30,
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Exit code: %d", result.ExitCode)
log.Printf("Output:\n%s", result.Stdout)
if result.Stderr != "" {
    log.Printf("Errors:\n%s", result.Stderr)
}
```

### 7. Get Cluster Logs

```go
logs, err := client.GetLogs(ctx, skypilot.LogsRequest{
    ClusterName: "llama3-8b-prod",
    TailLines:   100,
    Since:       "1h", // Last hour of logs
})
if err != nil {
    log.Fatal(err)
}

log.Println("Recent logs:")
log.Println(logs.Logs)
```

### 8. Estimate Cost Before Launch

```go
estimate, err := client.EstimateCost(ctx, skypilot.EstimateCostRequest{
    TaskYAML: taskYAML,
    Hours:    24, // Estimate for 24 hours
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Estimated cost for 24h: $%.2f %s",
    estimate.EstimatedCost,
    estimate.Currency)
log.Printf("Cost per hour: $%.2f", estimate.CostPerHour)
log.Printf("Provider: %s", estimate.Provider)
log.Printf("Region: %s", estimate.Region)
log.Printf("Instance: %s", estimate.InstanceType)
```

### 9. Terminate a Cluster

```go
resp, err := client.Terminate(ctx, "llama3-8b-prod", false)
if err != nil {
    log.Fatal(err)
}

log.Printf("Termination initiated, request ID: %s", resp.RequestID)

// Wait for termination to complete
status, err := client.WaitForRequest(ctx, resp.RequestID, 5*time.Second)
if err != nil {
    log.Fatalf("Termination failed: %v", err)
}

log.Println("Cluster terminated successfully")
```

## Advanced Configuration

### Custom Retry Configuration

```go
client := skypilot.NewClient(skypilot.Config{
    BaseURL:       "https://skypilot-api.example.com",
    Token:         "sky_sa_token",
    Timeout:       10 * time.Minute,

    // Retry configuration
    MaxRetries:    5,                  // Maximum retry attempts
    RetryDelay:    2 * time.Second,    // Initial retry delay
    RetryMaxDelay: 60 * time.Second,   // Maximum backoff delay

    // Connection pool configuration
    MaxIdleConns:    200,               // Maximum idle connections
    IdleConnTimeout: 120 * time.Second, // Idle connection timeout
}, logger)
```

### Context with Timeout

```go
// Set a timeout for the entire operation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

resp, err := client.Launch(ctx, launchReq)
if err != nil {
    if err == context.DeadlineExceeded {
        log.Println("Operation timed out after 30 minutes")
    }
    log.Fatal(err)
}
```

## Error Handling

### Checking Error Types

```go
status, err := client.GetStatus(ctx, "my-cluster")
if err != nil {
    // Check if it's an API error
    if apiErr, ok := err.(*skypilot.APIError); ok {
        switch {
        case apiErr.IsNotFound():
            log.Println("Cluster does not exist")
            return nil
        case apiErr.IsUnauthorized():
            log.Println("Invalid authentication token")
            return err
        case apiErr.IsRateLimited():
            log.Println("Rate limit exceeded, retry later")
            time.Sleep(60 * time.Second)
            return retryOperation()
        default:
            log.Printf("API error: %s (code: %s)",
                apiErr.Message,
                apiErr.ErrorCode)
        }
    }

    // Network or other errors
    log.Printf("Request failed: %v", err)
    return err
}
```

## Integration with Control Plane

### Using in Orchestrator

```go
package orchestrator

import (
    "github.com/crosslogic/control-plane/internal/skypilot"
)

type SkyPilotOrchestrator struct {
    db        *database.Database
    logger    *zap.Logger
    apiClient *skypilot.Client
}

func NewSkyPilotOrchestrator(
    db *database.Database,
    logger *zap.Logger,
    cfg config.Config,
) (*SkyPilotOrchestrator, error) {
    // Initialize SkyPilot API client
    skyClient := skypilot.NewClient(skypilot.Config{
        BaseURL: cfg.SkyPilot.APIServerURL,
        Token:   cfg.SkyPilot.ServiceAccountToken,
        Timeout: 10 * time.Minute,
    }, logger)

    return &SkyPilotOrchestrator{
        db:        db,
        logger:    logger,
        apiClient: skyClient,
    }, nil
}

func (o *SkyPilotOrchestrator) LaunchNode(
    ctx context.Context,
    nodeCfg NodeConfig,
) (*Node, error) {
    // Build SkyPilot task YAML
    taskYAML := o.buildTaskYAML(nodeCfg)

    // Get tenant's cloud credentials from database
    creds, err := o.db.GetTenantCredentials(ctx, nodeCfg.TenantID)
    if err != nil {
        return nil, err
    }

    // Launch cluster with tenant-specific credentials
    resp, err := o.apiClient.Launch(ctx, skypilot.LaunchRequest{
        ClusterName:       nodeCfg.ClusterName,
        TaskYAML:          taskYAML,
        RetryUntilUp:      true,
        Detach:            true,
        IdleMinutesToStop: 30,
        CloudCredentials:  creds.ToSkyPilotFormat(),
    })
    if err != nil {
        return nil, fmt.Errorf("launch cluster: %w", err)
    }

    o.logger.Info("cluster launch initiated",
        zap.String("cluster_name", nodeCfg.ClusterName),
        zap.String("request_id", resp.RequestID),
    )

    // Save to database
    node := &Node{
        ClusterName: nodeCfg.ClusterName,
        Status:      "launching",
        RequestID:   resp.RequestID,
        TenantID:    nodeCfg.TenantID,
    }

    if err := o.db.CreateNode(ctx, node); err != nil {
        return nil, err
    }

    return node, nil
}
```

## Best Practices

1. **Always use contexts**: Pass context for timeout and cancellation support
2. **Handle errors properly**: Check for specific error types (404, 401, 429)
3. **Use connection pooling**: The client reuses connections automatically
4. **Close when done**: Call `client.Close()` to release resources
5. **Log appropriately**: Use structured logging with zap
6. **Secure credentials**: Never log cloud credentials or tokens
7. **Monitor retries**: Watch for excessive retries indicating systemic issues
8. **Test timeout scenarios**: Ensure your code handles timeouts gracefully

## Environment Variables

For production deployment, configure via environment variables:

```bash
# SkyPilot API Server Configuration
SKYPILOT_API_SERVER_URL=https://skypilot-api.crosslogic.ai
SKYPILOT_SERVICE_ACCOUNT_TOKEN=sky_sa_xxxxxxxxxxxx

# Optional: Timeouts and retries
SKYPILOT_REQUEST_TIMEOUT=10m
SKYPILOT_MAX_RETRIES=3
SKYPILOT_RETRY_DELAY=1s
```

## Troubleshooting

### Connection Timeouts

If you see connection timeouts:
- Increase `Timeout` in client config
- Check network connectivity to API server
- Verify firewall rules allow HTTPS traffic

### Authentication Errors

If you see 401 Unauthorized:
- Verify service account token is valid
- Check token has not expired
- Ensure token has required permissions

### Rate Limiting

If you see 429 Too Many Requests:
- The client automatically retries with backoff
- Consider implementing request batching
- Contact API provider to increase rate limits

### Retry Exhaustion

If requests fail after all retries:
- Check API server health status
- Review logs for error patterns
- Increase `MaxRetries` if transient failures are common

## API Reference

See [models.go](./models.go) for complete API request/response types.

Key types:
- `LaunchRequest` / `LaunchResponse` - Cluster launch
- `TerminateRequest` / `TerminateResponse` - Cluster termination
- `ClusterStatus` - Detailed cluster state
- `RequestStatus` - Async operation status
- `CloudCredentials` - Multi-tenant credentials
- `APIError` - Structured error responses

## Testing

Run the test suite:

```bash
cd control-plane
go test -v ./internal/skypilot/...
```

Run specific tests:

```bash
go test -v -run TestLaunch ./internal/skypilot/...
```

## License

Part of the CrossLogic AI IaaS control plane.
