# SkyPilot Azure-Only Configuration Fix

## Root Cause Analysis

### Problem 1: `sky status` failing with exit code 2
**Location**: `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/internal/orchestrator/reconciler.go:96`

The reconciler runs `sky status --refresh --json` every minute, which fails because:
- AWS credentials are not configured
- SkyPilot tries to refresh AWS resources even though AWS is disabled
- Exit code 2 indicates AWS-related failure

**Log Evidence**:
```
failed to get skypilot clusters","error":"sky status failed: exit status 2
```

### Problem 2: Deployments trying to use AWS
**Location**: Database deployments table and SkyPilot launch failures

**Log Evidence**:
```
sky.exceptions.ResourcesUnavailableError: Task 'cic-aws-useast1-a10g-spot-d4c470' requires AWS which is not enabled
```

**Database State**:
```
mistral-7b-us-east | aws | us-east-1 | A10G
llama-3-70b-prod   | (empty) | (empty) | auto
```

### Problem 3: SkyPilot Configuration
Running `sky check` shows:
- **AWS**: disabled (no credentials)
- **Azure**: enabled ✓
- **All other clouds**: disabled

## Solution Overview

### Changes Required

1. **Remove AWS from .env** - Set dummy/empty values
2. **Fix reconciler.go** - Remove `--refresh` flag to avoid AWS API calls
3. **Update database** - Change deployments to use Azure regions
4. **Configure SkyPilot** - Explicitly disable AWS in config

## Detailed Fixes

### Fix 1: Update Environment Variables

**File**: `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/.env`

Change AWS credentials from:
```bash
AWS_ACCESS_KEY_ID=your_aws_access_key
AWS_SECRET_ACCESS_KEY=your_aws_secret_key
AWS_DEFAULT_REGION=us-east-1
```

To (comment out or remove):
```bash
# AWS_ACCESS_KEY_ID=
# AWS_SECRET_ACCESS_KEY=
# AWS_DEFAULT_REGION=
```

**Why**: Prevents SkyPilot from attempting AWS API calls.

### Fix 2: Update Reconciler to Skip AWS Refresh

**File**: `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/internal/orchestrator/reconciler.go`

**Line 96** - Change from:
```go
cmd := exec.CommandContext(ctx, "sky", "status", "--refresh", "--json")
```

To:
```go
// Removed --refresh flag to avoid AWS API calls when AWS is disabled
// This prevents exit code 2 errors in Azure-only deployments
cmd := exec.CommandContext(ctx, "sky", "status", "--json")
```

**Why**: The `--refresh` flag causes SkyPilot to query all enabled cloud providers' APIs. When AWS credentials are missing, this causes exit code 2. Removing it uses cached status only.

### Fix 3: Update Database Deployments

**Option A: Via SQL** (Quick fix for existing deployments)

```bash
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
UPDATE deployments
SET provider = 'azure',
    region = 'eastus',
    gpu_type = 'Standard_NC6s_v3'
WHERE name = 'mistral-7b-us-east';
"
```

**Option B: Via API/Dashboard** (Recommended for production)

1. Delete existing deployment `mistral-7b-us-east`
2. Create new deployment with Azure configuration:
   - Provider: `azure`
   - Region: `eastus` (or your preferred Azure region)
   - GPU: `Standard_NC6s_v3` (V100) or `Standard_NC24ads_A100_v4` (A100)

**Azure GPU Instance Types Reference**:
- `Standard_NC6s_v3` - 1x V100 (16GB VRAM) - Good for 7B-13B models
- `Standard_NC12s_v3` - 2x V100 (32GB VRAM)
- `Standard_NC24s_v3` - 4x V100 (64GB VRAM)
- `Standard_NC24ads_A100_v4` - 1x A100 (80GB VRAM) - Best for 70B models
- `Standard_ND96asr_v4` - 8x A100 (640GB VRAM) - For largest models

**Azure Regions with GPU Availability**:
- `eastus` - East US
- `westus2` - West US 2
- `westeurope` - West Europe
- `northeurope` - North Europe
- `southcentralus` - South Central US

### Fix 4: Create SkyPilot Config to Disable AWS

**File**: Create `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/scripts/skypilot-config.yaml`

```yaml
# SkyPilot Configuration for Azure-Only Deployment
# This file disables AWS and other cloud providers

allowed_clouds:
  - azure

# Azure-specific configuration
azure:
  prioritize_low_priority_vms: true  # Use spot instances for cost savings

# Disable AWS explicitly
disabled_clouds:
  - aws
  - gcp
  - lambda
  - oci
  - kubernetes
  - ibm
  - scp
  - cloudflare
```

