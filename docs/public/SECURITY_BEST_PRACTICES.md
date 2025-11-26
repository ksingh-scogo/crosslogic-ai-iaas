# CrossLogic AI - Security Best Practices

This guide outlines security best practices for integrating with the CrossLogic AI API.

## API Key Security

### Protect Your API Keys

API keys are credentials that provide access to your account. Treat them like passwords.

**Do:**
- Store API keys in environment variables or secure secret managers
- Use different keys for development and production
- Rotate keys periodically (every 90 days recommended)
- Set appropriate rate limits on each key
- Revoke unused or compromised keys immediately

**Don't:**
- Hard-code API keys in your source code
- Commit API keys to version control (Git)
- Share API keys via email or chat
- Expose keys in client-side code (browser/mobile apps)
- Log API keys in application logs

### Environment Variables

```bash
# .env file (add to .gitignore!)
CROSSLOGIC_API_KEY=clsk_live_xxxxxxxxxxxx
```

```python
# Python
import os
api_key = os.environ.get('CROSSLOGIC_API_KEY')
```

```javascript
// Node.js
const apiKey = process.env.CROSSLOGIC_API_KEY;
```

### Secret Managers

For production environments, use a secret manager:

- **AWS Secrets Manager**
- **Google Cloud Secret Manager**
- **Azure Key Vault**
- **HashiCorp Vault**

## Server-Side Integration

### Backend API Calls Only

Never make API calls directly from client-side code (browsers, mobile apps). Always route through your backend server.

**Correct Architecture:**
```
[User Browser] → [Your Backend] → [CrossLogic API]
```

**Backend Example (Node.js):**
```javascript
// Your backend endpoint
app.post('/api/chat', async (req, res) => {
  const { message } = req.body;

  // Call CrossLogic from your backend
  const response = await fetch('https://api.crosslogic.ai/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${process.env.CROSSLOGIC_API_KEY}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'meta-llama/Llama-3.1-8B-Instruct',
      messages: [{ role: 'user', content: message }]
    })
  });

  const data = await response.json();
  res.json(data);
});
```

## Input Validation

### Sanitize User Input

Always validate and sanitize user inputs before sending to the API.

```python
import re

def sanitize_input(text: str, max_length: int = 10000) -> str:
    """Sanitize user input before API call."""
    if not isinstance(text, str):
        raise ValueError("Input must be a string")

    # Limit length
    text = text[:max_length]

    # Remove control characters (except newlines)
    text = re.sub(r'[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]', '', text)

    return text.strip()
```

### Validate Response Data

Always validate API responses before using them in your application.

```python
def validate_response(response_data: dict) -> str:
    """Validate and extract content from API response."""
    if 'error' in response_data:
        raise APIError(response_data['error']['message'])

    choices = response_data.get('choices', [])
    if not choices:
        raise ValueError("No response choices returned")

    content = choices[0].get('message', {}).get('content', '')
    return content
```

## Rate Limiting

### Handle Rate Limits Gracefully

Implement exponential backoff when rate limited.

```python
import time
import random

def make_request_with_backoff(request_func, max_retries=5):
    """Make request with exponential backoff."""
    for attempt in range(max_retries):
        response = request_func()

        if response.status_code == 429:
            # Get retry-after or calculate backoff
            retry_after = int(response.headers.get('Retry-After', 0))
            if retry_after == 0:
                # Exponential backoff with jitter
                retry_after = (2 ** attempt) + random.uniform(0, 1)

            time.sleep(retry_after)
            continue

        return response

    raise Exception("Max retries exceeded")
```

### Client-Side Rate Limiting

Implement your own rate limiting to stay within limits:

```python
from functools import wraps
import time
import threading

class RateLimiter:
    def __init__(self, calls_per_minute: int):
        self.calls_per_minute = calls_per_minute
        self.calls = []
        self.lock = threading.Lock()

    def acquire(self):
        with self.lock:
            now = time.time()
            # Remove calls older than 1 minute
            self.calls = [t for t in self.calls if now - t < 60]

            if len(self.calls) >= self.calls_per_minute:
                sleep_time = 60 - (now - self.calls[0])
                if sleep_time > 0:
                    time.sleep(sleep_time)

            self.calls.append(time.time())

# Usage
limiter = RateLimiter(calls_per_minute=60)

def make_api_call():
    limiter.acquire()
    # Make your API call here
```

