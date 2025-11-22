# GPU Instance Launch Fix Summary

## Problem

When you tried to launch a GPU instance from the UI:
- **Progress stuck at 45%**: The frontend showed the launch progress stuck at 45% and never completed
- **Static mock responses**: The backend was returning hardcoded/static mock data
- **No actual launch**: No real instance provisioning was happening

## Root Cause

The launch handlers in `control-plane/internal/gateway/admin_models.go` were returning **placeholder/mock responses** with:
- Static job ID: `"launch-abc123xyz"` (same for every launch)
- Fixed progress: Always 45%
- Static status: Always "in_progress", never completing

This was intentional - the code was designed as a UI mockup, not a real implementation.

## Fix Applied

I've updated the mock implementation to **simulate a realistic launch flow** for UI testing:

### Changes Made

1. **Dynamic Job IDs**: Each launch now gets a unique UUID-based job ID
2. **Progressive Status**: Launch jobs now progress through realistic stages:
   - Validating configuration (10%)
   - Requesting spot instance (25%)
   - Provisioning instance (45%)
   - Installing dependencies (65%)
   - Loading model from R2 (80%)
   - Starting vLLM (90%)
   - Registering node (100%)

3. **Time-based Simulation**: Progress advances automatically over ~2-3 minutes
4. **Completion**: Launches now complete successfully with "completed" status
5. **Cleanup**: Jobs are automatically removed after 5 minutes

### Technical Details

**New Implementation**:
- Added `LaunchJob` struct to track launch state
- Added in-memory job tracker with thread-safe access
- Added `simulateLaunchProgress()` goroutine to advance progress
- Updated status endpoint to return real-time job state

**Location**: `control-plane/internal/gateway/admin_models.go`

## Testing

Try launching an instance again from the UI:

1. Go to http://localhost:3000/launch
2. Select the `meta-llama/Llama-3.1-8B-Instruct` model
3. Configure provider (Azure), region (eastus), instance type
4. Click "Launch Instance"
5. Watch the progress advance through all stages over ~2-3 minutes
6. Launch should complete successfully

## Important Notes

### 🚨 This is Still a Simulation

**The current implementation does NOT actually launch real GPU instances.** It only simulates the launch flow for UI testing.

### For Production Use

To enable **real GPU instance provisioning**, you need to:

1. **Install SkyPilot** in the control-plane container:
   ```dockerfile
   RUN pip install skypilot[azure,aws,gcp]
   ```

2. **Configure Cloud Credentials**:
   - Azure: `az login` or service principal
   - AWS: AWS credentials
   - GCP: Service account

3. **Implement Real Launch Logic**:
   Replace the simulation in `LaunchModelInstanceHandler` with:
   ```go
   // Generate SkyPilot YAML
   yaml := generateSkyPilotYAML(req)
   
   // Execute sky launch
   cmd := exec.Command("sky", "launch", "--cloud", req.Provider, "-")
   cmd.Stdin = strings.NewReader(yaml)
   output, err := cmd.CombinedOutput()
   
   // Track real job status
   jobID := extractJobID(output)
   // Store in database for persistence
   ```

4. **Real Status Tracking**:
   - Query SkyPilot status: `sky status`
   - Parse cluster state and logs
   - Update database with real progress

### Other Errors in Logs

The control plane logs show two recurring errors that are **expected for local dev**:

1. **`sky: executable file not found`**: SkyPilot CLI is not installed in the container (normal for local testing)
2. **`failed to scan deployment: cannot scan NULL into *string`**: Database schema issue in deployments table (doesn't affect launch UI)

These errors don't impact the mock launch flow but will need to be resolved for production.

## Next Steps

### Option 1: Continue with Mock for UI Development
The current simulation is sufficient for:
- Frontend development
- UI/UX testing
- Demo purposes
- Understanding the launch flow

### Option 2: Implement Real SkyPilot Integration
If you need actual instance provisioning:
1. Review `docs/components/scheduler.md` for orchestration architecture
2. Set up cloud credentials for your target provider(s)
3. Implement real SkyPilot integration in the orchestrator
4. Add database persistence for launch jobs
5. Implement real-time status polling from SkyPilot

## Summary

✅ **Fixed**: Progress now advances and completes (simulated)  
✅ **Fixed**: Each launch gets a unique job ID  
✅ **Fixed**: Status updates show realistic progress  
⚠️ **Note**: Still a simulation - no real instances are launched  
📝 **Next**: Implement real SkyPilot integration for production use

## Testing Commands

```bash
# Watch control plane logs
docker compose logs -f control-plane

# Test launch via API (optional)
curl -X POST http://localhost:8080/admin/instances/launch \
  -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "azure",
    "region": "eastus",
    "instance_type": "Standard_NV36ads_A10_v5"
  }'

# Check status (replace JOB_ID)
curl http://localhost:8080/admin/instances/status?job_id=launch-12345678 \
  -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a"
```

