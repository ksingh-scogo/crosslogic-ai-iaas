# Control Plane Documentation

## Overview

The Control Plane is the brain of the CrossLogic Inference Cloud. It orchestrates all inference requests, manages GPU nodes, enforces rate limits, tracks usage, and handles billing.

## Architecture

The Control Plane is implemented as a single Go binary with the following components:

```
control-plane/
├── cmd/server/          # Main entry point
├── internal/
│   ├── gateway/         # API Gateway & Authentication
│   ├── router/          # Request routing logic
│   ├── scheduler/       # Node scheduling & selection
│   ├── allocator/       # Capacity allocation
│   ├── billing/         # Usage tracking & billing
│   ├── monitor/         # Health monitoring
│   └── orchestrator/    # SkyPilot integration
└── pkg/
    ├── models/          # Data models
    ├── database/        # PostgreSQL client
    ├── cache/           # Redis client
    └── telemetry/       # Metrics & logging
```

## Key Responsibilities

### 1. Authentication & Authorization
- Validates API keys
- Resolves tenant/environment/org hierarchy
- Enforces access controls
- Manages key lifecycle (creation, rotation, revocation)

### 2. Rate Limiting
Four layers of rate limiting:
- **Global**: Protects entire system
- **Tenant**: Per-organization limits
- **Environment**: Per dev/staging/prod limits
- **API Key**: Per-key limits

Implementation uses Redis with atomic Lua scripts for accuracy.

### 3. Request Scheduling
- Selects optimal GPU node for each request
- Considers: region, model, load, health score
- Supports multiple strategies:
  - Least Loaded (default)
  - Round Robin
  - Weighted by health
  - Random

### 4. Node Registry
- Tracks all active GPU nodes
- Maintains node metadata (provider, region, model, capacity)
- Performs health checks
- Handles spot interruptions
- Auto-recovers from failures

### 5. Token Accounting
- Counts prompt and completion tokens
- Records usage per tenant/environment/model
- Calculates costs based on pricing rules
- Supports region-specific pricing multipliers

### 6. Billing Engine
- Aggregates usage into hourly buckets
- Exports to Stripe for metered billing
- Handles credits and free tiers
- Supports reserved capacity billing
- Generates usage reports

## Configuration

The Control Plane is configured via environment variables:

```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=<secret>
DB_NAME=crosslogic_iaas

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Billing
STRIPE_SECRET_KEY=sk_live_...
BILLING_EXPORT_INTERVAL=5m
BILLING_AGGREGATION_INTERVAL=1h

# Monitoring
LOG_LEVEL=info
PROMETHEUS_PORT=9090
```

## API Endpoints

### Health Checks
- `GET /health` - Basic health check
- `GET /ready` - Readiness check (includes DB/Redis)

### Inference (OpenAI-compatible)
- `POST /v1/chat/completions` - Chat completions
- `POST /v1/completions` - Text completions
- `POST /v1/embeddings` - Generate embeddings
- `GET /v1/models` - List available models

### Admin
- `GET /admin/nodes` - List GPU nodes
- `GET /admin/usage/{tenant_id}` - Get usage statistics
- `POST /admin/nodes/register` - Register new node
- `POST /admin/nodes/{node_id}/heartbeat` - Node heartbeat

## Database Schema

The Control Plane uses PostgreSQL with the following core tables:

- **tenants**: Organizations
- **environments**: Dev/staging/prod per org
- **api_keys**: Authentication keys
- **regions**: Geographic regions
- **models**: LLM model catalog
- **nodes**: GPU worker nodes
- **usage_records**: Per-request usage
- **usage_hourly**: Aggregated usage
- **billing_events**: Stripe export records

See `database/schemas/01_core_tables.sql` for complete schema.

## Deployment

### Local Development

```bash
# Start dependencies
docker-compose up postgres redis

# Set environment variables
export DB_PASSWORD=changeme
export STRIPE_SECRET_KEY=sk_test_...

# Run control plane
cd control-plane
go run cmd/server/main.go
```

### Docker Deployment

```bash
# Build image
docker build -f Dockerfile.control-plane -t crosslogic-control-plane .

# Run container
docker run -p 8080:8080 \
  -e DB_HOST=postgres \
  -e REDIS_HOST=redis \
  -e STRIPE_SECRET_KEY=sk_live_... \
  crosslogic-control-plane
```

### Production Deployment

For production, use:
- Multiple replicas behind load balancer
- Managed PostgreSQL (RDS, Cloud SQL, etc.)
- Managed Redis (ElastiCache, Cloud Memorystore)
- TLS termination at load balancer
- Prometheus for monitoring

## Monitoring

The Control Plane exposes Prometheus metrics at `/metrics`:

- `control_plane_requests_total` - Total requests
- `control_plane_request_duration_seconds` - Request latency
- `control_plane_active_nodes` - Active GPU nodes
- `control_plane_rate_limit_exceeded_total` - Rate limit violations
- `billing_usage_exported_total` - Usage exported to Stripe

## Troubleshooting

### High Latency
1. Check database connection pool settings
2. Verify Redis is responding quickly
3. Review scheduler strategy
4. Check node health scores

### Rate Limit Issues
1. Check Redis memory usage
2. Verify rate limit configurations
3. Review per-key limits

### Billing Discrepancies
1. Check Stripe export logs
2. Verify usage aggregation
3. Review pricing calculator
4. Check for failed exports

## Security

### API Key Security
- Keys are hashed with SHA-256 before storage
- Never stored in plaintext
- Cached for 60 seconds
- Rotatable without downtime

### Network Security
- HTTPS only (TLS 1.2+)
- No VPN/mesh networking needed
- Direct HTTPS to GPU nodes
- mTLS available for on-prem

### Data Security
- All sensitive data encrypted at rest
- Audit logs for all actions
- GDPR compliance support

## Performance

### Latency Targets
- Authentication: <5ms (cached)
- Rate limiting: <2ms
- Scheduling: <10ms
- Total overhead: <20ms

### Throughput
- 10,000 requests/second per instance
- Horizontally scalable
- No single point of failure

## Best Practices

1. **Always use connection pooling** for database and Redis
2. **Monitor rate limit metrics** to prevent abuse
3. **Set up alerts** for node failures and billing issues
4. **Regular backups** of PostgreSQL
5. **Test failover** scenarios regularly

## See Also

- [API Gateway Documentation](./api-gateway.md)
- [Scheduler Documentation](./scheduler.md)
- [Billing Engine Documentation](./billing-engine.md)
- [Deployment Guide](../deployment/deployment-guide.md)
