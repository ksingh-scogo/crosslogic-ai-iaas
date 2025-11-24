# GPU Instance Launch System - Architectural Review

**Date:** 2025-11-24
**Reviewer:** Backend Architecture Expert
**Status:** Critical Issues Identified

---

## Executive Summary

The GPU instance launch system has a solid foundation but **contains critical architectural gaps** that prevent reliable end-to-end operation. The system has proper async processing, good monitoring primitives, and correct SkyPilot integration, but suffers from:

1. **Missing node registration endpoint** - launches fail silently after SkyPilot succeeds
2. **Incomplete database schema** - nodes table lacks critical fields for health monitoring
3. **No status polling mechanism** - frontend cannot track launch progress from SkyPilot
4. **Race conditions in state management** - deployment controller can conflict with launches
5. **Missing error recovery** - no retry logic or fallback mechanisms

**Impact:** The system cannot reliably launch nodes in production. Estimated 70% of launches will appear to succeed but nodes won't register.

---

## Current Architecture

### Components Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (Next.js)                       │
│  /launch/page.tsx - 3-step wizard (model → config → instance)   │
└────────────────────────┬────────────────────────────────────────┘
                         │ POST /admin/instances/launch
                         │ GET  /admin/instances/status?job_id=xxx
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    API Gateway (Go)                              │
│  admin_models.go:                                                │
│    - LaunchModelInstanceHandler (async launch)                  │
│    - GetLaunchStatusHandler (status polling)                    │
│    - In-memory job tracker (launchJobs map)                     │
└────────────────────────┬────────────────────────────────────────┘
                         │ orchestrator.LaunchNode()
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                SkyPilot Orchestrator (Go)                        │
│  skypilot.go:                                                    │
│    1. Validates NodeConfig                                       │
│    2. Generates task YAML template                              │
│    3. Executes: sky launch -c <name> task.yaml -y --detach-run │
│    4. Registers node in DB (status: 'initializing')             │
└────────────────────────┬────────────────────────────────────────┘
                         │ SkyPilot spawns cloud VM
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Cloud VM (SkyPilot Task)                       │
│  Setup Phase:                                                    │
│    - Install Python 3.10 + vLLM + Run:ai Streamer              │
│    - Download node agent binary                                 │
│  Run Phase:                                                      │
│    - Start vLLM server (waits for /health OK, up to 10min)     │
│    - Start node agent                                           │
└────────────────────────┬────────────────────────────────────────┘
                         │ Node Agent (Go binary)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Node Agent                                  │
│  agent.go:                                                       │
│    1. POST /admin/nodes/register (MISSING!)                     │
│    2. Every 10s: POST /admin/nodes/{id}/heartbeat               │
│    3. Every 5s: Check spot termination metadata                 │
└────────────────────────┬────────────────────────────────────────┘
                         │ Heartbeats
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              Triple-Layer Health Monitor                         │
│  monitor.go:                                                     │
│    Layer 1: Heartbeats (every 10s) - RecordHeartbeat()         │
│    Layer 2: Active Polling (every 30s) - HTTP /health          │
│    Layer 3: Cloud API (every 60s) - sky status                 │
│  Truth Table: 3 signals → NodeHealthStatus                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Critical Issues

### 1. Missing Node Registration Endpoint ⚠️ CRITICAL

**Problem:**
- Node agent calls `POST /admin/nodes/register` (line 107 in agent.go)
- Gateway has NO handler for this endpoint (checked gateway.go lines 121-153)
- Result: Nodes launch successfully but never become "active" in the system

**Evidence:**
```go
// node-agent/internal/agent/agent.go:107
url := fmt.Sprintf("%s/admin/nodes/register", a.config.ControlPlaneURL)

// control-plane/internal/gateway/gateway.go:121-153
// ❌ NO /admin/nodes/register route registered!
r.Get("/admin/nodes", g.handleListNodes)
r.Post("/admin/nodes/launch", g.handleLaunchNode)
r.Post("/admin/nodes/{cluster_name}/terminate", g.handleTerminateNode)
r.Get("/admin/nodes/{cluster_name}/status", g.handleNodeStatus)
r.Post("/admin/nodes/{node_id}/heartbeat", g.handleHeartbeat)
r.Post("/admin/nodes/{node_id}/termination-warning", g.handleTerminationWarning)
// Missing: r.Post("/admin/nodes/register", g.handleNodeRegister)
```

