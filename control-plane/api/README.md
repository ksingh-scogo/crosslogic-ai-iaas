# CrossLogic AI IaaS - Model Management API

Complete CRUD APIs for managing AI models in the CrossLogic AI Infrastructure as a Service platform.

## Overview

This API provides comprehensive model management capabilities including:
- List all models with advanced filtering and pagination
- Get detailed information about specific models
- Create new model entries
- Update existing models (full or partial updates)
- Delete models
- Advanced search with complex filters

## API Endpoints

### Base URL
- **Local Development**: `http://localhost:8080`
- **Production**: `https://api.crosslogic.ai`

### Authentication
All admin endpoints require the `X-Admin-Token` header:

```bash
curl -H "X-Admin-Token: your-admin-token" \
  http://localhost:8080/api/v1/admin/models
```

## Endpoints

### 1. List Models
**GET** `/api/v1/admin/models`

List all models with optional filtering, sorting, and pagination.

#### Query Parameters
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `limit` | integer | Max items to return (1-100) | 50 |
| `offset` | integer | Number of items to skip | 0 |
| `family` | string | Filter by model family (e.g., llama, gpt) | - |
| `type` | string | Filter by type (completion, chat, embedding) | - |
| `status` | string | Filter by status (active, deprecated, beta) | - |
| `min_vram` | integer | Minimum VRAM required (GB) | - |
| `max_vram` | integer | Maximum VRAM required (GB) | - |
| `search` | string | Search by name (case-insensitive) | - |
| `sort_by` | string | Field to sort by | name |
| `sort_order` | string | Sort order (asc, desc) | asc |

#### Example Request
```bash
curl -X GET "http://localhost:8080/api/v1/admin/models?family=llama&status=active&limit=10" \
  -H "X-Admin-Token: your-admin-token"
```

#### Example Response
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "meta-llama/Llama-3.1-8B-Instruct",
      "family": "llama",
      "size": "8B",
      "type": "chat",
      "context_length": 8192,
      "vram_required_gb": 16,
      "price_input_per_million": 0.15,
      "price_output_per_million": 0.60,
      "tokens_per_second_capacity": 500,
      "status": "active",
      "metadata": {
        "storage": {
          "provider": "cloudflare-r2",
          "bucket": "crosslogic-models",
          "path": "llama/3.1/8b-instruct"
        },
        "capabilities": ["chat", "instruction-following"],
        "architecture": "transformer"
      },
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ],
  "pagination": {
    "total": 1,
    "limit": 10,
    "offset": 0,
    "has_more": false
  }
}
```

---

### 2. Get Model by ID
**GET** `/api/v1/admin/models/{id}`

Retrieve detailed information about a specific model.

#### Example Request
```bash
curl -X GET "http://localhost:8080/api/v1/admin/models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Admin-Token: your-admin-token"
```

#### Example Response
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "meta-llama/Llama-3.1-8B-Instruct",
  "family": "llama",
  "size": "8B",
  "type": "chat",
  "context_length": 8192,
  "vram_required_gb": 16,
  "price_input_per_million": 0.15,
  "price_output_per_million": 0.60,
  "tokens_per_second_capacity": 500,
  "status": "active",
  "metadata": {
    "storage": {
      "provider": "cloudflare-r2",
      "bucket": "crosslogic-models",
      "path": "llama/3.1/8b-instruct"
    }
  },
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

### 3. Create Model
**POST** `/api/v1/admin/models`

Create a new model entry in the system.

#### Request Body
```json
{
  "name": "meta-llama/Llama-3.1-70B-Instruct",
  "family": "llama",
  "size": "70B",
  "type": "chat",
  "context_length": 8192,
  "vram_required_gb": 140,
  "price_input_per_million": 0.90,
  "price_output_per_million": 2.70,
  "tokens_per_second_capacity": 200,
  "status": "active",
  "metadata": {
    "storage": {
      "provider": "cloudflare-r2",
      "bucket": "crosslogic-models",
      "path": "llama/3.1/70b-instruct"
    },
    "capabilities": ["chat", "instruction-following", "long-context"],
    "architecture": "transformer",
    "quantization": "fp16"
  }
}
```

#### Example Request
```bash
curl -X POST "http://localhost:8080/api/v1/admin/models" \
  -H "X-Admin-Token: your-admin-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "meta-llama/Llama-3.1-70B-Instruct",
    "family": "llama",
    "size": "70B",
    "type": "chat",
    "context_length": 8192,
    "vram_required_gb": 140,
    "price_input_per_million": 0.90,
    "price_output_per_million": 2.70,
    "status": "active"
  }'
