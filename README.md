# CrossLogic Inference Cloud (CIC)

**Multi-region, spot-GPU-powered LLM inference platform with OpenAI-compatible APIs**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

## 📋 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Deployment](#deployment)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Documentation](#documentation)
- [Development](#development)
- [Production Checklist](#production-checklist)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## 🎯 Overview

CrossLogic Inference Cloud (CIC) is a complete inference platform that makes LLM hosting **10x cheaper** through multi-cloud spot arbitrage. It provides:

- **OpenAI-compatible APIs** - Drop-in replacement for OpenAI
- **Multi-region deployment** - India, US, EU, APAC
- **Spot GPU optimization** - 70-90% cost savings
- **Multi-tenancy** - Isolated orgs, environments, API keys
- **Usage-based billing** - Stripe integration
- **On-premise support** - Hybrid cloud deployment

### Target Users

1. **Startups & Developers** - Cheap, fast inference with predictable pricing
2. **Enterprises** - Air-gapped on-premise deployment with cloud control plane
3. **Platform Builders** - White-label LLM inference infrastructure

## ✨ Features

### Core Features

- ✅ **OpenAI-Compatible API** - `/v1/chat/completions`, `/v1/embeddings`
- ✅ **Multi-Region Routing** - Automatic region selection and failover
- ✅ **Rate Limiting** - 4-layer protection (global, tenant, env, key)
- ✅ **Token Accounting** - Accurate billing per token
- ✅ **Stripe Billing** - Automated metered billing
- ✅ **Spot Instance Management** - Auto-recovery from interruptions
- ✅ **Health Monitoring** - Real-time node health tracking
- ✅ **Reserved Capacity** - Guaranteed tokens/sec for enterprises

### Supported Models

- **Llama 3** (8B, 70B)
- **Mistral** (7B, 8x7B)
- **Qwen 2.5** (7B, 72B)
- **Gemma** (7B)
- **Phi 3** (Mini, Medium)
- **Custom Models** - Easy to add new models

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     User Applications                        │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                    CloudFlare CDN                            │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Control Plane (Go Binary)                       │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐        │
│  │ API Gateway  │ │  Scheduler   │ │   Billing    │        │
│  └──────────────┘ └──────────────┘ └──────────────┘        │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐        │
│  │ Rate Limiter │ │ Node Registry│ │Token Accountant│      │
│  └──────────────┘ └──────────────┘ └──────────────┘        │
└────────────┬────────────────────────────────────────────────┘
             │
    ┌────────┼────────┐
    │        │        │
    ▼        ▼        ▼
┌────────┐ ┌────────┐ ┌────────┐
│AWS GPU │ │GCP GPU │ │Azure   │
│Nodes   │ │Nodes   │ │GPU     │
│(vLLM)  │ │(vLLM)  │ │Nodes   │
└────────┘ └────────┘ └────────┘

Storage Layer:
┌──────────────┐  ┌──────────────┐
│ PostgreSQL   │  │    Redis     │
│ (Metadata)   │  │ (Rate Limits)│
└──────────────┘  └──────────────┘
```

### Key Components

1. **Control Plane** - Central orchestration (Go)
2. **Node Agent** - Runs on each GPU worker (Go)
3. **Database** - PostgreSQL for persistence
4. **Cache** - Redis for rate limiting
5. **Billing** - Stripe integration
6. **Orchestration** - SkyPilot for GPU provisioning

## 🚀 Quick Start

### Prerequisites

- Docker **24+** & Docker Compose **v2**
- Stripe account (test mode is fine)
- Hugging Face account/token (to pull open models)
- GPU access (local CUDA host or any SkyPilot-supported cloud)

### 1. Clone Repository

```bash
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas
```

### 2. Configure Environment

```bash
cp config/.env.example .env

# Required minimum (edit .env)
BILLING_ENABLED=false  # disable Stripe while iterating locally; set true in staging/prod
DB_PASSWORD=supersecure
STRIPE_SECRET_KEY=sk_test_xxx
STRIPE_WEBHOOK_SECRET=whsec_xxx
ADMIN_API_TOKEN=cl_admin_$(openssl rand -hex 16)
VLLM_VERSION=0.6.2
TORCH_VERSION=2.4.0

# Optional: dashboard + auth
CROSSLOGIC_API_BASE_URL=http://localhost:8080
CROSSLOGIC_ADMIN_TOKEN=$ADMIN_API_TOKEN
CROSSLOGIC_DASHBOARD_TENANT_ID=<tenant-uuid-after-seeding>
```

### 3. Build & Start Services (Docker end-to-end)

```bash
# Build Go services
docker build -f Dockerfile.control-plane -t crosslogic/control-plane:dev .
docker build -f Dockerfile.node-agent -t crosslogic/node-agent:dev .

# Start infra + control plane
docker compose up -d postgres redis
docker compose run --rm postgres \
  psql -U crosslogic -d crosslogic_iaas \
  -f /docker-entrypoint-initdb.d/01_core_tables.sql

docker compose up -d control-plane

# Dashboard (Next.js) is optional but recommended
pushd control-plane/dashboard
npm install
npm run dev # or npm run build && npm start for prod
popd
```

> **First execution tip:** wait for `control-plane` logs to show `server exited` messages? Use `docker compose logs -f control-plane` and confirm DB/cache health checks succeed.

### 4. Seed Tenant, Environment & API Key

```bash
docker compose exec postgres psql -U crosslogic crosslogic_iaas <<'SQL'
INSERT INTO tenants (id, name, email, status, billing_plan)
VALUES (gen_random_uuid(), 'Demo Org', 'demo@example.com', 'active', 'serverless')
RETURNING id\gset

INSERT INTO environments (id, tenant_id, name, region, status)
VALUES (gen_random_uuid(), :'id', 'production', 'us-east', 'active')
RETURNING id\gset

-- Minimal llama-3-8b model seed
INSERT INTO models (id, name, family, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million)
VALUES (gen_random_uuid(), 'llama-3-8b', 'Llama', 'chat', 8192, 16, 0.05, 0.05);
SQL

# Create API key via admin endpoint
curl -X POST http://localhost:8080/admin/api-keys \
  -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"<TENANT_UUID>","environment_id":"<ENV_UUID>","name":"prod-key"}'
```

### 5. Smoke Test the OpenAI-compatible API

```bash
export API_KEY="clsk_live_..."   # value returned above
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-8b",
    "messages": [{"role":"user","content":"Hello CrossLogic!"}],
    "max_tokens": 64
  }'