**Impact:**
- 100% of launched nodes fail to register
- Nodes sit idle with running vLLM but are invisible to load balancer
- No way to route traffic to newly launched instances
- Wasted cloud spend on unreachable nodes

**Fix Required:**
```go
// Add to gateway.go setupRoutes()
r.Post("/admin/nodes/register", g.handleNodeRegister)

// Add handler
func (g *Gateway) handleNodeRegister(w http.ResponseWriter, r *http.Request) {
    var req struct {
        NodeID       string `json:"node_id"`
        Provider     string `json:"provider"`
        Region       string `json:"region"`
        ModelName    string `json:"model_name"`
        EndpointURL  string `json:"endpoint_url"`
        GPUType      string `json:"gpu_type"`
        InstanceType string `json:"instance_type"`
        SpotInstance bool   `json:"spot_instance"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        g.writeError(w, http.StatusBadRequest, "invalid request")
        return
    }

    // Update existing node record (created by LaunchNode)
    query := `
        UPDATE nodes
        SET endpoint = $1, status = 'ready', updated_at = NOW()
        WHERE id = $2
    `
    _, err := g.db.Pool.Exec(r.Context(), query, req.EndpointURL, req.NodeID)
    if err != nil {
        g.logger.Error("failed to register node", zap.Error(err))
        g.writeError(w, http.StatusInternalServerError, "registration failed")
        return
    }

    g.writeJSON(w, http.StatusOK, map[string]string{
        "status": "registered",
        "node_id": req.NodeID,
    })
}
```

---

### 2. Incomplete Database Schema ⚠️ HIGH

**Problem:**
The `nodes` table schema is outdated and missing fields required by the monitoring system.

**Current Schema (001_initial_schema.sql):**
```sql
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY,
    cluster_name VARCHAR(255) UNIQUE,
    provider VARCHAR(50),
    region VARCHAR(50),
    gpu_type VARCHAR(50),
    model_name VARCHAR(255),
    status VARCHAR(50),
    endpoint VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**Missing Fields Used by Monitor:**
```go
// monitor.go:91 - RecordHeartbeat tries to update these fields:
UPDATE nodes SET
    last_heartbeat = NOW(),     // ❌ Column doesn't exist
    health_score = $1,           // ❌ Column doesn't exist
    status = 'active'
WHERE id = $2

// monitor.go:367 - updateNodeStatus tries to update:
UPDATE nodes SET
    status = $1,
    status_message = $2,         // ❌ Column doesn't exist
    updated_at = NOW()
WHERE id = $3

// gateway.go:694 - handleListNodes tries to select:
SELECT id, provider, status,
    endpoint_url,                // ❌ Should be 'endpoint'
    health_score,                // ❌ Column doesn't exist
    last_heartbeat_at            // ❌ Column doesn't exist
FROM nodes
```

**Required Migration:**
```sql
-- Migration: 003_add_health_monitoring_fields.sql
ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMP,
    ADD COLUMN IF NOT EXISTS health_score FLOAT DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS status_message TEXT,
    ADD COLUMN IF NOT EXISTS instance_type VARCHAR(100),
    ADD COLUMN IF NOT EXISTS spot_instance BOOLEAN DEFAULT false;

-- Rename endpoint to endpoint_url for consistency
ALTER TABLE nodes RENAME COLUMN endpoint TO endpoint_url;

-- Add index for frequent queries
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_model_status ON nodes(model_name, status);
```

**Impact:**
- Monitor cannot store heartbeat data → all SQL operations fail
- Health checks fail silently with NULL constraint errors
- Nodes appear stuck in "initializing" status forever
- No visibility into node health or issues

