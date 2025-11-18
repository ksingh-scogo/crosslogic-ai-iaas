<!-- e032800e-3e9e-4722-979f-eacd5cf112f3 ef564413-b055-430a-acbf-70310b68fc70 -->
# CrossLogic Inference Cloud - Improvement Plan

## 1. Critical Security & Billing Fixes (Priority: P0)

The current codebase has severe security vulnerabilities and revenue leaks that must be addressed before any deployment.

### 1.1 Fix Admin Authentication

- **Issue**: `adminAuthMiddleware` checks for a non-empty token but does not validate it against any secret.
- **Fix**:
- Implement strict token validation against a `ADMIN_API_TOKEN` environment variable.
- Use constant-time comparison (`subtle.ConstantTimeCompare`) to prevent timing attacks.
- Add audit logging for all admin actions.

### 1.2 Implement Billing for Streaming Requests

- **Issue**: `vllm_proxy.go` streams chunks but ignores the final usage report from vLLM, causing 0 billing for streaming requests.
- **Fix**:
- In `streamResponse`, intercept the final SSE event (usually `[DONE]` or a usage event from vLLM).
- Parse the token usage from the final chunk.
- Asynchronously call `g.recordUsage` (requires refactoring `VLLMProxy` to have access to a recorder or return usage data).

### 1.3 Fix SkyPilot Template Injection

- **Issue**: `VLLMArgs` is injected directly into YAML templates, allowing potential command execution if an admin is compromised.
- **Fix**:
- Validate `VLLMArgs` against an allowlist of flags or strict regex.
- Quote the arguments in the YAML template to prevent breaking out of the command string.

## 2. Reliability & Architecture Improvements (Priority: P1)

### 2.1 Real Load Balancing

- **Issue**: `LeastLoadedStrategy` selects nodes based on *health score*, not actual load.
- **Fix**:
- Track active request counts per node in Redis (`INCR`/`DECR` on request start/end).
- Update `LeastLoadedStrategy` to query Redis for real-time concurrency.
- Add "Pending Tokens" tracking for more accurate load estimation (optional but recommended).

### 2.2 Robust Webhook Idempotency

- **Issue**: `processedEvents` is an in-memory map in `WebhookHandler`. It resets on restart and isn't shared across replicas.
- **Fix**:
- Replace in-memory map with Redis `SETNX` with a TTL (e.g., 24 hours).
- Alternatively, rely strictly on the `webhook_events` table unique constraint (already implemented) but ensure the check happens *before* processing logic to avoid wasted work.

### 2.3 Circuit Breaker Concurrency

- **Issue**: Race condition in `isNodeHealthy` when upgrading RLock to Lock.
- **Fix**:
- Simplify locking: Use a full Lock for state transitions, or use atomic CAS operations if performance is critical (unlikely here).

### 2.4 Update Dependencies

- **Issue**: SkyPilot template hardcodes `vllm==0.2.7`.
- **Fix**:
- Update to latest stable vLLM (e.g., `0.6.x`).
- Make versions configurable via `config.yaml` or environment variables.

## 3. Feature Completeness (Priority: P2)

### 3.1 Dashboard UI

- **Status**: Missing.
- **Plan**:
- Scaffold Next.js 15 app in `dashboard/`.
- Implement Auth (Google OAuth / Email).
- Build "API Keys" management page.
- Build "Usage & Billing" page.
- Connect to Control Plane Admin API.

### 3.2 Testing Suite

- **Status**: Missing.
- **Plan**:
- Add unit tests for `Scheduler` and `RateLimiter`.
- Create an integration test suite that spins up a mock vLLM server and verifies the full Gateway -> Proxy -> Billing pipeline.

## 4. Deployment Readiness (Priority: P3)

### 4.1 CI/CD Pipeline

- Create GitHub Actions workflow for:
- Linting (`golangci-lint`).
- Testing (`go test`).
- Docker Build & Push.

### 4.2 Infrastructure-as-Code

- Create Terraform/OpenTofu scripts for:
- Managed Postgres (RDS/CloudSQL).
- Managed Redis (ElastiCache/Memorystore).
- Control Plane VM/Container.

## 5. Execution Sequence

1.  **Security Patching**: Fix Auth & Injection risks.
2.  **Billing Fix**: Ensure streaming requests are charged.
3.  **Core Refactor**: Fix Scheduler & Webhooks.
4.  **Frontend**: Build the Dashboard.
5.  **Testing**: Write integration tests.