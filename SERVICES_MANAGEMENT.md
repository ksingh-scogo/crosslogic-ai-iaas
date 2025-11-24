# Service Management Guide

This guide explains how to manage individual Docker services in the CrossLogic AI IaaS platform.

## Quick Start

Use the `manage-services.sh` script to control individual services:

```bash
./manage-services.sh <command> <service>
```

## Available Services

| Service | Description | Port |
|---------|-------------|------|
| `postgres` | Main PostgreSQL database | 5432 |
| `postgres-skypilot` | SkyPilot state database | 5433 |
| `redis` | Cache for rate limiting | 6379 |
| `control-plane` | Go backend API server | 8080, 9090 |
| `dashboard` | React frontend dashboard | 3000 |
| `prometheus` | Metrics collection | 9091 |
| `grafana` | Metrics visualization | 3001 |

## Commands

### Start a Service
```bash
./manage-services.sh start <service>

# Examples:
./manage-services.sh start control-plane
./manage-services.sh start dashboard
```

### Stop a Service
```bash
./manage-services.sh stop <service>

# Examples:
./manage-services.sh stop control-plane
./manage-services.sh stop redis
```

### Restart a Service
```bash
./manage-services.sh restart <service>

# Examples:
./manage-services.sh restart control-plane
./manage-services.sh restart dashboard
```

### Rebuild a Service
**Use this after making code changes!**
```bash
./manage-services.sh rebuild <service>

# Examples:
./manage-services.sh rebuild control-plane
./manage-services.sh rebuild dashboard
```

### View Logs
```bash
./manage-services.sh logs <service>

# Examples:
./manage-services.sh logs control-plane
./manage-services.sh logs postgres

# Press Ctrl+C to exit
```

### Check Service Status
```bash
# Single service
./manage-services.sh status <service>

# All services
./manage-services.sh all-status
```

## Common Workflows

### After Changing Backend Code (Go)
```bash
./manage-services.sh rebuild control-plane
./manage-services.sh logs control-plane
```

### After Changing Frontend Code (React)
```bash
./manage-services.sh rebuild dashboard
./manage-services.sh logs dashboard
```

### Debugging Database Issues
```bash
# View logs
./manage-services.sh logs postgres

# Restart database
./manage-services.sh restart postgres
```

### Troubleshooting Control Plane
```bash
# Check status
./manage-services.sh status control-plane

# View logs
./manage-services.sh logs control-plane

# Rebuild if needed
./manage-services.sh rebuild control-plane
```

## Service Dependencies

Some services depend on others. The recommended startup order:
1. `postgres` (main database)
2. `postgres-skypilot` (SkyPilot state)
3. `redis` (cache)
4. `control-plane` (backend - depends on all above)
5. `dashboard` (frontend - depends on control-plane)
6. `prometheus` (monitoring - optional)
7. `grafana` (visualization - optional)

The script handles dependencies automatically, but keep this in mind when troubleshooting.

## Tips

- **Always rebuild after code changes** - The Docker image needs to be rebuilt to include your changes
- **Check logs when debugging** - Logs are your best friend for troubleshooting
- **Use all-status frequently** - Quickly see what's running and what's not
- **Stop unused services** - Save resources by stopping services you're not actively using

## Direct Docker Compose Commands

If you prefer using docker-compose directly:

```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Rebuild specific service
docker-compose build control-plane

# View logs
docker-compose logs -f control-plane

# Check status
docker-compose ps
```

## Accessing Services

Once running, access services at:

- **Dashboard**: http://localhost:3000
- **API**: http://localhost:8080
- **API Docs (Swagger)**: http://localhost:3000/api-docs
- **Prometheus**: http://localhost:9091
- **Grafana**: http://localhost:3001
- **Redis**: localhost:6379
- **PostgreSQL (main)**: localhost:5432
- **PostgreSQL (SkyPilot)**: localhost:5433
