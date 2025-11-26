# CrossLogic AI IaaS - Internal Architecture Documentation

**CONFIDENTIAL - Internal Use Only**

This document provides a comprehensive overview of the CrossLogic AI infrastructure architecture for internal engineering reference.

---

## System Overview

CrossLogic AI IaaS is a multi-tenant LLM inference platform built on the following principles:

1. **Separation of Control and Data Planes** - Control plane manages state; data plane handles inference
2. **Stateless Workers, Stateful Control** - GPU nodes are ephemeral; control plane maintains state
3. **Redis for Speed, PostgreSQL for Truth** - Hot data in Redis, persistent state in PostgreSQL
4. **Treat GPU Nodes as Cattle** - Nodes are replaceable and should be treated as ephemeral

---

## Architecture Diagram

```
                                     Load Balancer (TLS termination)
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              CONTROL PLANE (Go)                                      │
│                                                                                      │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐  ┌──────────────────────────┐  │
│  │ API Gateway │──│ Authenticator│──│Rate Limiter │──│ Intelligent Load Balancer│  │
│  │ (Chi Router)│  │ (SHA-256)    │  │ (Redis)     │  │ (Round Robin/Latency)    │  │
│  └─────────────┘  └──────────────┘  └─────────────┘  └──────────────────────────┘  │
│         │                                                         │                 │
│         ▼                                                         ▼                 │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐  ┌──────────────────────────┐  │
│  │ Orchestrator│  │Triple Safety │  │   Billing   │  │    Notification          │  │
│  │ (SkyPilot)  │  │   Monitor    │  │   Engine    │  │    Dispatcher            │  │
│  └─────────────┘  └──────────────┘  └─────────────┘  └──────────────────────────┘  │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
         │                │                    │                      │
         │                │                    │                      │
         ▼                ▼                    ▼                      ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  ┌──────────────────────┐
│  PostgreSQL  │  │    Redis     │  │      Stripe      │  │  Webhook Endpoints   │
│  (Persistent │  │   (Cache,    │  │  (Usage Records, │  │  (Discord, Slack,    │
│    State)    │  │ Rate Limits) │  │    Invoicing)    │  │   Email, Generic)    │
└──────────────┘  └──────────────┘  └──────────────────┘  └──────────────────────┘
         │
         └────────────────────────────────────┐
                                              │
                          ┌───────────────────┴───────────────────┐
                          │         DATA PLANE (GPU Nodes)         │
                          │                                        │
                          │  ┌──────────────────────────────────┐  │
                          │  │        Node Agent (Go)           │  │
                          │  │  - Registration                  │  │
                          │  │  - Heartbeat (every 15s)         │  │
                          │  │  - Health Reporting              │  │
                          │  │  - Spot Termination Detection    │  │
                          │  └──────────────────────────────────┘  │
                          │                  │                     │
                          │                  ▼                     │
                          │  ┌──────────────────────────────────┐  │
                          │  │          vLLM Runtime            │  │
                          │  │  - Model Loading (from R2)       │  │
                          │  │  - Inference Serving             │  │
                          │  │  - OpenAI-compatible API         │  │
                          │  └──────────────────────────────────┘  │
                          │                                        │
                          └────────────────────────────────────────┘
```

---

## Component Details

### 1. API Gateway (`control-plane/internal/gateway/`)

The API Gateway handles all incoming HTTP requests.

**Key Files:**
- `gateway.go` - Route setup, main handler logic
- `auth.go` - API key validation and caching
- `ratelimit.go` - Multi-layer rate limiting
- `security_middleware.go` - Security headers, request validation
- `metrics.go` - Prometheus metrics registration
- `loadbalancer.go` - Intelligent endpoint selection

**Middleware Stack (in order):**
1. Security Headers (HSTS, CSP, X-Frame-Options)
2. API Security (Content-Type validation, size limits)
3. Request Size Limit (10MB default)
4. Request ID Generation (chi middleware)
5. Real IP Extraction
6. Request ID Response Header
7. Request Logging
8. Metrics Collection
9. Panic Recovery
10. Timeout (60s default)
11. CORS

