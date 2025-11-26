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

# Available services (core services always available, monitoring via --profile)
SERVICES=(
    "postgres"
    "redis"
    "control-plane"
    "dashboard"
)

# Optional monitoring services (use: docker compose --profile monitoring)
MONITORING_SERVICES=(
    "prometheus"
    "grafana"
)

# Get service description
get_service_desc() {
    case "$1" in
        postgres) echo "PostgreSQL database" ;;
        redis) echo "Redis cache for rate limiting" ;;
        control-plane) echo "Go backend API server" ;;
        dashboard) echo "React frontend dashboard" ;;
        prometheus) echo "Metrics collection (monitoring profile)" ;;
        grafana) echo "Metrics visualization (monitoring profile)" ;;
        *) echo "Unknown service" ;;
    esac
}

# Print usage
usage() {
    echo -e "${BLUE}CrossLogic Service Manager${NC}"
    echo ""
    echo "Usage: $0 <command> [service]"
    echo ""
    echo "Commands:"
    echo "  start       - Start a service"
    echo "  stop        - Stop a service"
    echo "  restart     - Restart a service"
    echo "  rebuild     - Rebuild and restart a service"
    echo "  logs        - View service logs (follow)"
    echo "  status      - Show service status"
    echo "  all-status  - Show all services status"
    echo "  monitoring  - Start with monitoring services (Prometheus, Grafana)"
    echo ""
    echo "Core Services:"
    for service in "${SERVICES[@]}"; do
        echo "  ${service} - $(get_service_desc "$service")"
    done
    echo ""
    echo "Monitoring Services (optional):"
    for service in "${MONITORING_SERVICES[@]}"; do
        echo "  ${service} - $(get_service_desc "$service")"
    done
    echo ""
    echo "Examples:"
    echo "  $0 start control-plane"
    echo "  $0 rebuild dashboard"
    echo "  $0 logs control-plane"
    echo "  $0 monitoring           # Start all services + monitoring"
    echo "  $0 all-status"
}

# Validate service name (check both core and monitoring services)
validate_service() {
    local service=$1
    for s in "${SERVICES[@]}"; do
        if [[ "$s" == "$service" ]]; then
            return 0
        fi
    done
    for s in "${MONITORING_SERVICES[@]}"; do
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
    echo ""
    echo "Monitoring services (use --profile monitoring):"
    for s in "${MONITORING_SERVICES[@]}"; do
        echo "  - $s"
    done
    exit 1
}

# Check if service is a monitoring service
is_monitoring_service() {
    local service=$1
    for s in "${MONITORING_SERVICES[@]}"; do
        if [[ "$s" == "$service" ]]; then
            return 0
        fi
    done
    return 1
}

# Get docker compose command (with profile if needed)
get_compose_cmd() {
    local service=$1
    if is_monitoring_service "$service"; then
        echo "docker compose --profile monitoring"
    else
        echo "docker compose"
    fi
}

# Start service
start_service() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${BLUE}Starting $service...${NC}"
    $cmd up -d "$service"
    echo -e "${GREEN}✓ $service started${NC}"
}

# Stop service
stop_service() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${YELLOW}Stopping $service...${NC}"
    $cmd stop "$service"
    echo -e "${GREEN}✓ $service stopped${NC}"
}

# Restart service
restart_service() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${YELLOW}Restarting $service...${NC}"
    $cmd restart "$service"
    echo -e "${GREEN}✓ $service restarted${NC}"
}

# Rebuild service
rebuild_service() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${BLUE}Rebuilding $service...${NC}"
    $cmd stop "$service"
    $cmd build "$service"
    $cmd up -d "$service"
    echo -e "${GREEN}✓ $service rebuilt and started${NC}"
}

# View logs
view_logs() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${BLUE}Viewing logs for $service (Ctrl+C to exit)...${NC}"
    $cmd logs -f "$service"
}

# Show service status
service_status() {
    local service=$1
    local cmd=$(get_compose_cmd "$service")
    echo -e "${BLUE}Status for $service:${NC}"
    $cmd ps "$service"
}

# Show all services status
all_status() {
    echo -e "${BLUE}All Services Status:${NC}"
    echo ""
    docker compose ps
    echo ""
    echo -e "${YELLOW}Monitoring Services (use: $0 monitoring to start):${NC}"
    docker compose --profile monitoring ps 2>/dev/null || echo "  (not running)"
}

# Start all services with monitoring
start_monitoring() {
    echo -e "${BLUE}Starting all services with monitoring...${NC}"
    docker compose --profile monitoring up -d
    echo -e "${GREEN}✓ All services started (including Prometheus & Grafana)${NC}"
    echo ""
    echo "Access points:"
    echo "  Dashboard:   http://localhost:3000"
    echo "  API:         http://localhost:8080"
    echo "  Prometheus:  http://localhost:9091"
    echo "  Grafana:     http://localhost:3001 (admin/admin)"
}

# Main script logic
if [[ $# -eq 0 ]]; then
    usage
    exit 0
fi

COMMAND=$1

# Handle commands that don't need a service parameter
case "$COMMAND" in
    all-status)
        all_status
        exit 0
        ;;
    monitoring)
        start_monitoring
        exit 0
        ;;
esac

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
