# GPU Launch Implementation Guide

## Quick Start: Apply Fixes

This guide walks you through implementing the fixes for GPU instance launch functionality.

## Pre-requisites

- Docker and Docker Compose installed
- Cloud provider account (AWS, Azure, or GCP)
- Cloud provider credentials
- 20+ GB disk space for Docker images

---

## Step 1: Verify Current Setup

Run the verification script to check your current setup:

```bash
./scripts/verify-launch-setup.sh
```

This will check:
- Docker is running
- Containers are up
- SkyPilot is installed
- Cloud credentials configured
- Services are healthy

**Expected output**: List of checks with ✓ (pass), ✗ (fail), or ⚠ (warning)

---

## Step 2: Update Dockerfile (If Needed)

If the verification shows "SkyPilot is not installed", update the Dockerfile:

```bash
# Backup current Dockerfile
cp Dockerfile.control-plane Dockerfile.control-plane.backup

# Use the fixed version
cp Dockerfile.control-plane.fixed Dockerfile.control-plane
```

**What changed**:
- Added AWS CLI installation
- Added Azure CLI installation
- Added Google Cloud SDK installation
- Updated SkyPilot installation with all cloud providers
- Added verification step

---

## Step 3: Apply Code Fixes

### Option A: Manual Integration (Recommended for Production)

Integrate the fixes into your existing code:

1. **Update skypilot.go**:
```bash
# Review the fixed implementation
code control-plane/internal/orchestrator/skypilot_fixed.go

# Key changes to integrate:
# - validateNodeConfigWithCredentials() method
# - LaunchNodeFixed() method with progress callbacks
# - monitorSkyPilotOutput() for streaming
# - parseSkyPilotError() for user-friendly errors
# - verifyNodeHealth() for post-launch checks
```

2. **Update admin_models.go**:
```bash
# Review the fixed implementation
code control-plane/internal/gateway/admin_models_fixed.go

# Key changes to integrate:
# - LaunchJobFixed struct with error tracking
# - launchNodeWithProgress() with callbacks
# - Enhanced error messages
# - 20-minute timeout
```

### Option B: Direct Replacement (Testing/Development)

Replace the files directly for testing:

```bash
# CAUTION: This overwrites your current code
cp control-plane/internal/orchestrator/skypilot_fixed.go \
   control-plane/internal/orchestrator/skypilot.go

cp control-plane/internal/gateway/admin_models_fixed.go \
   control-plane/internal/gateway/admin_models.go
```

---

## Step 4: Configure Cloud Credentials

Edit your `.env` file to add cloud provider credentials:

```bash
# Open .env for editing
code .env
```

### For AWS:
```bash
# Add to .env
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**How to get AWS credentials**:
1. Go to AWS Console → IAM
2. Create user with EC2, VPC, and EBS permissions
3. Create access key
4. Copy credentials to .env

### For Azure:
```bash
# Add to .env
AZURE_SUBSCRIPTION_ID=12345678-1234-1234-1234-123456789012
AZURE_TENANT_ID=12345678-1234-1234-1234-123456789012
```

**How to get Azure credentials**:
1. Run `az login` on your machine
2. Run `az account show` to get subscription ID and tenant ID
3. Copy to .env

### For GCP:
```bash
# Add to .env
GCP_PROJECT_ID=my-project-12345
GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
```

**How to get GCP credentials**:
1. Go to GCP Console → IAM → Service Accounts
2. Create service account with Compute Engine Admin role
3. Create JSON key
4. Mount key file in Docker compose and set path

---

## Step 5: Rebuild and Restart

Rebuild the control plane with the new Dockerfile and code:

```bash
# Stop services
docker-compose down

# Rebuild control plane (this will take 5-10 minutes)
docker-compose build control-plane

# Start services
docker-compose up -d

