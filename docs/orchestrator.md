# SkyPilot Orchestrator

Provides hooks to launch and replace spot instances using SkyPilot or cloud-native scripts.

## Lifecycle
- **Launch**: `LaunchSpotInstance(region, model)` emits the command to start a node with the requested model.
- **Interruption**: `HandleInterruption(nodeID)` mirrors the PRD emergency playbook—drain, wait, replace.

## Networking Guidance
Per `PRD/mesh-network-not-needed.md`, nodes should be reachable over HTTPS directly or via Cloudflare Tunnels. Avoid Tailscale/WireGuard to keep latency and complexity down.

## Next Steps
- Integrate with actual `skypilot` CLI and parse YAML templates for provider-specific settings.
- Emit webhook notifications when nodes launch/terminate for dashboard updates.

