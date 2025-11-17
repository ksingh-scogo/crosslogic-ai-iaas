# CrossLogic.AI Inference Platform (MVP Scaffold)

This repository implements the control-plane skeleton and documentation described in the PRD files. It focuses on a simple, single-binary Go implementation, no mesh networking, and direct HTTPS access to GPU workers.

## Quickstart
1. **Install Go 1.22+** on your workstation.
2. **Run the control plane locally**:
   ```bash
   cd control-plane
   go test ./...
   go run ./cmd/server
   ```
3. **Issue a sample request** from another terminal:
   ```bash
   curl -X POST "http://localhost:8080/v1/chat/completions?model=llama-7b&region=ap-south-1&prompt=hello" \
     -H "Authorization: Bearer sk_dev_demo"
   ```
   You should see a JSON response indicating which node the router selected.
4. **Review module docs** in `docs/` for deeper guidance on extending each subsystem.

## Deployment (step-by-step)
1. **Prepare infrastructure**
   - Create PostgreSQL (e.g., Supabase) and Redis (e.g., Upstash) instances.
   - Provision a public HTTPS ingress per GPU node (Cloudflare Tunnel or cloud-native load balancer per `PRD/mesh-network-not-needed.md`).
2. **Configure environment**
   - Export database and Redis URLs as environment variables (see `docs/control-plane.md`).
   - Seed model/region metadata in the database.
3. **Build and ship**
   ```bash
   cd control-plane
   go build ./cmd/server
   ```
   Package the binary with a systemd unit or container image.
4. **Launch GPU workers**
   - Use SkyPilot or cloud-native scripts to start vLLM/SGLang with public HTTPS endpoints.
   - Register each node’s endpoint and model metadata with the control plane.
5. **Enable billing and observability**
   - Connect Stripe metered billing and push `usage_records` exports on a cadence.
   - Pipe logs to Grafana/Loki or another sink using the telemetry adapter.
6. **Test end-to-end**
   - Verify authentication, rate limiting, routing, and usage recording via the sample curl above.
   - Add synthetic health checks for each node using the monitor module.

## Repository layout
- `PRD/` — source requirements.
- `control-plane/` — Go-based control-plane scaffold with gateway, router, scheduler, allocator, billing, monitor, and orchestrator modules.
- `docs/` — modular documentation for each component and deployment workflow.

