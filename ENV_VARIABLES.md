# Environment Variables Reference

Complete reference of all environment variables used in CrossLogic AI IaaS.

## 📋 Quick Reference

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_PASSWORD` | PostgreSQL password | `my_secure_password_123` |
| `ADMIN_API_TOKEN` | Admin authentication token | `$(openssl rand -hex 32)` |
| `JWT_SECRET` | JWT signing secret | `$(openssl rand -hex 32)` |
| `R2_ENDPOINT` | Cloudflare R2 endpoint | `https://abc123.r2.cloudflarestorage.com` |
| `R2_ACCESS_KEY` | R2 access key | From Cloudflare dashboard |
| `R2_SECRET_KEY` | R2 secret key | From Cloudflare dashboard |
| `HUGGINGFACE_TOKEN` | HuggingFace API token | `hf_xxxxxxxxxxxxx` |

### Cloud Provider (At least one required)

| Variable | Provider | Description |
|----------|----------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS | AWS secret key |
| `AZURE_SUBSCRIPTION_ID` | Azure | Azure subscription ID |
| `AZURE_TENANT_ID` | Azure | Azure tenant ID |
| `GCP_PROJECT_ID` | GCP | GCP project ID |

---

## 📚 Complete Variable List

### 🔐 Security

#### DB_PASSWORD (REQUIRED)
- **Description**: PostgreSQL database password
- **Used by**: postgres, control-plane
- **Default**: None (must be set)
- **Example**: `my_secure_password_123`
- **Generation**: Use strong password (16+ chars)

#### ADMIN_API_TOKEN (REQUIRED)
- **Description**: Token for admin API authentication
- **Used by**: control-plane, dashboard
- **Default**: None (must be set)
- **Example**: `$(openssl rand -hex 32)`
- **Generation**: `openssl rand -hex 32`

#### JWT_SECRET (REQUIRED)
- **Description**: Secret for signing JWT tokens
- **Used by**: control-plane, dashboard
- **Default**: None (must be set)
- **Example**: `$(openssl rand -hex 32)`
- **Generation**: `openssl rand -hex 32`

#### API_KEY_HASH_ROUNDS
- **Description**: Bcrypt rounds for API key hashing
- **Used by**: control-plane
- **Default**: `12`
- **Range**: 10-14 (higher = more secure but slower)

#### TLS_ENABLED
- **Description**: Enable TLS/SSL
- **Used by**: control-plane
- **Default**: `false`
- **Values**: `true`, `false`

#### TLS_CERT_PATH
- **Description**: Path to TLS certificate
- **Used by**: control-plane
- **Default**: None
- **Example**: `/path/to/cert.pem`

#### TLS_KEY_PATH
- **Description**: Path to TLS private key
- **Used by**: control-plane
- **Default**: None
- **Example**: `/path/to/key.pem`

---

### ☁️ Cloudflare R2

#### R2_ENDPOINT (REQUIRED)
- **Description**: R2 S3-compatible endpoint
- **Used by**: control-plane, node-agent, scripts
- **Default**: None (must be set)
- **Example**: `https://abc123.r2.cloudflarestorage.com`
- **Get from**: Cloudflare R2 dashboard

#### R2_BUCKET
- **Description**: R2 bucket name for models
- **Used by**: control-plane, node-agent, scripts
- **Default**: `crosslogic-models`
- **Example**: `my-llm-models`

#### R2_ACCESS_KEY (REQUIRED)
- **Description**: R2 API access key
- **Used by**: control-plane, node-agent, scripts
- **Default**: None (must be set)
- **Get from**: Cloudflare R2 → Manage R2 API Tokens

#### R2_SECRET_KEY (REQUIRED)
- **Description**: R2 API secret key
- **Used by**: control-plane, node-agent, scripts
- **Default**: None (must be set)
- **Get from**: Cloudflare R2 → Manage R2 API Tokens

#### R2_CDN_DOMAIN
- **Description**: Custom CDN domain for R2
- **Used by**: control-plane
- **Default**: None
- **Example**: `models.yourdomain.com`

---

### 🤗 HuggingFace

#### HUGGINGFACE_TOKEN (REQUIRED)
- **Description**: HuggingFace API token
- **Used by**: scripts (upload-model-to-r2.py)
- **Default**: None (must be set)
- **Example**: `hf_xxxxxxxxxxxxx`
- **Get from**: https://huggingface.co/settings/tokens
- **Permissions**: Read access to repos

---

### ☁️ Cloud Providers

#### AWS_ACCESS_KEY_ID
- **Description**: AWS access key for SkyPilot
- **Used by**: control-plane (SkyPilot)
- **Required if**: Using AWS for GPU instances
- **Get from**: AWS IAM

#### AWS_SECRET_ACCESS_KEY
- **Description**: AWS secret key for SkyPilot
- **Used by**: control-plane (SkyPilot)
- **Required if**: Using AWS for GPU instances
- **Get from**: AWS IAM

#### AWS_DEFAULT_REGION
- **Description**: Default AWS region
- **Used by**: control-plane (SkyPilot)
- **Default**: `us-east-1`
- **Example**: `us-west-2`

