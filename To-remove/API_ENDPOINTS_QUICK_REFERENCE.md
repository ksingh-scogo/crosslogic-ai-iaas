# API Endpoints Quick Reference

## Admin Tenant Management Extended

### Delete Tenant
```
DELETE /admin/tenants/{id}
Headers: X-Admin-Token: <token>

Response 200:
{
  "status": "deleted",
  "message": "tenant deleted successfully"
}

Response 409: Tenant already deleted
Response 404: Tenant not found
```

### Suspend Tenant
```
POST /admin/tenants/{id}/suspend
Headers: X-Admin-Token: <token>
Body:
{
  "reason": "Payment overdue",
  "notes": "Account suspended until payment received"
}

Response 200:
{
  "status": "suspended",
  "message": "tenant suspended successfully",
  "reason": "Payment overdue"
}

Response 409: Already suspended or deleted
```

### Activate Tenant
```
POST /admin/tenants/{id}/activate
Headers: X-Admin-Token: <token>
Body:
{
  "notes": "Payment received, reactivating account"
}

Response 200:
{
  "status": "active",
  "message": "tenant activated successfully"
}

Response 409: Already active or deleted
```

### Get Tenant API Keys
```
GET /admin/tenants/{id}/api-keys?status=active&limit=50&offset=0
Headers: X-Admin-Token: <token>

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "key_prefix": "cl_test_abc...",
      "name": "Production Key",
      "role": "developer",
      "status": "active",
      "rate_limit_rpm": 60,
      "concurrency_limit": 5,
      "created_at": "2025-01-01T00:00:00Z",
      "last_used_at": "2025-01-25T12:00:00Z"
    }
  ],
  "pagination": {
    "total": 100,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

### Get Tenant Deployments
```
GET /admin/tenants/{id}/deployments?start_date=2025-01-01T00:00:00Z&end_date=2025-01-25T23:59:59Z
Headers: X-Admin-Token: <token>

Response 200:
{
  "tenant_id": "uuid",
  "start_date": "2025-01-01T00:00:00Z",
  "end_date": "2025-01-25T23:59:59Z",
  "data": [
    {
      "model_id": "uuid",
      "model_name": "llama-3-8b",
      "family": "Llama",
      "type": "chat",
      "first_used": "2025-01-05T10:00:00Z",
      "last_used": "2025-01-25T15:30:00Z",
      "total_tokens": 5000000,
      "total_requests": 10000,
      "total_cost_usd": 250.50
    }
  ]
}
```

### Get Tenant Detailed Usage
```
GET /admin/tenants/{id}/usage/detailed?group_by=model&start_date=2025-01-01T00:00:00Z
Headers: X-Admin-Token: <token>

Query Params:
- group_by: model, api_key, region, hour, day
- model_id: filter by specific model (uuid)
- api_key_id: filter by specific API key (uuid)
- region_id: filter by specific region (uuid)
- start_date, end_date: RFC3339 timestamps

Response 200:
{
  "tenant_id": "uuid",
  "start_date": "2025-01-01T00:00:00Z",
  "end_date": "2025-01-25T23:59:59Z",
  "group_by": "model",
  "data": [
    {
      "model_id": "uuid",
      "model_name": "llama-3-8b",
      "family": "Llama",
      "prompt_tokens": 3000000,
      "completion_tokens": 2000000,
      "total_tokens": 5000000,
      "cached_tokens": 500000,
      "cache_hit_rate_pct": 10.0,
      "total_requests": 10000,
      "avg_latency_ms": 250.5,
      "p95_latency_ms": 450.2,
      "total_cost_usd": 250.50
    }
  ]
}
```

## Admin Regions Management

### Create Region
```
POST /admin/regions
Headers: X-Admin-Token: <token>
Body:
{
  "code": "us-west-3",
  "name": "US West 3",
  "provider": "aws",
  "country": "USA",
  "city": "San Francisco",
  "available": true,
  "pricing_multiplier": 1.1,
  "metadata": {
    "datacenter_id": "dc-sf-01"
  }
}