```

#### Response
Returns the created model with `201 Created` status.

---

### 4. Update Model (Full Update)
**PUT** `/api/v1/admin/models/{id}`

Replace all fields of an existing model. All required fields must be provided.

#### Request Body
```json
{
  "name": "meta-llama/Llama-3.1-8B-Instruct",
  "family": "llama",
  "size": "8B",
  "type": "chat",
  "context_length": 8192,
  "vram_required_gb": 16,
  "price_input_per_million": 0.12,
  "price_output_per_million": 0.48,
  "tokens_per_second_capacity": 600,
  "status": "active",
  "metadata": {
    "storage": {
      "provider": "cloudflare-r2",
      "bucket": "crosslogic-models",
      "path": "llama/3.1/8b-instruct"
    }
  }
}
```

#### Example Request
```bash
curl -X PUT "http://localhost:8080/api/v1/admin/models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Admin-Token: your-admin-token" \
  -H "Content-Type: application/json" \
  -d @model-update.json
```

---

### 5. Partial Update Model
**PATCH** `/api/v1/admin/models/{id}`

Update specific fields of a model. Only provided fields will be updated.

#### Request Body (Example: Update pricing)
```json
{
  "price_input_per_million": 0.12,
  "price_output_per_million": 0.48
}
```

#### Request Body (Example: Update status)
```json
{
  "status": "deprecated"
}
```

#### Request Body (Example: Update metadata)
```json
{
  "metadata": {
    "storage": {
      "provider": "aws-s3",
      "bucket": "new-bucket",
      "region": "us-east-1"
    },
    "capabilities": ["chat", "instruction-following", "function-calling"]
  }
}
```

#### Example Request
```bash
curl -X PATCH "http://localhost:8080/api/v1/admin/models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Admin-Token: your-admin-token" \
  -H "Content-Type: application/json" \
  -d '{
    "price_input_per_million": 0.12,
    "price_output_per_million": 0.48
  }'
```

---

### 6. Delete Model
**DELETE** `/api/v1/admin/models/{id}`

Remove a model from the system. This is a hard delete and cannot be undone.

**Warning**: Deleting a model that is currently in use by active nodes may cause service disruption. Consider setting status to 'deprecated' instead.

#### Example Request
```bash
curl -X DELETE "http://localhost:8080/api/v1/admin/models/550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Admin-Token: your-admin-token"
```

#### Response
- `204 No Content` - Successfully deleted
- `404 Not Found` - Model doesn't exist
- `409 Conflict` - Model is in use and cannot be deleted

---

### 7. Advanced Search
**GET** `/api/v1/admin/models/search`

Search models with complex filters and full-text search.

#### Query Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Search query (searches name, family, metadata) |
| `families` | string | Comma-separated families (e.g., "llama,gpt") |
| `types` | string | Comma-separated types (e.g., "chat,completion") |
| `min_context_length` | integer | Minimum context length |
| `max_price_input` | float | Maximum input price per million |
| `limit` | integer | Max results (default: 50) |
| `offset` | integer | Pagination offset (default: 0) |

#### Example Request
```bash
curl -X GET "http://localhost:8080/api/v1/admin/models/search?q=instruct&families=llama,mistral&max_price_input=1.0" \
  -H "X-Admin-Token: your-admin-token"
```

#### Example Response
```json
{
  "data": [...],
  "pagination": {
    "total": 5,
    "limit": 50,
    "offset": 0,
    "has_more": false
  },
  "query": "instruct"
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid model ID format",
    "code": "INVALID_UUID"
  }
}
```

### HTTP Status Codes
- `200 OK` - Successful GET request
- `201 Created` - Successful POST (create)
- `204 No Content` - Successful DELETE
- `400 Bad Request` - Invalid request parameters or body
- `401 Unauthorized` - Missing or invalid authentication
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict (e.g., duplicate name, in-use model)
- `500 Internal Server Error` - Server error

### Common Error Types
- `invalid_request_error` - Invalid parameters or request body
- `validation_error` - Field validation failed
- `authentication_error` - Authentication failed
- `not_found_error` - Resource not found
- `conflict_error` - Resource conflict
- `api_error` - Internal server error

---

## Data Models

### Model Object

```typescript
{
  id: string (UUID)
  name: string                     // e.g., "meta-llama/Llama-3.1-8B-Instruct"
  family: string                   // e.g., "llama", "gpt", "claude"
  size?: string                    // e.g., "8B", "70B"
  type: "completion" | "chat" | "embedding"
  context_length: number           // Max context in tokens
  vram_required_gb: number         // VRAM required in GB
  price_input_per_million: number  // Price per 1M input tokens
  price_output_per_million: number // Price per 1M output tokens
  tokens_per_second_capacity?: number
  status: "active" | "deprecated" | "beta"
  metadata: object                 // JSONB field for flexible data
  created_at: string (ISO 8601)
  updated_at: string (ISO 8601)
}
```

### Metadata Schema

The `metadata` field is a flexible JSONB field that can contain:

```json
{
  "storage": {
    "provider": "cloudflare-r2" | "aws-s3" | "gcp-gcs",
    "bucket": "bucket-name",
    "path": "path/to/model",
    "region": "us-east-1"
  },
  "capabilities": ["chat", "instruction-following", "function-calling"],
  "architecture": "transformer",
  "quantization": "fp16" | "int8" | "int4",
  "license": "apache-2.0",
  "training_data": "...",
  // Any other custom fields
}
```

---

## Interactive Documentation

Visit the interactive API documentation at:
- **Local**: http://localhost:3000/api-docs
- **Production**: https://dashboard.crosslogic.ai/api-docs

The interactive docs provide:
- Full API reference with examples
- "Try it out" functionality to test endpoints
- Request/response schemas
- Authentication testing
- Download OpenAPI specification

---

## Code Examples

### Python

```python
import requests

