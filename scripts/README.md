# Model Upload Scripts

## Quick Start

Upload models to Cloudflare R2 for ultra-fast vLLM loading with Run:ai Streamer.

### 1. Install Dependencies

```bash
pip install awscli huggingface-hub python-dotenv tqdm
```

### 2. Configure Credentials

Create or edit `.env` file in project root:

```bash
# Cloudflare R2
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=your_r2_access_key
R2_SECRET_KEY=your_r2_secret_key

# HuggingFace
HUGGINGFACE_TOKEN=hf_your_token_here
```

### 3. Test Connection

```bash
python scripts/test-r2-connection.py
```

**Expected output:**
```
✓ Loaded .env from /path/to/.env
✓ All credentials found!
✓ AWS CLI installed
✓ Successfully connected to R2 bucket
✓ HuggingFace token valid
✅ All tests passed!
```

### 4. Upload Model

```bash
# Simple - credentials loaded from .env automatically
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct
```

**That's it!** No need to pass credentials explicitly.

## Available Scripts

### `test-r2-connection.py`

Tests your R2 connection and credentials before uploading.

**Usage:**
```bash
python scripts/test-r2-connection.py
```

**What it checks:**
- ✓ Loads credentials from `.env`
- ✓ Verifies all required variables are set
- ✓ Tests AWS CLI is installed
- ✓ Tests R2 connection
- ✓ Lists bucket contents
- ✓ Validates HuggingFace token

### `upload-model-to-r2.py`

Uploads HuggingFace models to R2 in safetensors format.

**Simple usage (recommended):**
```bash
# Credentials loaded from .env automatically
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct
```

**Advanced usage:**
```bash
# Override credentials if needed
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct \
  --hf-token hf_custom_token \
  --r2-endpoint https://custom.r2.cloudflarestorage.com \
  --r2-bucket custom-bucket
```

**Run in background:**
```bash
nohup python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct > upload.log 2>&1 &

# Monitor progress
tail -f upload.log
```

**What it does:**
1. Loads credentials from `.env`
2. Downloads model from HuggingFace (safetensors format only)
3. Verifies safetensors files exist
4. Uploads to R2 using AWS CLI
5. Verifies upload succeeded
6. Shows usage instructions

### `benchmark-model-loading.py`

Benchmarks vLLM loading performance: Standard vs Run:ai Streamer.

**Usage:**
```bash
python scripts/benchmark-model-loading.py s3://models/meta-llama/Meta-Llama-3-8B-Instruct
```

**Features:**
- Compares standard vs Run:ai Streamer loading
- Reports speedup metrics
- Extrapolates cost savings

## Recommended Models

### Small Models (7B-13B) - Great for Testing

```bash
# Llama 3 8B (~16GB, 20-30 min upload)
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct

# Mistral 7B (~14GB, 15-25 min upload)
python scripts/upload-model-to-r2.py mistralai/Mistral-7B-Instruct-v0.3

# Qwen 7B (~14GB, 15-25 min upload)
python scripts/upload-model-to-r2.py Qwen/Qwen2.5-7B-Instruct

# Gemma 7B (~14GB, 15-25 min upload)
python scripts/upload-model-to-r2.py google/gemma-7b-it
```

### Large Models (30B-70B) - Production Use

```bash
# Llama 3 70B (~140GB, 2-3 hours upload)
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-70B-Instruct

# Mixtral 8x7B (~90GB, 1-2 hours upload)
python scripts/upload-model-to-r2.py mistralai/Mixtral-8x7B-Instruct-v0.1

# Qwen 72B (~145GB, 2-3 hours upload)
python scripts/upload-model-to-r2.py Qwen/Qwen2.5-72B-Instruct
```

## Troubleshooting

### Error: "No .env file found"

**Solution:** Create `.env` file in project root:
```bash
cd /path/to/crosslogic-ai-iaas
cp config/env.example .env
nano .env  # Edit with your credentials
```

### Error: "AWS CLI not installed"

**Solution:**
```bash
pip install awscli
aws --version  # Verify installation
```

### Error: "Failed to connect to R2"

**Possible causes:**
1. Wrong R2_ENDPOINT format
   - Should be: `https://ACCOUNT_ID.r2.cloudflarestorage.com`
   - NOT: `https://ACCOUNT_ID.r2.dev` or `https://BUCKET.r2.dev`

2. Wrong credentials
   - Verify in Cloudflare Dashboard → R2 → Manage R2 API Tokens
   - Make sure token has Read & Write permissions

3. Wrong bucket name
   - Check bucket exists in R2 dashboard
   - Bucket name is case-sensitive

**Test connection:**
```bash
python scripts/test-r2-connection.py
```

### Error: "HuggingFace token invalid"

**Solution:**
1. Get token from: https://huggingface.co/settings/tokens
2. Create token with "Read" access
3. Accept model licenses:
   - Llama: https://huggingface.co/meta-llama/Meta-Llama-3-8B-Instruct
   - Mistral: https://huggingface.co/mistralai/Mistral-7B-Instruct-v0.3

### Error: "No safetensors files found"

**Cause:** Model is only available in PyTorch format (.bin or .pt files)

**Solution:** Most modern models have safetensors. Check model card on HuggingFace. If not available, the script will warn but continue upload.

### Upload is slow

**Expected:** 20-30 minutes for 7B models (~15GB)

**Factors:**
- Your internet upload speed
- Model size
- Cloudflare R2 region

**Tips:**
- Run in background: `nohup ... &`
- Use faster internet connection
- Upload during off-peak hours

## Performance with Run:ai Streamer

After uploading models, vLLM will load them using Run:ai Streamer:

| Model Size | Upload Time | First Load | Subsequent Loads |
|-----------|-------------|------------|------------------|
| 7B | 15-25 min | 4-8s | < 1s (cached) |
| 13B | 25-40 min | 6-12s | < 1s (cached) |
| 70B | 2-3 hours | 15-25s | < 1s (cached) |

**Compare to HuggingFace download:**
- 7B model: 5-10 minutes per launch
- 70B model: 30-45 minutes per launch

**Savings:** Upload once, load 50-180x faster every time!

## Verification

After upload, verify model is available:

```bash
# List models in R2
aws s3 ls s3://crosslogic-models/ \
  --endpoint-url $R2_ENDPOINT \
  --recursive | grep safetensors

# Check specific model
aws s3 ls s3://crosslogic-models/meta-llama/Meta-Llama-3-8B-Instruct/ \
  --endpoint-url $R2_ENDPOINT
```

## Next Steps

After uploading models:

1. **Launch GPU node** via dashboard or API
2. **vLLM automatically streams** from R2 with Run:ai Streamer
3. **Model loads in 4-23 seconds** (vs 5-10 minutes from HuggingFace)

See `UPDATED_LOCAL_SETUP.md` for full platform setup.