```

If you receive `no available nodes`, proceed to the [launch your first GPU node](#launch-a-1-billion-parameter-llm) section.

### 6. Dashboard Login (Optional)

1. Export `CROSSLOGIC_API_BASE_URL` & `CROSSLOGIC_ADMIN_TOKEN` in `.env.local`.
2. `npm run dev` inside `control-plane/dashboard`.
3. Visit `http://localhost:3000` → sign in via Google or email (NextAuth magic link).

All cards use the admin API (`/admin/nodes`, `/admin/api-keys`, `/admin/usage/{tenant_id}`) and fall back to mock data if the control plane is unreachable.

---

## 🐳 Full Docker + Services Runbook

| Component            | Command                                                  |
|----------------------|----------------------------------------------------------|
| PostgreSQL + Redis   | `docker compose up -d postgres redis`                    |
| Control Plane API    | `docker compose up -d control-plane`                     |
| Node Agent (GPU)     | `docker run --gpus all crosslogic/node-agent:dev ...`    |
| Dashboard (Next.js)  | `npm run dev` inside `control-plane/dashboard/`          |
| Background billing   | auto-starts inside control-plane container               |
| Stripe webhooks      | expose `http://localhost:8080/api/webhooks/stripe` via ngrok/Cloudflare and set the signing secret in `.env`. |

To stop everything: `docker compose down` (append `-v` to wipe volumes).

---

## 🚀 Launch a 1 Billion Parameter LLM

The stack ships with popular models, but you can bring any Hugging Face checkpoint. Below is a tested recipe for **TinyLlama/TinyLlama-1.1B-Chat-v1.0** (~1B parameters).

### 1. Register the Model

