# Azure Setup Guide for Real GPU Instance Launches

## Overview

The system is now configured to launch **real GPU instances on Azure** using SkyPilot. This guide will help you set up Azure credentials and test live launches.

## Current Status

✅ **SkyPilot installed** in control-plane container with Azure support  
✅ **Orchestrator integrated** with gateway launch endpoint  
✅ **Docker compose configured** to pass Azure credentials  
✅ **Automatic fallback** to mock if credentials not provided  

## Prerequisites

1. **Azure Account** with active subscription
2. **Azure CLI** installed on your local machine
3. **Owner** role on the Azure subscription (or **Contributor + User Access Administrator** roles)

## Step 1: Install Azure CLI (Local Machine)

### macOS
```bash
brew install azure-cli
```

### Linux
```bash
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
```

### Windows
Download from: https://aka.ms/installazurecliwindows

## Step 2: Login to Azure

```bash
# Login interactively
az login

# Set your subscription
az account set --subscription "<YOUR_SUBSCRIPTION_ID>"

# Verify
az account show
```

## Step 3: Get Azure Credentials

### Option A: Use Service Principal (Recommended for Production)

```bash
# Get your subscription ID first
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
echo "Subscription ID: $SUBSCRIPTION_ID"

# Create service principal with Contributor role
az ad sp create-for-rbac \
  --name "crosslogic-sp" \
  --role "Contributor" \
  --scopes "/subscriptions/$SUBSCRIPTION_ID"
```

This will output:
```json
{
  "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",      # CLIENT_ID
  "displayName": "crosslogic-sp",
  "password": "xxxxxxxxxxxxxxxxxxxxxxxxxxxx",            # CLIENT_SECRET
  "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"      # TENANT_ID
}
```

**Save these credentials immediately!** You cannot retrieve the password later.

#### IMPORTANT: Add User Access Administrator Role

SkyPilot requires the ability to assign roles to resources it creates. Add the **User Access Administrator** role:

```bash
# Use the App ID from the output above
APP_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# Add User Access Administrator role (REQUIRED for SkyPilot)
az role assignment create \
  --assignee $APP_ID \
  --role "User Access Administrator" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"
```

**Why Both Roles?**
- **Contributor**: Allows creating/managing Azure resources (VMs, networks, storage)
- **User Access Administrator**: Allows SkyPilot to assign roles to the resources it creates

**Alternative**: Use the **Owner** role instead, which includes both permissions:
```bash
# Delete the service principal and recreate with Owner role
az ad sp delete --id $APP_ID
az ad sp create-for-rbac \
  --name "crosslogic-sp" \
  --role "Owner" \
  --scopes "/subscriptions/$SUBSCRIPTION_ID"
```

### Verify Service Principal Creation

```bash
# Get the App ID from the output above
APP_ID="your-app-id-here"

# Verify the service principal exists
az ad sp show --id $APP_ID --query "{appId:appId, displayName:displayName, objectId:id}" --output table

# Verify the role assignment (use subscription scope with filter - more reliable than --assignee)
az role assignment list \
  --scope "/subscriptions/$SUBSCRIPTION_ID" \
  --query "[?principalId=='$(az ad sp show --id $APP_ID --query id -o tsv)']" \
  --output table
```

Expected output (you should see BOTH roles):
```
Name                                  RoleDefinitionName            PrincipalName  PrincipalType
------------------------------------  ----------------------------  -------------  ----------------
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx  Contributor                   crosslogic-sp  ServicePrincipal
yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy  User Access Administrator     crosslogic-sp  ServicePrincipal
```

**Note**: Azure RBAC propagation can take 2-5 minutes. If the roles don't appear immediately, wait a few minutes and try again.

### Option B: Use Interactive Login (Easier for Development)

```bash
# Get subscription ID
az account show --query id -o tsv

# Get tenant ID
az account show --query tenantId -o tsv
```

For interactive auth, you only need `SUBSCRIPTION_ID` and `TENANT_ID`.

## Step 4: Configure Environment Variables

Edit your `.env` file in the project root:

```bash
# Azure Credentials (Required for real launches)
AZURE_SUBSCRIPTION_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
AZURE_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# For Service Principal (Option A)
AZURE_CLIENT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
AZURE_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxx

# For Interactive Login (Option B)
# Leave CLIENT_ID and CLIENT_SECRET empty
```

## Step 5: Rebuild Control Plane

The control plane needs to be rebuilt to include SkyPilot:

```bash
cd /Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas

# Stop current services
docker compose down

# Rebuild control plane (this will take 5-10 minutes)
docker compose build control-plane

# Start services
docker compose up -d

# Watch logs
docker compose logs -f control-plane
```

