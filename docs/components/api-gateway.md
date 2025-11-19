# API Gateway

The API Gateway is the entry point for all requests to the CrossLogic Inference Cloud (CIC). It handles authentication, rate limiting, and request routing.

## Responsibilities

- **Authentication**: Validates API keys against the database.
- **Rate Limiting**: Enforces limits based on tenant, environment, and key policies using Redis.
- **Routing**: Forwards valid requests to the Scheduler or other internal services.
- **Metrics**: Collects request metrics for monitoring and billing.

## Architecture

The Gateway is implemented in Go and uses the standard `net/http` library (or a framework like Gin/Echo if used). It sits behind a load balancer (e.g., CloudFlare, AWS ALB).

### Authentication Flow

1.  **Extraction**: The Gateway extracts the Bearer token from the `Authorization` header.
2.  **Validation**: It checks the token against the `api_keys` table in the database (cached in Redis for performance).
3.  **Context**: If valid, it injects Tenant ID and Environment ID into the request context.

### Rate Limiting

Rate limiting is implemented using a sliding window algorithm backed by Redis.

-   **Levels**:
    -   **Global**: Protects the entire platform.
    -   **Tenant**: Limits per organization.
    -   **Key**: Limits per specific API key.
-   **Headers**: Standard `X-RateLimit-*` headers are returned.

### Request Routing

-   `/v1/chat/completions` -> **Scheduler**
-   `/v1/embeddings` -> **Scheduler**
-   `/v1/models` -> **Control Plane** (Metadata)
-   `/admin/*` -> **Admin Handlers** (Internal/Protected)

## Configuration

Key environment variables:

-   `SERVER_PORT`: Port to listen on (default: 8080).
-   `REDIS_HOST`: Redis connection for rate limiting.
-   `DB_HOST`: Database connection for API key validation.