```sql
INSERT INTO models (
    id, name, family, size, type, context_length,
    vram_required_gb, price_input_per_million, price_output_per_million, status
) VALUES (
    gen_random_uuid(),
    'TinyLlama/TinyLlama-1.1B-Chat-v1.0',
    'TinyLlama',
    '1.1B',
    'chat',
    4096,
    8,
    0.02,
    0.02,
    'active'
);
```

### 2. Start a GPU Worker (local or cloud)

```bash
docker run --gpus all --rm \
  -e CONTROL_PLANE_URL=http://host.docker.internal:8080 \
  -e NODE_ID=$(uuidgen) \
  -e MODEL_NAME="TinyLlama/TinyLlama-1.1B-Chat-v1.0" \
  -e REGION=us-east \
  -e PROVIDER=local \
  -e VLLM_ENDPOINT=http://localhost:8000 \
  -e HF_TOKEN=$HUGGINGFACE_TOKEN \
  crosslogic/node-agent:dev
```

> Alternatively, use SkyPilot to launch an A10G instance. The orchestrator now pins `vLLM`/`torch` versions via `VLLM_VERSION` and `TORCH_VERSION` env vars.

### 3. Update Scheduler Node Pool

The node agent registers itself automatically. Verify:

```bash
curl -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  http://localhost:8080/admin/nodes | jq
```

You should see your TinyLlama node in `active` status with a healthy score.

### 4. Run Inference

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
    "messages": [{"role":"user","content":"Summarize CrossLogic in 2 sentences."}],
    "max_tokens": 128
  }'
```

Streaming responses are fully metered: the control plane now parses SSE usage events emitted by vLLM and writes them to `usage_records`, ensuring billing works for both streaming and non-streaming calls.

---

## 📦 Deployment

### Local Development

```bash
# Start dependencies only
docker-compose up postgres redis

# Run control plane locally
cd control-plane
export DB_PASSWORD=changeme
export STRIPE_SECRET_KEY=sk_test_...
go run cmd/server/main.go

# In another terminal, run node agent
cd node-agent
export CONTROL_PLANE_URL=http://localhost:8080
export MODEL_NAME=llama-3-8b
go run cmd/main.go
```

### Docker Deployment

```bash
# Build images
docker build -f Dockerfile.control-plane -t crosslogic-control-plane:latest .
docker build -f Dockerfile.node-agent -t crosslogic-node-agent:latest .

# Run with docker-compose
docker-compose up -d
```

### Production Deployment

#### Option 1: Cloud VM Deployment

```bash
# 1. Provision VM (Ubuntu 22.04, 4 vCPUs, 16GB RAM)

# 2. Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 3. Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# 4. Clone repo and deploy
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas
cp config/.env.example .env
# Edit .env with production values
docker-compose up -d

# 5. Set up TLS (use Let's Encrypt)
# 6. Configure CloudFlare or load balancer
```

#### Option 2: Kubernetes Deployment

```bash
# Coming soon - Kubernetes manifests
# Will include:
# - Deployment for control plane
# - StatefulSet for PostgreSQL
# - Deployment for Redis
# - ConfigMaps and Secrets
# - HorizontalPodAutoscaler
# - Ingress configuration
```

#### Option 3: Managed Services

Use managed services for production:

- **Database**: AWS RDS PostgreSQL, GCP Cloud SQL, or Azure Database
- **Cache**: AWS ElastiCache Redis, GCP Memorystore, or Azure Cache
- **Compute**: AWS ECS/EKS, GCP Cloud Run/GKE, or Azure Container Instances
- **Load Balancer**: AWS ALB, GCP Load Balancer, or Azure Application Gateway
- **Monitoring**: Datadog, New Relic, or Prometheus + Grafana

### GPU Node Deployment (SkyPilot)

```bash
# Install SkyPilot
pip install skypilot[aws,gcp,azure]

# Configure cloud credentials
sky check

# Deploy GPU node
cat > gpu-node.yaml <<EOF
resources:
  accelerators: A10G:1
  cloud: aws
  region: us-east-1

setup: |
  pip install vllm
  wget https://releases.crosslogic.ai/node-agent
  chmod +x node-agent