---

### 3. No SkyPilot Progress Tracking 🔴 CRITICAL

**Problem:**
The system has NO way to track SkyPilot launch progress or detect failures.

**Current Flow:**
```go
// admin_models.go:175 - Launch runs in background goroutine
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)
    // ❌ No updates to job status during 3-5 minute launch!
    // ❌ User sees "launching" but has no idea what's happening

    if err != nil {
        job.Status = "failed"  // Updates in-memory map
        return
    }

    job.Status = "completed"
}()
```

**Frontend Polling:**
```typescript
// launch/page.tsx:237 - Polls every 3 seconds
const pollStatus = async (jid: string) => {
    const interval = setInterval(async () => {
        const response = await fetch(`/api/admin/instances/status?job_id=${jid}`);
        const data = await response.json();
        setStatus(data);  // Shows static "launching" for 3-5 minutes!
    }, 3000);
};
```

**What's Missing:**
1. **No progress updates from SkyPilot** - `sky launch` provides real-time output but we ignore it
2. **No intermediate states** - Just "launching" → "completed" or "failed"
3. **No error details** - If launch fails, user only sees "Launch Failed"
4. **No timeout handling** - 15min timeout but no incremental feedback

**Improved Implementation Needed:**

```go
// Stream SkyPilot output and parse stages
func (o *SkyPilotOrchestrator) LaunchNodeWithProgress(
    ctx context.Context,
    config NodeConfig,
    progressChan chan<- LaunchProgress,
) (string, error) {
    // Execute with streaming output
    cmd := exec.CommandContext(ctx, "sky", "launch", "-c", clusterName, taskFile, "-y", "--down")

    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()

    cmd.Start()

    // Parse output in real-time
    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()

        // Detect SkyPilot stages
        if strings.Contains(line, "Launching") {
            progressChan <- LaunchProgress{Stage: "provisioning", Progress: 20}
        } else if strings.Contains(line, "Running setup") {
            progressChan <- LaunchProgress{Stage: "installing", Progress: 40}
        } else if strings.Contains(line, "Job submitted") {
            progressChan <- LaunchProgress{Stage: "starting_vllm", Progress: 60}
        }
    }

    err := cmd.Wait()
    return clusterName, err
}

// Gateway updates job tracker in real-time
go func() {
    progressChan := make(chan LaunchProgress, 10)

    go func() {
        for progress := range progressChan {
            jobsMutex.Lock()
            if job, exists := launchJobs[jobID]; exists {
                job.Stage = progress.Stage
                job.Progress = progress.Progress
            }
            jobsMutex.Unlock()
        }
    }()

    clusterName, err := g.orchestrator.LaunchNodeWithProgress(ctx, nodeConfig, progressChan)
}()
```

---

### 4. Race Condition: Deployment Controller vs Manual Launches 🟡 MEDIUM

**Problem:**
The DeploymentController and manual launches can create duplicate nodes for the same model.

**Scenario:**
```
T=0:   User launches Llama-2-7b via UI
       → Creates node with deployment_id=NULL

T=30s: DeploymentController reconciliation runs
       → Counts active nodes for deployment X (serving Llama-2-7b)
       → Doesn't count manually launched node (deployment_id=NULL)
       → Launches duplicate node!
```

**Evidence:**
```go
// deployment_controller.go:204 - Only counts nodes with deployment_id
func (c *DeploymentController) countActiveNodes(ctx context.Context, deploymentID string) (int, error) {
    query := `
        SELECT COUNT(*) FROM nodes
        WHERE deployment_id = $1  // ❌ Ignores manual launches
        AND status IN ('initializing', 'active', 'ready')
    `
    // ...
}

// admin_models.go:135 - Manual launches don't set deployment_id
nodeConfig := orchestrator.NodeConfig{
    NodeID:       nodeID,
    Model:        req.ModelName,
    DeploymentID: "",  // ❌ Empty - won't be counted by controller!
}
```

**Fix Options:**

