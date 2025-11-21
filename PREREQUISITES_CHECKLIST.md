# ✅ Prerequisites Checklist - What You Need Before Starting

## 🎯 Quick Summary

To run this platform locally and test with real GPU instances, you need:

1. **3 cloud accounts** (Azure, AWS, Cloudflare R2)
2. **1 HuggingFace account**
3. **6 tools** installed locally
4. **~2 hours** of your time

**Estimated costs during testing**: $5-10 (GPU spot instances)

---

## 📝 Part 1: Accounts & Credentials (30 minutes setup)

### 1.1 Cloudflare R2 (REQUIRED - for model storage)

**Why**: Store models with zero egress fees

**Setup Steps**:
1. Go to https://dash.cloudflare.com/
2. Sign up (free tier available)
3. Navigate to **R2** in sidebar
4. Click **"Create bucket"**
   - Name: `crosslogic-models`
   - Location: Choose closest to your GPU regions
5. Click **"Manage R2 API Tokens"** → **"Create API Token"**
   - Name: `crosslogic-api`
   - Permissions: **Object Read & Write**
   - TTL: Forever (or set expiration)
6. **Save these values**:

```bash
Account ID: _________________________________ (from URL)
R2 Access Key: ______________________________ (looks like: abc123...)
R2 Secret Key: ______________________________ (looks like: xyz789...)
```

**You'll need**: `R2_ENDPOINT`, `R2_ACCESS_KEY`, `R2_SECRET_KEY` in `.env`

**Cost**: FREE for first 10GB, then $0.015/GB/month (storage only, zero egress)

---

### 1.2 HuggingFace (REQUIRED - for downloading models)

**Why**: Download Mistral 7B and Llama 3 models

**Setup Steps**:
1. Go to https://huggingface.co/
2. Sign up (free)
3. Go to https://huggingface.co/settings/tokens
4. Click **"New token"**
   - Name: `crosslogic-downloads`
   - Type: **Read**
5. Accept model licenses:
   - Mistral: https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3
   - Llama 3: https://huggingface.co/meta-llama/Meta-Llama-3-8B-Instruct
6. **Save this value**:

```bash
HuggingFace Token: hf_________________________________
```

**You'll need**: `HUGGINGFACE_TOKEN` in `.env`

**Cost**: FREE

---

### 1.3 Azure (REQUIRED - for first GPU instance)

**Why**: Test with Azure A10 GPU (high performance)

**Setup Steps**:
1. Go to https://azure.microsoft.com/
2. Sign up (free credits available)
3. **Option A: Use Azure CLI (easier)**:
   ```bash
   az login
   # That's it! SkyPilot will use your login
   ```
4. **Option B: Create Service Principal** (for production):
   ```bash
   az ad sp create-for-rbac --name crosslogic-sp --role Contributor
   ```
5. **Save these values**:

```bash
# If using Option A (az login):
Just run: az login

# If using Option B (service principal):
Subscription ID: ________________________________
Tenant ID: ______________________________________
Client ID: ______________________________________
Client Secret: __________________________________
```

**You'll need**: Azure credentials in `.env` (or just `az login`)

**Cost**: ~$1-2/hour for A10 spot instance

---

### 1.4 AWS (REQUIRED - for second GPU instance)

**Why**: Test with AWS T4 GPU (cost effective)

**Setup Steps**:
1. Go to https://aws.amazon.com/
2. Sign up
3. Go to **IAM** → **Users** → **Create user**
   - Name: `crosslogic-gpu`
   - Attach policy: `AmazonEC2FullAccess`
4. Go to **Security credentials** → **Create access key**
   - Use case: **CLI**
5. **Save these values**:

```bash
AWS Access Key: _________________________________
AWS Secret Key: _________________________________
```

**You'll need**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` in `.env`

**Cost**: ~$0.30-0.60/hour for g4dn spot instance

---

### 1.5 Stripe (OPTIONAL - for billing testing)

**Why**: Test billing features (can skip for now)

**Setup Steps**:
1. Go to https://stripe.com/
2. Sign up
3. Stay in **Test mode**
4. Go to **Developers** → **API keys**
5. Copy **Secret key** (starts with `sk_test_`)
6. Go to **Webhooks** → **Add endpoint**
   - URL: `http://localhost:8080/api/webhooks/stripe` (temporary)
   - Events: Select all
   - Copy **Signing secret** (starts with `whsec_`)

**You'll need**: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` in `.env`

**For now**: Use dummy values, set `BILLING_ENABLED=false`

---

## 🛠️ Part 2: Install Required Tools (15 minutes)

### 2.1 Docker & Docker Compose (REQUIRED)

```bash
# Check if installed
docker --version          # Need: 24.0+
docker compose version    # Need: v2.0+

# If not installed:
# macOS: Install Docker Desktop from https://docker.com/
# Linux: curl -fsSL https://get.docker.com | sh
```

---

### 2.2 Go (REQUIRED - for building control plane)

```bash
# Check if installed
go version  # Need: 1.22+

# If not installed:
# macOS: brew install go
# Linux: Download from https://go.dev/dl/
```

---

### 2.3 Node.js & npm (OPTIONAL - for dashboard)

```bash
# Check if installed
node --version  # Need: 18+
npm --version

# If not installed:
# macOS: brew install node
# Linux: Use nvm or package manager
```

---

### 2.4 Python 3 (REQUIRED - for upload scripts)

```bash
# Check if installed
python3 --version  # Need: 3.10+
pip3 --version

# Usually pre-installed on macOS/Linux
```

---

### 2.5 AWS CLI (REQUIRED - for R2 access)

```bash
# Check if installed
aws --version

# If not installed:
pip3 install awscli

# Or on macOS:
brew install awscli
```