run: |
  # Start vLLM
  python -m vllm.entrypoints.openai.api_server \
    --model meta-llama/Llama-3-8B \
    --port 8000 &

  # Start node agent
  export CONTROL_PLANE_URL=https://api.crosslogic.ai
  export MODEL_NAME=llama-3-8b
  ./node-agent
EOF

sky launch -c llama-node gpu-node.yaml
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `SERVER_HOST` | Server bind host | `0.0.0.0` | No |
| `SERVER_PORT` | Server port | `8080` | No |
| `DB_HOST` | PostgreSQL host | `localhost` | Yes |
| `DB_PORT` | PostgreSQL port | `5432` | No |
| `DB_USER` | Database user | `crosslogic` | Yes |
| `DB_PASSWORD` | Database password | - | **Yes** |
| `DB_NAME` | Database name | `crosslogic_iaas` | Yes |
| `REDIS_HOST` | Redis host | `localhost` | Yes |
| `REDIS_PORT` | Redis port | `6379` | No |
| `STRIPE_SECRET_KEY` | Stripe API key | - | **Yes** |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook secret | - | **Yes** |
| `LOG_LEVEL` | Logging level | `info` | No |

See `config/.env.example` for complete list.

### Database Configuration

PostgreSQL settings in `.env`:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=your_secure_password
DB_NAME=crosslogic_iaas
DB_SSL_MODE=require  # For production
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
```

### Redis Configuration

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=  # Optional
REDIS_DB=0
REDIS_POOL_SIZE=10
```

### Billing Configuration

```bash
STRIPE_SECRET_KEY=sk_live_...  # Use sk_test_... for testing
STRIPE_WEBHOOK_SECRET=whsec_...
BILLING_AGGREGATION_INTERVAL=1h
BILLING_EXPORT_INTERVAL=5m
```

## 📚 API Reference

### Authentication

All API requests require authentication via API key:

```http
Authorization: Bearer clsk_live_a4f5b2c8d9e0f1g2h3i4j5k6l7m8n9o0
```

### Endpoints

#### List Models

```http
GET /v1/models
```

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "llama-3-8b",
      "object": "model",
      "created": 1704067200,
      "owned_by": "crosslogic"
    }
  ]
}
```

#### Chat Completions

```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "llama-3-8b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 100
}
```

Response:

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1704067200,
  "model": "llama-3-8b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

#### Embeddings

```http
POST /v1/embeddings
Content-Type: application/json

{
  "model": "text-embedding-ada-002",
  "input": "The quick brown fox"
}
```

### Rate Limits

Rate limits are enforced at multiple levels:

- **Free tier**: 60 requests/minute, 100K tokens/day
- **Developer**: 600 requests/minute, 10M tokens/day
- **Business**: 6000 requests/minute, 100M tokens/day
- **Enterprise**: Custom limits

## 📖 Documentation

Comprehensive documentation is available in the `docs/` directory:

- [Control Plane Architecture](docs/components/control-plane.md)
- [Node Agent Guide](docs/components/node-agent.md)
- [API Gateway](docs/components/api-gateway.md)
- [Scheduler](docs/components/scheduler.md)
- [Billing Engine](docs/components/billing-engine.md)
- [Database Schema](docs/database-schema.md)
- [Deployment Guide](docs/deployment/deployment-guide.md)
- [Monitoring & Observability](docs/monitoring.md)
- [Security Best Practices](docs/security.md)

## 🧪 Testing

### Local Testing

To run tests locally, ensure you have Go installed and the database running:

```bash
# Start dependencies
docker-compose up -d postgres redis

# Run tests
cd control-plane
go test ./...
```

### Docker Testing

To run tests in a consistent Docker environment:

```bash
./tests/docker-test-runner.sh
```


## 🛠️ Development

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- PostgreSQL client tools
- Redis client tools

### Setup Development Environment

```bash
# Clone repository
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas

# Install dependencies
cd control-plane
go mod download

cd ../node-agent
go mod download

# Start development database
docker-compose up -d postgres redis

# Run migrations
psql -h localhost -U crosslogic -d crosslogic_iaas -f database/schemas/01_core_tables.sql
```

### Running Tests

```bash
# Run all tests
cd control-plane
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/gateway

