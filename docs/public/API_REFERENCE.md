# CrossLogic AI - API Reference

Complete API reference for the CrossLogic AI inference platform.

## Overview

The CrossLogic AI API provides OpenAI-compatible endpoints for LLM inference. Use the same patterns and SDKs you're familiar with.

**Base URL:** `https://api.crosslogic.ai`

**API Version:** v1

---

## Authentication

All API requests require authentication via Bearer token.

```http
Authorization: Bearer YOUR_API_KEY
```

API keys can be created and managed from your [Dashboard](https://dashboard.crosslogic.ai).

### API Key Format

API keys follow the format: `clsk_live_XXXXXXXXXXXXXXXX`

- `clsk` - CrossLogic prefix
- `live` - Environment (live/test)
- Remaining characters - Unique identifier

### Security Best Practices

- Never expose API keys in client-side code
- Rotate keys periodically
- Use environment-specific keys (test vs production)
- Set appropriate rate limits per key

---

## Endpoints

### Chat Completions

Create a chat completion response.

```http
POST /v1/chat/completions
```

#### Request Body

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model ID to use |
| `messages` | array | Yes | Array of message objects |
| `temperature` | number | No | Sampling temperature (0-2). Default: 1 |
| `max_tokens` | integer | No | Maximum tokens to generate |
| `stream` | boolean | No | Enable streaming. Default: false |
| `top_p` | number | No | Nucleus sampling parameter |
| `stop` | string/array | No | Stop sequences |

#### Message Object

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `role` | string | Yes | `system`, `user`, or `assistant` |
| `content` | string | Yes | Message content |

#### Example Request

```bash
curl https://api.crosslogic.ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

#### Response

```json
{
  "id": "chatcmpl-abc123def456",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "meta-llama/Llama-3.1-8B-Instruct",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 8,
    "total_tokens": 33
  }
}
```

---

### Text Completions

Create a text completion (legacy format).

```http
POST /v1/completions
```

#### Request Body

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model ID to use |
| `prompt` | string | Yes | Text prompt to complete |
| `max_tokens` | integer | No | Maximum tokens to generate |
| `temperature` | number | No | Sampling temperature (0-2) |
| `stream` | boolean | No | Enable streaming |

#### Example Request

```bash
curl https://api.crosslogic.ai/v1/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "prompt": "Once upon a time",
    "max_tokens": 50
  }'
```

---

### Embeddings

Generate embeddings for text.

```http
POST /v1/embeddings
```

#### Request Body

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Embedding model ID |
| `input` | string/array | Yes | Text to embed |

#### Example Request

```bash
curl https://api.crosslogic.ai/v1/embeddings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "BAAI/bge-base-en-v1.5",
    "input": "The quick brown fox jumps over the lazy dog"
  }'
```

#### Response

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0023, -0.0134, 0.0456, ...]
    }
  ],
  "model": "BAAI/bge-base-en-v1.5",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}
```

---

### List Models

List all available models.

```http
GET /v1/models
```

#### Example Request

```bash
curl https://api.crosslogic.ai/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "meta-llama/Llama-3.1-8B-Instruct",
      "object": "model",
      "created": 1700000000,
      "owned_by": "crosslogic"
    },
    {
      "id": "meta-llama/Llama-3.1-70B-Instruct",
      "object": "model",
      "created": 1700000000,
      "owned_by": "crosslogic"
    }
  ]
}
```

---

### Get Model

Retrieve details about a specific model.

```http
GET /v1/models/{model_id}
```

#### Example Request

```bash
curl https://api.crosslogic.ai/v1/models/meta-llama/Llama-3.1-8B-Instruct \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Usage & Billing

### Get Usage Summary

Retrieve your usage statistics.

```http
GET /v1/usage
```

#### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `start_date` | string | Start date (YYYY-MM-DD) |
| `end_date` | string | End date (YYYY-MM-DD) |

#### Example Request

```bash
curl "https://api.crosslogic.ai/v1/usage?start_date=2024-01-01&end_date=2024-01-31" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### Get Usage by Model

```http
GET /v1/usage/by-model
```

### Get Usage by API Key

```http
GET /v1/usage/by-key
```

---

## API Keys Management

### Create API Key

```http
POST /v1/api-keys
```

#### Request Body

```json
{
  "name": "Production Key",
  "rate_limit_requests_per_min": 60,
  "rate_limit_tokens_per_min": 100000
}
```

### List API Keys

```http
GET /v1/api-keys
```

### Revoke API Key

