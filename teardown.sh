#!/bin/bash
set -euo pipefail

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║   CrossLogic - Smart Teardown & Reset Script                  ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
FULL_RESET=false
KEEP_IMAGES=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --full)
      FULL_RESET=true
      shift
      ;;
    --keep-images)
      KEEP_IMAGES=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: ./teardown.sh [--full] [--keep-images]"
      echo "  --full         : Also remove Docker images (slower rebuild)"
      echo "  --keep-images  : Keep Docker images (faster restart)"
      exit 1
      ;;
  esac
done

echo -e "${YELLOW}Teardown mode:${NC}"
if [ "$FULL_RESET" = true ]; then
  echo "  • Full reset (including images)"
else
  echo "  • Smart reset (keeping images for faster restart)"
fi
echo ""

# Step 1: Stop all services
echo -e "${BLUE}[1/5] Stopping services...${NC}"
if docker compose ps --quiet 2>/dev/null | grep -q .; then
  docker compose down
  echo -e "${GREEN}✅ Services stopped${NC}"
else
  echo -e "${YELLOW}⚠️  No services running${NC}"
fi
echo ""

# Step 2: Remove volumes (database data, cache, etc.)
echo -e "${BLUE}[2/5] Cleaning up volumes...${NC}"
echo -e "${YELLOW}This will delete:${NC}"
echo "  • PostgreSQL database (all metadata, tenants, usage)"
echo "  • Redis cache (rate limits, sessions)"
echo "  • Grafana data (dashboards, configs)"
echo -e "${RED}  • R2 models are NOT touched (safe)${NC}"
echo ""
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  # Remove named volumes
  docker volume rm crosslogic-ai-iaas_postgres_data 2>/dev/null && echo "  ✓ PostgreSQL data removed" || echo "  - PostgreSQL data not found"
  docker volume rm crosslogic-ai-iaas_redis_data 2>/dev/null && echo "  ✓ Redis data removed" || echo "  - Redis data not found"
  docker volume rm crosslogic-ai-iaas_grafana_data 2>/dev/null && echo "  ✓ Grafana data removed" || echo "  - Grafana data not found"
  
  echo -e "${GREEN}✅ Volumes cleaned${NC}"
else
  echo -e "${YELLOW}⚠️  Skipped volume cleanup${NC}"
fi
echo ""

# Step 3: Clean up containers
echo -e "${BLUE}[3/5] Removing stopped containers...${NC}"
REMOVED_CONTAINERS=$(docker ps -a --filter "name=crosslogic" --format "{{.Names}}" | wc -l | tr -d ' ')
if [ "$REMOVED_CONTAINERS" -gt 0 ]; then
  docker ps -a --filter "name=crosslogic" --format "{{.Names}}" | xargs -r docker rm -f 2>/dev/null
  echo -e "${GREEN}✅ Removed $REMOVED_CONTAINERS container(s)${NC}"
else
  echo -e "${YELLOW}⚠️  No containers to remove${NC}"
fi
echo ""

# Step 4: Optionally remove images
echo -e "${BLUE}[4/5] Handling Docker images...${NC}"
if [ "$FULL_RESET" = true ]; then
  echo -e "${YELLOW}Removing Docker images (will require rebuild)...${NC}"
  docker images --filter "reference=crosslogic-ai-iaas*" --format "{{.Repository}}:{{.Tag}}" | xargs -r docker rmi -f 2>/dev/null
  echo -e "${GREEN}✅ Images removed (next start will rebuild)${NC}"
elif [ "$KEEP_IMAGES" = true ]; then
  echo -e "${GREEN}✅ Keeping images (faster restart)${NC}"
else
  # Default: Smart cleanup - remove dangling images only
  DANGLING=$(docker images -f "dangling=true" -q | wc -l | tr -d ' ')
  if [ "$DANGLING" -gt 0 ]; then
    docker images -f "dangling=true" -q | xargs -r docker rmi 2>/dev/null
    echo -e "${GREEN}✅ Removed $DANGLING dangling image(s)${NC}"
  else
    echo -e "${GREEN}✅ No dangling images (keeping built images)${NC}"
  fi
fi
echo ""

# Step 5: Clean up networks
echo -e "${BLUE}[5/5] Cleaning up networks...${NC}"
docker network rm crosslogic-network 2>/dev/null && echo -e "${GREEN}✅ Network removed${NC}" || echo -e "${YELLOW}⚠️  Network not found${NC}"
echo ""

# Optional: System prune (be careful!)
echo -e "${BLUE}Optional: Docker system cleanup${NC}"
echo "This will remove:"
echo "  • All stopped containers"
echo "  • All dangling images"
echo "  • All unused networks"
echo "  • Build cache"
echo ""
read -p "Run docker system prune? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  docker system prune -f
  echo -e "${GREEN}✅ System cleaned${NC}"
else
  echo -e "${YELLOW}⚠️  Skipped system prune${NC}"
fi
echo ""

# Summary
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    🎉 Teardown Complete!                       ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}Cleaned up:${NC}"
echo "  ✓ All services stopped"
echo "  ✓ Database data cleared"
echo "  ✓ Cache cleared"
echo "  ✓ Containers removed"
if [ "$FULL_RESET" = true ]; then
  echo "  ✓ Images removed (will rebuild on next start)"
else
  echo "  ✓ Images kept (faster restart)"
fi
echo ""
echo -e "${BLUE}Preserved:${NC}"
echo "  ✓ R2 models (safe in cloud)"
echo "  ✓ Source code"
echo "  ✓ Configuration files"
echo ""
echo -e "${YELLOW}To start fresh:${NC}"
echo "  ./start.sh"
echo ""
echo -e "${YELLOW}Estimated restart time:${NC}"
if [ "$FULL_RESET" = true ]; then
  echo "  • ~5-10 minutes (rebuild + start)"
else
  echo "  • ~30 seconds (start only)"
fi
echo ""
echo -e "${GREEN}Ready for fresh start! 🚀${NC}"
echo ""

# Exit codes
# 0 = success
# 1 = error
exit 0


