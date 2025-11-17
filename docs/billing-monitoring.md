# Billing and Monitoring

## Metering
- `internal/billing.Meter` records `usage_records` with input/output tokens, cached tokens, latency, and region.
- Export usage periodically to Stripe’s usage-based pricing API or store in PostgreSQL for invoicing.

## Monitoring
- `internal/monitor.Monitor` updates node heartbeats and surfaces stale nodes.
- Add alerting for nodes missing heartbeats beyond a threshold and trigger `orchestrator.HandleInterruption`.

## Dashboards
- Start with simple logs/metrics via the `telemetry.Logger`.
- Hook into Grafana Cloud or another SaaS by replacing the logger with a structured emitter.

## Testing Checklist
1. Call the gateway endpoint and confirm a usage record is created for the tenant.
2. Simulate node heartbeat loss and verify stale nodes are reported.
3. Ensure rate limits are enforced per API key; adjust token bucket sizes for production tiers.