BASE_URL = "http://localhost:8080"
ADMIN_TOKEN = "your-admin-token"

headers = {
    "X-Admin-Token": ADMIN_TOKEN,
    "Content-Type": "application/json"
}

# List models
response = requests.get(
    f"{BASE_URL}/api/v1/admin/models",
    headers=headers,
    params={"family": "llama", "status": "active"}
)
models = response.json()

# Create model
new_model = {
    "name": "meta-llama/Llama-3.1-8B-Instruct",
    "family": "llama",
    "size": "8B",
    "type": "chat",
    "context_length": 8192,
    "vram_required_gb": 16,
    "price_input_per_million": 0.15,
    "price_output_per_million": 0.60,
    "status": "active"
}

response = requests.post(
    f"{BASE_URL}/api/v1/admin/models",
    headers=headers,
    json=new_model
)
created_model = response.json()

# Update pricing
response = requests.patch(
    f"{BASE_URL}/api/v1/admin/models/{model_id}",
    headers=headers,
    json={"price_input_per_million": 0.12}
)
```

### JavaScript/TypeScript

```typescript
const BASE_URL = "http://localhost:8080";
const ADMIN_TOKEN = "your-admin-token";

const headers = {
  "X-Admin-Token": ADMIN_TOKEN,
  "Content-Type": "application/json",
};

// List models
const listModels = async () => {
  const response = await fetch(
    `${BASE_URL}/api/v1/admin/models?family=llama&status=active`,
    { headers }
  );
  const data = await response.json();
  return data;
};

// Create model
const createModel = async (model: any) => {
  const response = await fetch(`${BASE_URL}/api/v1/admin/models`, {
    method: "POST",
    headers,
    body: JSON.stringify(model),
  });
  return await response.json();
};

// Partial update
const updatePricing = async (modelId: string, pricing: any) => {
  const response = await fetch(`${BASE_URL}/api/v1/admin/models/${modelId}`, {
    method: "PATCH",
    headers,
    body: JSON.stringify(pricing),
  });
  return await response.json();
};
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const (
    baseURL    = "http://localhost:8080"
    adminToken = "your-admin-token"
)

type Model struct {
    ID                      string                 `json:"id,omitempty"`
    Name                    string                 `json:"name"`
    Family                  string                 `json:"family"`
    Size                    *string                `json:"size,omitempty"`
    Type                    string                 `json:"type"`
    ContextLength           int                    `json:"context_length"`
    VRAMRequiredGB          int                    `json:"vram_required_gb"`
    PriceInputPerMillion    float64                `json:"price_input_per_million"`
    PriceOutputPerMillion   float64                `json:"price_output_per_million"`
    TokensPerSecondCapacity *int                   `json:"tokens_per_second_capacity,omitempty"`
    Status                  string                 `json:"status"`
    Metadata                map[string]interface{} `json:"metadata"`
}

func createModel(model Model) (*Model, error) {
    body, _ := json.Marshal(model)

    req, _ := http.NewRequest("POST", baseURL+"/api/v1/admin/models", bytes.NewBuffer(body))
    req.Header.Set("X-Admin-Token", adminToken)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var created Model
    json.NewDecoder(resp.Body).Decode(&created)
    return &created, nil
}
```

---

## Best Practices

### 1. Use PATCH for Partial Updates
When updating only specific fields, use PATCH instead of PUT to avoid unnecessary data transfer and potential conflicts.

### 2. Leverage Metadata Field
Use the metadata JSONB field for flexible, model-specific data without schema changes.

### 3. Pagination
Always use pagination for large result sets. Default limit is 50, max is 100.

### 4. Search vs List
- Use `/models` with filters for simple queries
- Use `/models/search` for complex full-text searches

### 5. Status Management
Instead of deleting models, consider setting status to "deprecated" to maintain data integrity.

### 6. Error Handling
Always check HTTP status codes and parse error responses for detailed information.

---

## Support

For issues, questions, or feature requests:
- Email: support@crosslogic.ai
- Documentation: https://docs.crosslogic.ai
- GitHub: https://github.com/crosslogic/ai-iaas

---

## Changelog

### Version 1.0.0 (2024-01-15)
- Initial release with full CRUD operations
- Advanced filtering and search
- Pagination support
- JSONB metadata field
- Comprehensive error handling
