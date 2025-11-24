#!/bin/bash
set -e

echo "=========================================="
echo "Rolling Back Azure-Only Configuration"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo -e "${YELLOW}Finding most recent backup...${NC}"
LATEST_BACKUP=$(ls -t .env.backup.* 2>/dev/null | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo -e "${RED}✗ No backup found!${NC}"
    exit 1
fi

echo "Found backup: $LATEST_BACKUP"
echo ""

echo -e "${YELLOW}Step 1: Stopping services...${NC}"
docker compose down
echo -e "${GREEN}✓ Services stopped${NC}"
echo ""

echo -e "${YELLOW}Step 2: Restoring .env...${NC}"
cp "$LATEST_BACKUP" .env
echo -e "${GREEN}✓ .env restored${NC}"
echo ""

echo -e "${YELLOW}Step 3: Restoring reconciler.go...${NC}"
if [ -f control-plane/internal/orchestrator/reconciler.go.backup ]; then
    cp control-plane/internal/orchestrator/reconciler.go.backup control-plane/internal/orchestrator/reconciler.go
    echo -e "${GREEN}✓ reconciler.go restored${NC}"
else
    echo -e "${YELLOW}! No reconciler.go backup found, skipping${NC}"
fi
echo ""

echo -e "${YELLOW}Step 4: Rebuilding control-plane...${NC}"
docker compose build control-plane
echo -e "${GREEN}✓ Control-plane rebuilt${NC}"
echo ""

echo -e "${YELLOW}Step 5: Starting services...${NC}"
docker compose up -d
echo -e "${GREEN}✓ Services started${NC}"
echo ""

echo -e "${GREEN}=========================================="
echo "Rollback Complete"
echo "==========================================${NC}"
echo ""
echo "Configuration has been restored to previous state."
echo "Monitor logs: docker logs -f crosslogic-control-plane"
echo ""