# Check logs
docker-compose logs -f control-plane
```

**Expected output**:
```
crosslogic-control-plane  | 2024-11-24T12:00:00.000Z  INFO  starting CrossLogic Control Plane
crosslogic-control-plane  | 2024-11-24T12:00:01.000Z  INFO  connected to database
crosslogic-control-plane  | 2024-11-24T12:00:02.000Z  INFO  connected to Redis
crosslogic-control-plane  | 2024-11-24T12:00:03.000Z  INFO  initialized SkyPilot orchestrator
crosslogic-control-plane  | 2024-11-24T12:00:04.000Z  INFO  starting HTTP server  address=0.0.0.0:8080
```

---

## Step 6: Verify Installation

Run verification again:

```bash
./scripts/verify-launch-setup.sh
```

All checks should now pass (✓).

Additionally, verify SkyPilot directly:

```bash
# Check SkyPilot version
docker exec crosslogic-control-plane sky --version

# Check SkyPilot cloud configuration
docker exec crosslogic-control-plane sky check

# List current clusters (should be empty initially)
docker exec crosslogic-control-plane sky status
```

**Expected output**:
```
SkyPilot 0.6.1

Checking credentials...
  AWS: enabled ✓
  Azure: enabled ✓
  GCP: enabled ✓

No clusters found.
```

---

## Step 7: Test Launch Flow

### Test 1: Launch Small Instance

1. Open dashboard: http://localhost:3000
2. Navigate to "Launch" page
3. Fill in form:
   - Model: `meta-llama/Llama-2-7b-chat-hf`
   - Provider: `aws`
   - Region: `us-east-1`
   - GPU: `T4` (cost-effective for testing)
   - Use Spot: ✓ (checked)
4. Click "Launch Instance"

**Expected behavior**:
- Immediate response with job ID
- Progress updates every 5-10 seconds:
  ```
  → Validating configuration
  → Provisioning cloud resources
  → Installing dependencies
  → Loading model from R2
  → Starting vLLM
  → Verifying node health
  ✓ Node ready in 3m 45s
  ```

### Test 2: Error Handling - No Credentials

1. Remove credentials from .env:
   ```bash
   # Comment out AWS credentials
   # AWS_ACCESS_KEY_ID=...
   # AWS_SECRET_ACCESS_KEY=...
   ```
2. Restart: `docker-compose restart control-plane`
3. Attempt launch with AWS

**Expected error**:
```
✗ Launch failed
  → Cloud provider credentials missing

💡 Fix:
  • Configure AWS credentials in .env file
  • Restart the control plane after adding credentials
```

### Test 3: Error Handling - No Capacity

1. Try launching large GPU in constrained region:
   - GPU: `H100`
   - Region: `us-east-1`
   - Use Spot: ✓

**Expected error** (if capacity unavailable):
```
✗ Launch failed
  → SkyPilot tried all availability zones in us-east-1
  → No spot capacity available in any zone

💡 Suggestions:
  • Try a different region (westus2, centralindia)
  • Use on-demand instead of spot
  • Wait 10-15 minutes and retry
  • Try a different GPU type (A10, T4, V100)
```

---

## Step 8: Monitor and Debug

### View Real-time Logs

```bash
# Control plane logs
docker-compose logs -f control-plane

# Filter for launch events
docker-compose logs control-plane | grep "launching GPU node"

# Filter for errors
docker-compose logs control-plane | grep "ERROR"
```

### Check SkyPilot Status

```bash
# List all clusters
docker exec crosslogic-control-plane sky status

# Check specific cluster
docker exec crosslogic-control-plane sky status cic-aws-useast1-t4-spot-abc123

# View cluster logs
docker exec crosslogic-control-plane sky logs cic-aws-useast1-t4-spot-abc123
```

### Inspect Task Files

Task YAML files are created in `/tmp` inside the container:

```bash
# List task files
docker exec crosslogic-control-plane ls -la /tmp/sky-task-*.yaml

# View task file
docker exec crosslogic-control-plane cat /tmp/sky-task-<node-id>.yaml
```

### Database Inspection

```bash
# Connect to database
docker exec -it crosslogic-postgres psql -U crosslogic crosslogic_iaas

# Check nodes table
SELECT id, cluster_name, status, provider, region, gpu_type, model_name, created_at
FROM nodes
ORDER BY created_at DESC
LIMIT 10;

