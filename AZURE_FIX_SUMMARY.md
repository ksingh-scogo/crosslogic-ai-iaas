# Azure-Only Configuration Fix - Quick Reference

## Problem Summary

1. **sky status failing**: Exit code 2 every minute due to AWS refresh attempts
2. **AWS launches failing**: Deployments configured for AWS but AWS not enabled
3. **User requirement**: Only wants to use Azure, not AWS

## Root Causes

| Issue | Location | Cause |
|-------|----------|-------|
| sky status exit 2 | reconciler.go:96 | `--refresh` flag tries AWS API without credentials |
| AWS launch errors | Database deployments | Deployments use provider='aws', region='us-east-1' |
| Mixed configuration | .env file | AWS credentials set but not actually configured |

## Quick Fix (Automated)

```bash
cd /Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas

# Run automated fix
./fix-azure-only.sh

# Monitor results
docker logs -f crosslogic-control-plane

# If issues, rollback
./rollback-azure-fix.sh
```

## What the Fix Does

1. **Disables AWS in .env**
   - Comments out AWS_ACCESS_KEY_ID
   - Comments out AWS_SECRET_ACCESS_KEY
   - Comments out AWS_DEFAULT_REGION

2. **Fixes reconciler.go**
   - Removes `--refresh` flag from `sky status` command
   - Prevents AWS API calls that cause exit code 2

3. **Creates SkyPilot config**
   - `/home/crosslogic/.sky/config.yaml`
   - Explicitly enables only Azure
   - Disables all other clouds including AWS

4. **Updates Dockerfile**
   - Copies SkyPilot config into container
   - Ensures config persists across rebuilds

5. **Updates database deployments**
   - Changes `mistral-7b-us-east` to use Azure
   - Provider: aws → azure
   - Region: us-east-1 → eastus
   - GPU: A10G → Standard_NC6s_v3

## Manual Fix Steps

If you prefer manual fixes:

### 1. Update .env
```bash
nano .env
# Comment out these lines:
#AWS_ACCESS_KEY_ID=
#AWS_SECRET_ACCESS_KEY=
#AWS_DEFAULT_REGION=
```

### 2. Fix reconciler.go
File: `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/internal/orchestrator/reconciler.go`

Line 96, change:
```go
cmd := exec.CommandContext(ctx, "sky", "status", "--refresh", "--json")
```

To:
```go
// Removed --refresh flag to avoid AWS API calls when AWS is disabled
cmd := exec.CommandContext(ctx, "sky", "status", "--json")
```

### 3. Create SkyPilot config
```bash
mkdir -p control-plane/scripts
cat > control-plane/scripts/skypilot-config.yaml << 'EOF'
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

### 4. Update Dockerfile.control-plane
Add after line 66:
```dockerfile
# Copy SkyPilot configuration for Azure-only setup
COPY --chown=crosslogic:crosslogic control-plane/scripts/skypilot-config.yaml /home/crosslogic/.sky/config.yaml
```

### 5. Update database
```bash
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas << 'EOF'
UPDATE deployments
SET provider = 'azure',
    region = 'eastus',
    gpu_type = 'Standard_NC6s_v3'
