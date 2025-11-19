# Gemini 3.0 Pro Improvement Plan: CrossLogic Inference Cloud (CIC)

**Date**: November 19, 2025
**Reviewer**: Google Senior Staff Engineer (AI/Infrastructure)
**Status**: Proposed (v1)

## 1. Executive Summary

The CrossLogic Inference Cloud (CIC) backend is in a **strong MVP state**. The core control plane logic—including the API Gateway, Scheduler, Rate Limiting, and the newly added vLLM Proxy and SkyPilot Orchestrator—is well-structured and follows Go best practices.

However, a review of the codebase reveals specific gaps that prevent immediate production deployment:
1.  **Dependency Management**: The `control-plane` module is missing the `prometheus/client_golang` dependency despite the code using it.
2.  **Testing**: While the architecture is sound, there is a lack of comprehensive unit and integration tests for the new components (vLLM Proxy, Webhooks).
3.  **Dashboard Integration**: The dashboard code exists but lacks a production-grade strategy for mapping authenticated users (NextAuth) to backend Tenants.
4.  **Hybrid Cloud Support**: The current architecture focuses on public cloud (SkyPilot), but the "Enterprise" vision requires on-premise support.

This plan outlines a 5-phase approach to bridge these gaps, prioritizing system reliability and operator visibility before user-facing features.

## 2. Gap Analysis & Technical Review

### 2.1 Critical Gaps (Blocking Production)
| Component | Status | Issue | Impact |
|-----------|--------|-------|--------|
| **Dependencies** | ❌ Broken | `control-plane/go.mod` is missing `prometheus/client_golang`. | Build failure for metrics code. |
| **Testing** | ⚠️ Partial | No unit tests for `vllm_proxy.go` or `webhooks.go`. | High risk of regression. |
| **Dashboard Auth** | ⚠️ Incomplete | `lib/auth.ts` uses hardcoded Tenant IDs in dev mode. No logic to map Google/Email users to DB Tenants. | Users cannot access their specific data in production. |
| **Deployment** | ❌ Missing | No CI/CD pipelines. | Manual, error-prone deployment. |

### 2.2 Codebase Review Findings
*   **Gateway (`gateway.go`)**:
    *   **Good**: Clean middleware chain, proper context propagation.
    *   **Issue**: `metrics.go` is present but dependencies are missing in `go.mod`.
*   **Dashboard (`dashboard/`)**:
    *   **Good**: `lib/api.ts` is well-structured with fallback mocks.
    *   **Issue**: Authentication is decoupled from the backend database. We need a mechanism to resolve `session.user.email` -> `tenant_id`.
*   **Orchestrator (`skypilot.go`)**:
    *   **Good**: Template-based generation is flexible.
    *   **Opportunity**: Add Proxmox support for on-premise nodes using available MCP tools.

## 3. Improvement Plan

### Phase 1: Reliability & Observability (Day 1)
**Goal**: Fix build issues and ensure the system is observable.

1.  **Fix Dependencies**:
    *   ✅ Run `go mod tidy` in `control-plane` to resolve missing Prometheus dependencies.
    *   ✅ Verify `go build ./...` passes.
2.  **Verify Metrics**:
    *   ✅ Ensure `/metrics` endpoint returns expected Prometheus data.
    *   ✅ Add `up` metric for dependent services (Redis, Postgres).
3.  **Linter Pass**:
    *   ⚠️ Run `golangci-lint` and fix high-priority issues (errcheck, staticcheck). (Skipped: tool not installed, manual review done)

### Phase 2: Testing Strategy (Days 2-3)
**Goal**: Establish a safety net for refactoring and deployment.

1.  **Unit Tests**:
    *   ✅ Create `vllm_proxy_test.go`: Mock HTTP client to test streaming/non-streaming flows and error handling.
    *   ✅ Create `webhooks_test.go`: Test Stripe signature verification and event dispatching.
2.  **Integration Tests**:
    *   ✅ Create `tests/integration/api_test.go`: Spin up a test environment (using Docker Compose) and run end-to-end API calls (Register -> Key -> Chat).

### Phase 3: Dashboard Hardening (Days 4-5)
**Goal**: Connect the Dashboard to the Backend securely for multi-tenancy.

1.  **Tenant Resolution**:
    *   ✅ Implement a "Login/Signup" flow that checks if the email exists in the `tenants` table.
    *   ✅ If not, create a new Tenant (or prompt for invite code).
    *   ✅ Update `lib/auth.ts` to fetch the real `tenant_id` from the backend (via a new internal Admin endpoint or direct DB access if safe) and populate the JWT token.
2.  **CORS & Security**:
    *   ✅ Ensure `gateway.go` CORS settings allow requests from the Dashboard domain.
    *   ✅ Verify `X-Admin-Token` handling in `lib/api.ts`.

### Phase 4: Deployment Automation (Day 6)
**Goal**: Enable one-click deployment.

1.  **CI/CD Pipeline**:
    *   ✅ Create `.github/workflows/ci.yaml` for automated testing and linting.
    *   ✅ Create `.github/workflows/build.yaml` to build and push Docker images.
2.  **Kubernetes Review**:
    *   ✅ Review `deploy/k8s/` manifests. Ensure they use environment variables compatible with the Docker images.
    *   ✅ Updated `control-plane-deployment.yaml` to use GHCR image registry.

### Phase 5: Hybrid Cloud Expansion (Deferred)
**Goal**: Enable On-Premise GPU Nodes via Proxmox.

*   **Status**: Deferred by user request.

## 4. Immediate Next Steps

1.  **Approve Plan**: Review and approve this roadmap.
2.  **Execute Phase 1**: I will immediately fix the `go.mod` dependency issue and verify the build.