#### AZURE_SUBSCRIPTION_ID
- **Description**: Azure subscription ID
- **Used by**: control-plane (SkyPilot)
- **Required if**: Using Azure for GPU instances
- **Get from**: Azure portal

#### AZURE_TENANT_ID
- **Description**: Azure tenant ID
- **Used by**: control-plane (SkyPilot)
- **Required if**: Using Azure for GPU instances
- **Get from**: Azure portal

#### GCP_PROJECT_ID
- **Description**: GCP project ID
- **Used by**: control-plane (SkyPilot)
- **Required if**: Using GCP for GPU instances
- **Get from**: GCP console

---

### 🖥️ Server

#### SERVER_HOST
- **Description**: Server bind address
- **Used by**: control-plane
- **Default**: `0.0.0.0`
- **Production**: `0.0.0.0` (all interfaces)

#### SERVER_PORT
- **Description**: Server port
- **Used by**: control-plane
- **Default**: `8080`

#### CONTROL_PLANE_URL
- **Description**: Public URL for node registration
- **Used by**: control-plane, node-agent
- **Default**: `https://api.crosslogic.ai`
- **Development**: `http://localhost:8080`
- **Production**: Your domain (e.g., `https://api.yourdomain.com`)

#### SERVER_READ_TIMEOUT
- **Description**: HTTP read timeout
- **Used by**: control-plane
- **Default**: `30s`
- **Format**: Duration string (e.g., `30s`, `1m`)

#### SERVER_WRITE_TIMEOUT
- **Description**: HTTP write timeout
- **Used by**: control-plane
- **Default**: `30s`

#### SERVER_IDLE_TIMEOUT
- **Description**: HTTP idle timeout
- **Used by**: control-plane
- **Default**: `120s`

---

### 🗄️ Database

#### DB_HOST
- **Description**: PostgreSQL host
- **Used by**: control-plane
- **Default**: `localhost`
- **Docker**: `postgres` (service name)

#### DB_PORT
- **Description**: PostgreSQL port
- **Used by**: control-plane
- **Default**: `5432`

#### DB_USER
- **Description**: PostgreSQL username
- **Used by**: control-plane
- **Default**: `crosslogic`

#### DB_NAME
- **Description**: Database name
- **Used by**: control-plane
- **Default**: `crosslogic_iaas`

#### DB_SSL_MODE
- **Description**: SSL mode for Postgres
- **Used by**: control-plane
- **Default**: `disable`
- **Options**: `disable`, `require`, `verify-ca`, `verify-full`

#### DB_MAX_OPEN_CONNS
- **Description**: Max open connections
- **Used by**: control-plane
- **Default**: `25`

#### DB_MAX_IDLE_CONNS
- **Description**: Max idle connections
- **Used by**: control-plane
- **Default**: `5`

#### DB_CONN_MAX_LIFETIME
- **Description**: Connection max lifetime
- **Used by**: control-plane
- **Default**: `5m`

---

### 🔴 Redis

#### REDIS_HOST
- **Description**: Redis host
- **Used by**: control-plane
- **Default**: `localhost`
- **Docker**: `redis` (service name)

#### REDIS_PORT
- **Description**: Redis port
- **Used by**: control-plane
- **Default**: `6379`

#### REDIS_PASSWORD
- **Description**: Redis password
- **Used by**: control-plane
- **Default**: None (empty)

#### REDIS_DB
- **Description**: Redis database number
- **Used by**: control-plane
- **Default**: `0`

#### REDIS_POOL_SIZE
- **Description**: Redis connection pool size
- **Used by**: control-plane
- **Default**: `10`

---

### 💳 Billing (Stripe)

#### BILLING_ENABLED
- **Description**: Enable billing
- **Used by**: control-plane
- **Default**: `true`
- **Development**: `false`
- **Production**: `true`

#### STRIPE_SECRET_KEY
- **Description**: Stripe API secret key
- **Used by**: control-plane
- **Required if**: `BILLING_ENABLED=true`
- **Get from**: https://dashboard.stripe.com/apikeys
- **Development**: Use test key (`sk_test_...`)
- **Production**: Use live key (`sk_live_...`)

#### STRIPE_WEBHOOK_SECRET
- **Description**: Stripe webhook signing secret
- **Used by**: control-plane
- **Required if**: `BILLING_ENABLED=true`
- **Get from**: Stripe dashboard → Webhooks

#### BILLING_AGGREGATION_INTERVAL
- **Description**: Usage aggregation interval
- **Used by**: control-plane
- **Default**: `1h`
- **Format**: Duration string

#### BILLING_EXPORT_INTERVAL
- **Description**: Stripe export interval
- **Used by**: control-plane
- **Default**: `5m`

---

### 🎮 Runtime

#### VLLM_VERSION
- **Description**: vLLM version for GPU nodes
- **Used by**: control-plane (SkyPilot templates)
- **Default**: `0.6.2`
- **Example**: `0.6.2`, `0.7.0`

#### TORCH_VERSION
- **Description**: PyTorch version for GPU nodes
- **Used by**: control-plane (SkyPilot templates)
- **Default**: `2.4.0`

