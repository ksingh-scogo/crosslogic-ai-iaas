# Control Plane Blueprint

This document explains how the single-binary Go control plane aligns with the PRD and how to extend it beyond the MVP scaffold.

## Responsibilities
- API key validation and per-tenant rate limiting.
- Region- and model-aware routing to HTTPS worker endpoints (no VPN/mesh as per `PRD/mesh-network-not-needed.md`).
- Node registry and health monitoring.
- Usage metering for Stripe metered billing.
- Capacity reservations for reserved CUs.

## Configuration
- `DATABASE_URL`: PostgreSQL connection for production (SQLite allowed in development).
- `REDIS_URL`: Redis connection for rate limiting/caching; `LocalCache` can be used locally.
- `PORT`: HTTP bind address for the gateway/control-plane server.

## Developer workflow (sequential)
1. Start Postgres and Redis (or keep the in-memory defaults for local testing).
2. Seed tenants, API keys, and nodes (see `cmd/server/main.go` for examples).
3. Run `go test ./...` then `go run ./cmd/server`.
4. Issue a request with `Authorization: Bearer <api-key>` and query params `model`, `region`, and `prompt`.
5. Inspect usage records via `database.Store.ListUsageByTenant` to verify billing events.

## Extending
- Replace `InMemoryStore` with pgx-backed implementations.
- Swap `LocalCache` with Redis for distributed rate limiting.
- Add gRPC endpoints for node agents to register and heartbeat.

