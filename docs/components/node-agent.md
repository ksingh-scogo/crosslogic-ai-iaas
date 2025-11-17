# Node Agent Documentation

## Overview

The Node Agent is a lightweight Go binary that runs on each GPU worker node. It handles registration, health monitoring, heartbeats, and communication with the Control Plane.

## Responsibilities

1. **Node Registration**: Registers the node with the Control Plane on startup
2. **Heartbeats**: Sends periodic heartbeats to indicate liveness
3. **Health Monitoring**: Monitors vLLM/SGLang health and reports status
4. **Spot Interruption Handling**: Detects and reports spot instance termination warnings
5. **Graceful Shutdown**: Drains requests before termination

## Architecture

```
node-agent/
├── cmd/
│   └── main.go           # Entry point
└── internal/
    ├── agent/
    │   └── agent.go      # Core agent logic
    └── metrics/
        └── collector.go  # Metrics collection
```

## Configuration

Configured via environment variables:

```bash
CONTROL_PLANE_URL=https://control.crosslogic.ai
NODE_ID=auto-generated
PROVIDER=aws|gcp|azure|on-prem
REGION=us-east-1
MODEL_NAME=llama-3-8b
VLLM_ENDPOINT=http://localhost:8000
GPU_TYPE=A10G
INSTANCE_TYPE=g5.xlarge
SPOT_INSTANCE=true
```

## Deployment

### On Cloud Instances (SkyPilot)

The node agent is automatically deployed by SkyPilot:

```yaml
# skypilot-task.yaml
resources:
  accelerators: A10G:1
  cloud: aws
  region: us-east-1

setup: |
  # Install vLLM
  pip install vllm

  # Download node agent
  wget https://releases.crosslogic.ai/node-agent-linux-amd64
  chmod +x node-agent-linux-amd64

run: |
  # Start vLLM in background
  python -m vllm.entrypoints.openai.api_server \
    --model meta-llama/Llama-3-8B \
    --port 8000 &

  # Start node agent
  export CONTROL_PLANE_URL=https://control.crosslogic.ai
  export MODEL_NAME=llama-3-8b
  export PROVIDER=aws
  ./node-agent-linux-amd64
```

### On-Premise Deployment

For on-premise nodes:

```bash
# Download agent
wget https://releases.crosslogic.ai/node-agent-linux-amd64
chmod +x node-agent-linux-amd64

# Configure
cat > .env <<EOF
CONTROL_PLANE_URL=https://control.crosslogic.ai
PROVIDER=on-prem
REGION=on-prem-dc1
MODEL_NAME=llama-3-70b
VLLM_ENDPOINT=http://localhost:8000
GPU_TYPE=H100
INSTANCE_TYPE=on-prem
SPOT_INSTANCE=false
EOF

# Run
./node-agent-linux-amd64
```

## Communication Protocol

### Registration

On startup, the agent sends a registration request:

```http
POST /admin/nodes/register
Content-Type: application/json

{
  "provider": "aws",
  "region": "us-east-1",
  "model_name": "llama-3-8b",
  "endpoint_url": "https://34.123.45.67:8000",
  "gpu_type": "A10G",
  "instance_type": "g5.xlarge",
  "spot_instance": true,
  "status": "active"
}
```

Response:

```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "registered"
}
```

### Heartbeats

Every 10 seconds, the agent sends a heartbeat:

```http
POST /admin/nodes/{node_id}/heartbeat
Content-Type: application/json

{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "health_score": 98.5,
  "timestamp": 1704067200
}
```

### Deregistration

On shutdown, the agent deregisters:

```http
POST /admin/nodes/{node_id}/deregister
```

## Health Scoring

The agent calculates a health score (0-100) based on:

1. **vLLM Availability** (50 points)
   - vLLM `/health` endpoint responding

2. **GPU Temperature** (20 points)
   - <70°C: 20 points
   - 70-80°C: 15 points
   - 80-90°C: 10 points
   - >90°C: 0 points

3. **VRAM Usage** (15 points)
   - <80%: 15 points
   - 80-90%: 10 points
   - >90%: 5 points

4. **CPU Usage** (10 points)
   - <70%: 10 points
   - 70-90%: 5 points
   - >90%: 0 points

5. **Network Latency** (5 points)
   - <10ms to Control Plane: 5 points
   - 10-50ms: 3 points
   - >50ms: 0 points

## Spot Interruption Handling

For spot instances, the agent monitors for termination warnings:

### AWS
Checks EC2 metadata endpoint every 5 seconds:
```bash
curl http://169.254.169.254/latest/meta-data/spot/instance-action
```

### GCP
Monitors for preemption notice:
```bash
curl http://metadata.google.internal/computeMetadata/v1/instance/preempted \
  -H "Metadata-Flavor: Google"
```

### Azure
Checks for scheduled events:
```bash
curl -H Metadata:true \
  http://169.254.169.254/metadata/scheduledevents?api-version=2020-07-01
```

When interruption detected:
1. Agent marks node as "draining"
2. Stops accepting new requests
3. Waits for in-flight requests to complete (max 2 minutes)
4. Deregisters from Control Plane
5. Shuts down gracefully

## Metrics

The agent exposes Prometheus metrics at `:9091/metrics`:

- `node_agent_heartbeat_total` - Total heartbeats sent
- `node_agent_heartbeat_failures_total` - Failed heartbeats
- `node_agent_health_score` - Current health score
- `node_agent_vllm_healthy` - vLLM health status (0 or 1)
- `node_agent_uptime_seconds` - Agent uptime

## Logging

Logs are written to stdout in JSON format:

```json
{
  "level": "info",
  "time": "2025-01-17T10:30:00Z",
  "msg": "heartbeat sent",
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "health_score": 98.5
}
```

## Troubleshooting

### Agent Cannot Register

**Symptom**: Agent starts but fails to register with Control Plane

**Possible Causes**:
1. Control Plane URL incorrect
2. Network connectivity issue
3. vLLM not running

**Solutions**:
```bash
# Test Control Plane connectivity
curl -v $CONTROL_PLANE_URL/health

# Check vLLM
curl http://localhost:8000/health

# Review agent logs
./node-agent 2>&1 | tee agent.log
```

### Heartbeat Failures

**Symptom**: Heartbeats failing intermittently

**Possible Causes**:
1. Network issues
2. Control Plane overloaded
3. Agent misconfigured

**Solutions**:
```bash
# Check network latency
ping control.crosslogic.ai

# Review heartbeat logs
grep "heartbeat" agent.log

# Increase heartbeat interval
export HEARTBEAT_INTERVAL=30s
```

### Low Health Score

**Symptom**: Health score consistently below 80

**Solutions**:
1. Check GPU temperature
2. Monitor VRAM usage
3. Review vLLM logs
4. Check system load

## Security

### TLS Communication
The agent supports mTLS for secure communication:

```bash
export TLS_CERT_PATH=/etc/certs/node.crt
export TLS_KEY_PATH=/etc/certs/node.key
export TLS_CA_PATH=/etc/certs/ca.crt
```

### API Key Authentication
For on-premise deployments, use API key auth:

```bash
export AGENT_API_KEY=clsk_agent_...
```

## Best Practices

1. **Always run agent as systemd service** for auto-restart
2. **Monitor agent logs** for issues
3. **Set up alerts** for heartbeat failures
4. **Test spot interruption handling** before production
5. **Use TLS** in production environments

## See Also

- [Control Plane Documentation](./control-plane.md)
- [Deployment Guide](../deployment/deployment-guide.md)
- [SkyPilot Integration](./skypilot.md)
