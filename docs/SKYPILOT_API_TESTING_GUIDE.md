# SkyPilot API Testing Guide

This guide provides step-by-step instructions for testing the SkyPilot API Server integration without using the UI/Dashboard. All operations are performed via REST APIs.

## Prerequisites

### 1. Environment Setup

Ensure you have the following installed:
- Docker and Docker Compose
- `curl` or any HTTP client
- `jq` (optional, for JSON formatting)

### 2. Configuration

Copy and configure the environment file:

```bash
cd /path/to/crosslogic-ai-iaas
cp .env.example .env
```

Edit `.env` and configure these SkyPilot-specific settings:

```bash
# Enable SkyPilot API Server mode
SKYPILOT_USE_API_SERVER=true

# SkyPilot API Server URL (Docker internal network)
SKYPILOT_API_SERVER_URL=http://skypilot-api:46580

# Service account token (generate a random 32+ char string)
SKYPILOT_SERVICE_ACCOUNT_TOKEN=your_secure_token_here_minimum_32_chars

# Encryption key for cloud credentials (generate a random 32+ char string)
SKYPILOT_CREDENTIAL_ENCRYPTION_KEY=your_encryption_key_minimum_32_chars

# Admin API token
ADMIN_API_TOKEN=your_admin_token_at_least_32_chars_long

# Cloud credentials (for testing - configure at least one provider)
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_DEFAULT_REGION=us-west-2
```

---

## Step 1: Start the Services

```bash
# Start all services
docker-compose up -d

# Verify services are running
docker-compose ps

# Check control-plane logs
docker-compose logs -f control-plane
```

Expected services:
- `control-plane` - Main API server on port 8080
- `postgres` - Database on port 5432
- `redis` - Cache on port 6379
- `skypilot-api` - SkyPilot API Server on port 46580

---

## Step 2: Verify Services Health

### 2.1 Control Plane Health

```bash
curl -s http://localhost:8080/health | jq
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2024-01-15T10:30:00Z"
}
```

### 2.2 SkyPilot API Server Health (Internal)

```bash
# Connect to control-plane container to test internal connectivity
docker-compose exec control-plane curl -s http://skypilot-api:46580/api/health
```

---

## Step 3: Create a Tenant

First, create a tenant that will own the cloud credentials:

```bash
# Set admin token
ADMIN_TOKEN="your_admin_token_at_least_32_chars_long"

# Create tenant
curl -s -X POST http://localhost:8080/admin/tenants \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Organization",
    "email": "test@example.com"
  }' | jq
```

Expected response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Test Organization",
  "email": "test@example.com",
  "tier": "free",
  "status": "active",
  "created_at": "2024-01-15T10:30:00Z"
}
```

Save the tenant ID:
```bash
TENANT_ID="550e8400-e29b-41d4-a716-446655440000"
```

---

## Step 4: Store Cloud Credentials

### 4.1 Create AWS Credentials

```bash
curl -s -X POST http://localhost:8080/admin/credentials \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"provider\": \"aws\",
    \"name\": \"AWS Production\",
    \"is_default\": true,
    \"credentials\": {
      \"access_key_id\": \"AKIAIOSFODNN7EXAMPLE\",
      \"secret_access_key\": \"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\",
      \"region\": \"us-west-2\"
    }
  }" | jq
