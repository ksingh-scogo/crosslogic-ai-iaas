#!/bin/bash
# verify-launch-setup.sh - Verify GPU launch setup is correct

set -e

echo "=== CrossLogic GPU Launch Setup Verification ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    FAILED=1
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

FAILED=0

# Check 1: Docker is running
echo "1. Checking Docker..."
if docker ps > /dev/null 2>&1; then
    check_pass "Docker is running"
else
    check_fail "Docker is not running. Start Docker Desktop."
fi

# Check 2: Control plane container exists
echo
echo "2. Checking control plane container..."
if docker ps -a | grep -q crosslogic-control-plane; then
    if docker ps | grep -q crosslogic-control-plane; then
        check_pass "Control plane container is running"
    else
        check_fail "Control plane container exists but not running"
        echo "   Run: docker-compose up -d control-plane"
    fi
else
    check_fail "Control plane container not found"
    echo "   Run: docker-compose up -d"
fi

# Check 3: SkyPilot is installed in container
echo
echo "3. Checking SkyPilot installation..."
if docker exec crosslogic-control-plane sky --version > /dev/null 2>&1; then
    VERSION=$(docker exec crosslogic-control-plane sky --version 2>&1 | head -1)
    check_pass "SkyPilot is installed: $VERSION"
else
    check_fail "SkyPilot is not installed in container"
    echo "   Fix: Rebuild image with updated Dockerfile"
    echo "   Run: docker-compose build control-plane && docker-compose up -d"
fi

# Check 4: Cloud CLIs are available
echo
echo "4. Checking cloud CLI tools..."

if docker exec crosslogic-control-plane aws --version > /dev/null 2>&1; then
    check_pass "AWS CLI is installed"
else
    check_warn "AWS CLI not found (needed for AWS deployments)"
fi

if docker exec crosslogic-control-plane az --version > /dev/null 2>&1; then
    check_pass "Azure CLI is installed"
else
    check_warn "Azure CLI not found (needed for Azure deployments)"
fi

if docker exec crosslogic-control-plane gcloud --version > /dev/null 2>&1; then
    check_pass "GCloud CLI is installed"
else
    check_warn "GCloud CLI not found (needed for GCP deployments)"
fi

# Check 5: Environment variables
echo
echo "5. Checking environment configuration..."

if [ -f .env ]; then
    check_pass ".env file exists"

    # Check for cloud credentials
    if grep -q "AWS_ACCESS_KEY_ID=" .env && ! grep -q "AWS_ACCESS_KEY_ID=$" .env; then
        check_pass "AWS credentials configured"
    else
        check_warn "AWS credentials not configured (needed for AWS launches)"
    fi

    if grep -q "AZURE_SUBSCRIPTION_ID=" .env && ! grep -q "AZURE_SUBSCRIPTION_ID=$" .env; then
        check_pass "Azure credentials configured"
    else
        check_warn "Azure credentials not configured (needed for Azure launches)"
    fi

    if grep -q "GCP_PROJECT_ID=" .env && ! grep -q "GCP_PROJECT_ID=$" .env; then
        check_pass "GCP credentials configured"
    else
        check_warn "GCP project not configured (needed for GCP launches)"
    fi

    # Check R2 configuration
    if grep -q "R2_ENDPOINT=" .env && ! grep -q "R2_ENDPOINT=$" .env; then
        check_pass "R2 configuration found"
    else
        check_warn "R2 not configured (models will download from HuggingFace)"
    fi
else
    check_fail ".env file not found"
    echo "   Run: cp config/env.template .env"
    echo "   Then edit .env with your credentials"
fi

# Check 6: Database is accessible
echo
echo "6. Checking database..."
if docker exec crosslogic-control-plane wget -q --spider http://postgres:5432 2>&1 | grep -q "Connection refused"; then
    # Connection refused means port is open but not HTTP
    check_pass "Database is reachable"
elif docker ps | grep -q crosslogic-postgres; then
    check_pass "Database container is running"
else
    check_fail "Database is not accessible"
    echo "   Run: docker-compose up -d postgres"
fi

# Check 7: Redis is accessible
echo
echo "7. Checking Redis..."
if docker ps | grep -q crosslogic-redis; then
    check_pass "Redis container is running"
else
    check_fail "Redis is not running"
    echo "   Run: docker-compose up -d redis"
fi

# Check 8: Control plane is healthy
echo
echo "8. Checking control plane health..."
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    check_pass "Control plane is healthy"
else
    check_fail "Control plane health check failed"
    echo "   Check logs: docker-compose logs control-plane"
fi

# Summary
echo
echo "================================"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All checks passed!${NC}"
    echo
    echo "You can now launch GPU instances from the dashboard:"
    echo "  http://localhost:3000"
else
    echo -e "${RED}Some checks failed. Please fix the issues above.${NC}"
    echo
    echo "Common fixes:"
    echo "  1. Rebuild containers: docker-compose build"
    echo "  2. Restart services: docker-compose up -d"
    echo "  3. Configure credentials: edit .env file"
    echo "  4. Check logs: docker-compose logs control-plane"
fi
echo "================================"

exit $FAILED
