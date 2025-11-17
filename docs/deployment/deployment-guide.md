# Deployment Guide

This guide provides step-by-step instructions for deploying CrossLogic Inference Cloud in various environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Local Development Deployment](#local-development-deployment)
3. [Production Deployment](#production-deployment)
4. [GPU Node Deployment](#gpu-node-deployment)
5. [On-Premise Deployment](#on-premise-deployment)
6. [Monitoring Setup](#monitoring-setup)
7. [Backup & Recovery](#backup--recovery)

## Prerequisites

### Required Software

- Docker 24.0+ and Docker Compose 2.20+
- PostgreSQL 16+
- Redis 7+
- Go 1.22+ (for building from source)

### Required Accounts

- Stripe account (for billing)
- Cloud provider account (AWS/GCP/Azure) for GPU nodes
- Domain name for production deployment

### System Requirements

#### Control Plane
- **Minimum**: 2 vCPUs, 4GB RAM, 20GB disk
- **Recommended**: 4 vCPUs, 16GB RAM, 100GB disk
- **Production**: 8+ vCPUs, 32GB+ RAM, 500GB+ disk

#### Database (PostgreSQL)
- **Minimum**: 2 vCPUs, 4GB RAM, 50GB SSD
- **Recommended**: 4 vCPUs, 8GB RAM, 200GB SSD
- **Production**: Use managed service (RDS, Cloud SQL)

#### Cache (Redis)
- **Minimum**: 1GB RAM
- **Recommended**: 4GB RAM
- **Production**: Use managed service (ElastiCache, Memorystore)

## Local Development Deployment

### Step 1: Clone Repository

```bash
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas
```

### Step 2: Configure Environment

```bash
cp config/.env.example .env

# Edit configuration
nano .env
```

Minimum required settings:
```bash
DB_PASSWORD=changeme_dev_password
STRIPE_SECRET_KEY=sk_test_your_key_here
STRIPE_WEBHOOK_SECRET=whsec_your_secret_here
```

### Step 3: Start Services

```bash
# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f
```

### Step 4: Initialize Database

```bash
# Wait for PostgreSQL to be ready
docker-compose exec postgres pg_isready

# Run schema migration
docker-compose exec postgres psql -U crosslogic -d crosslogic_iaas -f /docker-entrypoint-initdb.d/01_core_tables.sql

# Verify
docker-compose exec postgres psql -U crosslogic -d crosslogic_iaas -c "\dt"
```

### Step 5: Create Test Tenant

```bash
# Access PostgreSQL
docker-compose exec postgres psql -U crosslogic crosslogic_iaas

# Create tenant
INSERT INTO tenants (name, email, status)
VALUES ('Dev Org', 'dev@example.com', 'active')
RETURNING id;

# Copy the returned tenant ID

# Create environment
INSERT INTO environments (tenant_id, name, region, status)
VALUES ('<tenant_id>', 'development', 'in-mumbai', 'active')
RETURNING id;

# Exit psql
\q
```

### Step 6: Generate API Key

```bash
# Run Go code to generate API key
cd control-plane
go run -exec "echo 'clsk_dev_$(uuidgen | tr -d - | cut -c1-32)'"

# Or use a simple script
export API_KEY="clsk_dev_$(openssl rand -hex 16)"
echo $API_KEY

# Hash the key (SHA-256)
echo -n $API_KEY | sha256sum | awk '{print $1}'

# Insert into database
docker-compose exec postgres psql -U crosslogic crosslogic_iaas -c "
INSERT INTO api_keys (key_hash, key_prefix, tenant_id, environment_id, status)
VALUES ('<hashed_key>', '$(echo $API_KEY | cut -c1-12)', '<tenant_id>', '<env_id>', 'active');
"
```

### Step 7: Test API

```bash
# Health check
curl http://localhost:8080/health

# List models
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer $API_KEY"

# Test chat completion (will need GPU node running)
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-8b",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Production Deployment

### Option 1: Single VM Deployment

#### Step 1: Provision VM

Requirements:
- Ubuntu 22.04 LTS
- 8 vCPUs, 32GB RAM
- 500GB SSD
- Static IP address
- Open ports: 80, 443, 8080 (internally)

```bash
# Example: AWS EC2
aws ec2 run-instances \
  --image-id ami-0c7217cdde317cfec \
  --instance-type c5.2xlarge \
  --key-name crosslogic-key \
  --security-group-ids sg-xxxxx \
  --subnet-id subnet-xxxxx
```

#### Step 2: Install Dependencies

```bash
# SSH into VM
ssh ubuntu@<vm-ip>

# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Logout and login again for docker group to take effect
exit
ssh ubuntu@<vm-ip>
```

#### Step 3: Deploy Application

```bash
# Clone repository
git clone https://github.com/crosslogic/crosslogic-ai-iaas.git
cd crosslogic-ai-iaas

# Create production .env
cp config/.env.example .env

# Edit with production values
nano .env
```

Production `.env`:
```bash
# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database (use managed service)
DB_HOST=your-rds-endpoint.amazonaws.com
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=<strong_password>
DB_NAME=crosslogic_iaas
DB_SSL_MODE=require
DB_MAX_OPEN_CONNS=50

# Redis (use managed service)
REDIS_HOST=your-elasticache-endpoint.amazonaws.com
REDIS_PORT=6379
REDIS_PASSWORD=<redis_password>

# Billing
STRIPE_SECRET_KEY=sk_live_<your_live_key>
STRIPE_WEBHOOK_SECRET=whsec_<your_webhook_secret>

# Security
TLS_ENABLED=true
TLS_CERT_PATH=/etc/letsencrypt/live/api.crosslogic.ai/fullchain.pem
TLS_KEY_PATH=/etc/letsencrypt/live/api.crosslogic.ai/privkey.pem

# Monitoring
LOG_LEVEL=info
MONITORING_ENABLED=true
```

```bash
# Start services
docker-compose up -d

# Check logs
docker-compose logs -f
```

#### Step 4: Set Up TLS with Let's Encrypt

```bash
# Install Certbot
sudo apt install certbot python3-certbot-nginx -y

# Get certificate
sudo certbot certonly --standalone -d api.crosslogic.ai

# Certificates will be in /etc/letsencrypt/live/api.crosslogic.ai/

# Set up auto-renewal
sudo crontab -e
# Add this line:
# 0 3 * * * certbot renew --quiet && docker-compose restart control-plane
```

#### Step 5: Set Up Nginx Reverse Proxy

```bash
sudo apt install nginx -y

# Create configuration
sudo nano /etc/nginx/sites-available/crosslogic
```

```nginx
upstream control-plane {
    server localhost:8080;
}

server {
    listen 80;
    server_name api.crosslogic.ai;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.crosslogic.ai;

    ssl_certificate /etc/letsencrypt/live/api.crosslogic.ai/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.crosslogic.ai/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    client_max_body_size 10M;

    location / {
        proxy_pass http://control-plane;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

```bash
# Enable site
sudo ln -s /etc/nginx/sites-available/crosslogic /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx
```

### Option 2: Kubernetes Deployment

Coming soon.

## GPU Node Deployment

### Using SkyPilot

#### Step 1: Install SkyPilot

```bash
pip install "skypilot[aws,gcp,azure]"

# Configure cloud credentials
sky check
```

#### Step 2: Create Task Configuration

Create `gpu-llama-8b.yaml`:

```yaml
name: crosslogic-llama-8b

resources:
  accelerators: A10G:1
  cloud: aws
  region: us-east-1
  disk_size: 512
  use_spot: true

num_nodes: 3  # Deploy 3 nodes

file_mounts:
  ~/.config/crosslogic:
    source: ./config
    mode: COPY

setup: |
  # Install CUDA drivers (if needed)
  # Install vLLM
  pip install vllm torch

  # Download node agent
  wget https://releases.crosslogic.ai/node-agent-linux-amd64 -O /usr/local/bin/node-agent
  chmod +x /usr/local/bin/node-agent

  # Download model (optional - vLLM can download on first run)
  # huggingface-cli download meta-llama/Llama-3-8B

run: |
  # Start vLLM server
  python -m vllm.entrypoints.openai.api_server \
    --model meta-llama/Llama-3-8B \
    --host 0.0.0.0 \
    --port 8000 \
    --tensor-parallel-size 1 &

  # Wait for vLLM to start
  sleep 30

  # Start node agent
  export CONTROL_PLANE_URL=https://api.crosslogic.ai
  export MODEL_NAME=llama-3-8b
  export PROVIDER=aws
  export REGION=us-east-1
  export VLLM_ENDPOINT=http://localhost:8000
  /usr/local/bin/node-agent
```

#### Step 3: Launch GPU Nodes

```bash
# Launch task
sky launch -c llama-cluster gpu-llama-8b.yaml

# Check status
sky status llama-cluster

# View logs
sky logs llama-cluster

# SSH to node
sky ssh llama-cluster

# Terminate when done
sky down llama-cluster
```

### Manual GPU Node Setup

For non-SkyPilot deployments:

```bash
# 1. Provision GPU instance (e.g., g5.xlarge on AWS)

# 2. Install dependencies
sudo apt update
sudo apt install -y python3-pip nvidia-cuda-toolkit

# 3. Install vLLM
pip install vllm

# 4. Download model
huggingface-cli download meta-llama/Llama-3-8B

# 5. Start vLLM
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Llama-3-8B \
  --host 0.0.0.0 \
  --port 8000 &

# 6. Download and run node agent
wget https://releases.crosslogic.ai/node-agent-linux-amd64
chmod +x node-agent-linux-amd64

export CONTROL_PLANE_URL=https://api.crosslogic.ai
export MODEL_NAME=llama-3-8b
export PROVIDER=aws
export REGION=us-east-1

./node-agent-linux-amd64
```

## On-Premise Deployment

For enterprise customers running inference on their own hardware:

### Control Plane (Cloud-Hosted)

Deploy control plane as described in Production Deployment section.

### GPU Nodes (On-Premise)

```bash
# On each GPU server:

# 1. Install dependencies
sudo apt update
sudo apt install -y nvidia-driver-535 cuda-12-2

# 2. Install vLLM
pip install vllm

# 3. Download node agent
wget https://releases.crosslogic.ai/node-agent-linux-amd64 -O /usr/local/bin/node-agent
chmod +x /usr/local/bin/node-agent

# 4. Create systemd service for vLLM
sudo tee /etc/systemd/system/vllm.service <<EOF
[Unit]
Description=vLLM Inference Server
After=network.target

[Service]
Type=simple
User=llm
ExecStart=/usr/bin/python3 -m vllm.entrypoints.openai.api_server \
  --model /models/llama-3-70b \
  --host 0.0.0.0 \
  --port 8000
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# 5. Create systemd service for node agent
sudo tee /etc/systemd/system/node-agent.service <<EOF
[Unit]
Description=CrossLogic Node Agent
After=network.target vllm.service

[Service]
Type=simple
User=llm
Environment="CONTROL_PLANE_URL=https://api.crosslogic.ai"
Environment="MODEL_NAME=llama-3-70b"
Environment="PROVIDER=on-prem"
Environment="REGION=on-prem-dc1"
Environment="VLLM_ENDPOINT=http://localhost:8000"
ExecStart=/usr/local/bin/node-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# 6. Start services
sudo systemctl daemon-reload
sudo systemctl enable vllm node-agent
sudo systemctl start vllm node-agent

# 7. Check status
sudo systemctl status vllm node-agent
```

## Monitoring Setup

### Prometheus

```bash
# Create prometheus.yml
cat > config/prometheus.yml <<EOF
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'control-plane'
    static_configs:
      - targets: ['control-plane:9090']

  - job_name: 'postgres-exporter'
    static_configs:
      - targets: ['postgres-exporter:9187']

  - job_name: 'redis-exporter'
    static_configs:
      - targets: ['redis-exporter:9121']
EOF

# Start Prometheus
docker-compose up -d prometheus

# Access at http://localhost:9091
```

### Grafana

```bash
# Start Grafana
docker-compose up -d grafana

# Access at http://localhost:3000
# Default credentials: admin/admin

# Add Prometheus data source
# URL: http://prometheus:9090

# Import dashboards from docs/grafana/
```

## Backup & Recovery

### Database Backup

```bash
# Automated backup script
cat > /usr/local/bin/backup-crosslogic-db.sh <<'EOF'
#!/bin/bash
BACKUP_DIR=/backups/crosslogic
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# Backup database
docker-compose exec -T postgres pg_dump -U crosslogic crosslogic_iaas | \
  gzip > $BACKUP_DIR/crosslogic_$DATE.sql.gz

# Keep only last 30 days
find $BACKUP_DIR -name "crosslogic_*.sql.gz" -mtime +30 -delete

# Upload to S3 (optional)
aws s3 cp $BACKUP_DIR/crosslogic_$DATE.sql.gz s3://crosslogic-backups/
EOF

chmod +x /usr/local/bin/backup-crosslogic-db.sh

# Add to crontab
crontab -e
# Add: 0 2 * * * /usr/local/bin/backup-crosslogic-db.sh
```

### Database Restore

```bash
# Restore from backup
gunzip -c /backups/crosslogic/crosslogic_20250117.sql.gz | \
  docker-compose exec -T postgres psql -U crosslogic crosslogic_iaas
```

## Post-Deployment Checklist

- [ ] All services are running
- [ ] Database is accessible and initialized
- [ ] Redis is responding
- [ ] TLS certificates are valid
- [ ] API endpoints are accessible
- [ ] Monitoring is working
- [ ] Backups are configured
- [ ] Alerts are configured
- [ ] Documentation is updated
- [ ] Team is trained

## Troubleshooting

See main [README.md](../../README.md#troubleshooting) for common issues.

## Support

For deployment assistance:
- Email: support@crosslogic.ai
- Discord: https://discord.gg/crosslogic
- Documentation: https://docs.crosslogic.ai