**Authentication Flow:**
```
Request → Extract Bearer Token → SHA-256 Hash → Cache Lookup → DB Lookup → Validate Status → Context Injection
```

**Rate Limiting Layers:**
1. API Key level (configurable per key)
2. Environment level (default: 10,000 req/min)
3. Tenant level (default: 50,000 req/min)
4. Global level (configurable)

### 2. Orchestrator (`control-plane/internal/orchestrator/`)

Manages GPU node lifecycle using SkyPilot.

**Key Files:**
- `skypilot.go` - SkyPilot task generation and execution
- `monitor.go` - Triple Safety Monitor for node health

**Node Launch Flow:**
```
1. Generate SkyPilot YAML task file
2. Execute `sky launch` command
3. Wait for node registration
4. Add to load balancer pool
5. Mark node as active
```

**Node Termination Flow:**
```
1. Mark node as draining
2. Remove from load balancer
3. Execute `sky down` command
4. Update node status to terminated
5. Publish termination event
```

**Triple Safety Monitor:**
- Tracks heartbeats from all nodes
- Detects stale nodes (no heartbeat > 60s)
- Handles spot termination warnings
- Triggers automatic recovery

### 3. Billing Engine (`control-plane/internal/billing/`)

Handles usage tracking and Stripe integration.

**Key Files:**
- `stripe.go` - Stripe API integration
- `usage.go` - Usage record aggregation
- `webhook.go` - Stripe webhook handling

**Usage Flow:**
```
1. Request completes with token counts
2. Usage record inserted to PostgreSQL
3. Background job aggregates hourly
4. Hourly aggregates pushed to Stripe
5. Stripe generates invoices
```

**Cost Calculation:**
```
cost = (prompt_tokens * input_price + completion_tokens * output_price) * region_multiplier
```

### 4. Node Agent (`node-agent/`)

Lightweight Go binary running on each GPU node.

**Responsibilities:**
- Register with control plane on startup
- Send heartbeat every 15 seconds
- Report health metrics
- Detect spot termination signals
- Graceful shutdown handling

**Registration Payload:**
```json
{
  "cluster_name": "crosslogic-llama-8b-us-east-1",
  "provider": "aws",
  "region": "us-east-1",
  "instance_type": "g5.2xlarge",
  "gpu_type": "A10G",
  "vram_total_gb": 24,
  "model_name": "meta-llama/Llama-3.1-8B-Instruct",
  "endpoint_url": "http://10.0.1.45:8000",
  "spot_instance": true
}
```

### 5. Load Balancer (`control-plane/internal/gateway/loadbalancer.go`)

Intelligent request routing to GPU nodes.

**Strategies:**
1. **Round Robin** (default) - Equal distribution
2. **Least Connections** - Route to least busy node
3. **Least Latency** - Route to fastest responding node

**Selection Algorithm:**
```
1. Filter nodes by model
2. Filter nodes by status (active only)
3. Filter by health score (> threshold)
4. Apply routing strategy
5. Record metrics
```

**Circuit Breaking:**
- Track error rates per node
- Remove nodes with >50% error rate
- Reintroduce after cool-down period

---

## Data Flow

### Inference Request Flow

```
┌────────┐     ┌─────────┐     ┌───────────┐     ┌────────────┐     ┌──────────┐
│ Client │────▶│ Gateway │────▶│ Auth/Rate │────▶│ Load       │────▶│ GPU Node │
│        │     │         │     │ Limiter   │     │ Balancer   │     │ (vLLM)   │
└────────┘     └─────────┘     └───────────┘     └────────────┘     └──────────┘
    ▲                                                                     │
    │                                                                     │
    └─────────────────────────────────────────────────────────────────────┘
                              Response + Usage Metrics
```

