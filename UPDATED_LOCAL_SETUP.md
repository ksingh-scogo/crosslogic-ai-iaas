# 🚀 Updated Local Setup Guide - 100% Dockerized

## What Changed?

Based on your excellent feedback, I've made these improvements:

### ✅ 1. Fully Containerized (No Local Dependencies!)

**Before**: Required Go, npm, Node.js locally  
**After**: Everything runs in Docker containers

- ✅ Control plane builds in Docker
- ✅ Dashboard builds in Docker
- ✅ Node agent builds in Docker
- ✅ **You only need Docker installed locally!**

### ✅ 2. UI-Driven Instance Management (No Manual CLI!)

**Before**: Manual `sky launch` commands  
**After**: Click "Launch" button in admin UI

- ✅ Admin UI lists models from R2
- ✅ Click "Launch" → API handles everything
- ✅ Real-time status updates
- ✅ No manual SkyPilot CLI operations

### ✅ 3. Dashboard in Docker Compose

**Before**: Dashboard missing from docker-compose  
**After**: Dashboard runs as a service

- ✅ Automatic startup with `docker compose up`
- ✅ Available at http://localhost:3000
- ✅ Connected to backend automatically

---

## 📋 Prerequisites (Simplified!)

### What You Need Locally

1. **Docker & Docker Compose** (that's it!)
   ```bash
   docker --version  # Need: 24+
   docker compose version  # Need: v2+
   ```

2. **Cloud Credentials** (for GPU instances)
   - Azure account
   - AWS account
   - Cloudflare R2 account
   - HuggingFace token

### What You DON'T Need Locally

❌ Go  
❌ Node.js/npm  
❌ Python (unless uploading models)  
❌ Manual SkyPilot CLI operations  

---

## 🎯 Quick Start (30 Minutes)

### Step 1: Create .env File (5 minutes)

```bash
cd /path/to/crosslogic-ai-iaas

cat > .env << 'EOF'
# Database
DB_PASSWORD=my_secure_password_123

# Security
ADMIN_API_TOKEN=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)

# Cloudflare R2
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=YOUR_R2_ACCESS_KEY
R2_SECRET_KEY=YOUR_R2_SECRET_KEY

# HuggingFace
HUGGINGFACE_TOKEN=hf_YOUR_TOKEN

# AWS (for SkyPilot)
AWS_ACCESS_KEY_ID=YOUR_AWS_KEY
AWS_SECRET_ACCESS_KEY=YOUR_AWS_SECRET

# Azure (for SkyPilot)
AZURE_SUBSCRIPTION_ID=YOUR_AZURE_SUB_ID
AZURE_TENANT_ID=YOUR_AZURE_TENANT_ID

# Billing (optional)
BILLING_ENABLED=false
EOF

# Edit and fill in YOUR credentials
nano .env  # or code .env
```

### Step 2: Build & Start Everything (5 minutes)

```bash
# Build all images
docker compose build

# Start all services
docker compose up -d

# Wait for services to be ready (~30 seconds)
sleep 30

# Verify services are running
docker compose ps

# Expected output:
# - control-plane (port 8080)
# - dashboard (port 3000)
# - postgres (port 5432)
# - redis (port 6379)
```

### Step 3: Run Migrations & Seed Data (2 minutes)

```bash
# Run database migrations
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas -f /docker-entrypoint-initdb.d/01_core_tables.sql

# Seed test data
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas << 'EOF'
-- Create test tenant
INSERT INTO tenants (id, name, email, status, billing_plan)
VALUES (gen_random_uuid(), 'Test Org', 'test@example.com', 'active', 'serverless');

-- Create environment
INSERT INTO environments (id, tenant_id, name, region, status)
SELECT gen_random_uuid(), id, 'production', 'us-east', 'active' FROM tenants LIMIT 1;

-- Add models
INSERT INTO models (id, name, family, size, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million, status)
VALUES 
(gen_random_uuid(), 'mistralai/Mistral-7B-Instruct-v0.3', 'Mistral', '7B', 'chat', 8192, 16, 0.05, 0.10, 'active'),
(gen_random_uuid(), 'meta-llama/Meta-Llama-3-8B-Instruct', 'Llama', '8B', 'chat', 8192, 16, 0.05, 0.10, 'active');
EOF
```

### Step 4: Access the Dashboard (instant!)

```bash
# Open in browser
open http://localhost:3000

# Or manually visit:
# http://localhost:3000
```

**You should see:**
- ✅ Admin dashboard with navigation
- ✅ Models list (from database)
- ✅ Launch instance button
- ✅ Node status (empty initially)

### Step 5: Upload Models to R2 (30 minutes - one time only)

```bash
# For this step only, you need Python locally
pip3 install awscli huggingface-hub

# Set credentials
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY

# Upload Mistral 7B
python3 scripts/upload-model-to-r2.py \
  mistralai/Mistral-7B-Instruct-v0.3 \
  --hf-token $HUGGINGFACE_TOKEN

# This takes 20-30 minutes, run in background:
# nohup python3 scripts/upload-model-to-r2.py ... &
```

---

## 🎮 Using the Admin UI (No CLI!)

### Launch GPU Instance from UI

1. **Open Dashboard**
   ```
   http://localhost:3000
   ```

2. **Go to "Models" Page**
   - See list of available models
   - Each model shows size, type, requirements

3. **Click "Launch Instance"**
   - Select model: Mistral 7B
   - Select cloud: Azure
   - Select region: East US
   - Select instance: Standard_NV36ads_A10_v5
   - Check "Use Spot"
   - Click "Launch"

4. **Watch Progress**
   - Status updates in real-time
   - Shows current stage:
     - ✓ Validating configuration
     - ✓ Requesting spot instance
     - → Provisioning instance (45%)
     - Installing dependencies
     - Starting vLLM
     - Registering node

5. **Instance Ready!**
   - Node appears in "Nodes" page
   - Status: Active
   - Endpoint: Auto-registered
   - Ready to serve requests

### Test from UI

1. **Go to "API Testing" Page**
2. **Enter prompt**: "What is quantum computing?"
3. **Select model**: Mistral 7B
4. **Click "Submit"**
5. **See response** in real-time

---

## 🔧 How It Works (Behind the Scenes)

### Architecture

```
Browser (http://localhost:3000)
    ↓
Dashboard Container (Next.js)
    ↓ API calls
Control Plane Container (Go)
    ↓ sky launch
SkyPilot (runs in control-plane container)
    ↓ provisions
Azure/AWS GPU Instance
    ↓ streams from
Cloudflare R2 (models)
```

### What Happens When You Click "Launch"

1. **Frontend** (`dashboard` container):
   - Sends POST to `/admin/instances/launch`
   - Payload: model, provider, region, instance type

2. **Backend** (`control-plane` container):
   - Receives request
   - Generates SkyPilot YAML
   - Executes `sky launch` command
   - Returns job ID

3. **SkyPilot** (inside `control-plane` container):
   - Provisions spot instance
   - Installs vLLM
   - Sets R2 credentials
   - Starts vLLM with S3 URL

4. **GPU Instance**:
   - vLLM streams model from R2
   - First load: 30-60 seconds
   - Registers with control plane
   - Ready to serve

5. **Frontend Updates**:
   - Polls `/admin/instances/status?job_id=xxx`
   - Shows real-time progress
   - Notifies when ready

---

## 📊 API Endpoints (For UI Integration)

### List Models from R2
```http
GET /admin/models/r2
Headers: X-Admin-Token: YOUR_ADMIN_TOKEN

Response:
{
  "models": [
    {
      "id": "uuid",
      "name": "mistralai/Mistral-7B-Instruct-v0.3",
      "family": "Mistral",
      "size": "7B",
      "vram_required_gb": 16
    }
  ]
}
```

### Launch Instance
```http
POST /admin/instances/launch
Headers: X-Admin-Token: YOUR_ADMIN_TOKEN
Body:
{
  "model_name": "mistralai/Mistral-7B-Instruct-v0.3",
  "provider": "azure",
  "region": "eastus",
  "instance_type": "Standard_NV36ads_A10_v5",
  "use_spot": true
}

Response:
{
  "status": "launching",
  "job_id": "launch-abc123",
  "estimated_time": "5-10 minutes"
}
```

### Check Launch Status
```http
GET /admin/instances/status?job_id=launch-abc123
Headers: X-Admin-Token: YOUR_ADMIN_TOKEN

Response:
{
  "job_id": "launch-abc123",
  "status": "in_progress",
  "stage": "provisioning_instance",
  "progress": 45,
  "stages": [
    "✓ Validating configuration",
    "✓ Requesting spot instance",
    "→ Provisioning instance (45%)",
    "  Installing dependencies",
    "  Starting vLLM",
    "  Registering node"
  ]
}
```

---

## 🐛 Troubleshooting

### Dashboard Not Loading

```bash
# Check if dashboard container is running
docker compose ps dashboard

# Check dashboard logs
docker compose logs dashboard

# Restart dashboard
docker compose restart dashboard
```

### Can't Launch Instance

```bash
# Check control plane logs
docker compose logs control-plane | grep -i launch

# Verify SkyPilot is configured
docker compose exec control-plane sky check

# Check cloud credentials
docker compose exec control-plane env | grep -i azure
docker compose exec control-plane env | grep -i aws
```

### Services Not Starting

```bash
# Check all logs
docker compose logs

# Restart everything
docker compose down
docker compose up -d

# Check health
curl http://localhost:8080/health
curl http://localhost:3000
```

---

## ✅ Updated Checklist

### Prerequisites
- [ ] Docker & Docker Compose installed (24+, v2+)
- [ ] `.env` file created with credentials
- [ ] Cloudflare R2 bucket created
- [ ] Models uploaded to R2

### Local Services
- [ ] `docker compose up -d` successful
- [ ] Control plane at http://localhost:8080/health
- [ ] Dashboard at http://localhost:3000
- [ ] Database seeded with models

### Launch Instance from UI
- [ ] Dashboard loads successfully
- [ ] Can see models list
- [ ] Click "Launch" triggers API
- [ ] Status updates show progress
- [ ] Instance appears in nodes list
- [ ] Can test via API testing page

---

## 🎉 Benefits of This Approach

### No Local Dependencies
✅ Just Docker - works on any OS  
✅ No version conflicts  
✅ Clean dev environment  
✅ Easy to onboard new developers  

### UI-Driven Operations
✅ No manual CLI commands  
✅ Visual feedback  
✅ Error handling in UI  
✅ Operational simplicity  

### Production-Ready
✅ Same setup for dev/staging/prod  
✅ Easy to scale  
✅ Proper containerization  
✅ Industry best practices  

---

## 📚 What's Next?

After testing locally:

1. **Production Deployment**
   - Deploy to Kubernetes
   - Use managed PostgreSQL
   - Add load balancer

2. **Enhanced UI**
   - WebSocket for real-time updates
   - Cost estimation
   - Resource utilization graphs

3. **Advanced Features**
   - Auto-scaling
   - Multi-region load balancing
   - Cost optimization

---

**Total Time to Start**: ~30 minutes  
**Local Dependencies**: Just Docker!  
**User Experience**: UI-driven, no CLI  

Let's test it! 🚀