---

### 2.6 Azure CLI (REQUIRED - for Azure GPU)

```bash
# Check if installed
az --version

# If not installed:
# macOS: brew install azure-cli
# Linux: curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
```

---

### 2.7 SkyPilot (REQUIRED - for GPU orchestration)

```bash
# Install with Azure and AWS support
pip3 install "skypilot[azure,aws]"

# Verify installation
sky check

# This will show you what's missing
```

---

## 📋 Part 3: Fill Out .env File

Use this template and fill in YOUR values:

```bash
# Copy this to your .env file in project root

# ============================================
# Cloudflare R2 (REQUIRED)
# ============================================
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=YOUR_R2_ACCESS_KEY_FROM_STEP_1.1
R2_SECRET_KEY=YOUR_R2_SECRET_KEY_FROM_STEP_1.1

# ============================================
# HuggingFace (REQUIRED)
# ============================================
HUGGINGFACE_TOKEN=hf_YOUR_TOKEN_FROM_STEP_1.2

# ============================================
# AWS (REQUIRED)
# ============================================
AWS_ACCESS_KEY_ID=YOUR_AWS_KEY_FROM_STEP_1.4
AWS_SECRET_ACCESS_KEY=YOUR_AWS_SECRET_FROM_STEP_1.4
AWS_DEFAULT_REGION=us-east-1

# ============================================
# Azure (REQUIRED - Option A or B)
# ============================================
# Option A: Just run 'az login' and leave these blank
# Option B: Fill these in if using service principal
AZURE_SUBSCRIPTION_ID=
AZURE_TENANT_ID=
AZURE_CLIENT_ID=
AZURE_CLIENT_SECRET=

# ============================================
# Database (Use these defaults for local)
# ============================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=crosslogic
DB_PASSWORD=my_secure_password_123
DB_NAME=crosslogic_iaas
DB_SSL_MODE=disable

# ============================================
# Redis (Use these defaults for local)
# ============================================
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# ============================================
# Server (Use these defaults for local)
# ============================================
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
CONTROL_PLANE_URL=http://localhost:8080

# ============================================
# Security (Generate these)
# ============================================
ADMIN_API_TOKEN=cl_admin_$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)

# ============================================
# Runtime Versions (Use these defaults)
# ============================================
VLLM_VERSION=0.6.2
TORCH_VERSION=2.4.0

# ============================================
# Billing (Optional - disable for now)
# ============================================
BILLING_ENABLED=false
STRIPE_SECRET_KEY=sk_test_dummy
STRIPE_WEBHOOK_SECRET=whsec_dummy

# ============================================
# Monitoring (Use these defaults)
# ============================================
MONITORING_ENABLED=true
LOG_LEVEL=debug
```

---

## ✅ Final Checklist Before Starting

### Accounts & Credentials
- [ ] Cloudflare R2 bucket created
- [ ] R2 API token created and saved
- [ ] HuggingFace account created
- [ ] HuggingFace token created
- [ ] Mistral 7B license accepted
- [ ] Llama 3 license accepted
- [ ] Azure account ready (az login OR service principal)
- [ ] AWS credentials created
- [ ] Stripe account created (optional)

### Tools Installed
- [ ] Docker & Docker Compose (v24+, v2+)
- [ ] Go (1.22+)
- [ ] Node.js & npm (18+) - optional for dashboard
- [ ] Python 3 (3.10+)
- [ ] AWS CLI
- [ ] Azure CLI
- [ ] SkyPilot (`sky check` passes)

### Configuration
- [ ] `.env` file created in project root
- [ ] All REQUIRED fields filled in
- [ ] Credentials tested (can login/access services)

### Time & Budget
- [ ] Have ~2 hours free
- [ ] Prepared for ~$5-10 in GPU costs
- [ ] Can monitor and stop instances

---

## 🎯 What You'll Get

After completing the setup with these prerequisites:

✅ **Local control plane** running on Docker  
✅ **Dashboard** accessible at http://localhost:3000  
✅ **Azure GPU node** with Mistral 7B  
✅ **AWS GPU node** with Llama 3 8B  
✅ **Models in R2** with zero egress fees  
✅ **API endpoint** to test chat completions  
✅ **Multi-cloud routing** through control plane  

---

## 🆘 Having Trouble?

### Can't create R2 bucket?
- Make sure you're signed in to Cloudflare
- R2 is available in the free tier
- Try a different bucket name if taken

### Can't get HuggingFace token?
- Make sure you're logged in
- Token must have "Read" access
- Accept model licenses before downloading

### Can't access Azure/AWS?
- Check your subscription is active
- Verify billing is enabled
- Check you have permission to create GPU instances
- Try `sky check` to diagnose

### SkyPilot not working?
```bash
# Reinstall with all dependencies
pip3 uninstall skypilot
pip3 install "skypilot[azure,aws]"

# Check setup
sky check

# Follow the prompts to configure each cloud
```

---

## 📞 Need Help?

- **Check logs first**: Most issues are credential/permission related
- **Documentation**: See `LOCAL_SETUP_GUIDE.md` for detailed steps
- **GitHub Issues**: Open an issue if stuck
- **Community**: Join our Discord

---

## ⏱️ Time Estimates

- **Account setup**: 30 minutes
- **Tool installation**: 15 minutes
- **Configuration**: 15 minutes
- **Local services**: 15 minutes
- **Model uploads**: 60 minutes (mostly waiting)
- **GPU instances**: 20 minutes
- **Testing**: 20 minutes

**Total**: ~2.5 hours (including model upload wait time)

---

**Ready to start?** 

Follow `LOCAL_SETUP_GUIDE.md` step by step! 🚀