# Check node by cluster name
SELECT * FROM nodes WHERE cluster_name = 'cic-aws-useast1-t4-spot-abc123';
```

---

## Troubleshooting

### Issue: "sky not found"

**Symptom**: Control plane logs show `sky: command not found`

**Solution**:
1. Verify Dockerfile has SkyPilot installation
2. Rebuild image: `docker-compose build control-plane`
3. Verify: `docker exec crosslogic-control-plane sky --version`

### Issue: "AWS credentials not configured"

**Symptom**: Launch fails immediately with credential error

**Solution**:
1. Check `.env` has `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
2. Verify credentials are not empty: `grep AWS_ .env`
3. Restart: `docker-compose restart control-plane`
4. Test: `docker exec crosslogic-control-plane sky check`

### Issue: "Failed to acquire resources in all zones"

**Symptom**: Launch fails after trying multiple zones

**Possible causes**:
1. No spot capacity in that region/GPU combination
2. Cloud account quotas exceeded
3. GPU type not available in selected region

**Solutions**:
- Try different region
- Use on-demand instead of spot
- Try different GPU type (T4, A10, V100)
- Wait 15 minutes and retry

### Issue: "Timeout after 20 minutes"

**Symptom**: Launch times out without completing

**Possible causes**:
1. Very slow instance provisioning
2. Large model download taking too long
3. Network issues

**Solutions**:
- Check cloud provider console for instance status
- Try smaller model for testing
- Check R2 configuration for faster downloads
- Increase timeout in code if needed

### Issue: Node launches but shows "degraded"

**Symptom**: Launch completes but node status is "degraded"

**Possible causes**:
1. vLLM failed to start
2. Health check endpoint not responding
3. Model loading failed

**Solutions**:
1. Check cluster logs: `sky logs <cluster-name>`
2. SSH into instance: `sky ssh <cluster-name>`
3. Check vLLM logs: `tail -f /tmp/vllm.log`
4. Verify model path is correct

---

## Performance Tuning

### Reduce Launch Time

1. **Use Warm Instances**: Keep instances running instead of cold starts
2. **Pre-cache Models in R2**: Upload models to R2 bucket beforehand
3. **Use Smaller Models**: Test with 7B models before deploying 70B+
4. **Choose Fast Regions**: `us-east-1`, `us-west-2` typically have better capacity

### Reduce Costs

1. **Use Spot Instances**: 60-90% savings vs on-demand
2. **Right-size GPUs**: T4 for small models, A10 for medium, A100 for large
3. **Terminate Idle Nodes**: Set up auto-termination after 1 hour idle
4. **Use Model Caching**: Reuse instances for same model

---

## Production Checklist

Before deploying to production:

- [ ] SkyPilot installed and verified
- [ ] Cloud credentials configured for all providers
- [ ] R2 bucket set up for model storage
- [ ] Database backups configured
- [ ] Monitoring and alerting set up
- [ ] Error notification system configured
- [ ] Load testing completed (10+ concurrent launches)
- [ ] Cost monitoring enabled
- [ ] Auto-scaling policies configured
- [ ] Disaster recovery plan documented

---

## Next Steps

After successful implementation:

1. **Scale Testing**: Test with 10+ concurrent launches
2. **Cost Optimization**: Analyze launch patterns and optimize
3. **Auto-scaling**: Set up deployment auto-scaling
4. **Monitoring**: Add Grafana dashboards for launch metrics
5. **Documentation**: Document your specific cloud setup

---

## Support Resources

### Logs
- Control plane: `docker-compose logs control-plane`
- SkyPilot: `docker exec crosslogic-control-plane sky logs <cluster>`

### Commands
- Verify setup: `./scripts/verify-launch-setup.sh`
- Check SkyPilot: `docker exec crosslogic-control-plane sky check`
- List clusters: `docker exec crosslogic-control-plane sky status`

### Documentation
- SkyPilot docs: https://skypilot.readthedocs.io
- Fix details: `GPU_LAUNCH_FIXES.md`
- Code fixes: `skypilot_fixed.go` and `admin_models_fixed.go`

---

## Success Metrics

After implementation, you should see:
- ✅ Launch success rate > 95%
- ✅ Average cold start time: 3-5 minutes
- ✅ Average warm start time: < 1 minute
- ✅ Clear error messages for all failure modes
- ✅ Real-time progress updates
- ✅ No orphaned resources

---

Good luck with your implementation! 🚀
