# Critical Fixes Required - GPU Launch System

## TL;DR - System Cannot Launch Nodes in Production

**Status:** 🔴 BROKEN - 0% of nodes will successfully register after launch

**Root Cause:** Missing node registration endpoint causes all launched nodes to fail after SkyPilot succeeds.

---

## The 3 Critical Fixes (Must Fix Before ANY Production Use)

### 1. Missing Node Registration Endpoint ⚠️ BLOCKING

**What's Broken:**
- Node agent tries to register at `POST /admin/nodes/register`
- Gateway has NO handler for this route
- Result: Nodes launch successfully but never become "ready"

**The Smoking Gun:**
```bash
# This is what happens now:
1. Frontend → "Launch Instance" → API Gateway ✅
2. Gateway → SkyPilot → Cloud VM starts ✅
3. VM → Installs vLLM → Starts successfully ✅
4. Node Agent → POST /admin/nodes/register → 404 NOT FOUND ❌
5. Load Balancer → No nodes available ❌
6. User → Requests fail with "no healthy nodes" ❌
```

**Fix (15 minutes):**

```go
// File: control-plane/internal/gateway/gateway.go
// Add to setupRoutes() at line ~127:
r.Post("/admin/nodes/register", g.handleNodeRegister)

// Add this handler at the end of the file:
func (g *Gateway) handleNodeRegister(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Provider     string `json:"provider"`
        Region       string `json:"region"`
        ModelName    string `json:"model_name"`
        EndpointURL  string `json:"endpoint_url"`
        GPUType      string `json:"gpu_type"`
        InstanceType string `json:"instance_type"`
        SpotInstance bool   `json:"spot_instance"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        g.writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Extract node ID from URL path
    nodeID := r.URL.Query().Get("node_id")
    if nodeID == "" {
        g.writeError(w, http.StatusBadRequest, "node_id is required")
        return
    }

    // Update node record (created during launch)
    query := `
        UPDATE nodes
        SET endpoint_url = $1,
            status = 'ready',
            updated_at = NOW()
        WHERE id = $2
    `

    result, err := g.db.Pool.Exec(r.Context(), query, req.EndpointURL, nodeID)
    if err != nil {
        g.logger.Error("failed to register node",
            zap.String("node_id", nodeID),
            zap.Error(err))
        g.writeError(w, http.StatusInternalServerError, "registration failed")
        return
    }

    if result.RowsAffected() == 0 {
        g.writeError(w, http.StatusNotFound, "node not found")
        return
    }

    g.logger.Info("node registered successfully",
        zap.String("node_id", nodeID),
        zap.String("endpoint", req.EndpointURL),
        zap.String("model", req.ModelName))

    g.writeJSON(w, http.StatusOK, map[string]interface{}{
        "status":  "registered",
        "node_id": nodeID,
    })
}
```

**Also update node agent to pass node_id:**
```go
// File: node-agent/internal/agent/agent.go
// Update line 107 from:
url := fmt.Sprintf("%s/admin/nodes/register", a.config.ControlPlaneURL)

// To:
url := fmt.Sprintf("%s/admin/nodes/register?node_id=%s",
    a.config.ControlPlaneURL,
    a.config.NodeID)
```

---

### 2. Database Schema Missing Health Fields ⚠️ BLOCKING

**What's Broken:**
- Monitor tries to UPDATE columns that don't exist
- Every heartbeat fails with SQL error
- Nodes stuck in "initializing" status forever

**The Error:**
```sql
-- Monitor tries to run this:
UPDATE nodes SET last_heartbeat = NOW(), health_score = 0.8 WHERE id = '...'
-- ERROR: column "last_heartbeat" does not exist

-- Gateway tries to SELECT this:
SELECT health_score, last_heartbeat_at FROM nodes
-- ERROR: column "health_score" does not exist
```

**Fix (5 minutes):**

Create file: `control-plane/migrations/003_add_health_monitoring_fields.sql`

```sql
-- Add missing health monitoring columns
ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMP,
    ADD COLUMN IF NOT EXISTS health_score FLOAT DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS status_message TEXT;

