# Self-Service Cloud Credential Management for PRO Tier Tenants

## Implementation Summary

This implementation adds self-service cloud credential management and vLLM instance launching capabilities for PRO and ENTERPRISE tier tenants in the CrossLogic AI IaaS control plane.

## Files Created

### 1. `/internal/gateway/middleware_tier.go`
**Purpose**: Tier enforcement middleware

**Key Components**:
- `TierLevel` type with constants: `free`, `starter`, `pro`, `enterprise`
- `RequireProOrEnterprise()` middleware - Checks tenant tier from database
- Returns 403 Forbidden for free/starter tiers
- Adds tier information to request context
- Caches tier info for performance

**Usage**:
```go
router.Use(g.RequireProOrEnterprise)
```

### 2. `/internal/gateway/tenant_credentials.go`
**Purpose**: Self-service credential management for PRO tenants

**Endpoints**:
- `POST /v1/credentials` - Create credential
- `GET /v1/credentials` - List all credentials
- `GET /v1/credentials/{id}` - Get credential details
- `PUT /v1/credentials/{id}` - Update credential
- `DELETE /v1/credentials/{id}` - Delete credential (soft delete)
- `POST /v1/credentials/{id}/validate` - Validate credential
- `POST /v1/credentials/{id}/default` - Set as default

**Key Features**:
- Tenant ID extracted from auth context (no tenant_id in URL)
- Automatic tenant ownership verification
- Uses existing credentials service
- Returns sanitized output (no decrypted secrets)
- Proper error handling and logging

### 3. `/internal/gateway/tenant_instances.go`
**Purpose**: Self-service vLLM instance management for PRO tenants

**Endpoints**:
- `POST /v1/instances` - Launch new vLLM instance
- `GET /v1/instances` - List own instances
- `GET /v1/instances/{id}` - Get instance details
- `DELETE /v1/instances/{id}` - Terminate instance
- `GET /v1/instances/{id}/logs/stream` - Stream instance logs (SSE)

**Key Features**:
- Uses tenant's stored cloud credentials
- Integrates with SkyPilot orchestrator
- Tenant ownership verification
- Supports optional credential_id or default credential
- SSE streaming for logs
- Proper resource cleanup

### 4. `/migrations/003_add_tenant_self_service.sql`
**Purpose**: Database schema updates

**Changes**:
- Adds `tenant_id` column to `nodes` table
- Adds `spot_instance` column to `nodes` table
- Adds `terminated_at` column to `nodes` table
- Creates indexes for performance:
  - `idx_nodes_tenant_id`
  - `idx_nodes_tenant_status`
- Creates `cloud_credentials` table (idempotent)
- Adds indexes for credentials:
  - `idx_cloud_credentials_tenant_id`
  - `idx_cloud_credentials_tenant_provider`
  - `idx_cloud_credentials_default`

## Modified Files

### `/internal/gateway/gateway.go`
**Changes**: Added new route group for PRO+ features

```go
// === SELF-SERVICE FEATURES (PRO & ENTERPRISE ONLY) ===
r.Group(func(proRouter chi.Router) {
    proRouter.Use(g.RequireProOrEnterprise)

    // Credential endpoints
    proRouter.Post("/v1/credentials", g.handleCreateTenantCredential)
    proRouter.Get("/v1/credentials", g.handleListTenantCredentials)
    // ... more routes

    // Instance endpoints
    proRouter.Post("/v1/instances", g.handleLaunchTenantInstance)
    proRouter.Get("/v1/instances", g.handleListTenantInstances)
    // ... more routes
})
```

## API Documentation

### Credential Management

#### Create Credential
```bash
POST /v1/credentials
Authorization: Bearer <tenant_api_key>
Content-Type: application/json

{
  "provider": "aws",
  "name": "My AWS Prod Account",
  "credentials": {
    "access_key_id": "AKIAIOSFODNN7EXAMPLE",
    "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    "region": "us-west-2"
  },
  "is_default": true
}
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "660e8400-e29b-41d4-a716-446655440000",
  "provider": "aws",
  "name": "My AWS Prod Account",
  "is_default": true,
  "status": "active",
  "created_at": "2025-11-27T10:00:00Z",
  "updated_at": "2025-11-27T10:00:00Z"
}
```