```http
DELETE /v1/api-keys/{key_id}
```

---

## Streaming

For streaming responses, set `stream: true` in your request. Responses are sent as Server-Sent Events (SSE).

### Stream Format

```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"meta-llama/Llama-3.1-8B-Instruct","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"meta-llama/Llama-3.1-8B-Instruct","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"meta-llama/Llama-3.1-8B-Instruct","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

### Python Streaming Example

```python
import requests

response = requests.post(
    "https://api.crosslogic.ai/v1/chat/completions",
    headers={
        "Authorization": "Bearer YOUR_API_KEY",
        "Content-Type": "application/json"
    },
    json={
        "model": "meta-llama/Llama-3.1-8B-Instruct",
        "messages": [{"role": "user", "content": "Tell me a story"}],
        "stream": True
    },
    stream=True
)

for line in response.iter_lines():
    if line:
        line = line.decode('utf-8')
        if line.startswith('data: ') and line != 'data: [DONE]':
            print(line[6:])
```

---

## Rate Limits

Rate limits are enforced per API key. Monitor your limits via response headers:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests per minute |
| `X-RateLimit-Remaining` | Requests remaining |
| `X-RateLimit-Reset` | Unix timestamp of window reset |
| `Retry-After` | Seconds to wait (on 429 response) |

### Handling Rate Limits

```python
import time
import requests

def make_request_with_retry(url, headers, data, max_retries=3):
    for attempt in range(max_retries):
        response = requests.post(url, headers=headers, json=data)

        if response.status_code == 429:
            retry_after = int(response.headers.get('Retry-After', 60))
            time.sleep(retry_after)
            continue

        return response

    raise Exception("Max retries exceeded")
```

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "message": "Human-readable error description",
    "type": "error_type",
    "code": "ERROR_CODE"
  }
}
```

### Error Types

| Type | HTTP Status | Description |
|------|-------------|-------------|
| `invalid_request_error` | 400 | Invalid request parameters |
| `authentication_error` | 401 | Invalid or missing API key |
| `permission_denied_error` | 403 | Insufficient permissions |
| `not_found_error` | 404 | Resource not found |
| `rate_limit_error` | 429 | Rate limit exceeded |
| `api_error` | 500 | Internal server error |
| `service_unavailable` | 503 | Service temporarily unavailable |

### Error Handling Example

```python
import requests

try:
    response = requests.post(url, headers=headers, json=data)
    response.raise_for_status()
    return response.json()
except requests.exceptions.HTTPError as e:
    error_data = e.response.json()
    error_type = error_data.get('error', {}).get('type', 'unknown')
    error_message = error_data.get('error', {}).get('message', 'Unknown error')

    if e.response.status_code == 429:
        # Handle rate limit
        retry_after = e.response.headers.get('Retry-After', 60)
        print(f"Rate limited. Retry after {retry_after} seconds")
    elif e.response.status_code == 401:
        # Handle authentication error
        print("Invalid API key")
    else:
        print(f"Error: {error_type} - {error_message}")
```

---

## Response Headers

All responses include helpful headers:

| Header | Description |
|--------|-------------|
| `X-Request-ID` | Unique request identifier (for support) |
| `X-RateLimit-*` | Rate limit information |
| `Content-Type` | Always `application/json` |

---

## Health Check

Check API availability:

```http
GET /health
```

Response:
```json
{
  "status": "healthy",
  "time": "2024-01-15T10:30:00Z"
}
```

---

## SDK Support

CrossLogic AI is compatible with OpenAI SDKs. Simply change the base URL:

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_CROSSLOGIC_API_KEY",
    base_url="https://api.crosslogic.ai/v1"
)
```

### JavaScript (OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'YOUR_CROSSLOGIC_API_KEY',
  baseURL: 'https://api.crosslogic.ai/v1'
});
```

### LangChain

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    model="meta-llama/Llama-3.1-8B-Instruct",
    openai_api_key="YOUR_CROSSLOGIC_API_KEY",
    openai_api_base="https://api.crosslogic.ai/v1"
)
```

---

## Changelog

### v1.0.0 (2024-01)
- Initial public release
- Chat completions, text completions, embeddings
- OpenAI API compatibility
- Rate limiting with headers
- Streaming support

---

## Support

- **Email:** support@crosslogic.ai
- **Documentation:** https://docs.crosslogic.ai
- **Status:** https://status.crosslogic.ai

When contacting support, please include:
- Your `X-Request-ID` from the response headers
- Timestamp of the issue
- Request details (without your API key)
