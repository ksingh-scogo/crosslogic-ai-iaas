# Docker Setup Guide - Local Development

This guide explains how to set up and run CrossLogic AI IaaS locally using Docker Compose.

## Prerequisites

- **Docker Desktop** or **Docker Engine** 20.10+ installed
- **Docker Compose** 2.0+ installed
- At least 8GB RAM allocated to Docker
- At least 20GB free disk space

## Quick Start

### 1. Clone and Navigate

```bash
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas
```

### 2. Create Environment File

Copy the example environment file and customize it:

```bash
cp .env.example .env
```

Edit `.env` and set your credentials:

```bash
# Minimal required configuration
DB_PASSWORD=your_secure_password_here
STRIPE_SECRET_KEY=sk_test_your_test_key  # Get from Stripe Dashboard
STRIPE_WEBHOOK_SECRET=whsec_your_webhook_secret
ADMIN_API_TOKEN=your_admin_token_minimum_32_chars
JWT_SECRET=your_jwt_secret_minimum_32_characters_long
```

### 3. Start All Services

```bash
docker-compose up -d
```

This will start:
- PostgreSQL (main database) on port 5432
- PostgreSQL (SkyPilot state) on port 5433
- Redis (cache + JuiceFS metadata) on port 6379
- Control Plane API on port 8080
- Prometheus on port 9091 (optional)
- Grafana on port 3000 (optional)

**Note:** Model storage uses your AWS S3 bucket configured in `.env`

### 4. Verify Services

Check that all services are running:

```bash
docker-compose ps
```

Expected output:
```
NAME                        STATUS
crosslogic-postgres         Up (healthy)
crosslogic-skypilot-db      Up (healthy)
crosslogic-redis            Up (healthy)
crosslogic-minio            Up (healthy)
crosslogic-control-plane    Up
```

### 5. Access Services

**Control Plane API:**
```bash
curl http://localhost:8080/health
```

**Grafana (optional):**
Open http://localhost:3000
- Username: `admin`
- Password: `admin` (or value from .env)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Docker Compose Environment               │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │  PostgreSQL  │      │  SkyPilot DB │                    │
│  │   (Main)     │      │  (State)     │                    │
│  │  :5432       │      │  :5433       │                    │
│  └──────┬───────┘      └──────┬───────┘                    │
│         │                     │                              │
│         ├─────────────────────┴────────┐                    │
│         │                              │                    │
│  ┌──────▼──────────────────────────────▼──────┐            │
│  │        Control Plane API                    │            │
│  │        :8080 (API) :9090 (Metrics)         │            │
│  └──────┬──────────────────────────────────┬──┘            │
│         │                                   │                │
│  ┌──────▼───────┐                   ┌──────▼───────┐       │
│  │    Redis     │                   │    MinIO     │       │
│  │  :6379       │                   │  :9000 :9001 │       │
│  │  Cache       │                   │  S3 Storage  │       │
│  │  + JuiceFS   │                   │  (Models)    │       │
│  └──────────────┘                   └──────────────┘       │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Detailed Services

### PostgreSQL (Main Database)

**Purpose:** Stores tenants, API keys, nodes, deployments, usage records, billing data

**Connection:**
```bash
psql postgresql://crosslogic:changeme@localhost:5432/crosslogic_iaas
```

**Tables:**
- `tenants` - Multi-tenant organizations
- `api_keys` - Authentication credentials
- `nodes` - GPU node registry
- `deployments` - Model deployments with auto-scaling
- `usage_records` - Token usage tracking
- `billing_events` - Stripe integration

**Migrations:** Automatically applied on first start via `/docker-entrypoint-initdb.d/`

### PostgreSQL (SkyPilot State)

**Purpose:** Stores SkyPilot cluster state for reconciliation

**Connection:**
```bash
psql postgresql://crosslogic:changeme@localhost:5433/skypilot_state
```

**Note:** This is used by the State Reconciler to track SkyPilot clusters and prevent orphans.

### Redis

**Purpose:**
1. API response caching
2. Rate limiting
3. JuiceFS metadata store

**Connection:**
```bash
redis-cli -h localhost -p 6379
```

**Configuration:**
- Max memory: 1GB with LRU eviction
- Persistence: AOF (Append-Only File) enabled
- Database 0: Cache + Rate Limiting
- Database 1: JuiceFS Metadata

### MinIO (S3 Storage)

**Purpose:** S3-compatible object storage for model weights via JuiceFS

**Web Console:** http://localhost:9001
**API Endpoint:** http://localhost:9000

**Default Credentials:**
- Access Key: `minioadmin`
- Secret Key: `minioadmin123`

**Buckets:**
- `crosslogic-models` - Model weights and checkpoints

**Usage Example:**
```bash
# Upload a model to MinIO (via mc cli)
docker exec crosslogic-minio mc cp ./models/llama-7b.bin local/crosslogic-models/llama-7b.bin
```

### Control Plane

**Purpose:** Main API server handling:
- Tenant management
- API key authentication
- Node orchestration via SkyPilot
- Load balancing
- Billing integration

**Endpoints:**
- Health: http://localhost:8080/health
- Metrics: http://localhost:9090/metrics
- API Docs: http://localhost:8080/docs (if enabled)

## Development Workflow

### 1. Making Code Changes

When you modify Go code:

```bash
# Rebuild and restart control plane
docker-compose up -d --build control-plane

# View logs
docker-compose logs -f control-plane
```

### 2. Database Migrations

To apply new migrations:

```bash
# Connect to PostgreSQL container
docker exec -it crosslogic-postgres psql -U crosslogic -d crosslogic_iaas

# Run migration SQL
\i /docker-entrypoint-initdb.d/02_new_migration.sql
```