## Error Handling

### Don't Expose Internal Errors

Never expose raw API errors to end users.

```python
def handle_api_response(response):
    """Handle API response with proper error messages."""
    if response.status_code == 200:
        return response.json()

    # Log full error internally
    logger.error(f"API Error: {response.status_code} - {response.text}")

    # Return generic error to user
    if response.status_code == 429:
        raise UserFacingError("Service is busy. Please try again shortly.")
    elif response.status_code == 401:
        raise UserFacingError("Authentication error. Please contact support.")
    elif response.status_code >= 500:
        raise UserFacingError("Service temporarily unavailable.")
    else:
        raise UserFacingError("Unable to process your request.")
```

## Logging

### Safe Logging Practices

Log requests for debugging but never log sensitive data.

```python
import logging

def log_api_request(model: str, prompt_length: int, response_status: int):
    """Log API request safely."""
    logging.info(
        "API Request",
        extra={
            "model": model,
            "prompt_length": prompt_length,  # Length, not content
            "response_status": response_status,
            # Never log: api_key, full prompt, full response
        }
    )
```

## Content Security

### Prompt Injection Prevention

Be cautious of prompt injection attacks when user input is included in prompts.

```python
def create_safe_prompt(system_prompt: str, user_input: str) -> list:
    """Create messages with clear separation."""
    return [
        {
            "role": "system",
            "content": system_prompt
        },
        {
            "role": "user",
            "content": user_input  # Keep user input separate
        }
    ]

# Bad practice - concatenating user input into system prompt
# system = f"You are a helpful assistant. User wants: {user_input}"

# Good practice - separate messages
messages = create_safe_prompt(
    "You are a helpful assistant.",
    user_input
)
```

### Output Filtering

Consider filtering AI outputs before displaying to users.

```python
def filter_output(content: str) -> str:
    """Filter potentially sensitive content from AI output."""
    # Implement based on your use case
    # Examples: PII detection, content moderation, etc.
    return content
```

## Network Security

### Use HTTPS

Always use HTTPS for API calls. Our API enforces TLS 1.2+.

### Request Timeouts

Set appropriate timeouts to prevent hanging requests.

```python
import requests

response = requests.post(
    'https://api.crosslogic.ai/v1/chat/completions',
    headers=headers,
    json=data,
    timeout=(5, 120)  # (connect timeout, read timeout)
)
```

## Compliance

### Data Handling

- CrossLogic does not store your prompts or completions
- Only usage metadata is retained for billing
- Review our [Privacy Policy](https://crosslogic.ai/privacy) for details

### Audit Logging

For compliance requirements, maintain your own audit logs:

```python
def log_audit_event(user_id: str, action: str, request_id: str):
    """Log audit event for compliance."""
    audit_logger.info({
        "timestamp": datetime.utcnow().isoformat(),
        "user_id": user_id,
        "action": action,
        "request_id": request_id,  # X-Request-ID from response
        "ip_address": get_client_ip()
    })
```

## Security Checklist

Before going to production:

- [ ] API keys stored in environment variables or secret manager
- [ ] API calls made from backend only (not client-side)
- [ ] Input validation and sanitization implemented
- [ ] Rate limiting handled with exponential backoff
- [ ] Error messages don't expose internal details
- [ ] Sensitive data not logged
- [ ] HTTPS enforced
- [ ] Request timeouts configured
- [ ] Audit logging in place (if required)

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly:

- **Email:** security@crosslogic.ai
- **Do not** disclose publicly until we've addressed the issue

We appreciate responsible disclosure and will acknowledge your contribution.

## Resources

- [API Reference](./API_REFERENCE.md)
- [Rate Limiting Guide](./RATE_LIMITING.md)
- [Privacy Policy](https://crosslogic.ai/privacy)
- [Terms of Service](https://crosslogic.ai/terms)