#### List Credentials
```bash
GET /v1/credentials
Authorization: Bearer <tenant_api_key>
```

Response:
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "tenant_id": "660e8400-e29b-41d4-a716-446655440000",
      "provider": "aws",
      "name": "My AWS Prod Account",
      "is_default": true,
      "status": "active",
      "last_used_at": "2025-11-27T10:00:00Z",
      "created_at": "2025-11-27T10:00:00Z",
      "updated_at": "2025-11-27T10:00:00Z"
    }
  ]
}
```

### Instance Management

#### Launch Instance
```bash
POST /v1/instances
Authorization: Bearer <tenant_api_key>
Content-Type: application/json

{
  "model": "meta-llama/Llama-3.1-8B-Instruct",
  "provider": "aws",
  "region": "us-west-2",
  "gpu": "A10G",
  "gpu_count": 1,
  "idle_minutes_to_autostop": 30,
  "credential_id": "550e8400-e29b-41d4-a716-446655440000",
  "use_spot": true,
  "disk_size": 256
}
```

Response:
```json
{
  "instance_id": "770e8400-e29b-41d4-a716-446655440000",
  "cluster_name": "cic-aws-uswest2-a10g-spot-770e84",
  "status": "launching",
  "message": "Instance is being launched. This may take 2-5 minutes."
}
```

#### List Instances
```bash
GET /v1/instances
Authorization: Bearer <tenant_api_key>
```

Response:
```json
{
  "data": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440000",
      "cluster_name": "cic-aws-uswest2-a10g-spot-770e84",
      "model": "meta-llama/Llama-3.1-8B-Instruct",
      "provider": "aws",
      "region": "us-west-2",
      "gpu": "A10G",
      "status": "active",
      "endpoint_url": "http://34.56.78.90:8000",
      "spot_instance": true,
      "created_at": "2025-11-27T10:00:00Z",
      "updated_at": "2025-11-27T10:05:00Z"
    }
  ]
}
```

#### Stream Logs
```bash
GET /v1/instances/{id}/logs/stream
Authorization: Bearer <tenant_api_key>
```

Response (SSE):
```
data: INFO: vLLM server started on port 8000
data: INFO: Model loaded: meta-llama/Llama-3.1-8B-Instruct
data: INFO: Accepting requests...
event: complete
data: log streaming complete
```

## Tier Access Matrix

| Feature | Free | Starter | Pro | Enterprise |
|---------|------|---------|-----|------------|
| Use shared inference | ✅ | ✅ | ✅ | ✅ |
| View usage | ✅ | ✅ | ✅ | ✅ |
| Manage API keys | ✅ | ✅ | ✅ | ✅ |
| **Self-service credentials** | ❌ | ❌ | ✅ | ✅ |
| **Self-service instances** | ❌ | ❌ | ✅ | ✅ |
| **Stream instance logs** | ❌ | ❌ | ✅ | ✅ |

## Error Handling

### Tier Restriction Error
When a free/starter tenant attempts to access PRO features:

```json
HTTP 403 Forbidden
{
  "error": {
    "message": "This feature is only available on PRO and ENTERPRISE plans. Please upgrade your subscription.",
    "type": "tier_restriction_error",
    "tier": "free",
    "required_tier": "pro"
  }
}
```

### Credential Not Found
```json
HTTP 404 Not Found
{
  "error": {
    "message": "credential not found",
    "type": "invalid_request_error"
  }
}
```

### Instance Launch Failed
```json
HTTP 500 Internal Server Error
{
  "error": {
    "message": "failed to launch instance: insufficient quota in region us-west-2",
    "type": "invalid_request_error"
  }
}
```

## Security Considerations

### Authorization
- All endpoints require Bearer token authentication
- Tenant ID extracted from API key (not from URL)
- Automatic tenant ownership verification
- Tier check enforced before resource access

### Data Protection
- Credentials encrypted at rest using AES-256-GCM
- No decrypted credentials returned in API responses
- Credential validation without exposing secrets
- Audit logging for all operations

### Resource Isolation
- Tenants can only access own credentials
- Tenants can only manage own instances
- Database queries filter by tenant_id
- No cross-tenant resource access

## Testing

### Manual Testing

1. **Test Tier Restriction**:
```bash
# As free tier tenant
curl -X POST http://localhost:8080/v1/credentials \
  -H "Authorization: Bearer free_tier_api_key" \
  -H "Content-Type: application/json" \
  -d '{"provider": "aws", "name": "test", "credentials": {}}'
