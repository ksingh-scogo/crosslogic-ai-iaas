# 🚀 Local Setup & Testing Guide - CrossLogic AI IaaS

Complete step-by-step guide to run the entire platform locally and test end-to-end with real GPU instances.

## 📋 Prerequisites Checklist

Before starting, gather these credentials and tools:

### ✅ Required Accounts & Credentials

1. **Azure Account** (for GPU spot instance)
   - [ ] Azure subscription ID
   - [ ] Azure tenant ID  
   - [ ] Azure service principal (or use `az login`)
   - [ ] Access to `Standard_NV36ads_A10_v5` in at least one region

2. **AWS Account** (for second GPU spot instance)
   - [ ] AWS access key ID
   - [ ] AWS secret access key
   - [ ] Access to GPU instances (g4dn, g5, p3, etc.)

3. **Cloudflare R2** (for model storage)
   - [ ] R2 bucket created: `crosslogic-models`
   - [ ] R2 Access Key ID
   - [ ] R2 Secret Access Key
   - [ ] R2 Account ID (from endpoint URL)

4. **HuggingFace** (for model downloads)
   - [ ] HuggingFace account
   - [ ] HuggingFace API token (from https://huggingface.co/settings/tokens)
   - [ ] Accept Mistral license: https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3
   - [ ] Accept Llama 3 license: https://huggingface.co/meta-llama/Meta-Llama-3-8B-Instruct

5. **Stripe** (for billing - optional for testing)
   - [ ] Stripe test account
   - [ ] Stripe secret key (sk_test_...)
   - [ ] Stripe webhook secret (whsec_...)

### ✅ Required Tools

Install these on your local machine:

```bash
# Docker & Docker Compose
docker --version  # Should be 24+
docker compose version  # Should be v2+

# Go (for building)
go version  # Should be 1.22+

# Node.js (for dashboard)
node --version  # Should be 18+
npm --version

# Python (for scripts)
python3 --version  # Should be 3.10+
pip3 --version

# AWS CLI
aws --version

# Azure CLI
az --version

# SkyPilot (for GPU orchestration)
pip3 install "skypilot[azure,aws]"
sky check
```

---

## 🔧 Part 1: Local Services Setup (30 minutes)

### Step 1.1: Clone & Configure

```bash
# Clone repository
cd /Users/ksingh/git/scogo/work/experiments
cd crosslogic-ai-iaas

# Create .env file
cat > .env << 'EOF'
# ============================================
# Server Configuration
# ============================================
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
CONTROL_PLANE_URL=http://localhost:8080

# ============================================
# Database Configuration
# ============================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=my_secure_password_123
DB_NAME=crosslogic_iaas
DB_SSL_MODE=disable

# ============================================
# Redis Configuration
# ============================================
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# ============================================
# Stripe Billing (Optional - use test keys)
# ============================================
BILLING_ENABLED=false
STRIPE_SECRET_KEY=sk_test_dummy_key
STRIPE_WEBHOOK_SECRET=whsec_dummy_secret

# ============================================
# Security Configuration
# ============================================
ADMIN_API_TOKEN=cl_admin_$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
TLS_ENABLED=false

# ============================================
# Runtime Versions
# ============================================
VLLM_VERSION=0.6.2
TORCH_VERSION=2.4.0

# ============================================
# Monitoring
# ============================================
MONITORING_ENABLED=true
LOG_LEVEL=debug

# ============================================
# Cloudflare R2 Configuration
# ============================================
# TODO: Fill in your R2 credentials
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=YOUR_R2_ACCESS_KEY
R2_SECRET_KEY=YOUR_R2_SECRET_KEY
R2_CDN_DOMAIN=  # Optional

# ============================================
# HuggingFace Configuration
# ============================================
# TODO: Fill in your HuggingFace token
HUGGINGFACE_TOKEN=hf_YOUR_TOKEN_HERE

# ============================================
# Cloud Provider Credentials (for SkyPilot)
# ============================================

# AWS
AWS_ACCESS_KEY_ID=YOUR_AWS_KEY
AWS_SECRET_ACCESS_KEY=YOUR_AWS_SECRET
AWS_DEFAULT_REGION=us-east-1

# Azure
AZURE_SUBSCRIPTION_ID=YOUR_AZURE_SUB_ID
AZURE_TENANT_ID=YOUR_AZURE_TENANT_ID
AZURE_CLIENT_ID=YOUR_AZURE_CLIENT_ID  # Optional if using az login
AZURE_CLIENT_SECRET=YOUR_AZURE_CLIENT_SECRET  # Optional if using az login

EOF

echo "✅ .env file created"
echo "⚠️  IMPORTANT: Edit .env and fill in your credentials!"
```

### Step 1.2: Fill in Required Credentials

Open `.env` and replace the following:

```bash
# Open in your editor
code .env  # or nano .env

# Required fields:
# 1. R2_ENDPOINT - Your Cloudflare account ID
# 2. R2_ACCESS_KEY - From R2 API tokens
# 3. R2_SECRET_KEY - From R2 API tokens
# 4. HUGGINGFACE_TOKEN - From HuggingFace settings
# 5. AWS credentials - From AWS IAM
# 6. AZURE credentials - From Azure portal
```

### Step 1.3: Build Docker Images

```bash
# Build control plane
echo "🔨 Building control plane..."
docker build -f Dockerfile.control-plane -t crosslogic/control-plane:latest .

# Build node agent
echo "🔨 Building node agent..."
docker build -f Dockerfile.node-agent -t crosslogic/node-agent:latest .

echo "✅ Docker images built successfully"
```

### Step 1.4: Start Infrastructure Services

```bash
# Start PostgreSQL and Redis
docker compose up -d postgres redis

# Wait for health checks
echo "⏳ Waiting for database to be ready..."
sleep 15

# Verify services are running
docker compose ps
```

### Step 1.5: Run Database Migrations

```bash
# Run migrations
echo "🗄️  Running database migrations..."
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas << 'EOF'
-- Core tables are created via init scripts
-- Verify tables exist
\dt
EOF

# Or use the migration script
chmod +x database/migrate.sh
./database/migrate.sh

echo "✅ Database initialized"
```

### Step 1.6: Seed Database with Test Data

```bash
# Create tenant, environment, and models
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas << 'EOF'
-- Create test tenant
INSERT INTO tenants (id, name, email, status, billing_plan)
VALUES (gen_random_uuid(), 'Test Organization', 'test@example.com', 'active', 'serverless')
RETURNING id;

-- Save the tenant ID and create environment
\gset tenant_
INSERT INTO environments (id, tenant_id, name, region, status)
VALUES (gen_random_uuid(), :'tenant_id', 'production', 'us-east', 'active')
RETURNING id;

-- Save environment ID
\gset env_

-- Create Mistral 7B model
INSERT INTO models (id, name, family, size, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million, status)
VALUES (gen_random_uuid(), 'mistralai/Mistral-7B-Instruct-v0.3', 'Mistral', '7B', 'chat', 8192, 16, 0.05, 0.10, 'active');

-- Create Llama 3 8B model
INSERT INTO models (id, name, family, size, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million, status)
VALUES (gen_random_uuid(), 'meta-llama/Meta-Llama-3-8B-Instruct', 'Llama', '8B', 'chat', 8192, 16, 0.05, 0.10, 'active');

-- Display results
SELECT id, name, email FROM tenants;
SELECT id, name FROM environments;
SELECT id, name FROM models;
EOF

echo "✅ Test data seeded"
```

### Step 1.7: Start Control Plane

```bash
# Start control plane
docker compose up -d control-plane

# Check logs
docker compose logs -f control-plane

# Wait for "Server started" message
# Press Ctrl+C to exit logs

echo "✅ Control plane running at http://localhost:8080"
```

### Step 1.8: Create API Key

```bash
# Load environment
source .env

# Get tenant and environment IDs
TENANT_ID=$(docker compose exec -T postgres psql -U crosslogic -d crosslogic_iaas -tAc "SELECT id FROM tenants LIMIT 1")
ENV_ID=$(docker compose exec -T postgres psql -U crosslogic -d crosslogic_iaas -tAc "SELECT id FROM environments LIMIT 1")

echo "Tenant ID: $TENANT_ID"
echo "Environment ID: $ENV_ID"

# Create API key
API_KEY=$(curl -s -X POST http://localhost:8080/admin/api-keys \
  -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"environment_id\": \"$ENV_ID\",
    \"name\": \"test-key\"
  }" | jq -r '.key')

echo "✅ API Key created: $API_KEY"
echo "export API_KEY=$API_KEY" >> .env
source .env
```

### Step 1.9: Start Dashboard (Optional)

```bash
# Navigate to dashboard
cd control-plane/dashboard

# Install dependencies
npm install

# Create .env.local
cat > .env.local << EOF
CROSSLOGIC_API_BASE_URL=http://localhost:8080
CROSSLOGIC_ADMIN_TOKEN=$ADMIN_API_TOKEN
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=$(openssl rand -base64 32)
EOF

# Start development server
npm run dev

# Dashboard available at: http://localhost:3000
```

### Step 1.10: Verify Local Services

```bash
# Test health endpoint
curl http://localhost:8080/health

# Test models endpoint
curl http://localhost:8080/v1/models

# Expected: Should return list of models
```

---

## ☁️ Part 2: Setup Cloudflare R2 (15 minutes)

### Step 2.1: Configure R2

```bash
# Go back to project root
cd /Users/ksingh/git/scogo/work/experiments/crosslogic-ai-iaas

# Load environment
source .env

# Export AWS credentials for R2
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY

# Run R2 setup
chmod +x scripts/setup-r2.sh
./scripts/setup-r2.sh
```

### Step 2.2: Verify R2 Connection

```bash
# List bucket contents
aws s3 ls s3://$R2_BUCKET/ --endpoint-url $R2_ENDPOINT

# Should succeed (bucket may be empty)
```

---

## 🎯 Part 3: Upload Mistral 7B to R2 (30 minutes)

### Step 3.1: Install Python Dependencies

```bash
# Install required packages
pip3 install huggingface-hub tqdm awscli
```

### Step 3.2: Upload Mistral 7B

```bash
# Set environment variables
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY

# Upload model (this will take 20-30 minutes)
python3 scripts/upload-model-to-r2.py \
  mistralai/Mistral-7B-Instruct-v0.3 \
  --hf-token $HUGGINGFACE_TOKEN \
  --r2-endpoint $R2_ENDPOINT \
  --r2-bucket $R2_BUCKET

# Expected output:
# 📥 Downloading mistralai/Mistral-7B-Instruct-v0.3 from HuggingFace...
# ✓ Downloaded to /tmp/model-cache/...
# 📊 Model size: 14.48 GB
# 📤 Uploading to R2: s3://crosslogic-models/mistralai/Mistral-7B-Instruct-v0.3
# ✓ Upload complete!
```

### Step 3.3: Verify Upload

```bash
# List models in R2
./scripts/list-models.sh

# Should show: mistralai/Mistral-7B-Instruct-v0.3
```

---

## 🚀 Part 4: Launch Azure GPU Instance (20 minutes)

### Step 4.1: Configure Azure

```bash
# Login to Azure (if not using service principal)
az login

# Or set service principal credentials
export AZURE_SUBSCRIPTION_ID=$AZURE_SUBSCRIPTION_ID
export AZURE_TENANT_ID=$AZURE_TENANT_ID

# Verify access
az account show
```

### Step 4.2: Create SkyPilot Task File for Mistral

```bash
# Create Azure GPU task file
cat > mistral-azure.yaml << 'EOF'
name: mistral-azure

resources:
  cloud: azure
  instance_type: Standard_NV36ads_A10_v5
  use_spot: true
  disk_size: 256
  disk_tier: Premium_LRS

file_mounts:
  /tmp/node-agent: ./node-agent/

setup: |
  set -e
  
  echo "=== Configuring Cloudflare R2 ==="
  export AWS_ACCESS_KEY_ID="__R2_ACCESS_KEY__"
  export AWS_SECRET_ACCESS_KEY="__R2_SECRET_KEY__"
  export AWS_ENDPOINT_URL="__R2_ENDPOINT__"
  export HF_HUB_ENABLE_HF_TRANSFER=1
  
  echo "=== Installing Python and vLLM ==="
  sudo apt-get update
  sudo apt-get install -y python3.10 python3-pip python3.10-venv
  
  python3.10 -m venv /opt/vllm-env
  source /opt/vllm-env/bin/activate
  
  pip install --upgrade pip setuptools wheel
  pip install vllm==0.6.2 torch==2.4.0
  
  echo "✅ Setup complete"

run: |
  set -e
  source /opt/vllm-env/bin/activate
  
  export AWS_ACCESS_KEY_ID="__R2_ACCESS_KEY__"
  export AWS_SECRET_ACCESS_KEY="__R2_SECRET_KEY__"
  export AWS_ENDPOINT_URL="__R2_ENDPOINT__"
  
  echo "=== Starting vLLM with Mistral 7B from R2 ==="
  MODEL_PATH="s3://__R2_BUCKET__/mistralai/Mistral-7B-Instruct-v0.3"
  
  python -m vllm.entrypoints.openai.api_server \
    --model $MODEL_PATH \
    --host 0.0.0.0 \
    --port 8000 \
    --gpu-memory-utilization 0.9 \
    --max-num-seqs 256 \
    --trust-remote-code
EOF

# Replace placeholders
sed -i '' "s|__R2_ACCESS_KEY__|$R2_ACCESS_KEY|g" mistral-azure.yaml
sed -i '' "s|__R2_SECRET_KEY__|$R2_SECRET_KEY|g" mistral-azure.yaml
sed -i '' "s|__R2_ENDPOINT__|$R2_ENDPOINT|g" mistral-azure.yaml
sed -i '' "s|__R2_BUCKET__|$R2_BUCKET|g" mistral-azure.yaml

echo "✅ Azure task file created: mistral-azure.yaml"
```

### Step 4.3: Launch Azure Instance

```bash
# Launch with SkyPilot
sky launch -c mistral-azure mistral-azure.yaml -y

# This will:
# 1. Provision Standard_NV36ads_A10_v5 spot instance (2-3 min)
# 2. Install vLLM and dependencies (3-5 min)
# 3. Stream Mistral 7B from R2 (30-60 sec)
# 4. Start vLLM server (1-2 min)
# Total: ~10 minutes

# Monitor progress
sky logs mistral-azure -f
```

### Step 4.4: Get Azure Instance Endpoint

```bash
# Get instance IP
AZURE_IP=$(sky status mistral-azure --ip)
echo "Azure Instance IP: $AZURE_IP"

# Test vLLM endpoint
curl http://$AZURE_IP:8000/health

# Expected: {"status": "ok"}
```

---

## 🧪 Part 5: Test Mistral Model (5 minutes)

### Step 5.1: Test Direct to vLLM

```bash
# Test chat completion
curl -X POST http://$AZURE_IP:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistralai/Mistral-7B-Instruct-v0.3",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 100
  }'

# Expected: JSON response with "Paris"
```

### Step 5.2: Register Node with Control Plane

```bash
# Get model ID
MISTRAL_MODEL_ID=$(docker compose exec -T postgres psql -U crosslogic -d crosslogic_iaas -tAc \
  "SELECT id FROM models WHERE name='mistralai/Mistral-7B-Instruct-v0.3'")

# Register node manually
curl -X POST http://localhost:8080/admin/nodes/register \
  -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"provider\": \"azure\",
    \"region\": \"eastus\",
    \"model_name\": \"mistralai/Mistral-7B-Instruct-v0.3\",
    \"endpoint_url\": \"http://$AZURE_IP:8000\",
    \"gpu_type\": \"A10\",
    \"instance_type\": \"Standard_NV36ads_A10_v5\",
    \"spot_instance\": true,
    \"status\": \"active\"
  }"

echo "✅ Node registered"
```

### Step 5.3: Test Through Control Plane

```bash
# Test via control plane (with routing)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistralai/Mistral-7B-Instruct-v0.3",
    "messages": [
      {"role": "user", "content": "Explain quantum computing in one sentence."}
    ],
    "max_tokens": 100
  }'

# Expected: Response routed through control plane to Azure node
```

### Step 5.4: Test from Dashboard

```bash
# Open dashboard: http://localhost:3000
# Navigate to API testing page
# Use the API key and test endpoint
# Submit a chat message
# Verify response
```

---

## 🚀 Part 6: Launch AWS GPU Instance (20 minutes)

### Step 6.1: Upload Llama 3 8B to R2

```bash
# Upload Llama 3 model
python3 scripts/upload-model-to-r2.py \
  meta-llama/Meta-Llama-3-8B-Instruct \
  --hf-token $HUGGINGFACE_TOKEN \
  --r2-endpoint $R2_ENDPOINT \
  --r2-bucket $R2_BUCKET

# This will take 20-30 minutes
```

### Step 6.2: Create SkyPilot Task for Llama 3

```bash
# Create AWS GPU task file
cat > llama-aws.yaml << 'EOF'
name: llama-aws

resources:
  cloud: aws
  instance_type: g4dn.xlarge  # Or g5.xlarge for better performance
  use_spot: true
  disk_size: 256

setup: |
  set -e
  
  echo "=== Configuring R2 ==="
  export AWS_ACCESS_KEY_ID="__R2_ACCESS_KEY__"
  export AWS_SECRET_ACCESS_KEY="__R2_SECRET_KEY__"
  export AWS_ENDPOINT_URL="__R2_ENDPOINT__"
  
  echo "=== Installing vLLM ==="
  sudo apt-get update
  sudo apt-get install -y python3.10 python3-pip python3.10-venv
  
  python3.10 -m venv /opt/vllm-env
  source /opt/vllm-env/bin/activate
  pip install --upgrade pip
  pip install vllm==0.6.2 torch==2.4.0
  
  echo "✅ Setup complete"

run: |
  set -e
  source /opt/vllm-env/bin/activate
  
  export AWS_ACCESS_KEY_ID="__R2_ACCESS_KEY__"
  export AWS_SECRET_ACCESS_KEY="__R2_SECRET_KEY__"
  export AWS_ENDPOINT_URL="__R2_ENDPOINT__"
  
  echo "=== Starting vLLM with Llama 3 8B from R2 ==="
  MODEL_PATH="s3://__R2_BUCKET__/meta-llama/Meta-Llama-3-8B-Instruct"
  
  python -m vllm.entrypoints.openai.api_server \
    --model $MODEL_PATH \
    --host 0.0.0.0 \
    --port 8000 \
    --gpu-memory-utilization 0.9 \
    --trust-remote-code
EOF

# Replace placeholders
sed -i '' "s|__R2_ACCESS_KEY__|$R2_ACCESS_KEY|g" llama-aws.yaml
sed -i '' "s|__R2_SECRET_KEY__|$R2_SECRET_KEY|g" llama-aws.yaml
sed -i '' "s|__R2_ENDPOINT__|$R2_ENDPOINT|g" llama-aws.yaml
sed -i '' "s|__R2_BUCKET__|$R2_BUCKET|g" llama-aws.yaml
```

### Step 6.3: Launch AWS Instance

```bash
# Launch
sky launch -c llama-aws llama-aws.yaml -y

# Monitor
sky logs llama-aws -f
```

### Step 6.4: Get AWS Endpoint

```bash
# Get IP
AWS_IP=$(sky status llama-aws --ip)
echo "AWS Instance IP: $AWS_IP"

# Test
curl http://$AWS_IP:8000/health
```

---

## 🧪 Part 7: Test Llama 3 Model (5 minutes)

### Step 7.1: Test Direct to vLLM

```bash
# Test Llama 3
curl -X POST http://$AWS_IP:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Meta-Llama-3-8B-Instruct",
    "messages": [
      {"role": "user", "content": "Write a haiku about AI."}
    ],
    "max_tokens": 100
  }'
```

### Step 7.2: Register AWS Node

```bash
# Register
curl -X POST http://localhost:8080/admin/nodes/register \
  -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"provider\": \"aws\",
    \"region\": \"us-east-1\",
    \"model_name\": \"meta-llama/Meta-Llama-3-8B-Instruct\",
    \"endpoint_url\": \"http://$AWS_IP:8000\",
    \"gpu_type\": \"T4\",
    \"instance_type\": \"g4dn.xlarge\",
    \"spot_instance\": true,
    \"status\": \"active\"
  }"
```

### Step 7.3: Test Through Control Plane

```bash
# Test routing
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Meta-Llama-3-8B-Instruct",
    "messages": [
      {"role": "user", "content": "What are the benefits of renewable energy?"}
    ],
    "max_tokens": 150
  }'
```

---

## ✅ Part 8: Verification Checklist

### Local Services
- [ ] PostgreSQL running and accessible
- [ ] Redis running
- [ ] Control plane started successfully
- [ ] Dashboard accessible (optional)
- [ ] API key created and working

### R2 Storage
- [ ] R2 bucket accessible
- [ ] Mistral 7B uploaded
- [ ] Llama 3 8B uploaded
- [ ] Models listed with `./scripts/list-models.sh`

### Azure GPU Node
- [ ] Instance launched successfully
- [ ] vLLM started and serving
- [ ] Model loaded from R2 (check logs for s3:// URL)
- [ ] Direct curl test successful
- [ ] Node registered in control plane
- [ ] Routed request successful

### AWS GPU Node
- [ ] Instance launched successfully
- [ ] vLLM started and serving
- [ ] Model loaded from R2
- [ ] Direct curl test successful
- [ ] Node registered
- [ ] Routed request successful

---

## 📊 Part 9: Monitor & Debug

### View Logs

```bash
# Control plane logs
docker compose logs -f control-plane

# Azure node logs
sky logs mistral-azure -f

# AWS node logs
sky logs llama-aws -f

# Database queries
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas
```

### Check Node Status

```bash
# Via API
curl -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  http://localhost:8080/admin/nodes | jq

# Via database
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas -c \
  "SELECT id, provider, model_name, status, endpoint_url FROM nodes;"
```

### Check Usage

```bash
# View usage records
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas -c \
  "SELECT * FROM usage_records ORDER BY timestamp DESC LIMIT 10;"
```

---

## 🧹 Part 10: Cleanup

### Stop GPU Instances

```bash
# Stop Azure instance
sky down mistral-azure -y

# Stop AWS instance  
sky down llama-aws -y

# Verify
sky status
```

### Stop Local Services

```bash
# Stop control plane and infrastructure
docker compose down

# Remove volumes (optional - deletes data)
docker compose down -v
```

---

## 🐛 Troubleshooting

### Control Plane Won't Start

```bash
# Check logs
docker compose logs control-plane

# Verify database
docker compose exec postgres psql -U crosslogic -c "SELECT 1"

# Verify Redis
docker compose exec redis redis-cli ping
```

### GPU Instance Fails to Launch

```bash
# Check SkyPilot status
sky status

# View detailed logs
sky logs <cluster-name> -f

# Check cloud credentials
sky check
```

### Model Not Loading from R2

```bash
# Verify R2 credentials
aws s3 ls s3://$R2_BUCKET/ --endpoint-url $R2_ENDPOINT

# Check vLLM logs for S3 errors
sky logs <cluster-name> | grep -i s3

# Test direct download
aws s3 cp s3://$R2_BUCKET/mistralai/Mistral-7B-Instruct-v0.3/config.json . \
  --endpoint-url $R2_ENDPOINT
```

### API Requests Failing

```bash
# Test API key
curl -v http://localhost:8080/v1/models \
  -H "Authorization: Bearer $API_KEY"

# Check node health
curl http://$AZURE_IP:8000/health
curl http://$AWS_IP:8000/health

# Verify node registration
curl -H "X-Admin-Token: $ADMIN_API_TOKEN" \
  http://localhost:8080/admin/nodes
```

---

## 🎉 Success!

If all steps completed successfully, you now have:

✅ **Local control plane** running with dashboard  
✅ **2 GPU nodes** (Azure A10 + AWS T4) serving models  
✅ **2 models** (Mistral 7B + Llama 3 8B) stored in R2  
✅ **Fast model loading** (30-60s from R2)  
✅ **Multi-cloud routing** through control plane  
✅ **Zero bandwidth costs** with Cloudflare R2  

## 📚 Next Steps

1. **Load Testing**: Use `tests/k6/load_test.js`
2. **Monitoring**: Set up Prometheus + Grafana
3. **Production**: Deploy control plane to cloud
4. **Scaling**: Add more GPU nodes
5. **Billing**: Enable Stripe integration

---

## 📞 Need Help?

- **Logs**: Always check logs first
- **Documentation**: See `docs/` folder
- **GitHub Issues**: Open an issue
- **Discord**: Join our community

---

**Time to Complete**: ~2 hours (excluding model uploads)

**Cost Estimate**:
- Azure A10 spot: ~$1-2/hour
- AWS g4dn spot: ~$0.30-0.60/hour
- R2 storage: ~$0.02/day

**Happy Testing!** 🚀

