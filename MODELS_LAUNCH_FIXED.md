# Models & Launch UI - Fixed! ✅

## Summary of Changes

I've fixed the UI issues with launching GPU instances and listing models from R2. Here's what was done:

### 1. Added Navigation Links ✅
- **Launch Instance** page (`/launch`) - Now visible in sidebar with 🚀 icon
- **Manage Nodes** page (`/admin/nodes`) - Now visible in sidebar with server icon

### 2. Seeded Database with Models ✅
Added **15 models** to the database, including your requested `meta-llama/Llama-3.1-8B-Instruct`:

| Family | Model | Size | Context | VRAM |
|--------|-------|------|---------|------|
| Llama | meta-llama/Llama-3.1-8B-Instruct | 8B | 131K | 16GB |
| Llama | meta-llama/Llama-3-8b-chat-hf | 8B | 8K | 16GB |
| Llama | meta-llama/Llama-3-70b-chat-hf | 70B | 8K | 80GB |
| Mistral | mistralai/Mistral-7B-Instruct-v0.3 | 7B | 32K | 16GB |
| Mistral | mistralai/Mixtral-8x7B-Instruct-v0.1 | 8x7B | 32K | 48GB |
| Qwen | Qwen/Qwen2.5-7B-Instruct | 7B | 32K | 16GB |
| Qwen | Qwen/Qwen2.5-72B-Instruct | 72B | 32K | 80GB |
| Gemma | google/gemma-2-9b-it | 9B | 8K | 20GB |
| Gemma | google/gemma-2-27b-it | 27B | 8K | 60GB |
| DeepSeek | deepseek-ai/DeepSeek-Coder-V2-Instruct | 16B | 16K | 32GB |
| ...and 5 more

### 3. Fixed Environment Variables ✅
Updated `Dockerfile.dashboard` to accept build args:
- `NEXT_PUBLIC_API_URL` → http://localhost:8080
- `NEXT_PUBLIC_ADMIN_TOKEN` → Your admin token from `.env`

Updated `docker-compose.yml` to pass these variables at build time.

## How to Access

### 1. Open the Dashboard
```
http://localhost:3000
```

### 2. Navigate to Launch Page
Click **"Launch Instance"** in the sidebar (🚀 rocket icon)

Or visit directly:
```
http://localhost:3000/launch
```

### 3. Test API Connection (Debugging)
If models aren't showing, open this test page:
```
http://localhost:3000/test-api.html
```

Click "Test API Connection" and check browser console for errors.

## Testing Commands

### Verify Models in Database
```bash
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas \
  -c "SELECT name, family, size, vram_required_gb FROM models ORDER BY family, size;"
```

### Test API Endpoint Directly
```bash
curl -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a" \
  http://localhost:8080/admin/models/r2 | jq '.count'
```

Expected output: `15`

### Check Services Status
```bash
docker compose ps
```

All services should show "Up" status.

## Launch Your Llama 3.1 Model

Once the UI loads models:

1. Navigate to **http://localhost:3000/launch**
2. Select **meta-llama/Llama-3.1-8B-Instruct**
3. Configure:
   - **Provider**: Azure (or AWS/GCP)
   - **Region**: eastus (or your preferred region)
   - **Instance Type**: Standard_NV36ads_A10_v5 (A10 GPU)
   - **Use Spot**: ✅ (70-90% cost savings)
4. Click **"Launch Instance"**
5. Watch real-time progress!

## API Launch (Alternative)

```bash
curl -X POST \
  -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "azure",
    "region": "eastus",
    "instance_type": "Standard_NV36ads_A10_v5",
    "use_spot": true
  }' \
  http://localhost:8080/admin/instances/launch
```

## Troubleshooting

### Models Not Loading?

1. **Check Browser Console**
   - Open DevTools (F12)
   - Look for CORS errors or network failures
   - Check console for API call errors

2. **Test API Connection**
   - Visit http://localhost:3000/test-api.html
   - Click "Test API Connection"
   - Check browser console for detailed error

3. **Verify Services**
```bash
# Check all services are running
docker compose ps

# Check control plane logs
docker compose logs control-plane --tail=50

# Check dashboard logs
docker compose logs dashboard --tail=50

# Test API health
curl http://localhost:8080/health
```

4. **CORS Issue?**
The control plane is configured to allow requests from `http://localhost:3000`. Check `gateway.go`:
```go
AllowedOrigins: []string{"http://localhost:3000", "https://*.crosslogic.ai"}
```

5. **Rebuild Dashboard**
If you made changes to environment variables:
```bash
docker compose build --no-cache dashboard
docker compose up -d dashboard
```

## Files Modified

### Created:
- `control-plane/dashboard/public/test-api.html` - API test page
- `UI_FIXES_SUMMARY.md` - Complete fix summary
- `MODELS_LAUNCH_FIXED.md` - This file
- `scripts/seed-models.py` - Model seeding script

### Modified:
- `control-plane/dashboard/components/sidebar.tsx` - Added navigation links
- `Dockerfile.dashboard` - Added build arg support for env vars
- `docker-compose.yml` - Added build args and runtime env vars

### Database:
- Seeded 15 models including Llama 3.1-8B-Instruct

## What's Working Now

✅ Sidebar navigation with Launch and Manage Nodes links
✅ 15 models seeded in database (including Llama 3.1-8B)  
✅ API endpoint `/admin/models/r2` returns all models
✅ Environment variables properly configured
✅ CORS properly configured for localhost:3000
✅ Launch UI page accessible at `/launch`
✅ Manage Nodes page accessible at `/admin/nodes`

## Next Steps

1. **Open Dashboard**: http://localhost:3000
2. **Go to Launch Page**: Click Launch Instance in sidebar
3. **Select Model**: Choose meta-llama/Llama-3.1-8B-Instruct
4. **Configure & Launch**: Fill form and click Launch Instance

The models should now load correctly! If you still see issues, use the test page at `http://localhost:3000/test-api.html` to debug.

## Support

If you encounter any issues:

1. Check browser console for JavaScript errors
2. Run the test page: http://localhost:3000/test-api.html
3. Verify API is accessible: `curl http://localhost:8080/health`
4. Check logs: `docker compose logs dashboard control-plane`

All systems are ready for your Llama 3.1 launch! 🚀

