#!/bin/bash

# CrossLogic Service Manager
# Manage individual docker-compose services easily

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Available services
SERVICES=(
    "postgres"
    "postgres-skypilot"
    "redis"
    "control-plane"
    "dashboard"
    "prometheus"
    "grafana"
)

# Get service description
get_service_desc() {
    case "$1" in
        postgres) echo "Main PostgreSQL database" ;;
        postgres-skypilot) echo "SkyPilot state database" ;;
        redis) echo "Redis cache for rate limiting" ;;
        control-plane) echo "Go backend API server" ;;
        dashboard) echo "React frontend dashboard" ;;
        prometheus) echo "Metrics collection" ;;
        grafana) echo "Metrics visualization" ;;
        *) echo "Unknown service" ;;
    esac
}

# Print usage
usage() {
    echo -e "${BLUE}CrossLogic Service Manager${NC}"
    echo ""
    echo "Usage: $0 <command> <service>"
    echo ""
    echo "Commands:"
    echo "  start       - Start a service"
    echo "  stop        - Stop a service"
    echo "  restart     - Restart a service"
    echo "  rebuild     - Rebuild and restart a service"
    echo "  logs        - View service logs (follow)"
    echo "  status      - Show service status"
    echo "  all-status  - Show all services status"
    echo ""
    echo "Services:"
    for service in "${SERVICES[@]}"; do
        echo "  ${service} - $(get_service_desc "$service")"
    done
    echo ""
    echo "Examples:"
    echo "  $0 start control-plane"
    echo "  $0 rebuild dashboard"
    echo "  $0 logs control-plane"
    echo "  $0 restart redis"
    echo "  $0 all-status"
}

# Validate service name
validate_service() {
    local service=$1
    for s in "${SERVICES[@]}"; do
        if [[ "$s" == "$service" ]]; then
            return 0
        fi
    done
    echo -e "${RED}Error: Invalid service name '$service'${NC}"
    echo ""
    echo "Available services:"
    for s in "${SERVICES[@]}"; do
        echo "  - $s"
    done
    exit 1
}

# Start service
start_service() {
    local service=$1
    echo -e "${BLUE}Starting $service...${NC}"
    docker-compose up -d "$service"
    echo -e "${GREEN}✓ $service started${NC}"
}

# Stop service
stop_service() {
    local service=$1
    echo -e "${YELLOW}Stopping $service...${NC}"
    docker-compose stop "$service"
    echo -e "${GREEN}✓ $service stopped${NC}"
}

# Restart service
restart_service() {
    local service=$1
    echo -e "${YELLOW}Restarting $service...${NC}"
    docker-compose restart "$service"
    echo -e "${GREEN}✓ $service restarted${NC}"
}

# Rebuild service
rebuild_service() {
    local service=$1
    echo -e "${BLUE}Rebuilding $service...${NC}"
    docker-compose stop "$service"
    docker-compose build "$service"
    docker-compose up -d "$service"
    echo -e "${GREEN}✓ $service rebuilt and started${NC}"
}

# View logs
view_logs() {
    local service=$1
    echo -e "${BLUE}Viewing logs for $service (Ctrl+C to exit)...${NC}"
    docker-compose logs -f "$service"
}

# Show service status
service_status() {
    local service=$1
    echo -e "${BLUE}Status for $service:${NC}"
    docker-compose ps "$service"
}

# Show all services status
all_status() {
    echo -e "${BLUE}All Services Status:${NC}"
    echo ""
    docker-compose ps
}

# Main script logic
if [[ $# -eq 0 ]]; then
    usage
    exit 0
fi

COMMAND=$1

# Handle all-status command (no service parameter needed)
if [[ "$COMMAND" == "all-status" ]]; then
    all_status
    exit 0
fi

if [[ $# -lt 2 ]]; then
    echo -e "${RED}Error: Missing service name${NC}"
    echo ""
    usage
    exit 1
fi

SERVICE=$2

# Validate service name
validate_service "$SERVICE"

# Execute command
case "$COMMAND" in
    start)
        start_service "$SERVICE"
        ;;
    stop)
        stop_service "$SERVICE"
        ;;
    restart)
        restart_service "$SERVICE"
        ;;
    rebuild)
        rebuild_service "$SERVICE"
        ;;
    logs)
        view_logs "$SERVICE"
        ;;
    status)
        service_status "$SERVICE"
        ;;
    *)
        echo -e "${RED}Error: Unknown command '$COMMAND'${NC}"
        echo ""
        usage
        exit 1
        ;;
esac
