# GPU Launch - Quick Fix Reference

## TL;DR - Fix It Now

```bash
# 1. Apply fixes (5 minutes)
cp Dockerfile.control-plane.fixed Dockerfile.control-plane
cp control-plane/internal/orchestrator/skypilot_fixed.go control-plane/internal/orchestrator/skypilot.go
cp control-plane/internal/gateway/admin_models_fixed.go control-plane/internal/gateway/admin_models.go

# 2. Rebuild (10 minutes)
docker-compose down
docker-compose build control-plane
docker-compose up -d

# 3. Verify (1 minute)
./scripts/verify-launch-setup.sh

# 4. Test
# Open http://localhost:3000 and launch an instance
```

## 7 Critical Issues Fixed

| # | Issue | Impact | Fixed |
|---|-------|--------|-------|
| 1 | SkyPilot not installed | BLOCKING | Dockerfile |
| 2 | Wrong SkyPilot arguments | HIGH | skypilot.go:441 |
| 3 | No credential validation | HIGH | skypilot.go:405 |
| 4 | Missing error propagation | HIGH | admin_models.go:175 |
| 5 | No progress updates | MEDIUM | admin_models.go:246 |
| 6 | Output not streamed | MEDIUM | skypilot.go:449 |
| 7 | Race condition in status | MEDIUM | admin_models.go:181 |

## Key Changes

### SkyPilot Command (Before → After)

```go
// BEFORE (WRONG)
cmd := exec.CommandContext(ctx, "sky", "launch",
    "-c", clusterName,
    taskFile,
    "-y",
    "--down",        // ❌ Terminates after job
    "--detach-run",  // ❌ Doesn't wait
)

// AFTER (FIXED)
cmd := exec.CommandContext(ctx, "sky", "launch",
    "-c", clusterName,
    "--cloud", config.Provider,   // ✅ Force provider
    "--region", config.Region,     // ✅ Force region
    "--retry-until-up",            // ✅ Retry zones
    "-y",
    taskFile,
)
```

### Error Messages (Before → After)

```diff
- "sky launch failed: exit status 1"
+ "No GPU capacity available for T4 in us-east-1 region.
+
+  Suggestions:
+    • Try a different region (westus2, centralindia)
+    • Use on-demand instead of spot
+    • Wait 10-15 minutes and retry"
```

### Progress Updates (Before → After)

```diff
- Status: "in_progress" (for 5 minutes)
+ Progress updates every 10 seconds:
+   → Validating configuration (10%)
+   → Provisioning cloud resources (30%)
+   → Installing dependencies (60%)
+   → Loading model from R2 (80%)
+   → Starting vLLM (90%)
+   ✓ Node ready (100%)
```

## Verification Commands

```bash
# Check SkyPilot installed
docker exec crosslogic-control-plane sky --version

# Check cloud credentials configured
docker exec crosslogic-control-plane sky check

# View control plane logs
docker-compose logs -f control-plane | grep -i launch

# List clusters
docker exec crosslogic-control-plane sky status

# Full verification
./scripts/verify-launch-setup.sh
```

## Environment Variables Required

```bash
# .env file - Pick ONE cloud provider minimum

# AWS
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret

# OR Azure
AZURE_SUBSCRIPTION_ID=your_sub_id
AZURE_TENANT_ID=your_tenant_id

# OR GCP
GCP_PROJECT_ID=your_project_id

# Recommended: R2 for fast model loading
R2_ENDPOINT=https://account.r2.cloudflarestorage.com
R2_BUCKET=crosslogic-models
R2_ACCESS_KEY=your_key
R2_SECRET_KEY=your_secret
```

## Testing Checklist

- [ ] `./scripts/verify-launch-setup.sh` passes
- [ ] Can launch T4 instance in us-east-1
- [ ] Progress updates appear in UI
- [ ] Clear error if credentials missing
- [ ] Node appears in active nodes list
- [ ] vLLM health check passes

## Troubleshooting

| Error | Solution |
|-------|----------|
| `sky not found` | Rebuild: `docker-compose build control-plane` |
| `AWS credentials not configured` | Add to .env, restart |
| `No spot capacity` | Try different region or on-demand |
| `Timeout after 20 minutes` | Check cloud console, try smaller model |

## Files Modified

```
Dockerfile.control-plane              (SkyPilot installation)
orchestrator/skypilot.go              (Launch logic)
gateway/admin_models.go               (Job tracking)
scripts/verify-launch-setup.sh        (New verification script)
```

## Rollback

```bash
git checkout Dockerfile.control-plane
git checkout control-plane/internal/orchestrator/skypilot.go
git checkout control-plane/internal/gateway/admin_models.go
docker-compose build control-plane && docker-compose up -d
```

## Success Criteria

- ✅ Launch success rate > 95%
- ✅ Cold start: 3-5 minutes
- ✅ Real-time progress updates
- ✅ Clear error messages
- ✅ No orphaned resources

## Documentation

- **Detailed Analysis**: `GPU_LAUNCH_FIXES.md`
- **Implementation Guide**: `IMPLEMENTATION_GUIDE.md`
- **Summary**: `GPU_LAUNCH_FIX_SUMMARY.md`
- **This Reference**: `QUICK_FIX_REFERENCE.md`

## Time Estimates

- Read documentation: 30 minutes
- Apply fixes: 5 minutes
- Rebuild containers: 10 minutes
- Test launch: 5 minutes
- **Total: ~50 minutes**

---

**Priority**: HIGH - Core feature is broken
**Confidence**: HIGH - All issues identified and fixed
**Risk**: LOW - Clear rollback path

🚀 Ready to fix!