**Update Dockerfile.control-plane** to copy this config:

Add after line 66 (before switching to non-root user):
```dockerfile
# Copy SkyPilot configuration for Azure-only setup
COPY --chown=crosslogic:crosslogic control-plane/scripts/skypilot-config.yaml /home/crosslogic/.sky/config.yaml
```

### Fix 5: Update Entrypoint Script

**File**: `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/scripts/entrypoint.sh`

Update the AWS check section (lines 30-33):

```bash
# AWS Credentials - Not used in Azure-only setup
if [ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "✓ AWS credentials detected (but not configured in this Azure-only deployment)"
else
    echo "⚠️  AWS not configured - using Azure-only deployment"
fi
```

## Implementation Steps

### Step 1: Stop Services
```bash
cd /Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas
docker compose down
```

### Step 2: Update .env File
```bash
# Edit .env and comment out AWS credentials
nano .env
# Or use sed:
sed -i.bak 's/^AWS_ACCESS_KEY_ID=/#AWS_ACCESS_KEY_ID=/' .env
sed -i.bak 's/^AWS_SECRET_ACCESS_KEY=/#AWS_SECRET_ACCESS_KEY=/' .env
sed -i.bak 's/^AWS_DEFAULT_REGION=/#AWS_DEFAULT_REGION=/' .env
```

### Step 3: Update reconciler.go
```bash
# Edit control-plane/internal/orchestrator/reconciler.go
# Change line 96 to remove --refresh flag (as shown above)
```

### Step 4: Create SkyPilot Config
```bash
mkdir -p control-plane/scripts
cat > control-plane/scripts/skypilot-config.yaml << 'EOF'
# SkyPilot Configuration for Azure-Only Deployment
allowed_clouds:
  - azure

azure:
  prioritize_low_priority_vms: true

disabled_clouds:
  - aws
  - gcp
  - lambda
  - oci
  - kubernetes
  - ibm
  - scp
  - cloudflare
EOF
```

### Step 5: Update Dockerfile
```bash
# Edit Dockerfile.control-plane
# Add after line 66:
# COPY --chown=crosslogic:crosslogic control-plane/scripts/skypilot-config.yaml /home/crosslogic/.sky/config.yaml
```

### Step 6: Rebuild and Restart
```bash
# Rebuild control-plane
docker compose build control-plane

# Start services
docker compose up -d

# Watch logs
docker logs -f crosslogic-control-plane
```

### Step 7: Update Database Deployments
```bash
# Update existing deployment to use Azure
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
UPDATE deployments
SET provider = 'azure',
    region = 'eastus',
    gpu_type = 'Standard_NC6s_v3'
WHERE name = 'mistral-7b-us-east';
"

# Verify
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
SELECT name, provider, region, gpu_type FROM deployments;
"
```

### Step 8: Verify SkyPilot Configuration
```bash
# Check SkyPilot sees only Azure
docker exec crosslogic-control-plane sky check

# Should show:
# ✔ Azure: enabled
# All other clouds: disabled
```

### Step 9: Test Deployment
```bash
# Check logs for errors
docker logs crosslogic-control-plane | grep -i "sky status"
docker logs crosslogic-control-plane | grep -i "aws"

# Should see no more "exit status 2" errors
# Should see no more "requires AWS" errors
```

## Expected Results

After applying all fixes:

1. **No more `sky status` errors**:
   - Reconciler runs successfully every minute
   - No exit code 2 errors
   - Logs show: `running state reconciliation` without errors

2. **Deployments use Azure**:
   - All deployments configured with Azure provider
   - Azure regions (eastus, westus2, etc.)
   - Azure GPU types (Standard_NC6s_v3, etc.)

3. **No AWS-related errors**:
   - No "requires AWS" errors
   - No AWS credential errors
   - SkyPilot only checks Azure

4. **Azure-only operation**:
   - `sky check` shows only Azure enabled
   - All launches use Azure
   - Cost optimization via Azure spot instances

## Verification Commands

```bash
# 1. Check reconciler is working
docker logs crosslogic-control-plane 2>&1 | grep "state reconciliation" | tail -5

# 2. Check for AWS errors
docker logs crosslogic-control-plane 2>&1 | grep -i "aws" | tail -10

# 3. Check deployments
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
SELECT name, provider, region, gpu_type, min_replicas, max_replicas
FROM deployments;
"

# 4. Verify SkyPilot config
docker exec crosslogic-control-plane sky check

# 5. Check active clusters
docker exec crosslogic-control-plane sky status
```

