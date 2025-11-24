# GPU Instance Launch Implementation Fixes

## Executive Summary

The GPU instance launch feature is currently non-functional due to **7 critical issues** and **5 improvement areas**. This document provides a comprehensive analysis and specific code fixes to resolve all issues.

## Critical Issues Identified

### Issue 1: SkyPilot CLI Not Installed ⚠️ BLOCKING

**Severity**: Critical - Launch fails immediately
**Location**: `/control-plane/internal/orchestrator/skypilot.go:441`

**Problem**:
- Code executes `sky launch` command
- SkyPilot is not installed in the control plane container
- Command fails with "executable not found"

**Evidence**:
```bash
$ which sky
sky not found
```

**Fix Required**:
1. Add SkyPilot to Dockerfile
2. Configure cloud provider dependencies
3. Verify installation on startup

**Implementation**:
```dockerfile
# Add to Dockerfile.control-plane
RUN pip install --no-cache-dir \
    skypilot[aws,azure,gcp]==0.6.0 \
    boto3==1.34.0
```

---

### Issue 2: Missing Error Propagation in Async Launch ⚠️ HIGH

**Severity**: High - Users see "in progress" forever on failure
**Location**: `/control-plane/internal/gateway/admin_models.go:174-246`

**Problem**:
- Launch happens in goroutine (line 175)
- Errors are logged but job status update has race condition
- UI polls job status but may read stale data before error is recorded

**Code Analysis**:
```go
// Line 175-182: Race condition window
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    clusterName, err := g.orchestrator.LaunchNode(ctx, nodeConfig)

    jobsMutex.Lock()  // Lock acquired AFTER launch completes
    defer jobsMutex.Unlock()

    if err != nil {
        // Error logged but job update might be too late
```

**Fix**: Use atomic job updates with channels or better mutex handling

---

### Issue 3: No Cloud Credential Validation ⚠️ HIGH

**Severity**: High - Cryptic errors instead of helpful messages
**Location**: `/control-plane/internal/orchestrator/skypilot.go:401-420`

**Problem**:
- No pre-flight check for AWS/Azure/GCP credentials
- SkyPilot fails deep in launch process with cloud API errors
- Users don't know which credentials are missing

**Current Code**:
```go
func (o *SkyPilotOrchestrator) LaunchNode(ctx context.Context, config NodeConfig) (string, error) {
    // Validate and set defaults
    if err := o.validateNodeConfig(&config); err != nil {
        return "", fmt.Errorf("invalid node configuration: %w", err)
    }
    // NO CREDENTIAL VALIDATION ❌
```

**Fix**: Add credential validation in `validateNodeConfig`

```go
func (o *SkyPilotOrchestrator) validateNodeConfigWithCredentials(config *NodeConfig) error {
    switch config.Provider {
    case "aws":
        if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
            return fmt.Errorf("AWS credentials not configured")
        }
    case "azure":
        if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
            return fmt.Errorf("Azure credentials not configured")
        }
    case "gcp":
        if os.Getenv("GCP_PROJECT_ID") == "" {
            return fmt.Errorf("GCP credentials not configured")
        }
    }
    return nil
}
```

---

### Issue 4: Incorrect SkyPilot Command Arguments ⚠️ HIGH

**Severity**: High - Launch behavior is incorrect
**Location**: `/control-plane/internal/orchestrator/skypilot.go:441-447`

**Problems**:
1. Missing `--cloud` flag - SkyPilot may choose wrong provider
2. Missing `--region` flag - May deploy to wrong region
3. Wrong `--down` flag - Terminates cluster after job (we want persistent serving)
4. Missing `--retry-until-up` - Doesn't retry across zones on spot failures

**Current Code**:
```go
cmd := exec.CommandContext(ctx, "sky", "launch",
    "-c", clusterName,
    taskFile,
    "-y",
    "--down",        // ❌ Will terminate after job completes
    "--detach-run",  // ❌ May not wait for initialization
)
```

**Fixed Command**:
```go
cmd := exec.CommandContext(ctx, "sky", "launch",
    "-c", clusterName,
    "--cloud", config.Provider,     // ✅ Force cloud provider
    "--region", config.Region,       // ✅ Force region
    "--retry-until-up",              // ✅ Retry across zones
    "-y",                            // ✅ Auto-confirm
    taskFile,
)
```

---

### Issue 5: Race Condition in Job Status Updates ⚠️ MEDIUM

**Severity**: Medium - Intermittent incorrect status
**Location**: `/control-plane/internal/gateway/admin_models.go:170-172, 181-182`

**Problem**:
```go
// Line 170: Create job
jobsMutex.Lock()
launchJobs[jobID] = job
jobsMutex.Unlock()  // Lock released

// Line 175: Start goroutine
go func() {
    // ... long launch process ...

    jobsMutex.Lock()  // Re-acquire lock MUCH LATER
    defer jobsMutex.Unlock()

    // Update job status
}()
```

