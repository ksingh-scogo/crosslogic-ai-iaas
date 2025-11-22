# 🚀 Simplified Model Upload Guide

## What Changed?

✅ **No more manual credential passing!** The upload script now automatically loads credentials from your `.env` file.

### Before (Complex)
```bash
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_KEY

python scripts/upload-model-to-r2.py \
  meta-llama/Meta-Llama-3-8B-Instruct \
  --hf-token $HUGGINGFACE_TOKEN \
  --r2-endpoint $R2_ENDPOINT \
  --r2-bucket $R2_BUCKET
```

### After (Simple)
```bash
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct
```

That's it! All credentials loaded from `.env` automatically.

---

## Quick Start (5 Minutes)

### Step 1: Install Dependencies

```bash
pip install awscli huggingface-hub python-dotenv tqdm
```

### Step 2: Verify .env File

Ensure your `.env` file (in project root) has these values:

```bash
# Cloudflare R2
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=your_r2_access_key
R2_SECRET_KEY=your_r2_secret_key

# HuggingFace
HUGGINGFACE_TOKEN=hf_your_token_here
```

**Don't have a .env file?** Copy the template:
```bash
cp config/env.example .env
nano .env  # Edit with your credentials
```

### Step 3: Test Connection

```bash
python scripts/test-r2-connection.py
```

**Expected output:**
```
✓ Loaded .env from /path/to/.env
✓ All credentials found!
✓ AWS CLI installed
✓ Successfully connected to R2 bucket: crosslogic-models
✓ HuggingFace token valid
✅ All tests passed! Ready to upload models.
```

### Step 4: Upload Model

```bash
# Upload Llama 3 8B (~16GB, takes 20-30 minutes)
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct
```

**Run in background:**
```bash
nohup python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct > upload.log 2>&1 &

# Monitor progress in another terminal
tail -f upload.log
```

---

## What the Script Does

1. **Loads credentials** from `.env` automatically
2. **Validates** all credentials before starting
3. **Downloads** model from HuggingFace (safetensors only)
4. **Verifies** safetensors files exist (required for Run:ai Streamer)
5. **Uploads** to R2 using AWS CLI
6. **Confirms** upload succeeded
7. **Shows** usage instructions

---

## Output Example

```
✓ Loaded credentials from /path/to/.env

🔍 Validating credentials...
✓ HuggingFace token found
✓ R2 endpoint: https://abc123.r2.cloudflarestorage.com
✓ R2 credentials configured
✓ R2 bucket: crosslogic-models
✓ All credentials validated!

✓ AWS CLI found: aws-cli/2.13.0

🚀 Uploading meta-llama/Meta-Llama-3-8B-Instruct to Cloudflare R2
   Format: safetensors (required for Run:ai Model Streamer)

📥 Downloading from HuggingFace...
✓ Downloaded to /tmp/model-cache/models--meta-llama--Meta-Llama-3-8B-Instruct

🔍 Verifying safetensors format...
✓ Found 4 safetensors files
  Compatible with Run:ai Model Streamer for ultra-fast loading

📊 Model size: 15.87 GB

📤 Uploading to R2: s3://crosslogic-models/meta-llama/Meta-Llama-3-8B-Instruct
  This may take 15-30 minutes depending on model size...
✓ Upload complete!

🔍 Verifying upload...
✓ Verified: 8 files in R2

✅ Model uploaded successfully!

📝 Usage in vLLM with Run:ai Streamer:
  python -m vllm.entrypoints.openai.api_server \
    --model s3://crosslogic-models/meta-llama/Meta-Llama-3-8B-Instruct \
    --load-format runai_streamer \
    --model-loader-extra-config '{"concurrency": 32}'

⚡ Performance with Run:ai Streamer:
  - First load: ~4-23s (ultra-fast parallel streaming)
  - Standard S3: ~30-60s (5-10x slower)
  - HuggingFace: ~5-10 minutes (50-180x slower)

💡 Tip: Run:ai Streamer streams directly to GPU memory
   No disk caching needed - maximum performance!
```

---

## Recommended Models for Testing

```bash
# Start with Llama 3 8B (best tested)
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct

# Or Mistral 7B (faster download)
python scripts/upload-model-to-r2.py mistralai/Mistral-7B-Instruct-v0.3

# Or Qwen 7B (multilingual)
python scripts/upload-model-to-r2.py Qwen/Qwen2.5-7B-Instruct
```

**Note:** First model takes 20-30 minutes to upload. Subsequent uploads are faster due to caching.

---

## Troubleshooting

### ❌ "No .env file found"

```bash
cd /path/to/crosslogic-ai-iaas
cp config/env.example .env
nano .env  # Add your credentials
```

### ❌ "AWS CLI not installed"

```bash
pip install awscli
aws --version  # Verify
```

### ❌ "HuggingFace token not found"

Check your `.env` file has:
```bash
HUGGINGFACE_TOKEN=hf_your_actual_token_here
```

Get token from: https://huggingface.co/settings/tokens

### ❌ "R2_ENDPOINT not set"

Check your `.env` file has:
```bash
R2_ENDPOINT=https://YOUR_ACCOUNT_ID.r2.cloudflarestorage.com
```

**Important:** Use your actual account ID, not "YOUR_ACCOUNT_ID"

Find it in: Cloudflare Dashboard → R2 → Settings

### ❌ "Failed to connect to R2"

Run test script to diagnose:
```bash
python scripts/test-r2-connection.py
```

Common issues:
- Wrong endpoint URL format
- Invalid credentials
- Bucket doesn't exist
- Typo in bucket name

### ⚠️ "No safetensors files found"

The model you're trying to upload doesn't have safetensors format. Run:ai Streamer requires safetensors.

**Solution:** Choose a different model that has safetensors, or the script will upload anyway with a warning (but Run:ai Streamer won't work).

---

## After Upload: Launch GPU Node

Once model is uploaded, launch a node from the dashboard:

1. **Open Dashboard**: http://localhost:3000
2. **Go to Models page**
3. **Click "Launch Instance"**
4. **Select your uploaded model**
5. **vLLM loads in 4-23 seconds!** 🚀

Or via API:
```bash
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "aws",
    "region": "us-east-1",
    "gpu": "A10G",
    "model": "meta-llama/Meta-Llama-3-8B-Instruct",
    "use_spot": true
  }'
```

---

## Performance Comparison

| Method | Load Time | Upload Once? | Cost per Launch |
|--------|-----------|--------------|-----------------|
| HuggingFace download | 5-10 min | ❌ No (every launch) | ~$0.50 bandwidth |
| Standard S3 | 30-60s | ✅ Yes | $0 (CDN) |
| **Run:ai Streamer** | **4-23s** | ✅ Yes | $0 (CDN) |

**ROI:** Upload takes 20-30 min once, saves 5-10 min per launch forever!

**For 100 launches:**
- Time saved: 8-17 hours
- Cost saved: ~$50 in bandwidth

---

## Summary

✅ **Simplified:** No manual credential passing  
✅ **Automatic:** Loads from `.env` file  
✅ **Validated:** Tests credentials before starting  
✅ **Verified:** Confirms upload succeeded  
✅ **Fast:** 50-180x faster loading with Run:ai Streamer  

**Total setup time:** 5 minutes + upload time (20-30 min per model)

---

**Ready to upload?** Run:
```bash
python scripts/test-r2-connection.py  # Test first
python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct
```

🎉 Done! Your models will now load in 4-23 seconds with Run:ai Streamer!