## Step 6: Verify SkyPilot Installation

```bash
# Check if SkyPilot is installed
docker compose exec control-plane sky check

# Expected output:
# Checking Azure...
# ✓ Azure credentials found
# ✓ Azure CLI configured
```

## Step 7: Test Real Launch from UI

1. Navigate to http://localhost:3000/launch
2. Select `meta-llama/Llama-3.1-8B-Instruct`
3. Configure:
   - **Provider**: Azure
   - **Region**: eastus
   - **Instance Type**: Standard_NV36ads_A10_v5
   - **Use Spot**: ✓ (checked)
4. Click **Launch Instance**

### What Happens

- **First time**: SkyPilot will provision resources (3-5 minutes)
- **Subsequent**: Uses cached resources (30s-1min)
- **Progress**: Real-time updates in UI
- **Logs**: View in control-plane container logs

### Expected Timeline

```
[0s]    Validating configuration          ✓
[5s]    Provisioning Azure resources      →
[60s]   Installing dependencies           →
[120s]  Loading model from R2             →
[180s]  Starting vLLM                      →
[240s]  Node registered                    ✓
```

## Step 8: Monitor Launch

### View Control Plane Logs
```bash
docker compose logs -f control-plane | grep -i "launch\|sky"
```

### Check SkyPilot Status
```bash
docker compose exec control-plane sky status
```

### View Instance Details
```bash
# Get cluster name from logs, then:
docker compose exec control-plane sky status cic-azure-eastus-a10-spot-xxxxxx
```

## Troubleshooting

### Issue: "AuthorizationFailed" when launching instances

**Error Message**:
```
The client does not have authorization to perform action
'Microsoft.Resources/subscriptions/resourcegroups/write'
```

**Solution**: Verify and assign the Contributor role:

```bash
# Get your subscription ID and App ID
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
APP_ID="your-app-id-here"  # From service principal creation

# Assign Contributor role (if not already assigned)
az role assignment create \
  --assignee $APP_ID \
  --role "Contributor" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"

# Verify the role assignment (WORKING METHOD)
az role assignment list \
  --scope "/subscriptions/$SUBSCRIPTION_ID" \
  --query "[?principalId=='$(az ad sp show --id $APP_ID --query id -o tsv)']" \
  --output table
```

**Important**:
- DO NOT use `az role assignment list --assignee $APP_ID` - it has caching issues and may return empty results
- USE the subscription scope query with JMESPath filter shown above
- Wait 2-5 minutes after creating role assignment for Azure RBAC propagation
- You should see `Contributor` role in the output

### Issue: "Microsoft.Authorization/roleAssignments/write" Permission Denied

**Error Message**:
```
The client does not have permission to perform action
'Microsoft.Authorization/roleAssignments/write' at scope
'/subscriptions/.../resourceGroups/.../providers/Microsoft.Authorization/roleAssignments/...'
```

**Root Cause**: SkyPilot needs to assign roles to resources it creates (VMs, storage, networks). The **Contributor** role alone is not sufficient.

**Solution**: Add the **User Access Administrator** role to your service principal:

```bash
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
APP_ID="your-app-id-here"  # Your service principal App ID

# Add User Access Administrator role
az role assignment create \
  --assignee $APP_ID \
  --role "User Access Administrator" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"

# Verify BOTH roles are assigned
az role assignment list \
  --scope "/subscriptions/$SUBSCRIPTION_ID" \
  --query "[?principalId=='$(az ad sp show --id $APP_ID --query id -o tsv)']" \
  --output table
```

Expected output should show **both** roles:
```
Name           RoleDefinitionName            PrincipalName  PrincipalType
-------------  ----------------------------  -------------  ----------------
xxxxx-xxxxx    Contributor                   crosslogic-sp  ServicePrincipal
yyyyy-yyyyy    User Access Administrator     crosslogic-sp  ServicePrincipal
```

**Wait 2-5 minutes** for Azure RBAC propagation, then retry your launch.

### Issue: "sky: executable file not found"

**Solution**: Rebuild control-plane container:
```bash
docker compose build control-plane
docker compose up -d control-plane
```

### Issue: "Azure credentials not found"

**Solution**: Verify `.env` file has Azure credentials:
```bash
grep AZURE .env
```

Then restart:
```bash
docker compose restart control-plane
```

### Issue: "Launch failed: quota exceeded"

**Solution**: Azure has GPU quotas. Request quota increase:
```bash
az vm list-usage --location eastus -o table | grep "Standard NV"
```

Request increase: https://aka.ms/ProdportalCRP/?#create/Microsoft.Support

