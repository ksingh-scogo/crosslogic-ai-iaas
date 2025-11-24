# GPU Instance Launch - Flow Diagrams

## Current Flow (BROKEN)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                 FRONTEND                                     │
│  Launch Page (3-step wizard)                                                │
│  1. Select Model: meta-llama/Llama-2-7b-chat-hf                            │
│  2. Config: azure, eastus, spot=true                                        │
│  3. Instance: Standard_NV36ads_A10_v5                                       │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ POST /admin/instances/launch
                                 │ {
                                 │   model_name: "meta-llama/Llama-2-7b-chat-hf",
                                 │   provider: "azure",
                                 │   region: "eastus",
                                 │   instance_type: "Standard_NV36ads_A10_v5",
                                 │   use_spot: true
                                 │ }
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              API GATEWAY                                     │
│  LaunchModelInstanceHandler:                                                │
│  1. Generate jobID: "launch-a1b2c3d4"                                      │
│  2. Create nodeID: "550e8400-e29b-41d4-a716-446655440000"                  │
│  3. Create in-memory job tracker                                            │
│  4. Launch async goroutine                                                  │
│  5. Return: {job_id: "launch-a1b2c3d4", status: "launching"}               │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ Background goroutine calls:
                                 │ orchestrator.LaunchNode(nodeConfig)
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SKYPILOT ORCHESTRATOR                                │
│  LaunchNode():                                                              │
│  1. Validate config ✅                                                      │
│  2. Generate cluster name: "cic-azure-eastus-a10-spot-550e84"              │
│  3. Generate YAML template with:                                            │
│     - Model: meta-llama/Llama-2-7b-chat-hf                                 │
│     - GPU: A10:1                                                            │
│     - Node ID: 550e8400-e29b-41d4-a716-446655440000                        │
│     - Control Plane URL: http://control-plane:8080                         │
│  4. Write /tmp/sky-task-550e84.yaml                                        │
│  5. Execute:                                                                │
│     $ sky launch -c cic-azure-eastus-a10-spot-550e84 \                     │
│                  /tmp/sky-task-550e84.yaml -y --down --detach-run          │
│  6. Wait for SkyPilot (3-5 minutes) ⏳                                     │
│  7. SkyPilot returns success ✅                                            │
│  8. registerNode() in database:                                             │
│     INSERT INTO nodes (id, cluster_name, status, endpoint, ...)            │
│     VALUES ('550e84...', 'cic-azure-eastus...', 'initializing', '', ...)   │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ SkyPilot provisions Azure VM
                                 │ (2-3 minutes)
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AZURE SPOT VM (STARTING)                            │
│  VM Name: sky-cic-azure-eastus-a10-spot-550e84                             │
│  Instance: Standard_NV36ads_A10_v5 (1x A10 GPU, 36 vCPU, 440GB RAM)       │
│  Status: Provisioning... ⏳                                                │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ VM boots, SkyPilot runs "setup:" commands
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         AZURE SPOT VM (SETUP PHASE)                         │
│  Running setup commands:                                                    │
│  1. apt install python3.10 ✅                                              │
│  2. pip install vllm[runai]==0.4.2 torch==2.3.0 ✅ (2 min)                │
│  3. wget node-agent binary ✅                                              │
│  4. Setup complete ✅                                                       │
│  Status: Setup complete, starting run phase... ⏳                          │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ SkyPilot runs "run:" commands
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          AZURE SPOT VM (RUN PHASE)                          │
│  Step 1: Start vLLM Server                                                 │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ $ python -m vllm.entrypoints.openai.api_server \                  │    │
│  │   --model s3://crosslogic-models/meta-llama/Llama-2-7b-chat-hf \  │    │
│  │   --load-format runai_streamer \                                  │    │
│  │   --host 0.0.0.0 --port 8000 \                                    │    │
│  │   --gpu-memory-utilization 0.95 \                                 │    │
│  │   --tensor-parallel-size 1                                        │    │
│  │                                                                    │    │
│  │ INFO: Downloading model from R2... (30s) ⏳                       │    │
│  │ INFO: Loading weights with Run:ai Streamer... (23s) ⏳           │    │
│  │ INFO: vLLM engine initialized ✅                                  │    │
│  │ INFO: Serving at http://0.0.0.0:8000 ✅                           │    │
│  └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Step 2: Health check loop (waits up to 10 minutes)                        │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ for i in {1..600}; do                                              │    │
│  │   curl http://localhost:8000/health                                │    │
│  │   if [ $? -eq 0 ]; then                                            │    │
│  │     echo "✓ vLLM is ready"                                         │    │
│  │     break                                                          │    │
│  │   fi                                                               │    │
│  │   sleep 1                                                          │    │
│  │ done                                                               │    │
│  │                                                                    │    │
│  │ ✓ vLLM is ready after 53 seconds ✅                               │    │
│  └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Step 3: Start Node Agent                                                  │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ $ /usr/local/bin/node-agent                                        │    │
│  │                                                                    │    │
│  │ ENV:                                                               │    │
│  │   CONTROL_PLANE_URL=http://control-plane:8080                     │    │
│  │   NODE_ID=550e8400-e29b-41d4-a716-446655440000                    │    │
│  │   MODEL_NAME=meta-llama/Llama-2-7b-chat-hf                        │    │
│  │   VLLM_ENDPOINT=http://localhost:8000                             │    │
│  │                                                                    │    │
│  │ INFO: Starting node agent...                                      │    │
│  └───────────────────────────────────────────────────────────────────┘    │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ Node Agent tries to register
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            NODE AGENT (RUNNING)                             │
│  agent.Start():                                                             │
│  1. Call register() function                                                │
│  2. Build registration payload:                                             │
│     {                                                                        │
│       "provider": "azure",                                                  │
│       "region": "eastus",                                                   │
│       "model_name": "meta-llama/Llama-2-7b-chat-hf",                       │
│       "endpoint_url": "http://10.128.0.4:8000",  // VM private IP         │
│       "gpu_type": "A10",                                                    │
│       "instance_type": "Standard_NV36ads_A10_v5",                          │
│       "spot_instance": true                                                 │
│     }                                                                        │
│  3. POST http://control-plane:8080/admin/nodes/register ❌                 │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ HTTP POST
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        API GATEWAY (RECEIVING REQUEST)                      │
│  Incoming: POST /admin/nodes/register                                      │
│                                                                             │
│  Registered routes:                                                         │
│  ✅ GET    /admin/nodes                                                     │
│  ✅ POST   /admin/nodes/launch                                              │
│  ✅ POST   /admin/nodes/{cluster_name}/terminate                            │
│  ✅ GET    /admin/nodes/{cluster_name}/status                               │
│  ✅ POST   /admin/nodes/{node_id}/heartbeat                                 │
│  ✅ POST   /admin/nodes/{node_id}/termination-warning                       │
│  ❌ POST   /admin/nodes/register  <-- ROUTE DOES NOT EXIST!                │
│                                                                             │
│  Response: 404 Not Found                                                   │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ 404 Not Found
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         NODE AGENT (ERROR STATE)                            │
│  ERROR: registration failed with status 404                                │
│                                                                             │
│  Node continues running but:                                                │
│  - Cannot send heartbeats (needs registered node_id)                       │
│  - vLLM server is healthy and running                                      │
│  - But control plane doesn't know about it                                 │
│  - Load balancer has 0 nodes to route to                                   │
│                                                                             │
│  VM is running and costing money but is UNREACHABLE ❌                     │
└─────────────────────────────────────────────────────────────────────────────┘

