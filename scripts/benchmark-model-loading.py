#!/usr/bin/env python3
"""
Benchmark model loading performance: Standard vs Run:ai Streamer

This script compares loading times between:
1. Standard vLLM loading
2. Run:ai Model Streamer (ultra-fast)

Usage:
    python benchmark-model-loading.py s3://models/meta-llama/Llama-3-8B-Instruct
"""

import argparse
import subprocess
import sys
import time
from typing import Optional

try:
    import requests
except ImportError:
    print("Error: requests package not found. Install with:")
    print("  pip install requests")
    sys.exit(1)


def wait_for_health(port: int, timeout: int = 600) -> Optional[float]:
    """Wait for vLLM health endpoint to be ready"""
    start_time = time.time()
    url = f"http://localhost:{port}/health"
    
    print(f"  Waiting for health endpoint at {url}...")
    while time.time() - start_time < timeout:
        try:
            resp = requests.get(url, timeout=1)
            if resp.status_code == 200:
                elapsed = time.time() - start_time
                print(f"  ✓ Ready after {elapsed:.2f}s")
                return elapsed
        except (requests.ConnectionError, requests.Timeout):
            pass
        time.sleep(1)
    
    print(f"  ✗ Timeout after {timeout}s")
    return None


def test_standard_loading(model_path: str, port: int = 8001) -> Optional[float]:
    """Test standard vLLM loading"""
    print("\n" + "="*60)
    print("Test 1: Standard vLLM Loading")
    print("="*60)
    
    cmd = [
        "python", "-m", "vllm.entrypoints.openai.api_server",
        "--model", model_path,
        "--host", "0.0.0.0",
        "--port", str(port),
        "--gpu-memory-utilization", "0.9",
    ]
    
    print(f"Command: {' '.join(cmd)}")
    print("Starting vLLM...")
    
    start_time = time.time()
    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Wait for health endpoint
    elapsed = wait_for_health(port)
    
    # Terminate process
    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()
    
    if elapsed:
        print(f"✓ Standard loading completed in {elapsed:.2f}s")
    else:
        print("✗ Standard loading failed or timed out")
    
    return elapsed


def test_runai_loading(model_path: str, port: int = 8002, concurrency: int = 32) -> Optional[float]:
    """Test Run:ai Streamer loading"""
    print("\n" + "="*60)
    print("Test 2: Run:ai Model Streamer")
    print("="*60)
    
    cmd = [
        "python", "-m", "vllm.entrypoints.openai.api_server",
        "--model", model_path,
        "--load-format", "runai_streamer",
        "--model-loader-extra-config", f'{{"concurrency": {concurrency}, "memory_limit": 5368709120}}',
        "--host", "0.0.0.0",
        "--port", str(port),
        "--gpu-memory-utilization", "0.95",
        "--dtype", "bfloat16",
    ]
    
    print(f"Command: {' '.join(cmd)}")
    print(f"Concurrency: {concurrency} threads")
    print("Starting vLLM with Run:ai Streamer...")
    
    start_time = time.time()
    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Wait for health endpoint
    elapsed = wait_for_health(port)
    
    # Terminate process
    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()
    
    if elapsed:
        print(f"✓ Run:ai Streamer completed in {elapsed:.2f}s")
    else:
        print("✗ Run:ai Streamer failed or timed out")
    
    return elapsed


def main():
    parser = argparse.ArgumentParser(
        description="Benchmark vLLM model loading: Standard vs Run:ai Streamer"
    )
    parser.add_argument(
        "model_path",
        help="Model path (e.g., s3://models/meta-llama/Llama-3-8B-Instruct)",
    )
    parser.add_argument(
        "--concurrency",
        type=int,
        default=32,
        help="Run:ai Streamer concurrency (default: 32)",
    )
    parser.add_argument(
        "--skip-standard",
        action="store_true",
        help="Skip standard loading test (only test Run:ai Streamer)",
    )
    
    args = parser.parse_args()
    
    print("\n" + "="*60)
    print("vLLM Model Loading Benchmark")
    print("="*60)
    print(f"Model: {args.model_path}")
    print(f"Run:ai concurrency: {args.concurrency}")
    print("="*60)
    
    standard_time = None
    runai_time = None
    
    # Test 1: Standard loading
    if not args.skip_standard:
        standard_time = test_standard_loading(args.model_path)
        if standard_time is None:
            print("\n⚠️  Standard loading failed, continuing with Run:ai test...")
    
    # Test 2: Run:ai Streamer
    runai_time = test_runai_loading(args.model_path, concurrency=args.concurrency)
    
    # Results
    print("\n" + "="*60)
    print("📊 BENCHMARK RESULTS")
    print("="*60)
    
    if standard_time:
        print(f"Standard loading:     {standard_time:6.2f}s")
    else:
        print(f"Standard loading:     FAILED")
    
    if runai_time:
        print(f"Run:ai Streamer:      {runai_time:6.2f}s")
    else:
        print(f"Run:ai Streamer:      FAILED")
    
    if standard_time and runai_time:
        speedup = standard_time / runai_time
        time_saved = standard_time - runai_time
        print(f"\nSpeedup:              {speedup:.2f}x faster ⚡")
        print(f"Time saved:           {time_saved:.2f}s per load")
        
        # Extrapolate to multiple launches
        print(f"\nExtrapolated savings (100 launches/day):")
        print(f"  Time saved/day:     {time_saved * 100 / 60:.1f} minutes")
        print(f"  Time saved/month:   {time_saved * 3000 / 3600:.1f} hours")
        
        if runai_time < 10:
            print(f"\n🎉 Excellent! Run:ai Streamer achieves <10s loading time")
        elif runai_time < 20:
            print(f"\n✅ Good! Run:ai Streamer loading time under 20s")
        else:
            print(f"\n💡 Tip: Try increasing --concurrency to {args.concurrency * 2} for faster loading")
    
    print("="*60)
    
    # Exit code based on results
    if runai_time and runai_time < 30:
        sys.exit(0)  # Success
    else:
        sys.exit(1)  # Failed or too slow


if __name__ == "__main__":
    main()