### Data Storage Flow

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        PostgreSQL (Primary State)                         │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   tenants   │  │   models    │  │    nodes    │  │  usage_records  │ │
│  │             │  │             │  │             │  │                 │ │
│  │ - id        │  │ - id        │  │ - id        │  │ - id            │ │
│  │ - name      │  │ - name      │  │ - cluster_  │  │ - tenant_id     │ │
│  │ - status    │  │ - family    │  │   name      │  │ - tokens        │ │
│  │ - stripe_id │  │ - vram_gb   │  │ - status    │  │ - cost          │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │ environments│  │  api_keys   │  │ deployments │  │ billing_events  │ │
│  │             │  │             │  │             │  │                 │ │
│  │ - id        │  │ - id        │  │ - id        │  │ - id            │ │
│  │ - tenant_id │  │ - key_hash  │  │ - model_id  │  │ - tenant_id     │ │
│  │ - region    │  │ - tenant_id │  │ - min_nodes │  │ - event_type    │ │
│  │ - quota     │  │ - role      │  │ - max_nodes │  │ - stripe_id     │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────┘ │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                        Redis (Hot Data / Cache)                           │
│                                                                          │
│  Rate Limiting:                  Caching:                                │
│  - ratelimit:key:{id}:minute     - api_key:{hash}                        │
│  - ratelimit:env:{id}:minute     - tenant:{id}                           │
│  - ratelimit:tenant:{id}:minute  - model:{id}                            │
│  - ratelimit:key:{id}:concurrency                                        │
│                                                                          │
│  Token Tracking:                 Node Health:                            │
│  - tokens:key:{id}:minute        - node:{id}:heartbeat                   │
│  - tokens:key:{id}:day           - node:{id}:health_score                │
│  - tokens:env:{id}:day                                                   │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Security Architecture

### Authentication

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     API Key Authentication Flow                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Client sends: Authorization: Bearer clsk_live_xxxxxxxxxxxx          │
│                                                                          │
│  2. Gateway extracts key, computes SHA-256 hash                         │
│                                                                          │
│  3. Check Redis cache: api_key:{hash}                                   │
│     - Hit: Return cached key info                                       │
│     - Miss: Query PostgreSQL                                            │
│                                                                          │
│  4. Validate key status, expiration, tenant status                      │
│                                                                          │
│  5. Cache key info in Redis (60s TTL)                                   │
│                                                                          │
│  6. Inject tenant_id, environment_id into request context               │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Admin Authentication

Admin endpoints use `X-Admin-Token` header with constant-time comparison:

```go
subtle.ConstantTimeCompare([]byte(adminToken), []byte(g.adminToken))
```

### Security Headers

All responses include:
- `Strict-Transport-Security: max-age=31536000; includeSubDomains; preload`
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- `Content-Security-Policy: default-src 'self'; frame-ancestors 'none'`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: geolocation=(), microphone=(), camera=()`

### Input Validation

- Request body size limited to 10MB
- Content-Type validation for POST/PUT/PATCH
- JSON schema validation on request bodies
- SQL injection prevention via parameterized queries
- XSS prevention via JSON responses (no HTML rendering)

---

## Deployment Architecture

### Kubernetes Deployment

```yaml
Namespace: crosslogic-prod
│
├── control-plane-deployment (3 replicas)
│   ├── Container: control-plane
│   │   - Port: 8080 (API)
│   │   - Port: 9090 (Metrics)
│   │   - Resources: 2 CPU, 4Gi RAM
│   │   - Probes: /health, /ready
│   └── ServiceAccount: control-plane-sa
│
├── postgres-statefulset (1 replica, HA optional)
│   ├── Container: postgres:16
│   │   - Port: 5432
│   │   - PVC: 100Gi
│   └── Secret: postgres-credentials
│
├── redis-deployment (1 replica, cluster optional)
│   ├── Container: redis:7
│   │   - Port: 6379
│   └── ConfigMap: redis-config
│
├── Services
│   ├── control-plane-service (ClusterIP)
│   ├── control-plane-external (LoadBalancer)
│   ├── postgres-service (ClusterIP)
│   └── redis-service (ClusterIP)
│
└── Ingress
    ├── Host: api.crosslogic.ai
    └── TLS: Certificate from cert-manager
