# Scheduler and Node Registry

The scheduler chooses nodes based on region/model availability with simple round-robin for the MVP.

## Strategies (from PRD)
- RoundRobin (implemented)
- LeastConnections
- WeightedResponse (latency)
- CostOptimized (spot price)
- Predictive (future enhancement)

## Node Registry
Nodes are stored in the database with provider, region, model, endpoint, and heartbeat metadata. Health status gates selection.

## Operational Flow
1. Router requests node candidates by `region` and `model`.
2. Scheduler filters healthy nodes and chooses the next target.
3. Monitor heartbeats update `LastHeartbeat`, and stale nodes can be excluded.
4. Allocator checks reserved capacity alignment before heavy workloads.

## Removing Mesh Networking
Nodes are assumed to expose HTTPS endpoints directly or via Cloudflare Tunnels. No VPN/mesh is needed, which keeps latency low and simplifies debugging.

