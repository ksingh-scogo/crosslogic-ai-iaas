# Security

Security is a top priority for the CrossLogic Inference Cloud. This document outlines the security mechanisms and best practices.

## Authentication & Authorization

-   **API Keys**: All API requests must be authenticated with a Bearer token. Keys are hashed (`SHA-256`) before storage in the database.
-   **Role-Based Access Control (RBAC)**:
    -   `admin`: Full access to all resources.
    -   `developer`: Can manage keys and view usage.
    -   `read-only`: Can only view data.
-   **Tenant Isolation**: All database queries are scoped by `tenant_id` to prevent data leakage.

## Network Security

-   **TLS**: In production, all traffic should be encrypted via TLS. The application expects to run behind a load balancer or reverse proxy (e.g., Nginx, CloudFlare) that handles TLS termination.
-   **Private Networking**: The Control Plane and Database should run in a private subnet, not directly exposed to the internet. Only the API Gateway port (8080) should be exposed via the Load Balancer.

## Data Protection

-   **Encryption at Rest**: Use PostgreSQL TDE or volume encryption for the database.
-   **Secrets Management**:
    -   Never commit `.env` files to version control.
    -   Use a secrets manager (e.g., AWS Secrets Manager, HashiCorp Vault) in production.
    -   Rotate `STRIPE_SECRET_KEY` and `ADMIN_API_TOKEN` regularly.

## Rate Limiting

To prevent DoS attacks and abuse, rate limiting is enforced at multiple levels:

1.  **Global**: Hard cap on total system throughput.
2.  **IP-based**: (Recommended at Load Balancer level).
3.  **Token-based**: Configurable limits per API key (requests/min, tokens/min).

## Compliance

-   **GDPR/CCPA**: The system is designed to minimize PII storage. User prompts and completions are **not** logged by default, only metadata (token counts) is stored for billing.