# Expected: 403 Forbidden
```

2. **Test Credential Creation (PRO tier)**:
```bash
curl -X POST http://localhost:8080/v1/credentials \
  -H "Authorization: Bearer pro_tier_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "aws",
    "name": "My AWS Account",
    "credentials": {
      "access_key_id": "AKIAIOSFODNN7EXAMPLE",
      "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
      "region": "us-west-2"
    },
    "is_default": true
  }'
# Expected: 201 Created
```

3. **Test Instance Launch**:
```bash
curl -X POST http://localhost:8080/v1/instances \
  -H "Authorization: Bearer pro_tier_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "region": "us-west-2",
    "gpu": "A10G",
    "gpu_count": 1,
    "provider": "aws"
  }'
# Expected: 201 Created
```

### Database Migration

Run the migration:
```bash
psql -h localhost -p 5432 -U crosslogic -d crosslogic_dev \
  -f migrations/003_add_tenant_self_service.sql
```

Verify tables:
```sql
-- Check nodes table has tenant_id
\d nodes

-- Check cloud_credentials table exists
\d cloud_credentials

-- Check indexes
\di idx_nodes_tenant_id
\di idx_cloud_credentials_tenant_id
```

## Monitoring and Observability

### Logging
All operations are logged with:
- Tenant ID
- Operation type
- Resource IDs
- Success/failure status
- Execution time

Example log entries:
```
INFO  tenant credential created  tenant_id=660e8400... credential_id=550e8400... provider=aws name="My AWS Prod Account"
INFO  tenant instance launched  tenant_id=660e8400... instance_id=770e8400... cluster_name=cic-aws-uswest2-a10g-spot-770e84
WARN  tenant attempted to access PRO+ feature  tenant_id=660e8400... tier=free path=/v1/credentials
```

### Metrics to Track
- Credential creation rate by provider
- Instance launch success rate
- Instance launch duration
- Tier restriction violations
- Credential validation failures

## Production Readiness Checklist

- [x] Input validation on all endpoints
- [x] Authorization checks (tenant ownership)
- [x] Tier enforcement middleware
- [x] Error handling with proper HTTP status codes
- [x] Audit logging for all operations
- [x] Database migration for schema changes
- [x] Credential encryption (using existing service)
- [x] Resource isolation (tenant-scoped queries)
- [ ] Rate limiting per tenant
- [ ] Credential validation per provider
- [ ] Instance auto-stop on idle
- [ ] Cost tracking per tenant instance
- [ ] Alert on instance launch failures
- [ ] Documentation for Dashboard integration

## Next Steps

1. **Run Database Migration**: Apply `003_add_tenant_self_service.sql`
2. **Update Dashboard**: Add UI for credential and instance management
3. **Add Cost Tracking**: Track costs for tenant-owned instances
4. **Implement Auto-Stop**: Stop idle instances after configured timeout
5. **Add Validation**: Implement per-provider credential validation
6. **Testing**: Create integration tests for all endpoints
7. **Documentation**: Add to API documentation and user guide

## Support

For questions or issues:
- Check logs: `/var/log/control-plane/`
- Database queries: Use tenant_id for filtering
- Credential issues: Check encryption key configuration
- Instance issues: Check SkyPilot orchestrator logs
