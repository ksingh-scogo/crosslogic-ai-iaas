#!/bin/bash
set -euo pipefail

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║   CrossLogic Inference Cloud - Quick Start Script             ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check Docker
echo -e "${YELLOW}[1/4] Checking prerequisites...${NC}"
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker not found. Please install Docker 24+ first.${NC}"
    exit 1
fi

if ! command -v docker compose &> /dev/null; then
    echo -e "${RED}❌ Docker Compose not found. Please install Docker Compose v2+.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Docker $(docker --version | cut -d' ' -f3) found${NC}"
echo -e "${GREEN}✅ Docker Compose $(docker compose version | cut -d' ' -f4) found${NC}"

# Check .env file
echo ""
echo -e "${YELLOW}[2/4] Checking configuration...${NC}"
if [ ! -f .env ]; then
    echo -e "${RED}❌ .env file not found!${NC}"
    echo ""
    echo "Please create .env file with required credentials:"
    echo "  cp config/.env.example .env"
    echo "  nano .env"
    echo ""
    echo "See PREREQUISITES_CHECKLIST.md for required variables."
    exit 1
fi

echo -e "${GREEN}✅ .env file found${NC}"

# Build images
echo ""
echo -e "${YELLOW}[3/4] Building Docker images...${NC}"
docker compose build

# Start services
echo ""
echo -e "${YELLOW}[4/4] Starting services...${NC}"
docker compose up -d

# Wait for health checks
echo ""
echo "Waiting for services to be ready..."
sleep 10

# Check service status
echo ""
echo "Service Status:"
docker compose ps

# Health check
echo ""
echo "Running health checks..."

# Check control plane
if curl -sf http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}✅ Control Plane: healthy (http://localhost:8080)${NC}"
else
    echo -e "${RED}❌ Control Plane: not responding${NC}"
fi

# Check dashboard
if curl -sf http://localhost:3000 > /dev/null; then
    echo -e "${GREEN}✅ Dashboard: running (http://localhost:3000)${NC}"
else
    echo -e "${YELLOW}⚠️  Dashboard: starting... (may take 30 seconds)${NC}"
fi

# Final message
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    🎉 Setup Complete!                          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "Services:"
echo "  📊 Dashboard:      http://localhost:3000"
echo "  🔌 API Gateway:    http://localhost:8080"
echo "  📈 Grafana:        http://localhost:3001 (admin/admin)"
echo ""
echo "Next Steps:"
echo "  1. Open dashboard: open http://localhost:3000"
echo "  2. Go to 'Launch' page"
echo "  3. Select a model and click 'Launch'"
echo "  4. Watch real-time progress!"
echo ""
echo "Documentation:"
echo "  • Quick Start:     QUICK_START.md"
echo "  • Full Guide:      UPDATED_LOCAL_SETUP.md"
echo "  • Prerequisites:   PREREQUISITES_CHECKLIST.md"
echo ""
echo "Troubleshooting:"
echo "  • View logs:       docker compose logs"
echo "  • Restart:         docker compose restart"
echo "  • Stop:            docker compose down"
echo ""
echo -e "${GREEN}Happy inferencing! 🚀${NC}"
echo ""