### Issue: "Model not found in R2"

**Solution**: Upload model to R2 first:
```bash
python scripts/upload-model-to-r2.py meta-llama/Llama-3.1-8B-Instruct
```

Or it will fallback to HuggingFace download (slower first time).

### Issue: "Launch stuck at provisioning"

**Solution**: Check Azure portal for any service health issues:
- https://portal.azure.com/#blade/Microsoft_Azure_Health/AzureHealthBrowseBlade/serviceIssues

Also check SkyPilot logs:
```bash
docker compose exec control-plane sky logs cic-azure-eastus-a10-spot-xxxxxx
```

## Verifying Real Launch vs Mock

### Real Launch Indicators
- ✅ Logs show: `launching GPU node with SkyPilot`
- ✅ Response includes: `Real GPU instance launch initiated via SkyPilot`
- ✅ Logs show: `sky launch` command execution
- ✅ Azure portal shows new VM in resource group
- ✅ Takes 3-5 minutes (not 82 seconds)

### Mock Launch Indicators  
- ❌ Logs show: `orchestrator not available, using mock launch simulation`
- ❌ Response includes: `SIMULATION`
- ❌ Completes in exactly 82 seconds
- ❌ No Azure resources created

## Cost Estimates

### Spot Instances (Recommended)
- **A10 (24GB VRAM)**: ~$0.30/hour
- **A100 (40GB VRAM)**: ~$1.00/hour
- **H100 (80GB VRAM)**: ~$2.50/hour

### On-Demand Instances
- **A10**: ~$1.20/hour
- **A100**: ~$3.50/hour
- **H100**: ~$8.00/hour

**Tip**: Always use spot instances for development/testing!

## Next Steps

### 1. Test Launch
```bash
# From UI at http://localhost:3000/launch
# Or via API:
curl -X POST http://localhost:8080/admin/instances/launch \
  -H "X-Admin-Token: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "azure",
    "region": "eastus",
    "instance_type": "Standard_NV36ads_A10_v5",
    "use_spot": true,
    "gpu_count": 1
  }'
```

### 2. Check Status
```bash
# Get job_id from launch response, then:
curl http://localhost:8080/admin/instances/status?job_id=launch-XXXXXXXX \
  -H "X-Admin-Token: YOUR_ADMIN_TOKEN"
```

### 3. List Active Nodes
```bash
docker compose exec control-plane sky status
```

### 4. Terminate Instance
```bash
docker compose exec control-plane sky down cic-azure-eastus-a10-spot-xxxxxx
```

## Security Best Practices

1. **Never commit credentials** to git
2. **Use service principals** for production
3. **Rotate secrets** regularly
4. **Set resource group** for cost tracking
5. **Enable auto-shutdown** for dev instances

## Support

If you encounter issues:
1. Check `docker compose logs control-plane`
2. Run `docker compose exec control-plane sky check`
3. Verify Azure credentials with `az account show`
4. Check Azure quotas in portal

## Summary

You now have:
- ✅ SkyPilot installed with Azure support
- ✅ Real GPU instance provisioning
- ✅ Automatic fallback to mock if no credentials
- ✅ Full integration with UI
- ✅ Cost-optimized spot instances

**Ready to launch your first GPU instance on Azure!** 🚀

---

## Quick Reference: Working Azure CLI Commands

### Create Service Principal
```bash
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
az ad sp create-for-rbac \
  --name "crosslogic-sp" \
  --role "Contributor" \
  --scopes "/subscriptions/$SUBSCRIPTION_ID"
```

### Verify Service Principal
```bash
APP_ID="your-app-id-here"
az ad sp show --id $APP_ID --query "{appId:appId, displayName:displayName, objectId:id}" -o table
```

### Verify Role Assignment (WORKING METHOD ✅)
```bash
# This works reliably
az role assignment list \
  --scope "/subscriptions/$SUBSCRIPTION_ID" \
  --query "[?principalId=='$(az ad sp show --id $APP_ID --query id -o tsv)']" \
  --output table

# This may return empty due to caching (AVOID ❌)
az role assignment list --assignee $APP_ID --output table
```

### Assign Contributor Role
```bash
az role assignment create \
  --assignee $APP_ID \
  --role "Contributor" \
  --scope "/subscriptions/$SUBSCRIPTION_ID"
```

### Get Credentials for .env
```bash
echo "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
echo "AZURE_TENANT_ID=$(az account show --query tenantId -o tsv)"
echo "AZURE_CLIENT_ID=$APP_ID"
echo "AZURE_CLIENT_SECRET=<from service principal creation output>"
```