---

### 📊 Monitoring

#### MONITORING_ENABLED
- **Description**: Enable Prometheus metrics
- **Used by**: control-plane
- **Default**: `true`

#### PROMETHEUS_PORT
- **Description**: Prometheus metrics port
- **Used by**: control-plane
- **Default**: `9090`

#### METRICS_PATH
- **Description**: Metrics endpoint path
- **Used by**: control-plane
- **Default**: `/metrics`

#### LOG_LEVEL
- **Description**: Logging level
- **Used by**: control-plane
- **Default**: `info`
- **Options**: `debug`, `info`, `warn`, `error`

---

### 📊 Grafana

#### GRAFANA_PASSWORD
- **Description**: Grafana admin password
- **Used by**: grafana
- **Default**: `admin`
- **Production**: Change to strong password

---

### 🌐 Dashboard

#### CROSSLOGIC_API_BASE_URL
- **Description**: API URL for dashboard
- **Used by**: dashboard
- **Default**: `http://control-plane:8080`
- **Docker**: Use service name
- **External**: Use public URL

#### CROSSLOGIC_ADMIN_TOKEN
- **Description**: Admin token for dashboard
- **Used by**: dashboard
- **Default**: Same as `ADMIN_API_TOKEN`

#### NEXTAUTH_URL
- **Description**: NextAuth public URL
- **Used by**: dashboard
- **Default**: `http://localhost:3000`
- **Production**: Your dashboard domain

#### NEXTAUTH_SECRET
- **Description**: NextAuth signing secret
- **Used by**: dashboard
- **Default**: Same as `JWT_SECRET`

---

## 🔍 Where Variables Are Used

### config.go
All application configuration is loaded from `control-plane/internal/config/config.go`:
- Server, Database, Redis configs
- Security (JWT, API keys, TLS)
- Billing (Stripe)
- Runtime (vLLM, PyTorch versions)
- Monitoring (Prometheus, logging)
- R2 (Cloudflare R2 configuration)

### docker-compose.yml
Environment variables for containers:
- Database connection strings
- R2 credentials
- Stripe keys
- Admin tokens
- Dashboard configuration

### Scripts
- `scripts/setup-r2.sh`: R2_ENDPOINT, R2_BUCKET, R2_ACCESS_KEY, R2_SECRET_KEY
- `scripts/upload-model-to-r2.py`: HUGGINGFACE_TOKEN, R2 credentials
- `scripts/list-models.sh`: R2 credentials

### SkyPilot
Cloud provider credentials passed to SkyPilot for GPU provisioning:
- AWS: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
- Azure: AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID
- GCP: GCP_PROJECT_ID

---

## 🚀 Setup Guide

### 1. Copy Example File

```bash
cp config/env.example .env
```

### 2. Generate Secrets

```bash
export DB_PASSWORD=$(openssl rand -base64 32)
export ADMIN_API_TOKEN=$(openssl rand -hex 32)
export JWT_SECRET=$(openssl rand -hex 32)

echo "DB_PASSWORD=$DB_PASSWORD"
echo "ADMIN_API_TOKEN=$ADMIN_API_TOKEN"
echo "JWT_SECRET=$JWT_SECRET"
```

### 3. Get Cloud Credentials

#### Cloudflare R2:
1. Go to Cloudflare dashboard → R2
2. Create bucket (e.g., `crosslogic-models`)
3. Generate API token → Manage R2 API Tokens
4. Copy endpoint, access key, secret key

#### HuggingFace:
1. Go to https://huggingface.co/settings/tokens
2. Create new token with read access
3. Copy token (starts with `hf_`)

#### AWS (if using):
1. Create IAM user with EC2, S3 permissions
2. Generate access key
3. Copy access key ID and secret

#### Azure (if using):
1. Go to Azure portal → Subscriptions
2. Copy subscription ID and tenant ID
3. Run `az login` for authentication

### 4. Fill .env File

```bash
nano .env
# Fill in all REQUIRED variables
```

### 5. Verify Configuration

```bash
# Check required variables are set
grep -E "^(DB_PASSWORD|ADMIN_API_TOKEN|JWT_SECRET|R2_ENDPOINT|R2_ACCESS_KEY)" .env

# Should show all 5 variables with values
```

---

## 📝 Notes

1. **Required vs Optional**:
   - All variables marked REQUIRED must be set
   - Optional variables have defaults in config.go

2. **Security**:
   - Never commit .env file to git
   - Use strong passwords (16+ chars)
   - Rotate secrets regularly in production

3. **Docker**:
   - docker-compose.yml reads from .env automatically
   - Service names are used for inter-container communication

4. **Billing**:
   - Set `BILLING_ENABLED=false` for development
   - Stripe webhooks need public endpoint

5. **Cloud Providers**:
   - Need at least one for GPU instances
   - Can configure multiple for multi-cloud

---

## 📚 See Also

- `config/env.example` - Complete example file
- `config/env.template` - Minimal template
- `QUICK_START.md` - Quick setup guide
- `PREREQUISITES_CHECKLIST.md` - Setup checklist


