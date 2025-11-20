# Next Plan of Action: Enterprise Architecture Strategy

This document outlines the strategy to achieve a **99.99% Uptime SLA** and a **fool-proof production-ready architecture** for CrossLogic AI IaaS.

## 1. Cluster vs. Node Strategy
**Recommendation: 1 Cluster = 1 Node**

*   **Why?** Fault isolation. If you launch a multi-node cluster (e.g., 5 nodes in one SkyPilot cluster), SkyPilot treats them as a single job unit. If the head node fails or network partitions occur, the entire cluster might become unmanageable or the job might fail.
*   **Spot Instances**: For spot instances, this is even more critical. You want independent lifecycle management. If one spot node is preempted, it shouldn't affect others.
*   **Implementation**: Continue using `sky launch` for individual nodes. Manage the "fleet" logic in your Control Plane, not in SkyPilot.

## 2. Naming Convention
**Action**: Update `control-plane/internal/orchestrator/skypilot.go` to use the new naming format.

*   **Format**: `cic-{provider}-{region}-{nodeID}`
*   **Example**: `cic-aws-ap-south1-abc123`
*   **Benefit**: Easier debugging and manual tracking of resources in cloud consoles.

## 3. Multi-GPU Support (Billions of Parameters)
**Current Status**: Partially supported but needs configuration updates.

*   **The Challenge**: Running large models (e.g., DeepSeek 67B, Llama-3-70B) requires multiple GPUs (Tensor Parallelism).
*   **Solution**:
    1.  **Update `NodeConfig`**: Add `GPUCount` field.
    2.  **Update Template**:
        ```yaml
        resources:
          accelerators: {{.GPU}}:{{.GPUCount}}  # e.g., H100:8
        ```
    3.  **Pass vLLM Args**: SkyPilot automatically exposes `$SKYPILOT_NUM_GPUS_PER_NODE`. We should configure vLLM to use this:
        ```bash
        python -m vllm.entrypoints.openai.api_server \
          --tensor-parallel-size $SKYPILOT_NUM_GPUS_PER_NODE \
          ...
        ```

## 4. Spot Price & Availability Checks
**Strategy**: Leverage SkyPilot's Optimizer + Catalog.

1.  **Automated Optimization (Recommended)**:
    *   Instead of pinning `provider` and `region` in `NodeConfig`, allow them to be empty.
    *   SkyPilot's `sky launch` optimizer will automatically scan the catalog and pick the cheapest available cloud/region for the requested GPU.
    *   **Implementation**: Update `NodeConfig` to make Provider/Region optional. If empty, let SkyPilot decide.
2.  **Pre-Launch Price Check (UI Feature)**:
    *   SkyPilot maintains a catalog in `~/.sky/catalogs/vms.csv` (or fetchable from GitHub).
    *   **Action**: Create a utility in Control Plane that parses this CSV to show "Estimated Price" to the user before they click launch.

## 5. SkyPilot State Management (Production Grade)
**The Problem**: `~/.sky/` is a single point of failure.

**Strategy**:
1.  **External Database**: Configure SkyPilot to use the main PostgreSQL database.
    *   **Config**: Set `~/.sky/config.yaml` to point to Postgres.
2.  **Reconciliation Loop**:
    *   Implement a background worker in Control Plane that runs `sky status --refresh` every 5 minutes.
    *   **Orphan Detection**: If in SkyPilot but not in DB -> `sky down`.
    *   **Ghost Detection**: If in DB but not in SkyPilot -> Mark as `lost` or attempt recovery.

## 6. Control Plane Resilience & Active Polling (Double Safety)
**Strategy**:
*   **Stateless Design**: Control Plane state lives in Postgres/Redis.
*   **Active Polling**:
    *   **Node Agent -> Control Plane**: Heatbeats every 10s (Existing).
    *   **Control Plane -> Node Agent**: The Control Plane should *also* actively poll `GET http://{node_ip}:8000/health` every 30s.
    *   **Failure Detection**:
        *   If Heartbeat missing > 30s OR Active Poll fails:
        *   **Action**: Mark node `unhealthy`.
        *   **Remediation**: If `unhealthy` for > 2 minutes, trigger `TerminateNode` and `LaunchNode` (replacement).

## 7. Replica Management & Load Balancing
**Strategy**: Introduce a "Deployment" abstraction.

*   **Concept**: Users don't just launch "Nodes", they launch "Deployments" (or "Services").
*   **Database Schema**:
    *   `deployments` table: `id`, `model`, `replica_count`, `strategy` (e.g., spread).
    *   `nodes` table: Add `deployment_id` FK.
*   **Controller Logic**:
    *   **Scale Up**: If `count(active_nodes) < replica_count`, launch new nodes.
    *   **Scale Down**: If `count(active_nodes) > replica_count`, terminate nodes.
*   **Load Balancing**:
    *   The API Gateway (or a dedicated proxy like Nginx/Envoy managed by Control Plane) maintains a list of healthy IPs for each Deployment.
    *   Incoming request for "Llama-7B" -> Round Robin to available replicas.

## 8. Spot Instance Lifecycle (Automated)
**Required Implementation**:
1.  **Node Agent Poller**:
    *   Poll cloud metadata (AWS/GCP) every 5s for termination warnings.
    *   On warning: Send `POST /api/nodes/{id}/termination-warning`.
2.  **Scheduler Logic**:
    *   Receive warning -> Mark `draining`.
    *   Stop routing new requests.
    *   **Immediate Replacement**: Launch a new node *immediately* (don't wait for the old one to die) to maintain replica count.

## 9. Instant Model Loading (JuiceFS)
**Strategy**: Use **JuiceFS** as a high-performance distributed file system layer on top of S3.

*   **Why JuiceFS?**
    *   **10x Faster**: Significantly faster than S3FS or standard object storage mounting.
    *   **Caching**: Intelligently caches frequently accessed blocks to local NVMe/SSD.
    *   **POSIX**: Fully POSIX compatible, so vLLM sees it as a normal local folder.
*   **Implementation**:
    1.  **Model Registry**: Store all model weights (safetensors) in a central S3 bucket (e.g., `s3://crosslogic-models`).
    2.  **SkyPilot Setup**: In the `setup` phase of the task YAML, install JuiceFS and mount the bucket to `/mnt/models`.
        ```bash
        # Example Setup
        curl -sSL https://d.juicefs.com/install | sh -
        juicefs format --storage s3 ... myjfs
        juicefs mount myjfs /mnt/models
        ```
    3.  **vLLM Launch**: Point vLLM to the mount path.
        ```bash
        python -m vllm.entrypoints.openai.api_server --model /mnt/models/llama-3-70b ...
        ```
*   **Result**:
    *   **Zero Download Step**: The "downloading model..." phase is eliminated.
    *   **Streaming Start**: vLLM starts reading weights immediately.
    *   **Edge Caching**: If the node is reused or if JuiceFS cache is persisted, subsequent starts are instant.

## Summary of Next Steps (Roadmap)

1.  **Refactor**: Update `skypilot.go` for Naming & Multi-GPU.
2.  **Hardening**: Configure SkyPilot to use Postgres.
3.  **Feature**: Implement Spot Termination Monitoring in `node-agent`.
4.  **Feature**: Implement "Deployment" logic for Replicas.
5.  **Feature**: Integrate JuiceFS for model mounting.
6.  **Docs**: Write the Provisioning Lifecycle document.
