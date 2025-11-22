#!/usr/bin/env python3
"""
Upload HuggingFace models directly to Cloudflare R2.

vLLM will stream models from R2 using native S3 support - no JuiceFS needed!

Usage:
    # Simple (loads credentials from .env file automatically)
    python upload-model-to-r2.py meta-llama/Llama-3-8B-Instruct
    
    # Or override credentials
    python upload-model-to-r2.py meta-llama/Llama-3-8B-Instruct --hf-token YOUR_TOKEN
"""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

# Load .env file if it exists
def load_env_file(env_path):
    """Load .env file manually if python-dotenv is not available"""
    if not env_path.exists():
        return False
    
    with open(env_path) as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#') and '=' in line:
                key, value = line.split('=', 1)
                # Remove quotes if present
                value = value.strip().strip('"').strip("'")
                os.environ[key] = value
    return True

# Try to load .env file
env_path = Path(__file__).parent.parent / ".env"

try:
    from dotenv import load_dotenv
    if env_path.exists():
        load_dotenv(env_path)
        print(f"✓ Loaded credentials from {env_path}")
    else:
        print(f"⚠️  No .env file found at {env_path}")
        print(f"   Will use environment variables or command-line arguments")
except ImportError:
    # Use fallback parser (works perfectly fine)
    if load_env_file(env_path):
        print(f"✓ Loaded credentials from {env_path}")
    else:
        print(f"⚠️  No .env file found at {env_path}")
        print(f"   Will use environment variables or command-line arguments")

try:
    from huggingface_hub import snapshot_download
    from tqdm import tqdm
except ImportError:
    print("Error: Required packages not found. Install with:")
    print("  pip install huggingface-hub tqdm python-dotenv")
    sys.exit(1)


def upload_model(model_id: str, hf_token: str, r2_endpoint: str, r2_bucket: str):
    """Upload model from HuggingFace to R2 in safetensors format for Run:ai Streamer"""
    
    print(f"\n🚀 Uploading {model_id} to Cloudflare R2")
    print("   Format: safetensors (required for Run:ai Model Streamer)\n")
    
    # Step 1: Download from HuggingFace (prefer safetensors format)
    print(f"📥 Downloading from HuggingFace...")
    try:
        local_path = snapshot_download(
            repo_id=model_id,
            token=hf_token,
            cache_dir="/tmp/model-cache",
            ignore_patterns=["*.bin", "*.pt"],  # Skip PyTorch files, prefer safetensors
        )
        print(f"✓ Downloaded to {local_path}")
    except Exception as e:
        print(f"✗ Download failed: {e}")
        sys.exit(1)
    
    # Step 1.5: Verify safetensors format (required for Run:ai Streamer)
    print(f"\n🔍 Verifying safetensors format...")
    safetensors_files = list(Path(local_path).glob("*.safetensors"))
    if not safetensors_files:
        print(f"⚠️  WARNING: No safetensors files found!")
        print(f"   Run:ai Streamer requires models in safetensors format.")
        print(f"   This model may not work with Run:ai Streamer.")
        print(f"   Continuing upload anyway...")
    else:
        print(f"✓ Found {len(safetensors_files)} safetensors files")
        print(f"  Compatible with Run:ai Model Streamer for ultra-fast loading")
    
    # Step 2: Calculate size
    total_size = sum(f.stat().st_size for f in Path(local_path).rglob("*") if f.is_file())
    print(f"📊 Model size: {total_size / 1e9:.2f} GB")
    
    # Step 3: Upload to R2 using AWS CLI
    # vLLM expects models at: s3://bucket/model-id/
    s3_path = f"s3://{r2_bucket}/{model_id}"
    
    print(f"\n📤 Uploading to R2: {s3_path}")
    print("  This may take 15-30 minutes depending on model size...")
    
    cmd = [
        "aws", "s3", "sync",
        local_path + "/",
        s3_path + "/",
        "--endpoint-url", r2_endpoint,
        "--no-progress",  # Cleaner output
    ]
    
    try:
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        print("✓ Upload complete!")
    except subprocess.CalledProcessError as e:
        print(f"✗ Upload failed: {e}")
        print(f"stderr: {e.stderr}")
        sys.exit(1)
    
    # Step 4: Verify upload
    print(f"\n🔍 Verifying upload...")
    verify_cmd = ["aws", "s3", "ls", s3_path + "/", "--endpoint-url", r2_endpoint]
    try:
        result = subprocess.run(verify_cmd, check=True, capture_output=True, text=True)
        file_count = len(result.stdout.strip().split("\n"))
        print(f"✓ Verified: {file_count} files in R2")
    except subprocess.CalledProcessError:
        print("⚠️  Could not verify upload (but it may have succeeded)")
    
    # Step 5: Show usage
    print(f"\n✅ Model uploaded successfully!")
    print(f"\n📝 Usage in vLLM with Run:ai Streamer:")
    print(f"  python -m vllm.entrypoints.openai.api_server \\")
    print(f"    --model s3://{r2_bucket}/{model_id} \\")
    print(f"    --load-format runai_streamer \\")
    print(f"    --model-loader-extra-config '{{\"concurrency\": 32}}'")
    print(f"\n📝 Or use in CrossLogic control plane:")
    print(f"  Model name: {model_id}")
    print(f"  vLLM will automatically stream from R2 with Run:ai Streamer")
    print(f"\n⚡ Performance with Run:ai Streamer:")
    print(f"  - First load: ~4-23s (ultra-fast parallel streaming)")
    print(f"  - Standard S3: ~30-60s (5-10x slower)")
    print(f"  - HuggingFace: ~5-10 minutes (50-180x slower)")
    print(f"\n💡 Tip: Run:ai Streamer streams directly to GPU memory")
    print(f"   No disk caching needed - maximum performance!")


