# CrossLogic AI - Quick Start Guide

Get started with CrossLogic AI inference API in minutes. This guide will help you make your first API call.

## Prerequisites

- A CrossLogic AI account
- An API key (get one from your [Dashboard](https://dashboard.crosslogic.ai))

## Base URL

```
https://api.crosslogic.ai
```

## Authentication

All API requests require authentication using a Bearer token in the Authorization header:

```bash
Authorization: Bearer YOUR_API_KEY
```

## Making Your First Request

### Chat Completions (Recommended)

```bash
curl https://api.crosslogic.ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

### Response

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "meta-llama/Llama-3.1-8B-Instruct",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing great, thank you for asking. How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 18,
    "total_tokens": 30
  }
}
```

## Streaming Responses

For real-time responses, enable streaming:

```bash
curl https://api.crosslogic.ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "meta-llama/Llama-3.1-8B-Instruct",
    "messages": [
      {"role": "user", "content": "Write a short poem about AI"}
    ],
    "stream": true
  }'
```

## Available Models

| Model | Best For | Context Length |
|-------|----------|----------------|
| `meta-llama/Llama-3.1-8B-Instruct` | General chat, fast responses | 8,192 tokens |
| `meta-llama/Llama-3.1-70B-Instruct` | Complex reasoning, high quality | 8,192 tokens |
| `mistralai/Mistral-7B-Instruct-v0.3` | Efficient, multilingual | 32,768 tokens |
| `Qwen/Qwen2.5-7B-Instruct` | Coding, math, multilingual | 32,768 tokens |

Get the full list of available models:

```bash
curl https://api.crosslogic.ai/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## SDK Examples

### Python

```python
import requests

API_KEY = "YOUR_API_KEY"
BASE_URL = "https://api.crosslogic.ai"

response = requests.post(
    f"{BASE_URL}/v1/chat/completions",
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    },
    json={
        "model": "meta-llama/Llama-3.1-8B-Instruct",
        "messages": [
            {"role": "user", "content": "Explain quantum computing in simple terms"}
        ],
        "max_tokens": 500
    }
)

data = response.json()
print(data["choices"][0]["message"]["content"])
```

### JavaScript/Node.js

```javascript
const response = await fetch('https://api.crosslogic.ai/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    model: 'meta-llama/Llama-3.1-8B-Instruct',
    messages: [
      { role: 'user', content: 'What is machine learning?' }
    ]
  })
});

const data = await response.json();
console.log(data.choices[0].message.content);
```

### Using OpenAI SDK (Compatible)

CrossLogic AI is OpenAI API compatible. Use the OpenAI SDK with our base URL:

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_CROSSLOGIC_API_KEY",
    base_url="https://api.crosslogic.ai/v1"
)

response = client.chat.completions.create(
    model="meta-llama/Llama-3.1-8B-Instruct",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)
```

## Rate Limits

Rate limits are applied per API key. You can monitor your usage via response headers:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests per minute |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when the window resets |
| `Retry-After` | Seconds to wait before retrying (on 429) |

## Error Handling

### Common Errors

| Status | Error Type | Description |
|--------|------------|-------------|
| 400 | `invalid_request_error` | Invalid request parameters |
| 401 | `authentication_error` | Invalid or missing API key |
| 429 | `rate_limit_error` | Rate limit exceeded |
| 500 | `api_error` | Internal server error |

### Error Response Format

```json
{
  "error": {
    "message": "Invalid API key provided",
    "type": "authentication_error"
  }
}
```

## Best Practices

1. **Store API keys securely** - Never expose keys in client-side code
2. **Handle rate limits** - Implement exponential backoff on 429 errors
3. **Use streaming** - For better UX on long responses
4. **Set max_tokens** - Control response length and costs
5. **Include X-Request-ID** - Useful for debugging and support tickets

## Request Tracing

Every response includes an `X-Request-ID` header. Include this ID when contacting support for faster issue resolution.

## Next Steps

- [Full API Reference](./API_REFERENCE.md)
- [Usage & Billing](./USAGE_BILLING.md)
- [Best Practices](./BEST_PRACTICES.md)
- [FAQ](./FAQ.md)

## Support

- Email: support@crosslogic.ai
- Documentation: https://docs.crosslogic.ai
- Status Page: https://status.crosslogic.ai