Meanwhile...

┌─────────────────────────────────────────────────────────────────────────────┐
│                          DATABASE STATE                                     │
│  nodes table:                                                               │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │ id          | status        | endpoint_url | last_heartbeat        │   │
│  ├────────────────────────────────────────────────────────────────────┤   │
│  │ 550e8400... | initializing  | (empty)      | NULL                  │   │
│  │             |               |              |                       │   │
│  │ ❌ Node stuck in "initializing" forever                            │   │
│  │ ❌ No endpoint URL (should be http://10.128.0.4:8000)              │   │
│  │ ❌ No heartbeats being received                                    │   │
│  └────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘

Meanwhile...

┌─────────────────────────────────────────────────────────────────────────────┐
│                             FRONTEND (POLLING)                              │
│  Every 3 seconds:                                                           │
│  GET /admin/instances/status?job_id=launch-a1b2c3d4                        │
│                                                                             │
│  Response from in-memory job tracker:                                       │
│  {                                                                          │
│    "job_id": "launch-a1b2c3d4",                                            │
│    "status": "completed",    ✅ Says "completed"!                          │
│    "stage": "ready",                                                        │
│    "progress": 100,                                                         │
│    "stages": [                                                              │
│      "✓ Validated configuration",                                          │
│      "✓ Provisioned cloud resources",                                      │
│      "✓ Installed dependencies",                                           │
│      "✓ Loaded model from R2",                                             │
│      "✓ Started vLLM",                                                     │
│      "✓ Node registered: cic-azure-eastus-a10-spot-550e84"                │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  ⚠️  Frontend thinks launch succeeded!                                     │
│  ⚠️  But node never actually registered!                                   │
└─────────────────────────────────────────────────────────────────────────────┘

Meanwhile...

┌─────────────────────────────────────────────────────────────────────────────┐
│                         LOAD BALANCER (SELECTING NODE)                      │
│  User sends inference request:                                              │
│  POST /v1/chat/completions                                                 │
│  { "model": "meta-llama/Llama-2-7b-chat-hf", ... }                         │
│                                                                             │
│  SelectEndpoint(model="meta-llama/Llama-2-7b-chat-hf"):                    │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ SELECT endpoint_url FROM nodes                                     │    │
│  │ WHERE model_name = 'meta-llama/Llama-2-7b-chat-hf'                │    │
│  │   AND status = 'ready'                                             │    │
│  │                                                                    │    │
│  │ Result: (empty) - NO ROWS RETURNED ❌                             │    │
│  └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Response to user:                                                          │
│  HTTP 503 Service Unavailable                                              │
│  {"error": {"message": "no healthy nodes for model"}}                      │
└─────────────────────────────────────────────────────────────────────────────┘

FINAL STATE:
┌─────────────────────────────────────────────────────────────────────────────┐
│  ✅ Azure VM: Running, costing $1.50/hr                                     │
│  ✅ vLLM Server: Healthy, serving at http://10.128.0.4:8000                │
│  ✅ Node Agent: Running but stuck (can't register)                         │
│  ❌ Control Plane: Thinks node is "initializing"                           │
│  ❌ Load Balancer: Sees 0 healthy nodes                                    │
│  ❌ User Requests: All fail with 503 errors                                │
│  ❌ Frontend UI: Shows "Instance launched successfully"                    │
│                                                                             │
│  RESULT: WASTED MONEY, NO SERVICE ❌                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Fixed Flow (WITH REGISTRATION ENDPOINT)

```
... (same as above until Node Agent starts) ...

┌─────────────────────────────────────────────────────────────────────────────┐
│                            NODE AGENT (RUNNING)                             │
│  agent.Start():                                                             │
│  1. POST http://control-plane:8080/admin/nodes/register?node_id=550e84...  │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ HTTP POST
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        API GATEWAY (RECEIVING REQUEST)                      │
│  Incoming: POST /admin/nodes/register?node_id=550e84...                    │
│                                                                             │
│  ✅ Route EXISTS: r.Post("/admin/nodes/register", g.handleNodeRegister)    │
│                                                                             │
│  handleNodeRegister():                                                      │
│  1. Parse request body                                                      │
│  2. Extract node_id from query params: "550e8400-e29b-41d4..."             │
│  3. UPDATE nodes SET                                                        │
│       endpoint_url = 'http://10.128.0.4:8000',                             │
│       status = 'ready',                                                     │
│       updated_at = NOW()                                                    │
│     WHERE id = '550e8400-e29b-41d4...'  ✅                                 │
│  4. Return: {"status": "registered", "node_id": "550e84..."}               │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 │ 200 OK
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         NODE AGENT (REGISTERED!)                            │
│  ✅ Registration successful!                                                │
│  ✅ Received node_id from control plane                                     │
│  ✅ Starting heartbeat loop (every 10 seconds)                              │
│  ✅ Starting health monitor (every 30 seconds)                              │
│  ✅ Starting spot termination monitor (every 5 seconds)                     │
└─────────────────────────────────────────────────────────────────────────────┘

After 10 seconds...

┌─────────────────────────────────────────────────────────────────────────────┐
│                       NODE AGENT (SENDING HEARTBEAT)                        │
│  POST /admin/nodes/550e8400.../heartbeat                                   │
│  {                                                                          │
│    "node_id": "550e8400-e29b-41d4-a716-446655440000",                      │
│    "health_score": 100.0,                                                  │
│    "timestamp": 1732407123                                                  │
│  }                                                                          │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        TRIPLE-LAYER HEALTH MONITOR                          │
│  RecordHeartbeat():                                                         │
│  ✅ UPDATE nodes SET                                                        │
│       last_heartbeat = NOW(),                                              │
│       health_score = 100.0,                                                 │
│       status = 'active'                                                     │
│     WHERE id = '550e8400...'                                                │
│                                                                             │
│  ✅ Store heartbeat signal in memory                                        │
│  ✅ Evaluate overall node health (3-layer truth table)                      │
│  ✅ Result: NodeHealthy ✅                                                  │
└─────────────────────────────────────────────────────────────────────────────┘

Now...

┌─────────────────────────────────────────────────────────────────────────────┐
│                          DATABASE STATE (FIXED!)                            │
│  nodes table:                                                               │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │ id          | status  | endpoint_url              | last_heartbeat │   │
│  ├────────────────────────────────────────────────────────────────────┤   │
│  │ 550e8400... | active  | http://10.128.0.4:8000   | 2 seconds ago  │   │
│  │             |         |                          |                │   │
│  │ ✅ Status: active                                                  │   │
│  │ ✅ Endpoint: http://10.128.0.4:8000                                │   │
│  │ ✅ Heartbeats: receiving every 10s                                 │   │
│  └────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘

User sends request...

┌─────────────────────────────────────────────────────────────────────────────┐
│                         LOAD BALANCER (SELECTING NODE)                      │
│  User sends inference request:                                              │
│  POST /v1/chat/completions                                                 │
│  { "model": "meta-llama/Llama-2-7b-chat-hf", ... }                         │
│                                                                             │
│  SelectEndpoint(model="meta-llama/Llama-2-7b-chat-hf"):                    │
│  ┌───────────────────────────────────────────────────────────────────┐    │
│  │ SELECT endpoint_url FROM nodes                                     │    │
│  │ WHERE model_name = 'meta-llama/Llama-2-7b-chat-hf'                │    │
│  │   AND status IN ('active', 'ready')                                │    │
│  │                                                                    │    │
│  │ Result: http://10.128.0.4:8000 ✅                                 │    │
│  └───────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ✅ Forward request to http://10.128.0.4:8000/v1/chat/completions          │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          vLLM SERVER (PROCESSING)                           │
│  Received: POST /v1/chat/completions                                       │
│  ✅ Generate response with Llama-2-7b-chat-hf                              │
│  ✅ Return: {"choices": [{"message": {...}}]}                              │
└────────────────────────────────┬────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               USER                                          │
│  ✅ Received response in 2.3 seconds                                        │
│  ✅ Inference successful!                                                   │
└─────────────────────────────────────────────────────────────────────────────┘

FINAL STATE (FIXED):
┌─────────────────────────────────────────────────────────────────────────────┐
│  ✅ Azure VM: Running, costing $1.50/hr                                     │
│  ✅ vLLM Server: Healthy, serving requests                                  │
│  ✅ Node Agent: Registered, sending heartbeats                              │
│  ✅ Control Plane: Node status = "active"                                   │
│  ✅ Load Balancer: Sees 1 healthy node                                      │
│  ✅ User Requests: Working perfectly!                                       │
│  ✅ Frontend UI: Accurate status                                            │
│                                                                             │
│  RESULT: SYSTEM WORKING AS DESIGNED ✅                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Takeaways

### The Problem in One Sentence:
**"Nodes launch successfully but can't register because the registration endpoint doesn't exist."**

### Why It's Critical:
- 100% of launches fail to become usable
- Wasted cloud spend on unreachable VMs
- User sees "success" but system is broken
- No error messages to debug

### The Fix in One Sentence:
**"Add the missing registration endpoint handler in gateway.go"**

### Impact of Fix:
- 0% success rate → 100% success rate
- Nodes become "active" and receive traffic
- Health monitoring works correctly
- System fully functional

---

## Timeline of a Successful Launch (After Fix)

```
T+0s:     User clicks "Launch Instance"
T+1s:     API Gateway receives request, starts async launch
T+2s:     SkyPilot begins provisioning Azure VM
T+120s:   Azure VM boots, starts setup phase
T+180s:   vLLM installs, downloads model from R2
T+240s:   vLLM starts loading model (Run:ai Streamer)
T+263s:   vLLM health check passes
T+264s:   Node agent starts
T+265s:   Node agent registers successfully ✅
T+275s:   First heartbeat received ✅
T+276s:   Node status → "active" ✅
T+277s:   Load balancer includes node ✅
T+278s:   User request succeeds ✅
```

**Total time: 4 minutes 38 seconds from click to serving requests**