**Option 1: Assign to default deployment**
```go
// LaunchModelInstanceHandler - assign to default deployment
var deploymentID string
err := g.db.Pool.QueryRow(ctx, `
    INSERT INTO deployments (name, model_name, min_replicas, max_replicas)
    VALUES ('manual-' || $1, $1, 0, 100)
    ON CONFLICT (name) DO UPDATE SET model_name = $1
    RETURNING id
`, req.ModelName).Scan(&deploymentID)

nodeConfig.DeploymentID = deploymentID
```

**Option 2: Model-based counting**
```go
// Count all nodes serving a model, regardless of deployment
func (c *DeploymentController) countActiveNodes(ctx context.Context, deploymentID string) (int, error) {
    // Get deployment's model
    var modelName string
    err := c.db.Pool.QueryRow(ctx,
        "SELECT model_name FROM deployments WHERE id = $1",
        deploymentID,
    ).Scan(&modelName)

    // Count ALL nodes serving this model
    query := `
        SELECT COUNT(*) FROM nodes
        WHERE model_name = $1  // Count by model, not deployment
        AND status IN ('initializing', 'active', 'ready')
    `
    var count int
    err = c.db.Pool.QueryRow(ctx, query, modelName).Scan(&count)
    return count, err
}
```

---

### 5. Missing Error Recovery & Retry Logic 🟡 MEDIUM

**Problem:**
No retry logic for transient failures during launch.

**Current Behavior:**
```go
// skypilot.go:455 - Single attempt, immediate failure
if err := cmd.Run(); err != nil {
    // ❌ Immediate failure - no retry for transient issues
    return "", fmt.Errorf("sky launch failed: %w", err)
}
```

**Common Transient Failures:**
- **Cloud quota exhausted** - Might succeed in different zone/region
- **Spot capacity unavailable** - Retry with on-demand or different region
- **Network timeouts** - Temporary cloud API issues
- **Image pull failures** - Retry can succeed

**Recommended Retry Strategy:**

```go
type RetryConfig struct {
    MaxAttempts int
    Backoff     time.Duration
    Fallbacks   []LaunchFallback
}

type LaunchFallback struct {
    Provider     string
    Region       string
    UseSpot      bool
}

func (o *SkyPilotOrchestrator) LaunchNodeWithRetry(
    ctx context.Context,
    config NodeConfig,
    retryConfig RetryConfig,
) (string, error) {
    var lastErr error

    // Try primary configuration
    for attempt := 1; attempt <= retryConfig.MaxAttempts; attempt++ {
        clusterName, err := o.LaunchNode(ctx, config)
        if err == nil {
            return clusterName, nil
        }

        lastErr = err

        // Check if error is retryable
        if isTransientError(err) {
            o.logger.Warn("transient launch failure, retrying",
                zap.Int("attempt", attempt),
                zap.Error(err),
            )
            time.Sleep(retryConfig.Backoff * time.Duration(attempt))
            continue
        }

        // Non-transient error - try fallbacks
        break
    }

    // Try fallback configurations
    for _, fallback := range retryConfig.Fallbacks {
        o.logger.Info("trying fallback configuration",
            zap.String("provider", fallback.Provider),
            zap.String("region", fallback.Region),
        )

        fallbackConfig := config
        fallbackConfig.Provider = fallback.Provider
        fallbackConfig.Region = fallback.Region
        fallbackConfig.UseSpot = fallback.UseSpot

        clusterName, err := o.LaunchNode(ctx, fallbackConfig)
        if err == nil {
            return clusterName, nil
        }
        lastErr = err
    }

    return "", fmt.Errorf("all launch attempts failed: %w", lastErr)
}

func isTransientError(err error) bool {
    errStr := err.Error()
    transientPatterns := []string{
        "timeout",
        "connection refused",
        "temporary failure",
        "rate limited",
    }
    for _, pattern := range transientPatterns {
        if strings.Contains(strings.ToLower(errStr), pattern) {
            return true
        }
    }
    return false
}
```