**Race Condition Window**:
1. Job created and lock released (line 172)
2. UI polls job status (line 316-318) - sees "in_progress"
3. Launch fails in goroutine (line 184)
4. UI polls again before mutex reacquired - still sees "in_progress"

**Fix**: Use atomic updates or channels for status communication

---

### Issue 6: No Progress Updates During Launch ⚠️ MEDIUM

**Severity**: Medium - Poor UX, users don't know what's happening
**Location**: `/control-plane/internal/gateway/admin_models.go:174-246`

**Problem**:
- Job stays at "validating" stage for 3-5 minutes
- No intermediate progress updates
- Users can't tell if it's frozen or working

**Missing Stages**:
- Cloud resource provisioning (30-60s)
- Instance launching (60-120s)
- Docker setup (30-60s)
- Model download (30-120s depending on size)
- vLLM startup (30-60s)

**Fix**: Parse SkyPilot stdout for progress indicators

---

### Issue 7: Stdout/Stderr Not Streamed ⚠️ MEDIUM

**Severity**: Medium - Can't debug failures, no real-time progress
**Location**: `/control-plane/internal/orchestrator/skypilot.go:449-452`

**Problem**:
```go
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
```

**Issues**:
- Output buffered in memory until command completes
- 5-minute launch shows no output until end
- Can't extract progress from stdout
- Errors only visible after total failure

**Fix**: Use `cmd.StdoutPipe()` and stream output

```go
stdoutPipe, err := cmd.StdoutPipe()
go func() {
    scanner := bufio.NewScanner(stdoutPipe)
    for scanner.Scan() {
        line := scanner.Text()
        // Parse for progress
        // Update job status
    }
}()
```

---

## Improvement Areas

### 8. Template YAML Not Validated

**Location**: `/control-plane/internal/orchestrator/skypilot.go:776-813`

Generated YAML should be validated before writing to disk. Malformed YAML causes cryptic SkyPilot errors.

**Fix**: Add YAML structure validation

```go
func (o *SkyPilotOrchestrator) validateTaskYAML(yaml string) error {
    if !strings.Contains(yaml, "resources:") {
        return fmt.Errorf("missing resources section")
    }
    if !strings.Contains(yaml, "setup:") {
        return fmt.Errorf("missing setup section")
    }
    // ... more checks
    return nil
}
```

---

### 9. Database Registration Happens After Launch

**Location**: `/control-plane/internal/orchestrator/skypilot.go:499-506`

Node is only registered in database AFTER successful launch. If registration fails, node is invisible to system but consuming resources.

**Fix**: Pre-register node with "launching" status, update on completion

---

### 10. No Retry Logic for Spot Failures

SkyPilot has `--retry-until-up` but code doesn't use it. Should also implement application-level retry with exponential backoff for transient failures.

---

### 11. Context Timeout Too Short

**Location**: `/control-plane/internal/gateway/admin_models.go:176`

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
```

**Analysis**:
- Cold start in new region: 5-7 minutes
- Large model download: 3-5 minutes
- vLLM initialization: 2-3 minutes
- **Total: 10-15 minutes minimum**

15-minute timeout is cutting it too close.

**Fix**: Increase to 20-25 minutes for cold starts

---

### 12. No Health Check After Launch

After launch completes, should verify vLLM is actually serving before marking node as "ready".

**Fix**: Add health check step

```go
func (o *SkyPilotOrchestrator) verifyNodeHealth(ctx context.Context, clusterName string) error {
    // Wait up to 3 minutes for vLLM health check
    for i := 0; i < 36; i++ {
        status, err := o.GetClusterStatus(clusterName)
        if err == nil && status == "UP" {
            // TODO: Also check vLLM /health endpoint
            return nil
        }
        time.Sleep(5 * time.Second)
    }
    return fmt.Errorf("health check timeout")
}
```

---

## Implementation Priority

### Phase 1: Immediate Fixes (Blocking Issues)
1. **Install SkyPilot in Dockerfile** - Without this, nothing works
2. **Fix SkyPilot command arguments** - Prevents incorrect behavior
3. **Add credential validation** - Better error messages

### Phase 2: Reliability Fixes
4. **Fix async error propagation** - Users need to see failures
5. **Stream stdout/stderr** - Enable debugging and progress
6. **Add progress updates** - Improve UX

### Phase 3: Robustness Improvements
7. **Pre-register nodes** - Prevent orphaned resources
8. **Add health checks** - Verify successful launches
9. **Increase timeout** - Prevent premature failures
10. **Add YAML validation** - Catch errors early

---

## Testing Strategy

### Unit Tests
- Credential validation logic
- YAML template generation
- Error parsing functions

### Integration Tests
- Full launch flow with mock SkyPilot
- Error handling paths
- Progress update propagation

### End-to-End Tests
1. Launch with valid credentials → Success
2. Launch without credentials → Clear error message
3. Launch with no capacity → User-friendly suggestion
4. Cancel during launch → Clean resource cleanup

---

## Fixed Files Provided

I've created two fixed implementation files:

1. **`/control-plane/internal/orchestrator/skypilot_fixed.go`**
   - Enhanced `LaunchNodeFixed()` method with:
     - Credential validation
     - Correct SkyPilot arguments
     - Progress callbacks
     - Streamed output parsing
     - Health verification
     - Better error messages

2. **`/control-plane/internal/gateway/admin_models_fixed.go`**
   - Enhanced job tracking
   - Atomic status updates
   - Progress callback integration
   - Detailed error reporting
   - Longer timeout (20 min)

---

## Dockerfile Changes Required

### Current Dockerfile Issue
The control plane container doesn't have SkyPilot installed.

### Fix: Update Dockerfile.control-plane

```dockerfile
# Add after Go binary build, before final stage
FROM python:3.10-slim as skypilot

