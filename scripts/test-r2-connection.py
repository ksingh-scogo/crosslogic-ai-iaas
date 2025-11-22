#!/usr/bin/env python3
"""
Test R2 connection and credentials before uploading models.

Usage:
    python test-r2-connection.py
"""

import os
import subprocess
import sys
from pathlib import Path

# Load .env file
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
        print(f"✓ Loaded .env from {env_path}\n")
    else:
        print(f"⚠️  No .env file found at {env_path}\n")
except ImportError:
    # Use fallback parser (works perfectly fine)
    if load_env_file(env_path):
        print(f"✓ Loaded .env from {env_path}\n")
    else:
        print(f"⚠️  No .env file found at {env_path}\n")

print("=" * 60)
print("Testing R2 Connection & Credentials")
print("=" * 60)

# Check credentials
print("\n1. Checking credentials...")

required_vars = {
    "R2_ENDPOINT": os.getenv("R2_ENDPOINT"),
    "R2_BUCKET": os.getenv("R2_BUCKET"),
    "R2_ACCESS_KEY": os.getenv("R2_ACCESS_KEY"),
    "R2_SECRET_KEY": os.getenv("R2_SECRET_KEY"),
    "HUGGINGFACE_TOKEN": os.getenv("HUGGINGFACE_TOKEN"),
}

missing = []
for var, value in required_vars.items():
    if value:
        # Mask sensitive values
        if "KEY" in var or "TOKEN" in var:
            masked = value[:8] + "..." + value[-4:] if len(value) > 12 else "***"
            print(f"   ✓ {var}: {masked}")
        else:
            print(f"   ✓ {var}: {value}")
    else:
        print(f"   ✗ {var}: NOT SET")
        missing.append(var)

if missing:
    print(f"\n❌ Missing credentials: {', '.join(missing)}")
    print("\nAdd them to your .env file:")
    for var in missing:
        print(f"   {var}=your_value_here")
    sys.exit(1)

print("\n✓ All credentials found!")

# Check AWS CLI
print("\n2. Checking AWS CLI...")
try:
    result = subprocess.run(["aws", "--version"], capture_output=True, text=True, check=True)
    print(f"   ✓ AWS CLI installed: {result.stdout.strip()}")
except (FileNotFoundError, subprocess.CalledProcessError):
    print("   ✗ AWS CLI not found")
    print("\n   Install with: pip install awscli")
    sys.exit(1)

# Test R2 connection
print("\n3. Testing R2 connection...")

# Set AWS credentials for boto3
os.environ["AWS_ACCESS_KEY_ID"] = required_vars["R2_ACCESS_KEY"]
os.environ["AWS_SECRET_ACCESS_KEY"] = required_vars["R2_SECRET_KEY"]

try:
    cmd = [
        "aws", "s3", "ls",
        f"s3://{required_vars['R2_BUCKET']}/",
        "--endpoint-url", required_vars["R2_ENDPOINT"]
    ]
    
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
    
    if result.returncode == 0:
        print(f"   ✓ Successfully connected to R2 bucket: {required_vars['R2_BUCKET']}")
        
        # List contents
        if result.stdout.strip():
            lines = result.stdout.strip().split("\n")
            print(f"   ✓ Bucket contents ({len(lines)} items):")
            for line in lines[:5]:  # Show first 5 items
                print(f"      {line}")
            if len(lines) > 5:
                print(f"      ... and {len(lines) - 5} more")
        else:
            print("   ℹ️  Bucket is empty (ready for first upload)")
    else:
        print(f"   ✗ Failed to connect to R2")
        print(f"\n   Error: {result.stderr}")
        print(f"\n   Check your credentials in .env file:")
        print(f"      R2_ENDPOINT={required_vars['R2_ENDPOINT']}")
        print(f"      R2_BUCKET={required_vars['R2_BUCKET']}")
        sys.exit(1)
        
except subprocess.TimeoutExpired:
    print("   ✗ Connection timed out")
    print(f"   Check R2_ENDPOINT: {required_vars['R2_ENDPOINT']}")
    sys.exit(1)
except Exception as e:
    print(f"   ✗ Error: {e}")
    sys.exit(1)

# Test HuggingFace token
print("\n4. Testing HuggingFace token...")
try:
    from huggingface_hub import HfApi
    api = HfApi(token=required_vars["HUGGINGFACE_TOKEN"])
    user = api.whoami()
    print(f"   ✓ HuggingFace token valid")
    print(f"   ✓ Logged in as: {user.get('name', 'Unknown')}")
except ImportError:
    print("   ⚠️  huggingface-hub not installed")
    print("   Install with: pip install huggingface-hub")
except Exception as e:
    print(f"   ✗ HuggingFace token invalid: {e}")
    print(f"   Get token from: https://huggingface.co/settings/tokens")
    sys.exit(1)

# Success
print("\n" + "=" * 60)
print("✅ All tests passed! Ready to upload models.")
print("=" * 60)
print("\nNext step:")
print("  python scripts/upload-model-to-r2.py meta-llama/Meta-Llama-3-8B-Instruct")
print()

