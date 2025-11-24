# CrossLogic AI IaaS - Quick Access Guide

## 🚀 Your System is Ready!

All services are running and configured correctly.

## Access URLs

| Service | URL | Purpose |
|---------|-----|---------|
| **Dashboard** | http://localhost:3000 | Main UI - Launch instances, manage nodes |
| **Launch Page** | http://localhost:3000/launch | Launch GPU instances with models |
| **Manage Nodes** | http://localhost:3000/admin/nodes | View and manage GPU nodes |
| **API Test** | http://localhost:3000/test-api.html | Debug API connectivity |
| **Control Plane API** | http://localhost:8080 | REST API endpoint |
| **Grafana** | http://localhost:3001 | Monitoring (admin/admin) |
| **Prometheus** | http://localhost:9091 | Metrics |

## ✅ System Status

- **Services Running**: 7/7
- **Models Available**: 15 (including Llama 3.1-8B)
- **API Status**: ✅ Healthy
- **Database**: ✅ Connected

## Launch Your Llama 3.1 Model

### Quick Start (UI):
1. Open http://localhost:3000/launch
2. Select `meta-llama/Llama-3.1-8B-Instruct`
3. Configure cloud provider (Azure/AWS/GCP)
4. Click "Launch Instance"

### Quick Start (API):
```bash
curl -X POST http://localhost:8080/admin/instances/launch \
  -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "azure",
    "region": "eastus",
    "instance_type": "Standard_NV36ads_A10_v5",
    "use_spot": true
  }'
```

## Available Models (15 total)

### Llama Family
- `meta-llama/Llama-3.1-8B-Instruct` - **Your model!** (131K context)
- `meta-llama/Llama-3-8b-chat-hf` (8K context)
- `meta-llama/Llama-3-70b-chat-hf` (8K context)
- `llama-3-8b`
- `llama-3-70b`

### Mistral Family
- `mistralai/Mistral-7B-Instruct-v0.3`
- `mistralai/Mixtral-8x7B-Instruct-v0.1`
- `mistral-7b`

### Qwen Family
- `Qwen/Qwen2.5-7B-Instruct`
- `Qwen/Qwen2.5-72B-Instruct`
- `qwen-2.5-7b`

### Gemma Family
- `google/gemma-2-9b-it`
- `google/gemma-2-27b-it`
- `gemma-7b`

### DeepSeek Family
- `deepseek-ai/DeepSeek-Coder-V2-Instruct`

## Quick Commands

### View All Models
```bash
curl -H "X-Admin-Token: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a" \
  http://localhost:8080/admin/models/r2 | jq '.models[].name'
```

### Check Service Status
```bash
docker compose ps
```

### View Logs
```bash
# All services
docker compose logs -f

# Specific service
docker compose logs dashboard -f
docker compose logs control-plane -f
```

### Restart Services
```bash
# Restart all
docker compose restart

# Restart specific
docker compose restart dashboard
```

### Stop All Services
```bash
docker compose down
```

### Start All Services
```bash
./start.sh
# or
docker compose up -d
```

## Troubleshooting

### Models Not Loading in UI?
1. Visit http://localhost:3000/test-api.html
2. Click "Test API Connection"
3. Check browser console (F12) for errors

### API Not Responding?
```bash
# Check control plane health
curl http://localhost:8080/health

# Check logs
docker compose logs control-plane --tail=50
```

### Database Issues?
```bash
# Check postgres is running
docker compose ps postgres

# Access database
docker compose exec postgres psql -U crosslogic -d crosslogic_iaas
```

## Key Environment Variables

Your system is configured with these variables from `.env`:

- **ADMIN_API_TOKEN**: df865f5814d7136b6adfd164b5142e98eefb845ac8e490093909d4d9e91e5b2a
- **DATABASE**: crosslogic_iaas (PostgreSQL on port 5432)
- **REDIS**: localhost:6379
- **CONTROL_PLANE**: localhost:8080
- **DASHBOARD**: localhost:3000

## What's Next?

1. **Launch Your Model**: Go to http://localhost:3000/launch
2. **Monitor Progress**: Watch real-time launch progress on the UI
3. **Manage Nodes**: View active nodes at http://localhost:3000/admin/nodes
4. **API Integration**: Use the REST API to integrate with your applications

## Documentation

- **Full Setup Guide**: `MODELS_LAUNCH_FIXED.md`
- **UI Fixes Summary**: `UI_FIXES_SUMMARY.md`
- **Prerequisites**: `PREREQUISITES_CHECKLIST.md`
- **Quick Start**: `QUICK_START.md`

## Support & Issues

If you encounter any problems:
1. Check logs: `docker compose logs [service]`
2. Run test page: http://localhost:3000/test-api.html
3. Verify API: `curl http://localhost:8080/health`
4. Check this guide: `MODELS_LAUNCH_FIXED.md`

---

**Ready to launch! 🚀** Visit http://localhost:3000/launch to get started!

