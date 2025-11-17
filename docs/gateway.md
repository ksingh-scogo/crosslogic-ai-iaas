# API Gateway

Implements the OpenAI-compatible entrypoint with authentication, rate limiting, and request validation.

## Pipeline
1. **Parse** HTTP headers and query params into a `models.Request`.
2. **Authenticate** using `Authenticator.ValidateAPIKey`, caching lookups for 5 minutes.
3. **Rate limit** with token bucket semantics (default 10 tokens/second per key in-memory).
4. **Validate** required fields and enrich with tenant/environment metadata.
5. **Route** via `router.Router` to select a healthy node.
6. **Account** for tokens and latency, emitting a `models.Response` suitable for billing and logging.

## Notes from PRD
- API key format `sk_<env>_<random>` is supported by treating the full header as the key.
- Direct HTTPS routing to nodes keeps latency low; mesh VPNs are intentionally excluded.
- Middleware chain can be extended with CORS, audit logging, or schema validation.

## Local Testing
```bash
curl -X POST "http://localhost:8080/v1/chat/completions?model=llama-7b&region=ap-south-1&prompt=hello" \
  -H "Authorization: Bearer sk_dev_demo"
```
The response reports which node was selected and token counts for billing.

