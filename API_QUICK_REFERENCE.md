# API Quick Reference Guide

## Authentication

### Admin API
```bash
curl -H "X-Admin-Token: your-admin-token" http://localhost:8080/admin/...
```

### Tenant API
```bash
curl -H "Authorization: Bearer sk-..." http://localhost:8080/v1/...
```

---

## Admin API Endpoints

### Models
```bash
# List models
GET /api/v1/admin/models

# Create model
POST /api/v1/admin/models

# Update model
PUT /api/v1/admin/models/{id}

# Delete model
DELETE /api/v1/admin/models/{id}
```

### Nodes
```bash
# List nodes
GET /admin/nodes

# Launch node
POST /admin/nodes/launch

# Terminate node
POST /admin/nodes/{cluster_name}/terminate

# Node status
GET /admin/nodes/{cluster_name}/status
```

### Deployments
```bash
# Create deployment
POST /admin/deployments

# List deployments
GET /admin/deployments

# Scale deployment
PUT /admin/deployments/{id}/scale

# Delete deployment
DELETE /admin/deployments/{id}
```

### Platform
```bash
# Platform health
GET /admin/platform/health

# Platform metrics
GET /admin/platform/metrics?period=24h
```

---

## Tenant API Endpoints

### API Keys
```bash
# List own API keys
GET /v1/api-keys

# Create API key
POST /v1/api-keys

# Revoke own API key
DELETE /v1/api-keys/{key_id}
```

### Inference
```bash
# Chat completions
POST /v1/chat/completions

# Text completions
POST /v1/completions

# Embeddings
POST /v1/embeddings

# List models
GET /v1/models
```

### Usage
```bash
# Overall usage
GET /v1/usage

# Usage by model
GET /v1/usage/by-model

# Usage by API key
GET /v1/usage/by-key

# Time-series usage
GET /v1/usage/by-date
```

### Metrics
```bash
# Latency metrics
GET /v1/metrics/latency

# Token metrics
GET /v1/metrics/tokens
```