Response 201:
{
  "id": "uuid",
  "code": "us-west-3",
  "name": "US West 3",
  "provider": "aws",
  "country": "USA",
  "city": "San Francisco",
  "status": "active",
  "pricing_multiplier": 1.1
}

Response 409: Region code already exists
```

### Update Region
```
PUT /admin/regions/{id}
Headers: X-Admin-Token: <token>
Body:
{
  "name": "US West 3 (Updated)",
  "available": false,
  "pricing_multiplier": 1.2
}

Response 200:
{
  "status": "updated",
  "message": "region updated successfully"
}

Note: code and provider are immutable
```

### Delete Region
```
DELETE /admin/regions/{id}
Headers: X-Admin-Token: <token>

Response 200:
{
  "status": "deleted",
  "message": "region deleted successfully"
}

Response 409: Active nodes exist in region
```

### Get Region Availability
```
GET /admin/regions/{id}/availability
Headers: X-Admin-Token: <token>

Response 200:
{
  "region_id": "uuid",
  "region_code": "us-east-1",
  "region_name": "US East 1",
  "provider": "aws",
  "status": "active",
  "available_instances": [
    {
      "id": 1,
      "instance_type": "g4dn.xlarge",
      "instance_name": "g4dn.xlarge",
      "gpu_model": "NVIDIA T4",
      "gpu_count": 1,
      "gpu_memory_gb": 16,
      "vcpu_count": 4,
      "memory_gb": 16,
      "price_per_hour": 0.526,
      "spot_price_per_hour": 0.158,
      "supports_spot": true,
      "is_available": true,
      "stock_status": "available"
    }
  ],
  "quota_limits": {
    "max_nodes": 100,
    "current_nodes": 5,
    "available_quota": 95
  },
  "estimated_launch_time_seconds": 180
}
```

## Admin Instance Types Management

### Create Instance Type
```
POST /admin/instance-types
Headers: X-Admin-Token: <token>
Body:
{
  "provider": "aws",
  "instance_type": "g4dn.xlarge",
  "instance_name": "g4dn.xlarge",
  "vcpu_count": 4,
  "memory_gb": 16,
  "gpu_count": 1,
  "gpu_memory_gb": 16,
  "gpu_model": "NVIDIA T4",
  "gpu_compute_capability": "7.5",
  "price_per_hour": 0.526,
  "spot_price_per_hour": 0.158,
  "available": true,
  "supports_spot": true
}

Response 201:
{
  "id": 1,
  "provider": "aws",
  "instance_type": "g4dn.xlarge",
  "gpu_model": "NVIDIA T4",
  "gpu_count": 1,
  "price_per_hour": 0.526,
  "spot_price_per_hour": 0.158,
  "available": true
}

Response 409: Instance type already exists
```

### Update Instance Type
```
PUT /admin/instance-types/{id}
Headers: X-Admin-Token: <token>
Body:
{
  "price_per_hour": 0.550,
  "spot_price_per_hour": 0.165,
  "available": true
}

Response 200:
{
  "status": "updated",
  "message": "instance type updated successfully"
}

Note: GPU specs are immutable
```

### Delete Instance Type
```
DELETE /admin/instance-types/{id}
Headers: X-Admin-Token: <token>

Response 200:
{
  "status": "deleted",
  "message": "instance type deleted successfully"
}

Response 409: Active nodes using this instance type
```

### Associate Instance Type with Regions
```
POST /admin/instance-types/{id}/regions
Headers: X-Admin-Token: <token>
Body:
{
  "region_codes": ["us-east-1", "us-west-2", "eu-west-1"],
  "is_available": true,
  "stock_status": "available"
}

Response 200:
{
  "status": "success",
  "message": "regions associated successfully",
  "inserted": 2,
  "updated": 1
}

stock_status options: available, limited, out_of_stock
```

### Get Instance Type Pricing
```
GET /admin/instance-types/{id}/pricing
Headers: X-Admin-Token: <token>

