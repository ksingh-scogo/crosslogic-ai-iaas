# Monitoring & Observability

CrossLogic Inference Cloud includes a comprehensive monitoring stack based on Prometheus and Grafana.

## Components

### Prometheus

Prometheus scrapes metrics from the Control Plane and Node Agents.

-   **Port**: `9090`
-   **Config**: `config/prometheus.yml`
-   **Scrape Targets**:
    -   `control-plane:8080/metrics`
    -   `node-agent:8080/metrics`

### Grafana

Grafana provides visualization dashboards.

-   **Port**: `3000`
-   **Default Login**: `admin` / `admin` (or set via `GRAFANA_PASSWORD`)
-   **Data Source**: Pre-configured to use the local Prometheus instance.

## Key Metrics

### Control Plane

-   `http_requests_total`: Total API requests (labeled by status, method, path).
-   `http_request_duration_seconds`: Latency distribution.
-   `active_connections`: Current open connections.

### Scheduler

-   `scheduler_nodes_available`: Count of active GPU nodes.
-   `scheduler_queue_depth`: Pending requests waiting for a node.
-   `scheduler_scheduling_duration_seconds`: Time taken to select a node.

### Node Agent (GPU)

-   `gpu_utilization`: % of GPU compute used.
-   `gpu_memory_used_bytes`: VRAM usage.
-   `vllm_request_queue_size`: Requests queued inside vLLM.

## Setup

The monitoring stack is included in the default `docker-compose.yml`.

```bash
docker-compose up -d prometheus grafana
```

Access Grafana at `http://localhost:3000`.
