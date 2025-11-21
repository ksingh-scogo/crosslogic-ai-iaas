#!/usr/bin/env python3
"""
Upload HuggingFace models directly to Cloudflare R2.

vLLM will stream models from R2 using native S3 support - no JuiceFS needed!

Usage:
    python upload-model-to-r2.py meta-llama/Llama-3-8B-Instruct --hf-token YOUR_TOKEN
"""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

try:
    from huggingface_hub import snapshot_download
    from tqdm import tqdm
except ImportError:
    print("Error: Required packages not found. Install with:")
    print("  pip install huggingface-hub tqdm")
    sys.exit(1)


def upload_model(model_id: str, hf_token: str, r2_endpoint: str, r2_bucket: str):
    """Upload model from HuggingFace to R2"""
    
    print(f"\n🚀 Uploading {model_id} to Cloudflare R2\n")
    
    # Step 1: Download from HuggingFace
    print(f"📥 Downloading from HuggingFace...")
    try:
        local_path = snapshot_download(
            repo_id=model_id,
            token=hf_token,
            cache_dir="/tmp/model-cache",
            resume_download=True,
        )
        print(f"✓ Downloaded to {local_path}")
    except Exception as e:
        print(f"✗ Download failed: {e}")
        sys.exit(1)
    
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
    print(f"\n📝 Usage in vLLM:")
    print(f"  python -m vllm.entrypoints.openai.api_server \\")
    print(f"    --model s3://{r2_bucket}/{model_id}")
    print(f"\n📝 Or use in CrossLogic control plane:")
    print(f"  Model name: {model_id}")
    print(f"  vLLM will automatically stream from R2")
    print(f"\n⚡ Performance:")
    print(f"  - First load: ~30-60s (stream from R2 + cache)")
    print(f"  - Subsequent loads: ~5-10s (local HF cache)")
    print(f"  - Compare to: 5-10 minutes (direct HuggingFace download)")


def main():
    parser = argparse.ArgumentParser(
        description="Upload HuggingFace models to Cloudflare R2 for fast vLLM loading"
    )
    parser.add_argument(
        "model_id",
        help="HuggingFace model ID (e.g., meta-llama/Llama-3-8B-Instruct)",
    )
    parser.add_argument(
        "--hf-token",
        required=True,
        help="HuggingFace API token",
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
    
    args = parser.parse_args()
    
    # Validate environment
    if not args.r2_endpoint:
        print("Error: R2_ENDPOINT not set")
        print("Export it or pass --r2-endpoint:")
        print("  export R2_ENDPOINT='https://account-id.r2.cloudflarestorage.com'")
        sys.exit(1)
    
    if not os.getenv("AWS_ACCESS_KEY_ID") or not os.getenv("AWS_SECRET_ACCESS_KEY"):
        print("Error: AWS credentials not set")
        print("Export them:")
        print("  export AWS_ACCESS_KEY_ID='your-r2-access-key'")
        print("  export AWS_SECRET_ACCESS_KEY='your-r2-secret-key'")
        sys.exit(1)
    
    # Check for AWS CLI
    if subprocess.run(["which", "aws"], capture_output=True).returncode != 0:
        print("Error: AWS CLI not installed")
        print("Install it: pip install awscli")
        sys.exit(1)
    
    upload_model(args.model_id, args.hf_token, args.r2_endpoint, args.r2_bucket)


if __name__ == "__main__":
    main()


