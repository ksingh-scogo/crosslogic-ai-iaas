# Scheduler

The Scheduler is the core orchestration component of CIC. It is responsible for finding the best available GPU node for a given inference request.

## Responsibilities

-   **Node Discovery**: Maintains a registry of active GPU nodes (Node Agents).
-   **Load Balancing**: Distributes requests across available nodes to maximize utilization and minimize latency.
-   **Fault Tolerance**: Handles node failures and retries requests.
-   **Model Routing**: Ensures requests are sent to nodes hosting the requested model.

## Architecture

The Scheduler maintains an in-memory state of the cluster, synchronized with the database and updated by heartbeats from Node Agents.

### Node Selection Algorithm

1.  **Filter**: Select nodes that:
    -   Are `active`.
    -   Have the requested `model` loaded.
    -   Are in the requested `region` (if specified).
    -   Have sufficient capacity (based on max concurrent requests).
2.  **Score**: Rank nodes based on:
    -   Current load (fewer requests is better).
    -   Network latency (if known).
    -   Health score.
3.  **Select**: Pick the top-ranked node.

### vLLM Proxy

Once a node is selected, the Scheduler acts as a reverse proxy to the vLLM instance running on that node.

-   **Protocol**: Forwards the HTTP request to the Node Agent.
-   **Streaming**: Supports Server-Sent Events (SSE) for streaming responses.
-   **Usage Tracking**: Intercepts the response (or stream end) to calculate token usage for billing.

## Configuration

-   `SCHEDULER_STRATEGY`: `least_loaded` | `random` | `round_robin` (default: `least_loaded`).
-   `NODE_TIMEOUT`: Time after which a node is considered offline if no heartbeat is received.