Response 200:
{
  "instance_type_id": 1,
  "provider": "aws",
  "instance_type": "g4dn.xlarge",
  "gpu_model": "NVIDIA T4",
  "gpu_count": 1,
  "supports_spot": true,
  "base_pricing": {
    "on_demand_price": 0.526,
    "spot_price": 0.158
  },
  "pricing_range": {
    "on_demand": {
      "min": 0.368,
      "max": 0.579,
      "avg": 0.500
    },
    "spot": {
      "min": 0.110,
      "max": 0.174,
      "avg": 0.150
    }
  },
  "regional_pricing": [
    {
      "region_code": "us-east-1",
      "region_name": "US East 1",
      "location": "Virginia, USA",
      "is_available": true,
      "stock_status": "available",
      "pricing_multiplier": 1.0,
      "on_demand_price": 0.526,
      "spot_price": 0.158,
      "spot_savings_pct": 70.0
    }
  ]
}
```

## Tenant Usage Extended

### Get Detailed Usage
```
GET /v1/usage/detailed?group_by=model&limit=100&offset=0
Headers: Authorization: Bearer <api_key>

Query Params:
- group_by: model, api_key, region, hour, day
- model_id: filter by model (uuid)
- api_key_id: filter by API key (uuid)
- start_date, end_date: RFC3339 timestamps
- limit: max 1000
- offset: pagination offset

Response 200:
{
  "start_date": "2025-01-01T00:00:00Z",
  "end_date": "2025-01-25T23:59:59Z",
  "group_by": "model",
  "data": [
    {
      "model_id": "uuid",
      "model_name": "llama-3-8b",
      "family": "Llama",
      "type": "chat",
      "prompt_tokens": 3000000,
      "completion_tokens": 2000000,
      "total_tokens": 5000000,
      "cached_tokens": 500000,
      "cache_hit_rate_pct": 10.0,
      "total_requests": 10000,
      "avg_latency_ms": 250.5,
      "min_latency_ms": 50.0,
      "max_latency_ms": 2000.0,
      "total_cost_usd": 250.50
    }
  ],
  "pagination": {
    "limit": 100,
    "offset": 0
  }
}
```

### Get Usage by Hour
```
GET /v1/usage/by-hour?hours=24
Headers: Authorization: Bearer <api_key>

Query Params:
- hours: 24-168 (default 24)

Response 200:
{
  "hours": 24,
  "start_date": "2025-01-24T15:00:00Z",
  "end_date": "2025-01-25T15:00:00Z",
  "data": [
    {
      "hour": "2025-01-25T14:00:00Z",
      "total_tokens": 100000,
      "total_requests": 200,
      "total_cost_usd": 5.00,
      "avg_latency_ms": 250.5
    }
  ]
}
```

### Get Usage by Day
```
GET /v1/usage/by-day?days=30
Headers: Authorization: Bearer <api_key>

Query Params:
- days: 30-90 (default 30)

Response 200:
{
  "days": 30,
  "start_date": "2024-12-26T00:00:00Z",
  "end_date": "2025-01-25T23:59:59Z",
  "data": [
    {
      "day": "2025-01-25T00:00:00Z",
      "total_tokens": 2000000,
      "prompt_tokens": 1200000,
      "completion_tokens": 800000,
      "total_requests": 4000,
      "total_cost_usd": 100.00,
      "avg_latency_ms": 250.5
    }
  ]
}
```

### Get Usage by Week
```
GET /v1/usage/by-week?weeks=12
Headers: Authorization: Bearer <api_key>

Query Params:
- weeks: 12-52 (default 12)

Response 200:
{
  "weeks": 12,
  "data": [
    {
      "week_start": "2025-01-20T00:00:00Z",
      "total_tokens": 14000000,
      "total_requests": 28000,
      "total_cost_usd": 700.00
    }
  ]
}
```

### Get Usage by Month
```
GET /v1/usage/by-month?months=12
Headers: Authorization: Bearer <api_key>

Query Params:
- months: 12-24 (default 12)

