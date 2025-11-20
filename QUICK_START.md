# 🚀 Quick Start - 5 Minutes to Running System

## What You Need

1. **Docker** (that's it!)
   ```bash
   docker --version  # Need 24+
   docker compose version  # Need v2+
   ```

2. **Cloud Accounts** (for GPU instances)
   - Azure or AWS
   - Cloudflare R2
   - HuggingFace

---

## Step 1: Clone & Configure (3 minutes)

```bash
# Clone repo
cd /path/to/crosslogic-ai-iaas

# Create .env file
cat > .env << 'EOF'
# Database
DB_PASSWORD=supersecret123

# Security (generate with: openssl rand -hex 32)
ADMIN_API_TOKEN=your_admin_token_here
JWT_SECRET=your_jwt_secret_here

# Cloudflare R2
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=your_r2_access_key
R2_SECRET_KEY=your_r2_secret_key

# Cloud Providers (at least one)
AWS_ACCESS_KEY_ID=your_aws_key
AWS_SECRET_ACCESS_KEY=your_aws_secret
# OR
AZURE_SUBSCRIPTION_ID=your_azure_sub
AZURE_TENANT_ID=your_azure_tenant

# Billing (optional)
BILLING_ENABLED=false
EOF

# Edit with your credentials
nano .env
```

---

## Step 2: Start Everything (2 minutes)

```bash
# Build and start all services
docker compose up -d

# Wait for services to be ready
sleep 30

# Check status
docker compose ps

# Expected output:
# NAME                  STATUS
# control-plane         Up (healthy)
# dashboard             Up
# postgres              Up (healthy)
# redis                 Up (healthy)
```

---

## Step 3: Access Dashboard (instant!)

```bash
# Open dashboard
open http://localhost:3000

# You should see:
✅ Admin Dashboard
✅ Navigation menu
✅ Models page
✅ Nodes page
```

---

## Step 4: Seed Database (1 minute)

```bash
# Add sample models
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas << 'EOF'
-- Add models
INSERT INTO models (id, name, family, size, type, context_length, vram_required_gb, price_input_per_million, price_output_per_million, status)
VALUES 
(gen_random_uuid(), 'mistralai/Mistral-7B-Instruct-v0.3', 'Mistral', '7B', 'chat', 8192, 16, 0.05, 0.10, 'active'),
(gen_random_uuid(), 'meta-llama/Meta-Llama-3-8B-Instruct', 'Llama', '8B', 'chat', 8192, 16, 0.05, 0.10, 'active');

-- Create test tenant
INSERT INTO tenants (id, name, email, status, billing_plan)
VALUES (gen_random_uuid(), 'Test Org', 'test@example.com', 'active', 'serverless');
EOF

# Refresh dashboard - you should see 2 models!
```

---

## Step 5: Upload Model to R2 (optional - 30 min)

```bash
# Install Python dependencies (one-time)
pip3 install awscli huggingface-hub

# Upload Mistral 7B
python3 scripts/upload-model-to-r2.py \
  mistralai/Mistral-7B-Instruct-v0.3 \
  --hf-token YOUR_HF_TOKEN

# This takes 20-30 minutes
# Can run in background: nohup python3 ... &
```

---

## ✅ You're Ready!

### What's Running?

- ✅ **Control Plane**: http://localhost:8080
- ✅ **Dashboard**: http://localhost:3000
- ✅ **PostgreSQL**: localhost:5432
- ✅ **Redis**: localhost:6379

### What You Can Do Now?

1. **View Models**
   - Go to http://localhost:3000/launch
   - See available models

2. **Launch GPU Instance**
   - Click "Launch Instance"
   - Select model, provider, region
   - Click "Launch"
   - Watch real-time progress!

3. **Monitor Nodes**
   - Go to "Nodes" page
   - See active instances
   - Monitor usage

---

## 🧪 Test the APIs

### Health Check
```bash
curl http://localhost:8080/health
# {"status": "healthy"}
```

### List Models
```bash
curl -H "X-Admin-Token: YOUR_ADMIN_TOKEN" \
  http://localhost:8080/admin/models/r2
```

### Launch Instance
```bash
curl -X POST \
  -H "X-Admin-Token: YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "mistralai/Mistral-7B-Instruct-v0.3",
    "provider": "azure",
    "region": "eastus",
    "instance_type": "Standard_NV36ads_A10_v5",
    "use_spot": true
  }' \
  http://localhost:8080/admin/instances/launch
```

---

## 🐛 Troubleshooting

### Services not starting?
```bash
# Check logs
docker compose logs

# Restart
docker compose down
docker compose up -d
```

### Dashboard not loading?
```bash
# Check dashboard logs
docker compose logs dashboard

# Rebuild dashboard
docker compose build dashboard
docker compose up -d dashboard
```

### Can't connect to control plane?
```bash
# Check health
curl http://localhost:8080/health

# Check logs
docker compose logs control-plane
```

---

## 📚 Next Steps

1. **Follow Full Guide**: See `UPDATED_LOCAL_SETUP.md`
2. **Launch Real Instance**: Follow Azure/AWS setup
3. **Test LLM Inference**: Submit chat requests
4. **Monitor Usage**: Check billing/metrics

---

## 🎯 What You Get

✅ **Zero local dependencies** (just Docker)  
✅ **UI-driven operations** (no CLI needed)  
✅ **Real-time monitoring** (visual dashboard)  
✅ **Production-ready** (containerized, scalable)  

**Total Time**: 5-10 minutes  
**Complexity**: Minimal  
**Experience**: Professional  

Let's go! 🚀

