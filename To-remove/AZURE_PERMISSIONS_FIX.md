# Azure Permissions Fix for CrossLogic AI IaaS

## Problem
```
AuthorizationFailed: The client '8046cf1e-d8b7-4f4e-825a-0f011093fffa' does not have authorization to perform action 'Microsoft.Resources/subscriptions/resourcegroups/write'
```

## Solution

Your Azure service principal needs **Contributor** role on the subscription.

### Option 1: Azure Portal (Recommended)

1. Go to [Azure Portal](https://portal.azure.com)
2. Navigate to **Subscriptions**
3. Select subscription: `5bf6669e-9a6c-4632-bb18-aa2055904028`
4. Click **Access control (IAM)** in left menu
5. Click **+ Add** → **Add role assignment**
6. Select **Contributor** role
7. Click **Next**
8. Click **+ Select members**
9. Search for your service principal:
   - App ID: `8046cf1e-d8b7-4f4e-825a-0f011093fffa`
   - Object ID: `e9e88cd5-e72f-4cf6-b230-a2ffcc4069fc`
10. Click **Review + assign**

### Option 2: Azure CLI

```bash
# Assign Contributor role to the service principal
az role assignment create \
  --assignee 8046cf1e-d8b7-4f4e-825a-0f011093fffa \
  --role "Contributor" \
  --scope /subscriptions/5bf6669e-9a6c-4632-bb18-aa2055904028

# Verify the assignment
az role assignment list \
  --assignee 8046cf1e-d8b7-4f4e-825a-0f011093fffa \
  --output table
```

### Option 3: Resource Group Level (More Restrictive)

If you want to limit permissions to specific resource groups:

```bash
# Create a resource group for CrossLogic clusters
az group create --name crosslogic-clusters --location eastus

# Assign Contributor role only to this resource group
az role assignment create \
  --assignee 8046cf1e-d8b7-4f4e-825a-0f011093fffa \
  --role "Contributor" \
  --scope /subscriptions/5bf6669e-9a6c-4632-bb18-aa2055904028/resourceGroups/crosslogic-clusters
```

However, note that SkyPilot creates resource groups dynamically, so **subscription-level Contributor is recommended**.

## Required Permissions

The service principal needs these specific permissions:
- `Microsoft.Resources/subscriptions/resourcegroups/write` (create resource groups)
- `Microsoft.Resources/subscriptions/resourcegroups/delete` (delete resource groups)
- `Microsoft.Compute/*` (create VMs)
- `Microsoft.Network/*` (create virtual networks)
- `Microsoft.Storage/*` (create storage accounts)

The **Contributor** role includes all of these.

## Verify After Assignment

After assigning the role, wait 5-10 minutes for Azure RBAC propagation, then test:

```bash
# Test resource group creation
az group create --name test-skypilot-rg --location eastus

# If successful, clean up
az group delete --name test-skypilot-rg --yes --no-wait
```

## Alternative: Use Custom Role (Advanced)

If you want minimal permissions:

```json
{
  "Name": "SkyPilot Cluster Manager",
  "Description": "Minimal permissions for SkyPilot to manage GPU clusters",
  "Actions": [
    "Microsoft.Resources/subscriptions/resourcegroups/*",
    "Microsoft.Compute/virtualMachines/*",
    "Microsoft.Network/virtualNetworks/*",
    "Microsoft.Network/networkInterfaces/*",
    "Microsoft.Network/publicIPAddresses/*",
    "Microsoft.Network/networkSecurityGroups/*",
    "Microsoft.Storage/storageAccounts/*"
  ],
  "NotActions": [],
  "AssignableScopes": [
    "/subscriptions/5bf6669e-9a6c-4632-bb18-aa2055904028"
  ]
}
```

Save as `skypilot-role.json` and create:

```bash
az role definition create --role-definition skypilot-role.json
az role assignment create \
  --assignee 8046cf1e-d8b7-4f4e-825a-0f011093fffa \
  --role "SkyPilot Cluster Manager" \
  --scope /subscriptions/5bf6669e-9a6c-4632-bb18-aa2055904028
```

## Next Steps

After fixing permissions:
1. Wait 5-10 minutes for RBAC propagation
2. Retry launching the node from the UI
3. Monitor logs: `docker-compose logs -f control-plane`
