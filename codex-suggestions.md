# CrossLogic Inference Cloud — Codebase Review & Recommendations

## High-Level Understanding
- **Product goal:** Multi-region, cost-efficient inference cloud with OpenAI-compatible APIs, a Go control plane, Go node agent, and optional Next.js dashboard. Control plane orchestrates scheduling, billing, rate limiting, and node lifecycle; node agent registers GPU workers and proxies vLLM endpoints.
- **Deployment shape:** Docker-compose for local dev; SkyPilot for GPU orchestration; PostgreSQL + Redis for persistence and rate limiting; Stripe for usage-based billing.

## Current Strengths
- Clear separation between control plane (`control-plane/`), node agent (`node-agent/`), and UI (`control-plane/dashboard`).
- API gateway already layers auth, rate limiting, and intelligent load balancing hooks.
- Billing pipeline wires metering, aggregation, and Stripe export with background jobs.
- Documentation is reasonably detailed for quick start and model onboarding.

## Gaps & Production Risks
- **Hard dependency on Stripe credentials** blocked local bring-up. Added `BILLING_ENABLED` toggle, but follow-up: guard dashboard assumptions when billing is off and consider synthetic invoices for smoke tests.
- **Stripe export placeholder**: subscription item ID is hardcoded (`si_placeholder`) which will break real billing. Needs retrieval from tenant plan/config before launch.
- **Webhook robustness**: handler lacks retry/backoff and alerting; Redis-based idempotency exists but no DLQ for poison messages.
- **Rate limiting**: concurrency counter decremented via middleware; ensure every entry point (including future streaming SSE or websocket paths) uses the middleware so counters don’t leak.
- **Security**: Admin endpoints use static token header; recommend mTLS or OIDC for production dashboards. JWT secret currently optional in config validation.
- **Observability**: Gateway metrics are stubbed; no tracing or RED metrics on model-level latency beyond aggregation jobs.
- **Resiliency**: Orchestrator background loops (cache warmer, reconciler) lack panic recovery; consider supervisors so one panic doesn’t take down the server.

## Feature Suggestions (Prioritized)
1. **Billing plan plumbing** — store Stripe subscription item per tenant/environment and feed it into `ExportToStripe`; add test fixtures to avoid silent billing skips.
2. **Operational toggles** — extend `BILLING_ENABLED` to also silence the webhook route in the dashboard and to swap pricing calculators with fixed-rate mocks for CI.
3. **Deployment hardening** — add readiness checks for DB/Redis before serving traffic; surface health in `/ready` with dependency probes.
4. **Rate limit telemetry** — emit Prometheus metrics for rejections and latency per tenant/key to detect noisy neighbors.
5. **Node agent trust** — sign heartbeats with shared secret or short-lived token to prevent rogue node registrations; add TLS support for agent ↔ control plane traffic.
6. **Model registry UX** — add admin endpoint to list supported model templates and per-model VRAM requirements to prevent bad deployments.
7. **Incident tooling** — add structured audit logs for admin actions and alert hooks (PagerDuty/email) on billing export failures or orchestrator reconciliation errors.

## Quick Deployment Notes
- Local dev can run with `BILLING_ENABLED=false` (Stripe-free) while still retaining metering for analytics.
- Ensure PostgreSQL migrations (`migrations/`) are applied before starting the control plane; docker-compose currently requires manual psql invocation.
- GPU nodes launched via SkyPilot must align `VLLM_VERSION`/`TORCH_VERSION` with the control plane runtime config to avoid ABI mismatches.

## Next Steps After This Patch
- Wire subscription item IDs and add end-to-end billing tests using Stripe’s test mode.
- Expand `/ready` endpoint to check DB and Redis, and fail fast on startup if prerequisites are missing.
- Add dashboards for rate-limit rejection counts and per-model latency to validate the scheduler’s queue monitoring.