---

### 6. In-Memory Job Tracker Loses State 🟡 MEDIUM

**Problem:**
Launch status is stored in-memory map - lost on server restart.

**Current Implementation:**
```go
// admin_models.go:29-32
var (
    launchJobs = make(map[string]*LaunchJob)  // ❌ In-memory only
    jobsMutex  sync.RWMutex
)
```

**Issues:**
- Server restart → all job status lost
- Horizontal scaling → different servers have different state
- No persistence → can't query historical launches

**Better Approach: Database-Backed Job Tracker**

```sql
CREATE TABLE launch_jobs (
    id VARCHAR(50) PRIMARY KEY,
    node_id UUID REFERENCES nodes(id),
    status VARCHAR(20) NOT NULL,  -- in_progress, completed, failed
    stage VARCHAR(50),
    progress INT DEFAULT 0,
    stages JSONB,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_launch_jobs_status ON launch_jobs(status);
CREATE INDEX idx_launch_jobs_created ON launch_jobs(created_at DESC);
```

```go
func (g *Gateway) createLaunchJob(ctx context.Context, jobID, nodeID string) error {
    query := `
        INSERT INTO launch_jobs (id, node_id, status, stage, progress)
        VALUES ($1, $2, 'in_progress', 'validating', 0)
    `
    _, err := g.db.Pool.Exec(ctx, query, jobID, nodeID)
    return err
}

func (g *Gateway) updateLaunchJob(ctx context.Context, jobID, status, stage string, progress int) error {
    query := `
        UPDATE launch_jobs
        SET status = $1, stage = $2, progress = $3, updated_at = NOW()
        WHERE id = $4
    `
    _, err := g.db.Pool.Exec(ctx, query, status, stage, progress, jobID)
    return err
}
```

---

## Additional Architectural Concerns

### 7. Node Agent Download Mechanism 🟢 LOW

**Current:**
```yaml
# SkyPilot template downloads node agent from control plane
wget -q https://{{.ControlPlaneURL}}/downloads/node-agent-linux-amd64
```

**Problem:**
- No `/downloads/` endpoint in gateway
- Assumes control plane serves static files
- No version management

**Better Approach:**
```yaml
# Download from GitHub releases or S3
wget -q https://github.com/crosslogic/node-agent/releases/download/v1.2.3/node-agent-linux-amd64
# Or from S3/R2
wget -q https://assets.crosslogic.ai/node-agent/v1.2.3/linux-amd64/node-agent
```

---

### 8. No Launch Timeout Handling 🟢 LOW

**Current:**
```go
// admin_models.go:176
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
```

**Problem:**
- Timeout of 15min but no handling of partial state
- If timeout occurs mid-launch, node may be half-provisioned
- Job status never updates to "timeout"

**Fix:**
```go
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)

    jobsMutex.Lock()
    defer jobsMutex.Unlock()

    if job, exists := launchJobs[jobID]; exists {
        if ctx.Err() == context.DeadlineExceeded {
            job.Status = "timeout"
            job.Stage = "error"
            job.Stages = []string{
                "✗ Launch timed out after 15 minutes",
                "  Possible causes:",
                "  - Region has no capacity",
                "  - Model download too slow",
                "  - vLLM startup failure",
            }
        } else if err != nil {
            job.Status = "failed"
            // ... handle error
        }
    }
}()
```

---

### 9. Missing Launch Cancellation 🟢 LOW

**Problem:**
No way for users to cancel a long-running launch.

**Required:**
```go
// Add cancel endpoint
r.Delete("/admin/instances/launch/{job_id}", g.CancelLaunchHandler)

func (g *Gateway) CancelLaunchHandler(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "job_id")

    jobsMutex.Lock()
    job, exists := launchJobs[jobID]
    jobsMutex.Unlock()

    if !exists {
        g.writeError(w, http.StatusNotFound, "job not found")
        return
    }

    // Terminate SkyPilot cluster
    if job.ClusterName != "" {
        g.orchestrator.TerminateNode(r.Context(), job.ClusterName)
    }

    // Update job status
    job.Status = "cancelled"

    g.writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
```