```

Expected response:
```json
{
  "id": "cred-12345678-1234-1234-1234-123456789012",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "aws",
  "name": "AWS Production",
  "is_default": true,
  "is_valid": null,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### 4.2 Create Azure Credentials (Optional)

```bash
curl -s -X POST http://localhost:8080/admin/credentials \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"provider\": \"azure\",
    \"name\": \"Azure Production\",
    \"is_default\": true,
    \"credentials\": {
      \"subscription_id\": \"12345678-1234-1234-1234-123456789012\",
      \"tenant_id\": \"87654321-4321-4321-4321-210987654321\",
      \"client_id\": \"11111111-2222-3333-4444-555555555555\",
      \"client_secret\": \"your_client_secret\"
    }
  }" | jq
```

### 4.3 Create GCP Credentials (Optional)

```bash
curl -s -X POST http://localhost:8080/admin/credentials \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"provider\": \"gcp\",
    \"name\": \"GCP Production\",
    \"is_default\": true,
    \"credentials\": {
      \"type\": \"service_account\",
      \"project_id\": \"my-gcp-project\",
      \"private_key_id\": \"key123\",
      \"private_key\": \"-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\",
      \"client_email\": \"service-account@my-gcp-project.iam.gserviceaccount.com\"
    }
  }" | jq
```

### 4.4 List Credentials

```bash
curl -s "http://localhost:8080/admin/credentials?tenant_id=$TENANT_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

### 4.5 Validate Credentials

```bash
CREDENTIAL_ID="cred-12345678-1234-1234-1234-123456789012"

curl -s -X POST "http://localhost:8080/admin/credentials/$CREDENTIAL_ID/validate?tenant_id=$TENANT_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

---

## Step 5: Register a Model

Before launching a node, register the model you want to deploy:

```bash
curl -s -X POST http://localhost:8080/admin/models \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "meta-llama/Llama-3.1-8B-Instruct",
    "family": "llama",
    "size": "8B",
    "type": "chat",
    "context_length": 8192,
    "vram_required_gb": 16,
    "price_input_per_million": 0.15,
    "price_output_per_million": 0.60,
    "huggingface_id": "meta-llama/Llama-3.1-8B-Instruct"
  }' | jq
```

---

## Step 6: Launch GPU Instance via API

### 6.1 Launch a Node

```bash
curl -s -X POST http://localhost:8080/admin/nodes/launch \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "aws",
    "region": "us-west-2",
    "gpu": "A10G",
    "gpu_count": 1,
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "idle_minutes_to_autostop": 30
  }' | jq
```

Expected response:
```json
{
  "id": "node-12345678-1234-1234-1234-123456789012",
  "cluster_name": "cl-abc123def456",
  "model_name": "meta-llama/Llama-3.1-8B-Instruct",
  "provider": "aws",
  "region": "us-west-2",
  "gpu": "A10G",
  "gpu_count": 1,
  "status": "launching",
  "request_id": "req-87654321",
  "created_at": "2024-01-15T10:30:00Z"
}
```

Save the node ID and cluster name:
```bash
NODE_ID="node-12345678-1234-1234-1234-123456789012"
CLUSTER_NAME="cl-abc123def456"
```

### 6.2 Monitor Launch Progress

Poll for status updates:

```bash
# Check node status
curl -s "http://localhost:8080/admin/nodes/$NODE_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq '.status'
```

Status progression:
1. `launching` - Initial state, SkyPilot is provisioning
2. `initializing` - Instance is up, vLLM is starting
3. `active` - Ready to serve inference requests
4. `failed` - Launch failed (check logs)

### 6.3 List All Nodes

```bash
curl -s "http://localhost:8080/admin/nodes" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

---

## Step 7: Monitor Cluster Status

### 7.1 Get Cluster Status via SkyPilot API (Internal)

From within the control-plane container:

```bash
docker-compose exec control-plane curl -s \
  -H "Authorization: Bearer $SKYPILOT_SERVICE_ACCOUNT_TOKEN" \
  "http://skypilot-api:46580/api/v1/clusters/$CLUSTER_NAME" | jq
```

### 7.2 List All Clusters via SkyPilot API (Internal)

```bash
docker-compose exec control-plane curl -s \
  -H "Authorization: Bearer $SKYPILOT_SERVICE_ACCOUNT_TOKEN" \
  "http://skypilot-api:46580/api/v1/clusters" | jq
```

---

## Step 8: Test Inference (Once Node is Active)

### 8.1 Create Tenant API Key

```bash
curl -s -X POST http://localhost:8080/admin/api-keys \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$TENANT_ID\",
    \"name\": \"test-key\"
  }" | jq
```

Save the API key:
```bash
API_KEY="clsk_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

### 8.2 Run Inference

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 100
  }' | jq
```

---

## Step 9: Terminate Instance via API

### 9.1 Terminate Single Node

```bash
curl -s -X POST "http://localhost:8080/admin/nodes/$NODE_ID/terminate" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

Expected response:
```json
{
  "status": "terminating",
  "message": "Node termination initiated"
}
```

### 9.2 Force Terminate (Purge)

To forcefully terminate and clean up all resources:

```bash
curl -s -X POST "http://localhost:8080/admin/nodes/$NODE_ID/terminate?purge=true" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

### 9.3 Verify Termination

```bash
# Check node status
curl -s "http://localhost:8080/admin/nodes/$NODE_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq '.status'
```

Status should progress: `terminating` -> `terminated` -> (eventually deleted)

---

## NEW: Step 9.5 - Stream Node Launch Logs in Real-Time

Monitor node launch progress with real-time log streaming via Server-Sent Events (SSE).

### 9.5.1 Stream Logs (SSE)

```bash
# Stream logs with curl (follow mode)
curl -N -H "X-Admin-Token: $ADMIN_TOKEN" \
  "http://localhost:8080/admin/nodes/$NODE_ID/logs/stream?follow=true&tail=50"
```

You'll see events like:
```
event: log
data: {"timestamp":"2024-01-15T10:30:00Z","level":"info","message":"Launching cluster...","phase":"provisioning","progress":10}

event: status
data: {"phase":"installing","progress":45,"message":"Installing vLLM..."}

event: done
data: {"status":"active","endpoint":"http://10.0.0.1:8000","message":"Node ready"}
```

### 9.5.2 Get Historical Logs (JSON)

```bash
curl -s "http://localhost:8080/admin/nodes/$NODE_ID/logs?tail=100" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

### 9.5.3 Log Phases

| Phase | Progress | Description |
|-------|----------|-------------|
| `queued` | 0% | Request received |
| `provisioning` | 10-30% | SkyPilot provisioning cloud resources |
| `instance_ready` | 35% | Cloud instance is running |
| `installing` | 40-60% | Installing dependencies/vLLM |
| `model_loading` | 65-85% | Loading model weights |
| `health_check` | 90% | Running health checks |
| `active` | 100% | Node is ready |
| `failed` | - | Launch failed |

---

## NEW: PRO Tier Self-Service Features

PRO and ENTERPRISE tier tenants can manage their own cloud credentials and launch dedicated instances.

### Prerequisite: Create PRO Tier Tenant

```bash
# Create a PRO tier tenant
curl -s -X POST http://localhost:8080/admin/tenants \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "PRO Customer Inc",
    "email": "pro@example.com",
    "tier": "pro"
  }' | jq

PRO_TENANT_ID="<id from response>"

# Create API key for PRO tenant
curl -s -X POST http://localhost:8080/admin/api-keys \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"$PRO_TENANT_ID\",
    \"name\": \"pro-key\"
  }" | jq

PRO_API_KEY="<key from response>"
```

### Self-Service Credential Management

#### Create Cloud Credential

```bash
curl -s -X POST http://localhost:8080/v1/credentials \
  -H "Authorization: Bearer $PRO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "aws",
    "name": "My AWS Production",
    "is_default": true,
    "credentials": {
      "access_key_id": "AKIAIOSFODNN7EXAMPLE",
      "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
      "region": "us-west-2"
    }
  }' | jq

CREDENTIAL_ID="<id from response>"
```

#### List My Credentials

```bash
curl -s http://localhost:8080/v1/credentials \
  -H "Authorization: Bearer $PRO_API_KEY" | jq
```

#### Validate Credential

```bash
curl -s -X POST "http://localhost:8080/v1/credentials/$CREDENTIAL_ID/validate" \
  -H "Authorization: Bearer $PRO_API_KEY" | jq
```

### Self-Service Instance Management

#### Launch My Own vLLM Instance

```bash
curl -s -X POST http://localhost:8080/v1/instances \
  -H "Authorization: Bearer $PRO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "aws",
    "region": "us-west-2",
    "gpu": "A10G",
    "gpu_count": 1,
    "idle_minutes_to_autostop": 30,
    "use_spot": true
  }' | jq

INSTANCE_ID="<id from response>"
```

#### List My Instances

```bash
curl -s http://localhost:8080/v1/instances \
  -H "Authorization: Bearer $PRO_API_KEY" | jq
```

#### Stream Instance Launch Logs

```bash
curl -N -H "Authorization: Bearer $PRO_API_KEY" \
  "http://localhost:8080/v1/instances/$INSTANCE_ID/logs/stream?follow=true"
```

#### Terminate My Instance

```bash
curl -s -X DELETE "http://localhost:8080/v1/instances/$INSTANCE_ID" \
  -H "Authorization: Bearer $PRO_API_KEY" | jq
```

### Tier Access Requirements