WHERE name = 'mistral-7b-us-east';
EOF
```

### 6. Rebuild and restart
```bash
docker compose down
docker compose build control-plane
docker compose up -d
```

## Verification

### Check 1: No more sky status errors
```bash
docker logs crosslogic-control-plane 2>&1 | grep "sky status" | tail -10
# Should see no "exit status 2" errors
```

### Check 2: No AWS errors
```bash
docker logs crosslogic-control-plane 2>&1 | grep -i "aws" | tail -20
# Should see no "requires AWS which is not enabled" errors
```

### Check 3: Deployments use Azure
```bash
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "
SELECT name, provider, region, gpu_type FROM deployments;
"
# Should show Azure provider and regions
```

### Check 4: SkyPilot Azure-only
```bash
docker exec crosslogic-control-plane sky check
# Should show:
#   ✔ Azure: enabled
#   AWS: disabled
```

### Check 5: Reconciler working
```bash
docker logs crosslogic-control-plane 2>&1 | grep "state reconciliation" | tail -5
# Should see regular reconciliation without errors
```

## Expected Results

After fix:
- ✓ No exit code 2 errors from sky status
- ✓ No "requires AWS" errors
- ✓ Reconciler runs every 60 seconds successfully
- ✓ All deployments configured for Azure
- ✓ SkyPilot only checks Azure availability

## Azure Configuration Reference

### GPU Instance Types

| Instance Type | GPU | VRAM | vCPU | RAM | Best For | Price/hr (spot) |
|--------------|-----|------|------|-----|----------|-----------------|
| Standard_NC6s_v3 | 1x V100 | 16GB | 6 | 112GB | 7B-13B models | ~$0.90 |
| Standard_NC12s_v3 | 2x V100 | 32GB | 12 | 224GB | 13B-30B models | ~$1.80 |
| Standard_NC24s_v3 | 4x V100 | 64GB | 24 | 448GB | 30B-70B models | ~$3.60 |
| Standard_NC24ads_A100_v4 | 1x A100 | 80GB | 24 | 220GB | 70B models | ~$1.10 |

### Azure Regions

| Region Code | Name | Location | GPU Availability |
|-------------|------|----------|-----------------|
| eastus | East US | Virginia, USA | ✓ High |
| westus2 | West US 2 | Washington, USA | ✓ High |
| westeurope | West Europe | Netherlands | ✓ High |
| northeurope | North Europe | Ireland | ✓ Medium |
| southcentralus | South Central US | Texas, USA | ✓ Medium |

## Troubleshooting

### Issue: Still seeing AWS errors after fix

**Check if container picked up changes:**
```bash
docker exec crosslogic-control-plane env | grep AWS
# Should be empty or show no credentials
```

**Solution:**
```bash
docker compose restart control-plane
```

### Issue: sky check still shows AWS enabled

**Check SkyPilot config:**
```bash
docker exec crosslogic-control-plane cat /home/crosslogic/.sky/config.yaml
# Should show allowed_clouds: [azure]
```

**Solution:**
```bash
docker compose down
docker compose build control-plane
docker compose up -d
```

### Issue: Reconciler still failing

**Check logs:**
```bash
docker logs crosslogic-control-plane 2>&1 | grep reconciler | tail -20
```

**Common causes:**
- reconciler.go not rebuilt (rebuild container)
- SkyPilot config not loaded (check file exists in container)
- Azure credentials invalid (check Azure auth in logs)

## Files Modified

1. `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/.env`
2. `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/internal/orchestrator/reconciler.go`
3. `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/Dockerfile.control-plane`
4. `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/scripts/skypilot-config.yaml` (new)
5. `/Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas/control-plane/scripts/entrypoint.sh`

## Backups Created

The automated script creates:
- `.env.backup.YYYYMMDD_HHMMSS`
- `control-plane/internal/orchestrator/reconciler.go.backup`

## Rollback

If you need to revert changes:

```bash
./rollback-azure-fix.sh
```

Or manually:
```bash
# Restore .env
cp .env.backup.* .env

# Restore reconciler.go
cp control-plane/internal/orchestrator/reconciler.go.backup control-plane/internal/orchestrator/reconciler.go

# Rebuild and restart
docker compose down
docker compose build control-plane
docker compose up -d
```

## Additional Resources

- Full details: `SKYPILOT_AZURE_ONLY_FIX.md`
- SkyPilot docs: https://skypilot.readthedocs.io/
- Azure GPU pricing: https://azure.microsoft.com/en-us/pricing/details/virtual-machines/linux/

## Support

If issues persist after applying fix:

1. Check all verification commands above
2. Review full logs: `docker logs crosslogic-control-plane | less`
3. Verify Azure credentials: `docker exec crosslogic-control-plane az account show`
4. Check SkyPilot state: `docker exec crosslogic-control-plane sky status`
5. Review database state: Check deployments and nodes tables

## Performance Impact

After fix:
- **Faster reconciliation**: 60s interval without AWS API timeouts
- **Lower error rate**: No credential failures
- **Cleaner logs**: No AWS-related warnings
- **Better monitoring**: Focus on single cloud provider
- **Cost savings**: Azure spot instances 60-90% cheaper than on-demand