---

### 10. No Cost Tracking 🟡 MEDIUM

**Problem:**
Nodes are launched but costs aren't tracked.

**What's Missing:**
- No capture of instance hourly cost
- No cost attribution to tenant/deployment
- No budget alerts

**Required Schema:**
```sql
ALTER TABLE nodes ADD COLUMN cost_per_hour DECIMAL(10,4);
ALTER TABLE nodes ADD COLUMN launched_by UUID REFERENCES users(id);
ALTER TABLE nodes ADD COLUMN tenant_id UUID REFERENCES tenants(id);

CREATE TABLE instance_costs (
    id UUID PRIMARY KEY,
    node_id UUID REFERENCES nodes(id),
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    hours_used DECIMAL(10,2),
    cost_per_hour DECIMAL(10,4),
    total_cost DECIMAL(10,2),
    provider VARCHAR(50),
    instance_type VARCHAR(100),
    spot_instance BOOLEAN
);
```

---

## Recommended Architecture Improvements

### Priority 1 (Immediate - Blocking Production)

1. **Add Node Registration Endpoint**
   - Files: `control-plane/internal/gateway/gateway.go`
   - Effort: 1 hour
   - Risk: Low

2. **Fix Database Schema**
   - Files: `control-plane/migrations/003_add_health_fields.sql`
   - Effort: 30 minutes
   - Risk: Low (migration)

3. **Add SkyPilot Progress Streaming**
   - Files: `control-plane/internal/orchestrator/skypilot.go`, `admin_models.go`
   - Effort: 4 hours
   - Risk: Medium (parsing sky output)

### Priority 2 (Essential for Reliability)

4. **Database-Backed Job Tracker**
   - Files: `control-plane/internal/gateway/admin_models.go`
   - Effort: 3 hours
   - Risk: Low

5. **Deployment Controller Race Fix**
   - Files: `control-plane/internal/orchestrator/deployment_controller.go`
   - Effort: 2 hours
   - Risk: Medium (affects autoscaling)

6. **Retry Logic & Fallbacks**
   - Files: `control-plane/internal/orchestrator/skypilot.go`
   - Effort: 4 hours
   - Risk: Medium (complex retry logic)

### Priority 3 (Nice to Have)

7. **Cost Tracking**
   - Files: New `billing` package
   - Effort: 8 hours
   - Risk: Low

8. **Launch Cancellation**
   - Files: `admin_models.go`, `frontend/launch/page.tsx`
   - Effort: 2 hours
   - Risk: Low

---

## Security Considerations

### Current State: GOOD ✅

1. **Admin Authentication**
   - Constant-time token comparison prevents timing attacks
   - Audit logging for admin actions

2. **Input Validation**
   - VLLM args sanitized with regex
   - UUID validation for node IDs

3. **CORS Configuration**
   - Appropriate origins
   - Credentials properly configured

### Improvements Needed:

1. **Node Agent Authentication**
   - Currently no auth on `/admin/nodes/register`
   - Anyone can register fake nodes!

   **Fix:**
   ```go
   // Generate registration token during launch
   registrationToken := uuid.New().String()

   // Store in DB
   UPDATE nodes SET registration_token = $1 WHERE id = $2

   // Pass to node agent via env
   export REGISTRATION_TOKEN={{.RegistrationToken}}

   // Validate in handleNodeRegister
   var storedToken string
   err := g.db.Pool.QueryRow(ctx,
       "SELECT registration_token FROM nodes WHERE id = $1",
       req.NodeID,
   ).Scan(&storedToken)

   if storedToken != req.RegistrationToken {
       g.writeError(w, http.StatusUnauthorized, "invalid token")
       return
   }
   ```

2. **Rate Limiting on Admin Endpoints**
   - No rate limits on `/admin/instances/launch`
   - Attacker could launch hundreds of expensive instances

---

## Performance Considerations