-- Rename endpoint to endpoint_url for consistency with code
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'nodes' AND column_name = 'endpoint'
    ) THEN
        ALTER TABLE nodes RENAME COLUMN endpoint TO endpoint_url;
    END IF;
END $$;

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_model_status ON nodes(model_name, status);
CREATE INDEX IF NOT EXISTS idx_nodes_last_heartbeat ON nodes(last_heartbeat DESC);

-- Update existing nodes to have default health score
UPDATE nodes SET health_score = 1.0 WHERE health_score IS NULL;
```

**Then run:**
```bash
cd control-plane
psql $DATABASE_URL -f migrations/003_add_health_monitoring_fields.sql
```

---

### 3. No SkyPilot Progress Tracking ⚠️ UX CRITICAL

**What's Broken:**
- User sees "Launching..." for 3-5 minutes with no updates
- If launch fails, no details about what went wrong
- No way to know if it's actually progressing or stuck

**Current User Experience:**
```
User clicks "Launch Instance"
→ Shows: "Launching... 0%"
→ 3 minutes pass... still "Launching... 0%"
→ 5 minutes pass... still "Launching... 0%"
→ User refreshes page, confused
→ Eventually: "Launch Failed" (no details!)
```

**Quick Fix (Parse SkyPilot stderr):**

```go
// File: control-plane/internal/orchestrator/skypilot.go
// Replace LaunchNode implementation starting at line 401

func (o *SkyPilotOrchestrator) LaunchNode(ctx context.Context, config NodeConfig) (string, error) {
    startTime := time.Now()

    // Validate config...
    if err := o.validateNodeConfig(&config); err != nil {
        return "", fmt.Errorf("invalid node configuration: %w", err)
    }

    clusterName := GenerateClusterName(config)

    // Generate task YAML...
    taskYAML, err := o.generateTaskYAML(config, clusterName)
    if err != nil {
        return "", fmt.Errorf("failed to generate task YAML: %w", err)
    }

    // Write task file...
    taskFile := fmt.Sprintf("/tmp/sky-task-%s.yaml", config.NodeID)
    if err := os.WriteFile(taskFile, []byte(taskYAML), 0644); err != nil {
        return "", fmt.Errorf("failed to write task file: %w", err)
    }
    defer os.Remove(taskFile)

    // Execute SkyPilot with output capture
    cmd := exec.CommandContext(ctx, "sky", "launch",
        "-c", clusterName,
        taskFile,
        "-y",
        "--down",
        "--detach-run",
    )

    // Capture BOTH stdout and stderr
    var stdoutBuf, stderrBuf bytes.Buffer
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf

    // Execute
    o.logger.Info("executing sky launch command",
        zap.String("cluster_name", clusterName),
        zap.String("task_file", taskFile))

    err = cmd.Run()

    // Capture all output for debugging
    stdout := stdoutBuf.String()
    stderr := stderrBuf.String()

    if err != nil {
        o.logger.Error("SkyPilot launch failed",
            zap.Error(err),
            zap.String("stdout", stdout),
            zap.String("stderr", stderr))

        // Parse error for user-friendly message
        userMsg := parseSkyPilotError(stderr, config)
        return "", fmt.Errorf("%s\n\nDetails: %w", userMsg, err)
    }

    launchDuration := time.Since(startTime)

    o.logger.Info("GPU node launched successfully",
        zap.String("cluster_name", clusterName),
        zap.Duration("launch_duration", launchDuration))

    // Register node in database...
    if err := o.registerNode(ctx, config, clusterName); err != nil {
        o.logger.Warn("node launched but registration failed",
            zap.Error(err),
            zap.String("cluster_name", clusterName))
    }

    return clusterName, nil
}