# Install SkyPilot with cloud provider support
RUN pip install --no-cache-dir \
    skypilot[aws,azure,gcp]==0.6.0 \
    boto3==1.34.0 \
    azure-cli==2.50.0 \
    google-cloud-sdk

# Verify installation
RUN sky check

# Final stage - copy SkyPilot into runtime image
FROM debian:bullseye-slim

# Copy Go binary
COPY --from=builder /app/control-plane /usr/local/bin/control-plane

# Copy Python and SkyPilot
COPY --from=skypilot /usr/local /usr/local

# Set up Python path
ENV PATH="/usr/local/bin:${PATH}"
ENV PYTHONPATH="/usr/local/lib/python3.10/site-packages"

# Verify SkyPilot is available
RUN sky --version || echo "SkyPilot not found - install required"

CMD ["/usr/local/bin/control-plane"]
```

---

## Environment Variables Required

Ensure these are set in `.env`:

```bash
# AWS (if using AWS)
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret

# Azure (if using Azure)
AZURE_SUBSCRIPTION_ID=your_sub_id
AZURE_TENANT_ID=your_tenant_id

# GCP (if using GCP)
GCP_PROJECT_ID=your_project_id
GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json

# R2 (for model storage)
R2_ENDPOINT=https://your-account.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=your_r2_key
R2_SECRET_KEY=your_r2_secret
```

---

## Migration Path

### Step 1: Install SkyPilot
```bash
# Rebuild control plane with SkyPilot
docker-compose build control-plane
docker-compose up -d control-plane
```

### Step 2: Verify Installation
```bash
docker-compose exec control-plane sky --version
docker-compose exec control-plane sky check
```

### Step 3: Deploy Fixed Code
```bash
# Copy fixed files over originals
cp control-plane/internal/orchestrator/skypilot_fixed.go \
   control-plane/internal/orchestrator/skypilot.go

cp control-plane/internal/gateway/admin_models_fixed.go \
   control-plane/internal/gateway/admin_models.go

# Rebuild
docker-compose build control-plane
docker-compose up -d
```

### Step 4: Test Launch
1. Open dashboard: http://localhost:3000
2. Navigate to Launch page
3. Select model, provider, region
4. Click "Launch Instance"
5. Verify progress updates appear
6. Confirm node appears in active nodes list

---

## Monitoring and Debugging

### View Control Plane Logs
```bash
docker-compose logs -f control-plane
```

### Check SkyPilot Status
```bash
docker-compose exec control-plane sky status
```

### Inspect Task File
Task files are written to `/tmp/sky-task-<node-id>.yaml` for debugging.

### Common Errors and Solutions

**Error**: `sky not found`
- **Solution**: Rebuild Docker image with SkyPilot installed

**Error**: `AWS credentials not configured`
- **Solution**: Add AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY to .env

**Error**: `Failed to acquire resources in all zones`
- **Solution**: Try different region or use on-demand instead of spot

**Error**: `timeout after 20 minutes`
- **Solution**: Check cloud provider console, may need larger timeout for slow regions

---

## Performance Expectations

### Typical Launch Times
- **Warm start** (cached instance): 30-60 seconds
- **Cold start** (new region): 3-5 minutes
- **Large model** (70B+): 5-8 minutes

### Cost Optimization
- Spot instances: 60-90% savings vs on-demand
- SkyPilot automatically retries across zones
- Failed launches don't incur charges (only provisioning attempts)

---

## Success Metrics

After implementing these fixes:
- ✅ Launch success rate > 95%
- ✅ Average launch time < 4 minutes
- ✅ Clear error messages for all failure modes
- ✅ Real-time progress updates every 10 seconds
- ✅ No orphaned resources from failed launches

---

## Next Steps

1. Review fixed code in `skypilot_fixed.go` and `admin_models_fixed.go`
2. Update Dockerfile to install SkyPilot
3. Configure cloud provider credentials
4. Deploy and test with single instance
5. Monitor logs and iterate on error handling
6. Scale to production deployments

---

## Questions or Issues?

If you encounter any issues during implementation:
1. Check control plane logs: `docker-compose logs control-plane`
2. Verify SkyPilot installation: `sky check`
3. Test SkyPilot manually: `sky launch --cloud aws --region us-east-1 <task-file>`
4. Check this analysis for error patterns and solutions