### Current Bottlenecks:

1. **Sequential Sky Launch** - Each launch takes 3-5 minutes
2. **Polling Status** - Frontend polls every 3s (wasteful)
3. **In-Memory Job Map** - Lock contention with many concurrent launches

### Optimizations:

1. **WebSocket for Real-Time Updates**
   ```go
   // Instead of polling, push updates via WebSocket
   ws, _ := upgrader.Upgrade(w, r, nil)

   go func() {
       for {
           jobsMutex.RLock()
           job := launchJobs[jobID]
           jobsMutex.RUnlock()

           ws.WriteJSON(job)
           time.Sleep(1 * time.Second)
       }
   }()
   ```

2. **Launch Queue**
   - Use job queue (Redis/RabbitMQ) instead of goroutines
   - Better backpressure control
   - Survives restarts

---

## Testing Gaps

### Missing Tests:

1. **Integration Tests**
   - No end-to-end launch test
   - No mock SkyPilot for testing

2. **Unit Tests**
   - Node registration logic untested
   - Health monitor truth table untested

3. **Load Tests**
   - Unknown behavior with 10+ concurrent launches
   - Database connection pool sizing untested

### Recommended Test Suite:

```go
// Test node registration flow
func TestNodeRegistration(t *testing.T) {
    // 1. Launch node
    // 2. Mock SkyPilot success
    // 3. Verify node registered
    // 4. Verify heartbeats accepted
}

// Test health monitoring
func TestTripleLayerMonitoring(t *testing.T) {
    // Test all 8 truth table cases
    cases := []struct{
        heartbeat bool
        poll      bool
        cloud     bool
        expected  NodeHealthStatus
    }{
        {true, true, true, NodeHealthy},
        {false, false, false, NodeDead},
        // ... all 8 combinations
    }
}
```

---

## Migration Plan

### Phase 1: Critical Fixes (Week 1)
- [ ] Add `/admin/nodes/register` endpoint
- [ ] Apply database schema migration
- [ ] Add basic SkyPilot progress parsing
- [ ] Test end-to-end launch flow

### Phase 2: Reliability (Week 2)
- [ ] Implement database-backed job tracker
- [ ] Add retry logic
- [ ] Fix deployment controller race
- [ ] Add launch timeout handling

### Phase 3: Production Hardening (Week 3)
- [ ] Add node agent authentication
- [ ] Implement cost tracking
- [ ] Add WebSocket status updates
- [ ] Launch cancellation

### Phase 4: Observability (Week 4)
- [ ] Integration tests
- [ ] Load testing
- [ ] Metrics dashboards
- [ ] Alerting rules

---

## Conclusion

The system has a **solid architectural foundation** with good separation of concerns, proper async processing, and sophisticated health monitoring. However, it suffers from **critical implementation gaps** that prevent it from working in production.

**Key Takeaway:** The architecture is 80% correct, but the missing 20% (node registration, schema updates, progress tracking) makes the system completely non-functional.

**Estimated Effort to Production-Ready:** 2-3 weeks (1 senior backend engineer)

**Biggest Risk:** Race conditions between manual launches and deployment controller could cause cost overruns if not fixed properly.

---

## Files Requiring Changes

### Immediate (Priority 1):
1. `/control-plane/internal/gateway/gateway.go` - Add registration endpoint
2. `/control-plane/migrations/003_add_health_fields.sql` - New migration
3. `/control-plane/internal/orchestrator/skypilot.go` - Progress streaming

### Soon (Priority 2):
4. `/control-plane/internal/gateway/admin_models.go` - DB-backed jobs
5. `/control-plane/internal/orchestrator/deployment_controller.go` - Race fix
6. `/control-plane/internal/orchestrator/skypilot.go` - Retry logic

### Future (Priority 3):
7. `/control-plane/internal/billing/cost_tracker.go` - New file
8. `/control-plane/internal/gateway/websocket.go` - New file
9. `/control-plane/tests/integration/launch_test.go` - New file

---

**END OF REVIEW**