| Feature | Free | Starter | Pro | Enterprise |
|---------|------|---------|-----|------------|
| Shared inference | Yes | Yes | Yes | Yes |
| View usage | Yes | Yes | Yes | Yes |
| Manage API keys | Yes | Yes | Yes | Yes |
| Self-service credentials | No | No | Yes | Yes |
| Self-service instances | No | No | Yes | Yes |
| Stream instance logs | No | No | Yes | Yes |

---

## Step 10: Cleanup

### 10.1 Delete Credentials

```bash
curl -s -X DELETE "http://localhost:8080/admin/credentials/$CREDENTIAL_ID?tenant_id=$TENANT_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

### 10.2 Delete Tenant

```bash
curl -s -X DELETE "http://localhost:8080/admin/tenants/$TENANT_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" | jq
```

### 10.3 Stop Services

```bash
docker-compose down
```

---

## Troubleshooting

### Check Control Plane Logs

```bash
docker-compose logs -f control-plane
```

### Check SkyPilot API Server Logs

```bash
docker-compose logs -f skypilot-api
```

### Check SkyPilot Cluster State

```bash
# Inside the skypilot-api container
docker-compose exec skypilot-api sky status
```

### Common Issues

1. **"credential not found"**: Ensure credentials are created for the correct tenant and provider
2. **"SkyPilot API unreachable"**: Check if `skypilot-api` container is running and healthy
3. **"Launch failed"**: Check cloud provider credentials are valid and have necessary permissions
4. **"Insufficient quota"**: Request quota increase from cloud provider for GPU instances

---

## API Endpoints Summary

### Admin Credential Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/admin/credentials` | Create credential |
| GET | `/admin/credentials?tenant_id=` | List credentials |
| GET | `/admin/credentials/{id}?tenant_id=` | Get credential |
| PUT | `/admin/credentials/{id}?tenant_id=` | Update credential |
| DELETE | `/admin/credentials/{id}?tenant_id=` | Delete credential |
| POST | `/admin/credentials/{id}/validate?tenant_id=` | Validate credential |
| POST | `/admin/credentials/{id}/default?tenant_id=` | Set default |

### Admin Node Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/admin/nodes/launch` | Launch GPU node |
| GET | `/admin/nodes` | List all nodes |
| GET | `/admin/nodes/{id}` | Get node details |
| POST | `/admin/nodes/{id}/terminate` | Terminate node |
| GET | `/admin/nodes/{id}/logs/stream` | Stream logs (SSE) |
| GET | `/admin/nodes/{id}/logs` | Get historical logs |

### Tenant Self-Service Credentials (PRO+)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/credentials` | Create credential |
| GET | `/v1/credentials` | List credentials |
| GET | `/v1/credentials/{id}` | Get credential |
| PUT | `/v1/credentials/{id}` | Update credential |
| DELETE | `/v1/credentials/{id}` | Delete credential |
| POST | `/v1/credentials/{id}/validate` | Validate credential |
| POST | `/v1/credentials/{id}/default` | Set default |

### Tenant Self-Service Instances (PRO+)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/instances` | Launch vLLM instance |
| GET | `/v1/instances` | List instances |
| GET | `/v1/instances/{id}` | Get instance details |
| DELETE | `/v1/instances/{id}` | Terminate instance |
| GET | `/v1/instances/{id}/logs/stream` | Stream logs (SSE) |

### Tenant Inference
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/chat/completions` | Chat completion |
| POST | `/v1/completions` | Text completion |
| POST | `/v1/embeddings` | Generate embeddings |
| GET | `/v1/models` | List available models |

---

## Environment Variables Reference

| Variable | Description | Required |
|----------|-------------|----------|
| `SKYPILOT_USE_API_SERVER` | Enable API Server mode | Yes |
| `SKYPILOT_API_SERVER_URL` | SkyPilot API Server URL | Yes |
| `SKYPILOT_SERVICE_ACCOUNT_TOKEN` | Auth token for SkyPilot API | Yes |
| `SKYPILOT_CREDENTIAL_ENCRYPTION_KEY` | Key for encrypting credentials | Yes |
| `ADMIN_API_TOKEN` | Admin authentication token | Yes |
| `AWS_ACCESS_KEY_ID` | AWS access key (for testing) | No |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key (for testing) | No |
| `AZURE_*` | Azure credentials (for testing) | No |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP service account path | No |