# Run integration tests
go test -tags=integration ./...
```

### Code Structure

```
crosslogic-ai-iaas/
├── control-plane/          # Control Plane (Go)
│   ├── cmd/server/         # Main entry point
│   ├── internal/           # Internal packages
│   │   ├── gateway/        # API Gateway
│   │   ├── scheduler/      # Request scheduler
│   │   ├── billing/        # Billing engine
│   │   └── ...
│   └── pkg/                # Public packages
│       ├── models/         # Data models
│       ├── database/       # DB client
│       └── cache/          # Redis client
├── node-agent/             # Node Agent (Go)
├── database/               # Database schemas
├── docs/                   # Documentation
├── config/                 # Configuration files
└── scripts/                # Deployment scripts
```

## ✅ Production Checklist

Before deploying to production:

### Infrastructure

- [ ] Use managed PostgreSQL (RDS, Cloud SQL, etc.)
- [ ] Use managed Redis (ElastiCache, Memorystore, etc.)
- [ ] Set up load balancer with TLS termination
- [ ] Configure auto-scaling for control plane
- [ ] Set up CloudFlare or CDN

### Security

- [ ] Enable TLS/HTTPS everywhere
- [ ] Rotate database credentials
- [ ] Set strong PostgreSQL password
- [ ] Enable Redis authentication
- [ ] Configure firewall rules
- [ ] Set up VPC/private networking
- [ ] Enable audit logging

### Monitoring

- [ ] Set up Prometheus metrics collection
- [ ] Configure Grafana dashboards
- [ ] Set up alerting (PagerDuty, Slack, etc.)
- [ ] Enable log aggregation (ELK, Datadog, etc.)
- [ ] Monitor billing export job
- [ ] Track node health metrics

### Backup & Recovery

- [ ] Enable PostgreSQL automated backups
- [ ] Test database restore procedure
- [ ] Set up Redis persistence (AOF)
- [ ] Document recovery procedures
- [ ] Test failover scenarios

### Billing

- [ ] Configure Stripe production keys
- [ ] Set up webhook endpoints
- [ ] Test billing flow end-to-end
- [ ] Configure pricing models
- [ ] Set up invoice emails

### Performance

- [ ] Load test control plane
- [ ] Optimize database queries
- [ ] Configure connection pooling
- [ ] Set up caching strategies
- [ ] Test rate limiting

## 🔍 Troubleshooting

### Common Issues

#### Control Plane Won't Start

**Symptom**: Control plane exits immediately

**Solutions**:
```bash
# Check logs
docker-compose logs control-plane

# Verify database connection
docker-compose exec postgres psql -U crosslogic -c "SELECT 1"

# Verify Redis connection
docker-compose exec redis redis-cli ping

# Check environment variables
docker-compose exec control-plane env | grep DB_
```

#### High Latency

**Symptom**: Slow API responses

**Solutions**:
1. Check database query performance
2. Verify Redis is responding quickly
3. Review node health scores
4. Check network latency to GPU nodes

#### Rate Limit Errors

**Symptom**: 429 Too Many Requests

**Solutions**:
1. Check current rate limits
2. Upgrade to higher tier
3. Request limit increase
4. Implement client-side backoff

#### Billing Discrepancies

**Symptom**: Usage not matching Stripe

**Solutions**:
1. Check billing export logs
2. Verify Stripe webhook is working
3. Review usage aggregation query
4. Check for failed exports

### Getting Help

- **Documentation**: Check `docs/` directory
- **GitHub Issues**: https://github.com/crosslogic/crosslogic-ai-iaas/issues
- **Discord**: https://discord.gg/crosslogic
- **Email**: support@crosslogic.ai

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests
5. Run tests (`go test ./...`)
6. Commit (`git commit -m 'Add amazing feature'`)
7. Push (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **SkyPilot** - GPU orchestration
- **vLLM** - Fast LLM inference
- **Stripe** - Payment processing
- **PostgreSQL** - Reliable database
- **Redis** - Fast caching

## 📞 Support

- **Website**: https://crosslogic.ai
- **Email**: support@crosslogic.ai
- **Discord**: https://discord.gg/crosslogic
- **Twitter**: @crosslogic_ai

---

**Built with ❤️ for the LLM community**
