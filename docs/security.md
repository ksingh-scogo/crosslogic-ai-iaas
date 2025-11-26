# Security

Security is a top priority for the CrossLogic Inference Cloud. This document outlines the security mechanisms and best practices.

## Authentication & Authorization

### API Key Security

-   **API Keys**: All API requests must be authenticated with a Bearer token.
-   **Secure Storage**: Keys are hashed using `SHA-256` before storage. Only the prefix is stored in plaintext for identification.
-   **Key Format**: `clsk_{env}_{random}` (e.g., `clsk_live_a4f5b2c8d9e0...`)
-   **Expiration**: Keys can have optional expiration dates.
-   **Caching**: Validated keys are cached in Redis for 60 seconds to reduce database load.

### Role-Based Access Control (RBAC)

-   `admin`: Full access to all resources within the tenant.
-   `developer`: Can manage keys, view usage, and make API calls.
-   `read-only`: Can only view data, cannot modify or make inference calls.

### Admin Authentication

-   Admin endpoints require `X-Admin-Token` header.
-   Tokens are compared using constant-time comparison to prevent timing attacks.
-   All admin actions are logged for audit purposes.

### Tenant Isolation

-   All database queries are scoped by `tenant_id` to prevent data leakage.
-   Environment separation (dev/staging/prod) within each tenant.
-   API keys are bound to specific tenant and environment.

## Network Security

### Transport Layer Security

-   **TLS 1.2+**: All production traffic must be encrypted via TLS.
-   **Certificate Management**: Use cert-manager or similar for automatic certificate rotation.
-   **HSTS**: Strict-Transport-Security header enforced (max-age=31536000).

### Network Architecture

-   **Private Networking**: Control Plane and Database in private subnet.
-   **Load Balancer**: Only port 443 exposed via Load Balancer.
-   **Internal Communication**: Service mesh or private networking for inter-service communication.

### Security Headers

All API responses include security headers:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'; frame-ancestors 'none'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
Cache-Control: no-store, no-cache, must-revalidate
```

## Input Validation & Sanitization

### Request Validation

-   **Size Limits**: Request bodies limited to 10MB.
-   **Content-Type**: JSON content-type enforced for API endpoints.
-   **Schema Validation**: Request payloads validated against expected schema.
-   **SQL Injection Prevention**: All database queries use parameterized statements.

### Output Sanitization

-   **Error Messages**: Internal details not exposed in error responses.
-   **Logging**: API keys anonymized in logs (only prefix shown).
-   **Stack Traces**: Never exposed to clients, only logged internally.

## Rate Limiting

Multi-layer rate limiting to prevent DoS attacks and abuse:

### Layer 1: Global Rate Limit
-   Hard cap on total system throughput.
-   Protects entire infrastructure.

### Layer 2: Tenant Rate Limit
-   Default: 50,000 requests/minute per tenant.
-   Configurable per pricing plan.

### Layer 3: Environment Rate Limit
-   Default: 10,000 requests/minute per environment.
-   Allows isolation between dev/staging/prod.

### Layer 4: API Key Rate Limit
-   Configurable per key: `rate_limit_requests_per_min`, `rate_limit_tokens_per_min`.
-   Concurrency limits: `concurrency_limit` (default: 10).

### Rate Limit Headers

Responses include rate limit information:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1705315200
Retry-After: 30  (on 429 responses)
```

## Data Protection

### Encryption at Rest

-   **Database**: PostgreSQL with volume-level encryption (AWS EBS encryption or equivalent).
-   **Model Storage**: Cloudflare R2 with server-side encryption.
-   **Backups**: Encrypted backups stored in separate region.

### Encryption in Transit

-   TLS 1.2+ for all external connections.
-   Internal services communicate over private network.

### Secrets Management

-   Never commit `.env` files to version control.
-   Use secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.) in production.
-   Rotate credentials regularly:
    -   `STRIPE_SECRET_KEY`: Every 90 days.
    -   `ADMIN_API_TOKEN`: Every 30 days.
    -   Database credentials: Every 90 days.

### Data Minimization

-   User prompts and completions are **not** stored.
-   Only metadata (token counts, latency) retained for billing.
-   Usage data retained for 90 days by default.

## Webhook Security

### Stripe Webhooks

-   Signature verification using `STRIPE_WEBHOOK_SECRET`.
-   Idempotency handling to prevent duplicate processing.
-   Webhook events logged with status.

### Outgoing Webhooks (Notifications)

-   HMAC-SHA256 signature included in `X-Signature` header.
-   Retry with exponential backoff for failed deliveries.
-   TLS required for webhook endpoints.

## Audit Logging

### Events Logged

-   API key creation/revocation.
-   Admin authentication attempts.
-   Tenant status changes.
-   Node launch/termination.
-   Billing events.

### Log Format

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_abc123",
  "action": "api_key_created",
  "actor_id": "user_xyz",
  "tenant_id": "tenant_123",
  "ip_address": "192.168.1.1",
  "details": {}
}
```

## Compliance

### GDPR/CCPA

-   Minimal PII storage.
-   Data deletion capabilities.
-   Usage data retention limits.
-   Clear privacy policy.

### SOC 2 Preparation

-   Audit logging enabled.
-   Access controls documented.
-   Incident response procedures.
-   Regular security reviews.

### PCI DSS (if applicable)

-   No card data stored (delegated to Stripe).
-   Stripe handles all payment processing.

## Incident Response

### Security Incident Procedure

1. **Detection**: Monitor alerts, logs, and customer reports.
2. **Containment**: Isolate affected systems.
3. **Investigation**: Determine scope and impact.
4. **Remediation**: Fix vulnerabilities, rotate credentials.
5. **Communication**: Notify affected parties if required.
6. **Post-mortem**: Document lessons learned.

### Contact

-   Security issues: security@crosslogic.ai
-   Bug bounty: See responsible disclosure policy.

## Security Checklist for Deployment

- [ ] TLS certificates configured and auto-renewal enabled
- [ ] Security headers middleware enabled
- [ ] Rate limiting configured at all layers
- [ ] Database credentials rotated from defaults
- [ ] Admin token set to secure value
- [ ] Stripe webhook secret configured
- [ ] Private networking configured
- [ ] Audit logging enabled
- [ ] Backup encryption enabled
- [ ] Monitoring and alerting configured
