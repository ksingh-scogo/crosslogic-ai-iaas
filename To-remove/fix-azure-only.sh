#!/bin/bash
set -e

echo "=========================================="
echo "CrossLogic Azure-Only Configuration Fix"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo -e "${YELLOW}Step 1: Creating backup of current configuration...${NC}"
cp .env .env.backup.$(date +%Y%m%d_%H%M%S)
cp control-plane/internal/orchestrator/reconciler.go control-plane/internal/orchestrator/reconciler.go.backup
echo -e "${GREEN}✓ Backups created${NC}"
echo ""

echo -e "${YELLOW}Step 2: Updating .env to disable AWS...${NC}"
# Comment out AWS credentials
sed -i.tmp 's/^AWS_ACCESS_KEY_ID=/#AWS_ACCESS_KEY_ID=/' .env
sed -i.tmp 's/^AWS_SECRET_ACCESS_KEY=/#AWS_SECRET_ACCESS_KEY=/' .env
sed -i.tmp 's/^AWS_DEFAULT_REGION=/#AWS_DEFAULT_REGION=/' .env
rm -f .env.tmp
echo -e "${GREEN}✓ AWS credentials disabled in .env${NC}"
echo ""

echo -e "${YELLOW}Step 3: Creating SkyPilot Azure-only config...${NC}"
mkdir -p control-plane/scripts
cat > control-plane/scripts/skypilot-config.yaml << 'EOF'
# SkyPilot Configuration for Azure-Only Deployment
# This disables AWS and enables only Azure

allowed_clouds:
  - azure

azure:
  # Use spot instances for 60-90% cost savings
  prioritize_low_priority_vms: true

# Explicitly disable all other clouds
disabled_clouds:
  - aws
  - gcp
  - lambda
  - oci
  - kubernetes
  - ibm
  - scp
  - cloudflare
  - runpod
  - paperspace
  - vsphere
  - cudo
  - fluidstack
EOF
echo -e "${GREEN}✓ SkyPilot config created${NC}"
echo ""

echo -e "${YELLOW}Step 4: Fixing reconciler.go to remove --refresh flag...${NC}"
# Update reconciler.go line 96
sed -i.tmp 's/cmd := exec.CommandContext(ctx, "sky", "status", "--refresh", "--json")/cmd := exec.CommandContext(ctx, "sky", "status", "--json")/' control-plane/internal/orchestrator/reconciler.go

# Add comment above the line
sed -i.tmp '/cmd := exec.CommandContext(ctx, "sky", "status", "--json")/i\
	// Removed --refresh flag to avoid AWS API calls when AWS is disabled\
	// This prevents exit code 2 errors in Azure-only deployments
' control-plane/internal/orchestrator/reconciler.go

rm -f control-plane/internal/orchestrator/reconciler.go.tmp
echo -e "${GREEN}✓ Reconciler updated${NC}"
echo ""

echo -e "${YELLOW}Step 5: Updating Dockerfile to include SkyPilot config...${NC}"
# Check if the line already exists
if ! grep -q "skypilot-config.yaml" Dockerfile.control-plane; then
    # Find the line number where we copy entrypoint.sh (line 66)
    # Insert the new COPY command after it
    sed -i.tmp '/COPY --chown=crosslogic:crosslogic control-plane\/scripts\/entrypoint.sh/a\
# Copy SkyPilot configuration for Azure-only setup\
COPY --chown=crosslogic:crosslogic control-plane/scripts/skypilot-config.yaml /home/crosslogic/.sky/config.yaml
' Dockerfile.control-plane
    rm -f Dockerfile.control-plane.tmp
    echo -e "${GREEN}✓ Dockerfile updated${NC}"
else
    echo -e "${GREEN}✓ Dockerfile already contains SkyPilot config${NC}"
fi
echo ""

echo -e "${YELLOW}Step 6: Updating entrypoint.sh...${NC}"
# Update the AWS message in entrypoint.sh
sed -i.tmp 's/echo "✓ AWS credentials detected"/echo "⚠️  AWS credentials detected but not configured in this Azure-only deployment"/' control-plane/scripts/entrypoint.sh
rm -f control-plane/scripts/entrypoint.sh.tmp
echo -e "${GREEN}✓ Entrypoint script updated${NC}"
echo ""

echo -e "${YELLOW}Step 7: Stopping services...${NC}"
docker compose down
echo -e "${GREEN}✓ Services stopped${NC}"
echo ""

echo -e "${YELLOW}Step 8: Rebuilding control-plane container...${NC}"
docker compose build control-plane
echo -e "${GREEN}✓ Control-plane rebuilt${NC}"
echo ""

echo -e "${YELLOW}Step 9: Starting services...${NC}"
docker compose up -d
echo ""
echo "Waiting for services to be healthy..."
sleep 10
echo -e "${GREEN}✓ Services started${NC}"
echo ""

echo -e "${YELLOW}Step 10: Updating database deployments to use Azure...${NC}"
# Wait for postgres to be ready
sleep 5

# Update mistral-7b-us-east to use Azure
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas << 'EOSQL'
-- Update AWS deployment to Azure
UPDATE deployments
SET provider = 'azure',
    region = 'eastus',
    gpu_type = 'Standard_NC6s_v3'
WHERE name = 'mistral-7b-us-east';

-- Update llama deployment to have proper provider
UPDATE deployments
SET provider = 'azure',
    region = 'eastus',
    gpu_type = 'Standard_NC24ads_A100_v4'
WHERE name = 'llama-3-70b-prod' AND provider IS NULL;
EOSQL

echo -e "${GREEN}✓ Database deployments updated to Azure${NC}"
echo ""

echo -e "${YELLOW}Step 11: Verifying configuration...${NC}"
echo ""

echo "Database deployments:"
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "SELECT name, provider, region, gpu_type FROM deployments;"
echo ""

echo "Checking SkyPilot configuration..."
sleep 5
docker exec crosslogic-control-plane sky check 2>&1 | grep -E "Azure|AWS" || true
echo ""

echo -e "${GREEN}=========================================="
echo "Fix Applied Successfully!"
echo "==========================================${NC}"
echo ""
echo "Next steps:"
echo "1. Monitor logs: docker logs -f crosslogic-control-plane"
echo "2. Check for errors: docker logs crosslogic-control-plane 2>&1 | grep -i 'error'"
echo "3. Verify no AWS errors: docker logs crosslogic-control-plane 2>&1 | grep -i 'aws'"
echo "4. Access dashboard: http://localhost:3000"
echo ""
echo "Backups created:"
echo "  - .env.backup.*"
echo "  - control-plane/internal/orchestrator/reconciler.go.backup"
echo ""
echo "To rollback:"
echo "  ./rollback-azure-fix.sh"
echo ""