Response 200:
{
  "months": 12,
  "data": [
    {
      "month": "2025-01-01T00:00:00Z",
      "total_tokens": 50000000,
      "total_requests": 100000,
      "total_cost_usd": 2500.00
    }
  ]
}
```

## Tenant Metrics Extended

### Get Performance Metrics
```
GET /v1/metrics/performance?period=24h&model=llama-3-8b
Headers: Authorization: Bearer <api_key>

Query Params:
- period: 1h, 24h, 7d, 30d (default 24h)
- model: optional model name filter

Response 200:
{
  "period": "24h",
  "start_date": "2025-01-24T15:00:00Z",
  "end_date": "2025-01-25T15:00:00Z",
  "metrics": [
    {
      "model_name": "llama-3-8b",
      "total_requests": 10000,
      "successful_requests": 9950,
      "failed_requests": 50,
      "success_rate_pct": 99.5,
      "total_tokens": 5000000,
      "cached_tokens": 500000,
      "cache_hit_rate_pct": 10.0,
      "tokens_per_second": 2000.0,
      "latency": {
        "avg_ms": 250.5,
        "p50_ms": 200.0,
        "p95_ms": 450.0,
        "p99_ms": 800.0
      }
    }
  ]
}
```

### Get Throughput Metrics
```
GET /v1/metrics/throughput?period=24h
Headers: Authorization: Bearer <api_key>

Response 200:
{
  "period": "24h",
  "throughput": {
    "total_requests": 10000,
    "total_tokens": 5000000,
    "avg_tokens_per_request": 500.0,
    "avg_rps": 0.116,
    "peak_rps": 5.2,
    "peak_timestamp": "2025-01-25T14:30:00Z"
  },
  "cache_distribution": {
    "cached_tokens": 500000,
    "uncached_tokens": 4500000,
    "cached_pct": 10.0,
    "uncached_pct": 90.0
  },
  "time_series": [
    {
      "timestamp": "2025-01-25T14:59:00Z",
      "requests": 10,
      "tokens": 5000,
      "rps": 0.167
    }
  ]
}
```

### Get Model Metrics
```
GET /v1/metrics/by-model?period=24h
Headers: Authorization: Bearer <api_key>

Response 200:
{
  "period": "24h",
  "models": [
    {
      "model_id": "uuid",
      "model_name": "llama-3-8b",
      "family": "Llama",
      "type": "chat",
      "total_tokens": 5000000,
      "prompt_tokens": 3000000,
      "completion_tokens": 2000000,
      "total_requests": 10000,
      "avg_tokens_per_request": 500.0,
      "usage_percentage": 75.5,
      "total_cost_usd": 250.00,
      "avg_latency_ms": 250.5,
      "p95_latency_ms": 450.0
    }
  ]
}
```

## Error Responses

All endpoints follow consistent error format:

```json
{
  "error": {
    "message": "Human-readable error message",
    "type": "invalid_request_error"
  }
}
```

Common status codes:
- 400: Bad request (invalid input)
- 401: Unauthorized (missing/invalid token)
- 403: Forbidden (insufficient permissions)
- 404: Not found
- 409: Conflict (duplicate, in use, invalid state)
- 422: Validation error
- 429: Rate limit exceeded
- 500: Internal server error

## Rate Limiting

Tenant endpoints are rate limited per API key:
- Default: 60 requests/minute
- Configurable per API key
- Returns 429 when exceeded
- Headers: X-RateLimit-Remaining, X-RateLimit-Reset

## Authentication

### Admin Endpoints
```
X-Admin-Token: <admin_token>
```

### Tenant Endpoints
```
Authorization: Bearer <api_key>
```

API keys format: `cl_test_*` or `cl_live_*`

## Pagination

List endpoints support pagination:
```
?limit=50&offset=0
```

Response includes:
```json
{
  "data": [...],
  "pagination": {
    "total": 100,
    "limit": 50,
    "offset": 0,
    "has_more": true
  }
}
```

## Date Ranges

All timestamps use RFC3339 format:
```
2025-01-25T15:30:00Z
```

Query parameters:
```
?start_date=2025-01-01T00:00:00Z&end_date=2025-01-25T23:59:59Z
```

Default: Last 30 days if not specified