## Monitoring

After fixes are applied, monitor for:

1. **Reconciler health**: Should run every 60 seconds without errors
2. **Deployment launches**: Should only attempt Azure launches
3. **No AWS API calls**: Check logs for any AWS-related errors
4. **Azure credentials**: Ensure Azure auth working in entrypoint

## Rollback Plan

If issues occur:

1. **Stop services**: `docker compose down`
2. **Restore .env**: `mv .env.bak .env`
3. **Restore reconciler.go**: `git checkout control-plane/internal/orchestrator/reconciler.go`
4. **Restart**: `docker compose up -d`

## Additional Recommendations

### 1. Update Region Seeding Script

If you have a region seeding script, update it to only include Azure regions:

```sql
-- Clear existing regions
DELETE FROM regions;

-- Add Azure regions only
INSERT INTO regions (code, name, country, city, provider, cloud_providers, status) VALUES
('azure-eastus', 'East US', 'United States', 'Virginia', 'azure', '["azure"]'::jsonb, 'active'),
('azure-westus2', 'West US 2', 'United States', 'Washington', 'azure', '["azure"]'::jsonb, 'active'),
('azure-westeurope', 'West Europe', 'Netherlands', 'Amsterdam', 'azure', '["azure"]'::jsonb, 'active'),
('azure-northeurope', 'North Europe', 'Ireland', 'Dublin', 'azure', '["azure"]'::jsonb, 'active');
```

### 2. Update Instance Type Catalog

Ensure your instance types table only includes Azure GPU types:

```sql
-- Example Azure GPU instance types
INSERT INTO instance_types (cloud_provider, instance_type, gpu_type, gpu_count, vcpu, memory_gb, price_per_hour, price_per_hour_spot) VALUES
('azure', 'Standard_NC6s_v3', 'V100', 1, 6, 112, 3.06, 0.90),
('azure', 'Standard_NC12s_v3', 'V100', 2, 12, 224, 6.12, 1.80),
('azure', 'Standard_NC24s_v3', 'V100', 4, 24, 448, 12.24, 3.60),
('azure', 'Standard_NC24ads_A100_v4', 'A100', 1, 24, 220, 3.67, 1.10);
```

### 3. Documentation Updates

Update any documentation to reflect Azure-only setup:
- Remove AWS setup instructions
- Add Azure-specific GPU availability notes
- Update cost estimates with Azure pricing
- Add Azure region selection guide

## Troubleshooting

### Issue: Still seeing AWS errors after fix

**Check**:
```bash
# Verify .env doesn't have AWS credentials
docker exec crosslogic-control-plane env | grep AWS
```

**Solution**: Restart container to pick up new environment:
```bash
docker compose restart control-plane
```

### Issue: sky check still shows AWS enabled

**Check**:
```bash
# Verify SkyPilot config was copied
docker exec crosslogic-control-plane cat /home/crosslogic/.sky/config.yaml
```

**Solution**: Rebuild container with updated Dockerfile

### Issue: Deployments still trying to use AWS regions

**Check**:
```bash
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
SELECT name, provider, region FROM deployments;
"
```

**Solution**: Update database as shown in Fix 3

### Issue: Azure authentication failing

**Check**:
```bash
docker logs crosslogic-control-plane 2>&1 | grep -i "azure" | head -20
```

**Solution**: Verify Azure credentials in .env are correct and service principal has proper permissions

## Performance Considerations

With Azure-only setup:

1. **Faster reconciliation**: No AWS API timeout delays
2. **Lower error rate**: No AWS credential errors
3. **Simplified monitoring**: Single cloud provider to track
4. **Cost optimization**: Azure spot instances 60-90% cheaper

## Security Benefits

1. **Reduced attack surface**: Only Azure credentials needed
2. **Simplified IAM**: Single cloud provider permissions
3. **Better audit trail**: All operations in one cloud
4. **Cleaner secrets management**: Fewer credentials to rotate

## Summary

This fix converts the deployment from multi-cloud to Azure-only by:

1. Removing AWS credentials and configuration
2. Fixing reconciler to not refresh AWS state
3. Updating deployments to use Azure regions
4. Configuring SkyPilot to only enable Azure
5. Ensuring no AWS API calls are made

Result: Clean Azure-only deployment with no AWS-related errors.