// Add helper function to parse SkyPilot errors
func parseSkyPilotError(stderr string, config NodeConfig) string {
    if strings.Contains(stderr, "Failed to acquire resources in all zones") {
        return fmt.Sprintf(
            "No spot capacity available in %s region.\n\n"+
            "Try:\n"+
            "1. Different region (westus2, centralindia, southindia)\n"+
            "2. Use on-demand instead of spot\n"+
            "3. Wait 10-15 minutes and retry",
            config.Region)
    }

    if strings.Contains(stderr, "ResourcesUnavailableError") {
        return fmt.Sprintf(
            "Cloud provider has no capacity for %s GPU in %s.\n\n"+
            "Try a different GPU type or region.",
            config.GPU, config.Region)
    }

    if strings.Contains(stderr, "timeout") || strings.Contains(stderr, "timed out") {
        return "Launch timed out. The region might be slow to provision.\n\nTry again or use a different region."
    }

    // Generic error
    return "Launch failed. Check the error details below."
}
```

**Update admin_models.go error handling:**

```go
// File: control-plane/internal/gateway/admin_models.go
// Update the error handling in LaunchModelInstanceHandler (around line 190)

if err != nil {
    g.logger.Error("failed to launch node",
        zap.Error(err),
        zap.String("job_id", jobID))

    if job, exists := launchJobs[jobID]; exists {
        job.Status = "failed"
        job.Stage = "error"

        // Split error message for better display
        errorLines := strings.Split(err.Error(), "\n")
        stages := []string{"✗ Launch failed"}

        for _, line := range errorLines {
            if line != "" {
                stages = append(stages, "  " + line)
            }
        }

        job.Stages = stages
    }
    return
}
```

---

## Testing the Fixes

### Test 1: Node Registration
```bash
# 1. Start control plane
cd control-plane
go run cmd/server/main.go

# 2. In another terminal, test registration endpoint
curl -X POST http://localhost:8080/admin/nodes/register?node_id=test-123 \
  -H "Content-Type: application/json" \
  -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  -d '{
    "provider": "azure",
    "region": "eastus",
    "model_name": "meta-llama/Llama-2-7b-chat-hf",
    "endpoint_url": "http://10.0.0.1:8000",
    "gpu_type": "A10",
    "instance_type": "Standard_NV36ads_A10_v5",
    "spot_instance": true
  }'

# Expected: {"status":"registered","node_id":"test-123"}
```

### Test 2: Database Schema
```bash
# Check if columns exist
psql $DATABASE_URL -c "SELECT last_heartbeat, health_score, status_message FROM nodes LIMIT 1;"

# Should NOT error
```

### Test 3: End-to-End Launch
```bash
# 1. Launch instance from UI
# 2. Watch control-plane logs for:
#    - "GPU node launched successfully"
#    - "node registered successfully"
# 3. Check database:
psql $DATABASE_URL -c "SELECT id, status, endpoint_url FROM nodes WHERE status = 'ready';"

# Should show the new node with endpoint_url populated
```

---

## Why These Fixes Are Critical

### Without Fix #1 (Registration):
- **0% of launches succeed** (nodes launch but never register)
- **100% wasted cloud spend** (VMs running but unreachable)
- **Load balancer has 0 nodes** (all requests fail)

### Without Fix #2 (Schema):
- **Health monitoring completely broken**
- **Heartbeats fail silently** (SQL errors ignored)
- **Cannot detect dead nodes** (all stuck in "initializing")

### Without Fix #3 (Progress):
- **Poor user experience** (no feedback for 5 minutes)
- **Hard to debug failures** (no error details)
- **Users will spam launch** (thinking it's stuck)

---

## Estimated Time to Fix

- **Fix #1 (Registration):** 15 minutes
- **Fix #2 (Schema):** 5 minutes
- **Fix #3 (Progress):** 30 minutes

**Total: 50 minutes to make system functional**

---

## After These Fixes

The system will:
1. ✅ Successfully launch nodes via SkyPilot
2. ✅ Nodes register and become "ready"
3. ✅ Health monitoring works correctly
4. ✅ Users see detailed error messages
5. ✅ Load balancer can route traffic to nodes

**Next Priority:** Fix the deployment controller race condition (prevents duplicate launches)

---

## Files to Modify

1. `/control-plane/internal/gateway/gateway.go` - Add registration endpoint
2. `/control-plane/migrations/003_add_health_monitoring_fields.sql` - New file
3. `/control-plane/internal/orchestrator/skypilot.go` - Better error parsing
4. `/control-plane/internal/gateway/admin_models.go` - Better error display
5. `/node-agent/internal/agent/agent.go` - Pass node_id in URL

**APPLY THESE FIXES BEFORE ANY PRODUCTION TESTING**