```

### Environment Variables

```bash
# Database
DATABASE_URL=postgres://user:pass@host:5432/crosslogic
DB_MAX_CONNECTIONS=50
DB_MIN_CONNECTIONS=10

# Redis
REDIS_URL=redis://host:6379
REDIS_PASSWORD=secret

# Authentication
ADMIN_API_TOKEN=secure-admin-token

# Billing
STRIPE_SECRET_KEY=sk_live_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
STRIPE_PRICE_ID_USAGE=price_xxx

# Cloud
R2_ACCOUNT_ID=xxx
R2_ACCESS_KEY_ID=xxx
R2_SECRET_ACCESS_KEY=xxx
R2_BUCKET_NAME=crosslogic-models

# Monitoring
PROMETHEUS_ENABLED=true
LOG_LEVEL=info
```

---

## Monitoring & Observability

### Prometheus Metrics

```
# API Metrics
http_requests_total{method, path, status}
http_request_duration_seconds{method, path}
http_request_size_bytes{method, path}
http_response_size_bytes{method, path}

# Rate Limiting
rate_limit_exceeded_total{key_id, reason}
rate_limit_remaining{key_id}

# Authentication
auth_attempts_total{status}
auth_failures_total{reason}

# Load Balancer
loadbalancer_requests_total{model, node}
loadbalancer_latency_seconds{model, node}
loadbalancer_errors_total{model, node, reason}

# Node Health
node_health_score{node_id}
node_heartbeat_age_seconds{node_id}
nodes_active_total
nodes_draining_total

# Billing
tokens_processed_total{model, tenant_id}
usage_records_created_total
billing_exports_total{status}
```

### Alerting Rules

```yaml
groups:
  - name: crosslogic
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m

      - alert: NodeUnhealthy
        expr: node_health_score < 50
        for: 2m

      - alert: NoActiveNodes
        expr: nodes_active_total == 0
        for: 1m
        severity: critical

      - alert: HighLatency
        expr: histogram_quantile(0.95, http_request_duration_seconds) > 30
        for: 5m
```

---

## Disaster Recovery

### Backup Strategy

| Component | Frequency | Retention | Method |
|-----------|-----------|-----------|--------|
| PostgreSQL | Hourly | 30 days | pg_dump + S3 |
| Redis | N/A | N/A | Ephemeral |
| Models | N/A | N/A | Stored in R2 |
| Configs | On change | 90 days | Git |

### Recovery Procedures

See: [TROUBLESHOOTING_RUNBOOK.md](./TROUBLESHOOTING_RUNBOOK.md)

---

## Performance Tuning

### PostgreSQL

```sql
-- Connection pooling
max_connections = 200
shared_buffers = 4GB
effective_cache_size = 12GB
work_mem = 256MB

-- Indexing
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_usage_records_tenant ON usage_records(tenant_id, timestamp);
CREATE INDEX idx_nodes_status ON nodes(status) WHERE status = 'active';
```

### Redis

```
maxmemory 2gb
maxmemory-policy allkeys-lru
```

### Go Application

```go
// HTTP client pooling
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}
```

---

## Future Architecture

### Planned Improvements

1. **Multi-Region Control Plane** - Deploy control plane replicas per region
2. **Distributed Scheduler** - Shard scheduler by region using etcd
3. **Event Sourcing** - Replace direct DB writes with event log
4. **gRPC Internal API** - Replace HTTP for node-agent communication
5. **Model Auto-Migration** - ML-based regional load balancing

### Scaling Milestones

| Customers | Architecture Changes |
|-----------|---------------------|
| 0-100 | Single control plane, single region |
| 100-500 | HA Postgres, Redis cluster |
| 500-2000 | Multi-region, regional schedulers |
| 2000+ | Full event sourcing, global load balancing |

---

## References

- [Control Plane Architecture PRD](../../PRD/Control-plane.md)
- [Database Schema](../database-schema.md)
- [Deployment Guide](../deployment/deployment-guide.md)
- [Security Documentation](../security.md)