Or use the migration script:

```bash
# Copy to container and run
docker cp database/schemas/02_new_migration.sql crosslogic-postgres:/tmp/
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -f /tmp/02_new_migration.sql
```

### 3. Viewing Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f control-plane

# Last 100 lines
docker-compose logs --tail=100 control-plane
```

### 4. Debugging

**Database Queries:**
```bash
# Execute SQL directly
docker exec crosslogic-postgres psql -U crosslogic -d crosslogic_iaas -c "SELECT * FROM nodes;"
```

**Redis Keys:**
```bash
# List all keys
docker exec crosslogic-redis redis-cli KEYS "*"

# Get specific key
docker exec crosslogic-redis redis-cli GET "api_key:sk_test_xxx"
```

**MinIO Files:**
```bash
# List files in bucket
docker exec crosslogic-minio mc ls local/crosslogic-models
```

## Testing JuiceFS Setup

Verify JuiceFS can mount the model storage:

```bash
# Install JuiceFS locally (macOS)
brew install juicefs

# Or Linux
curl -sSL https://d.juicefs.com/install | sh -

# Format filesystem (one-time setup)
juicefs format \
  --storage minio \
  --bucket http://localhost:9000/crosslogic-models \
  --access-key minioadmin \
  --secret-key minioadmin123 \
  redis://localhost:6379/1 \
  crosslogic-models

# Mount filesystem
mkdir -p /tmp/juicefs-test
juicefs mount crosslogic-models /tmp/juicefs-test \
  --cache-dir /tmp/juicefs-cache \
  --cache-size 500 \
  --background

# Test by uploading a model
echo "test model" > /tmp/juicefs-test/test-model.bin

# Verify in MinIO
docker exec crosslogic-minio mc ls local/crosslogic-models/

# Unmount
juicefs umount /tmp/juicefs-test
```

## Monitoring with Prometheus + Grafana

### Enable Monitoring Stack

The monitoring services use Docker Compose profiles:

```bash
# Start with monitoring
docker-compose --profile monitoring up -d
```

### Access Metrics

**Prometheus:** http://localhost:9091
- Query metrics from Control Plane
- View targets and alerts

**Grafana:** http://localhost:3000
- Default credentials: admin/admin
- Add Prometheus data source: http://prometheus:9090

### Key Metrics

Control Plane exposes metrics on `:9090/metrics`:
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency
- `node_status` - GPU node health
- `deployment_replicas` - Current replica counts

## Cleanup

### Stop Services (keep data)

```bash
docker-compose down
```

### Stop and Remove All Data

```bash
docker-compose down -v
```

This removes:
- All containers
- All volumes (databases, models, cache)
- Network

### Reset Everything

```bash
# Stop and remove everything
docker-compose down -v

# Remove Docker images
docker rmi $(docker images -q crosslogic*)

# Fresh start
docker-compose up -d --build
```

## Troubleshooting

### Services Won't Start

**Check Docker Resources:**
```bash
docker info | grep -E 'CPUs|Memory'
```

Ensure at least:
- 4 CPUs
- 8GB Memory

**Check Port Conflicts:**
```bash
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis
lsof -i :8080  # Control Plane
lsof -i :9000  # MinIO
```

### Database Connection Errors

**Check PostgreSQL is healthy:**
```bash
docker-compose ps postgres
docker-compose logs postgres
```

**Test connection:**
```bash
docker exec crosslogic-postgres pg_isready -U crosslogic
```

### MinIO Not Accessible

**Check MinIO health:**
```bash
docker exec crosslogic-minio curl -f http://localhost:9000/minio/health/live
```

**Recreate bucket:**
```bash
docker exec crosslogic-minio mc mb local/crosslogic-models --ignore-existing
```

### Control Plane Crashes

**Check logs:**
```bash
docker-compose logs control-plane | tail -50
```

**Common issues:**
1. Missing environment variables - Check `.env` file
2. Database not ready - Wait for `(healthy)` status
3. Invalid Stripe keys - Use test mode keys

## Production Deployment

⚠️ **This Docker Compose setup is for local development only.**

For production:
- Use managed PostgreSQL (AWS RDS, Google Cloud SQL)
- Use managed Redis (AWS ElastiCache, Redis Cloud)
- Use real S3 or equivalent (not MinIO)
- Enable SSL/TLS
- Use production-grade secrets management
- Set up proper monitoring and alerting
- Enable auto-scaling

See [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md) for full production setup.

## FAQ

**Q: Can I use this with Apple Silicon (M1/M2)?**
A: Yes, all images support ARM64 architecture. Docker Desktop will automatically use the correct architecture.

**Q: How much disk space do I need?**
A: Minimum 20GB. Model storage via MinIO can grow significantly.

**Q: Can I connect from outside Docker network?**
A: Yes, all ports are exposed on `localhost`. Use the port mappings defined in docker-compose.yml.

**Q: How do I backup my data?**
A: See database/README.md for backup procedures. Volumes are persisted in Docker volumes.

**Q: Can I run without MinIO?**
A: Yes, but you'll need to provide S3 credentials in `.env` and remove MinIO from docker-compose.yml dependencies.

## Next Steps

1. **Configure Cloud Credentials:** Set up AWS/GCP/Azure credentials for SkyPilot
2. **Test Node Launch:** Launch a test GPU node to verify orchestration
3. **Set up Monitoring:** Enable Grafana dashboards
4. **Configure Notifications:** Add Discord/Slack webhooks for alerts

See [Getting Started Guide](./docs/GETTING_STARTED.md) for detailed tutorials.