def main():
    parser = argparse.ArgumentParser(
        description="Upload HuggingFace models to Cloudflare R2 for fast vLLM loading",
        epilog="Credentials are loaded from .env file automatically if present."
    )
    parser.add_argument(
        "model_id",
        help="HuggingFace model ID (e.g., meta-llama/Llama-3-8B-Instruct)",
    )
    parser.add_argument(
        "--hf-token",
        default=os.getenv("HUGGINGFACE_TOKEN") or os.getenv("HF_TOKEN"),
        help="HuggingFace API token (default: from HUGGINGFACE_TOKEN or HF_TOKEN env var)",
    )
    parser.add_argument(
        "--r2-endpoint",
        default=os.getenv("R2_ENDPOINT"),
        help="R2 endpoint (default: from R2_ENDPOINT env var)",
    )
    parser.add_argument(
        "--r2-bucket",
        default=os.getenv("R2_BUCKET", "crosslogic-models"),
        help="R2 bucket name (default: from R2_BUCKET env var or 'crosslogic-models')",
    )
    parser.add_argument(
        "--r2-access-key",
        default=os.getenv("R2_ACCESS_KEY"),
        help="R2 access key (default: from R2_ACCESS_KEY env var)",
    )
    parser.add_argument(
        "--r2-secret-key",
        default=os.getenv("R2_SECRET_KEY"),
        help="R2 secret key (default: from R2_SECRET_KEY env var)",
    )
    
    args = parser.parse_args()
    
    # Validate credentials
    print("\n🔍 Validating credentials...")
    
    # HuggingFace token
    if not args.hf_token:
        print("\n❌ Error: HuggingFace token not found")
        print("   Set in .env file: HUGGINGFACE_TOKEN=hf_xxxxx")
        print("   Or pass: --hf-token hf_xxxxx")
        sys.exit(1)
    print("✓ HuggingFace token found")
    
    # R2 endpoint
    if not args.r2_endpoint:
        print("\n❌ Error: R2_ENDPOINT not set")
        print("   Set in .env file: R2_ENDPOINT=https://account-id.r2.cloudflarestorage.com")
        print("   Or pass: --r2-endpoint https://...")
        sys.exit(1)
    print(f"✓ R2 endpoint: {args.r2_endpoint}")
    
    # R2 credentials (check both direct args and AWS env vars)
    r2_access_key = args.r2_access_key or os.getenv("AWS_ACCESS_KEY_ID")
    r2_secret_key = args.r2_secret_key or os.getenv("AWS_SECRET_ACCESS_KEY")
    
    if not r2_access_key or not r2_secret_key:
        print("\n❌ Error: R2 credentials not found")
        print("   Set in .env file:")
        print("     R2_ACCESS_KEY=your_access_key")
        print("     R2_SECRET_KEY=your_secret_key")
        print("   Or set AWS environment variables:")
        print("     AWS_ACCESS_KEY_ID=your_access_key")
        print("     AWS_SECRET_ACCESS_KEY=your_secret_key")
        sys.exit(1)
    
    # Set AWS credentials for boto3/awscli
    os.environ["AWS_ACCESS_KEY_ID"] = r2_access_key
    os.environ["AWS_SECRET_ACCESS_KEY"] = r2_secret_key
    print("✓ R2 credentials configured")
    
    print(f"✓ R2 bucket: {args.r2_bucket}")
    print("✓ All credentials validated!\n")
    
    # Check for AWS CLI
    try:
        result = subprocess.run(["aws", "--version"], capture_output=True, text=True)
        print(f"✓ AWS CLI found: {result.stdout.split()[0]}")
    except FileNotFoundError:
        print("\n❌ Error: AWS CLI not installed")
        print("   Install: pip install awscli")
        sys.exit(1)
    
    upload_model(args.model_id, args.hf_token, args.r2_endpoint, args.r2_bucket)


if __name__ == "__main__":
    main()


