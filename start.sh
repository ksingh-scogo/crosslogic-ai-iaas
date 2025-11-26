#!/bin/bash
set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
MONITORING=false
SKIP_BUILD=false
for arg in "$@"; do
    case $arg in
        --monitoring|-m)
            MONITORING=true
            ;;
        --skip-build|-s)
            SKIP_BUILD=true
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --monitoring, -m   Include Prometheus and Grafana"
            echo "  --skip-build, -s   Skip Docker image build (faster restart)"
            echo "  --help, -h         Show this help"
            exit 0
            ;;
    esac
done

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║   CrossLogic Inference Cloud - Quick Start                     ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

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
    echo -e "${YELLOW}⚠️  .env file not found, using defaults${NC}"
    echo "   (copy config/.env.example to .env for custom settings)"
else
    echo -e "${GREEN}✅ .env file found${NC}"
fi

# Build compose command
COMPOSE_CMD="docker compose"
if [ "$MONITORING" = true ]; then
    COMPOSE_CMD="docker compose --profile monitoring"
    echo -e "${BLUE}ℹ️  Monitoring enabled (Prometheus + Grafana)${NC}"
fi

# Build images
echo ""
if [ "$SKIP_BUILD" = true ]; then
    echo -e "${YELLOW}[3/4] Skipping build (--skip-build)...${NC}"
else
    echo -e "${YELLOW}[3/4] Building Docker images...${NC}"
    $COMPOSE_CMD build
fi

# Start services
echo ""
echo -e "${YELLOW}[4/4] Starting services...${NC}"
$COMPOSE_CMD up -d

# Wait for health checks
echo ""
echo "Waiting for services to be ready..."
sleep 8

# Check service status
echo ""
echo "Service Status:"
$COMPOSE_CMD ps

# Health check
echo ""
echo "Running health checks..."

# Check control plane
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Control Plane: healthy (http://localhost:8080)${NC}"
else
    echo -e "${YELLOW}⚠️  Control Plane: starting...${NC}"
fi

# Check dashboard
if curl -sf http://localhost:3000 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Dashboard: running (http://localhost:3000)${NC}"
else
    echo -e "${YELLOW}⚠️  Dashboard: starting... (may take 30 seconds)${NC}"
fi

# Check monitoring if enabled
if [ "$MONITORING" = true ]; then
    if curl -sf http://localhost:9091 > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Prometheus: running (http://localhost:9091)${NC}"
    fi
    if curl -sf http://localhost:3001 > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Grafana: running (http://localhost:3001)${NC}"
    fi
fi

# Final message
echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    Setup Complete!                              ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "Services:"
echo "  📊 Dashboard:      http://localhost:3000"
echo "  🔌 API Gateway:    http://localhost:8080"
if [ "$MONITORING" = true ]; then
    echo "  📈 Prometheus:     http://localhost:9091"
    echo "  📉 Grafana:        http://localhost:3001 (admin/admin)"
fi
echo ""
echo "Quick Commands:"
echo "  • View logs:       docker compose logs -f"
echo "  • Restart:         docker compose restart"
echo "  • Stop:            docker compose down"
echo "  • Fast restart:    ./start.sh --skip-build"
if [ "$MONITORING" = false ]; then
    echo "  • With monitoring: ./start.sh --monitoring"
fi
echo ""
echo -e "${GREEN}Ready! Open http://localhost:3000 to get started.${NC}"
echo ""


