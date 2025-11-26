# Real-Time Node Log Streaming API

## Overview

The Node Log Streaming API provides real-time visibility into GPU node launch processes using Server-Sent Events (SSE). This enables administrators to monitor node provisioning, installation, model loading, and health checks in real-time.

## Endpoints

### 1. Stream Node Logs (SSE)

**Endpoint:** `GET /admin/nodes/{id}/logs/stream`

**Authentication:** Admin token required via `X-Admin-Token` header

**Description:** Streams node launch logs in real-time using Server-Sent Events (SSE). The connection remains open and sends log entries as they occur.

#### Query Parameters

| Parameter | Type    | Default | Description                                           |
|-----------|---------|---------|-------------------------------------------------------|
| `follow`  | boolean | `true`  | Keep connection open and stream new logs              |
| `tail`    | integer | `100`   | Number of recent lines to send initially              |
| `since`   | string  | -       | Only return logs after this timestamp (RFC3339 format) |

#### Example Request

```bash
curl -N -H "X-Admin-Token: your-admin-token" \
  "https://api.crosslogic.ai/admin/nodes/550e8400-e29b-41d4-a716-446655440000/logs/stream?follow=true&tail=50"
```

#### Response Format (SSE)

The endpoint returns Server-Sent Events in the following formats:

##### Log Event
```
event: log
data: {"timestamp":"2024-01-15T10:30:00Z","level":"info","message":"Launching cluster...","phase":"provisioning","progress":10}
```

##### Status Event
```
event: status
data: {"phase":"installing","progress":45,"message":"Installing vLLM..."}
```

##### Error Event
```
event: error
data: {"error":"Failed to provision","details":"Instance type not available in region","phase":"provisioning"}
```

##### Done Event
```
event: done
data: {"status":"active","endpoint":"http://10.0.0.1:8000","message":"Node is ready and serving requests"}
```

### 2. Get Node Logs (JSON)

**Endpoint:** `GET /admin/nodes/{id}/logs`

**Authentication:** Admin token required via `X-Admin-Token` header

**Description:** Retrieves historical node logs as JSON (non-streaming).

#### Query Parameters

| Parameter | Type    | Default | Description                                           |
|-----------|---------|---------|-------------------------------------------------------|
| `tail`    | integer | `100`   | Number of recent lines to return                      |
| `since`   | string  | -       | Only return logs after this timestamp (RFC3339 format) |

#### Example Request

```bash
curl -H "X-Admin-Token: your-admin-token" \
  "https://api.crosslogic.ai/admin/nodes/550e8400-e29b-41d4-a716-446655440000/logs?tail=100"
```

#### Example Response

```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "count": 15,
  "logs": [
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "level": "info",
      "message": "Node launch request queued: cic-aws-uswest2-a100-spot-550e84",
      "phase": "queued",
      "progress": 0
    },
    {
      "timestamp": "2024-01-15T10:30:05Z",
      "level": "info",
      "message": "Starting cloud resource provisioning...",
      "phase": "provisioning",
      "progress": 10
    },
    {
      "timestamp": "2024-01-15T10:32:30Z",
      "level": "info",
      "message": "Cloud instance is starting up...",
      "phase": "instance_ready",
      "progress": 50
    },
    {
      "timestamp": "2024-01-15T10:33:00Z",
      "level": "info",
      "message": "Installing dependencies and vLLM...",
      "phase": "installing",
      "progress": 60
    },
    {
      "timestamp": "2024-01-15T10:34:30Z",
      "level": "info",
      "message": "Loading model meta-llama/Llama-2-7b-chat-hf...",
      "phase": "model_loading",
      "progress": 70
    },
    {
      "timestamp": "2024-01-15T10:35:45Z",
      "level": "info",
      "message": "Running health checks...",
      "phase": "health_check",
      "progress": 85
    },
    {
      "timestamp": "2024-01-15T10:36:00Z",
      "level": "info",
      "message": "Node is ready and serving requests!",
      "phase": "active",
      "progress": 100
    }
  ]
}
```

## Log Phases

The node launch process goes through the following phases:

| Phase            | Description                                          | Typical Progress |
|------------------|------------------------------------------------------|------------------|
| `queued`         | Request received and validated                       | 0-10%            |
| `provisioning`   | SkyPilot is provisioning cloud resources             | 10-50%           |
| `instance_ready` | Cloud instance is running                            | 50-60%           |
| `installing`     | Installing dependencies and vLLM                     | 60-70%           |
| `model_loading`  | Loading model weights                                | 70-85%           |
| `health_check`   | Running health checks                                | 85-95%           |
| `active`         | Node is ready and serving requests                   | 100%             |
| `failed`         | Launch failed (terminal state)                       | -                |

## Log Levels

| Level   | Description                                          |
|---------|------------------------------------------------------|
| `info`  | Informational message about normal operation         |
| `warn`  | Warning message about potential issues               |
| `error` | Error message indicating a problem occurred          |
| `debug` | Detailed debugging information                       |

## JavaScript/TypeScript Example

### Using EventSource for SSE

```typescript
interface LogEntry {
  timestamp: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  message: string;
  phase: string;
  progress?: number;
  details?: string;
}

interface StatusEvent {
  phase: string;
  progress: number;
  message: string;
}

interface ErrorEvent {
  error: string;
  details: string;
  phase: string;
}

interface DoneEvent {
  status: string;
  endpoint?: string;
  message: string;
}

function streamNodeLogs(nodeId: string, adminToken: string) {
  const url = `https://api.crosslogic.ai/admin/nodes/${nodeId}/logs/stream?follow=true`;

  const eventSource = new EventSource(url, {
    headers: {
      'X-Admin-Token': adminToken
    }
  });

  eventSource.addEventListener('log', (event) => {
    const log: LogEntry = JSON.parse(event.data);
    console.log(`[${log.level.toUpperCase()}] ${log.message}`);

    // Update UI with log entry
    updateLogDisplay(log);
  });

  eventSource.addEventListener('status', (event) => {
    const status: StatusEvent = JSON.parse(event.data);
    console.log(`Status: ${status.phase} - ${status.progress}%`);

    // Update progress bar
    updateProgressBar(status.progress);
  });

  eventSource.addEventListener('error', (event) => {
    const error: ErrorEvent = JSON.parse(event.data);
    console.error(`Error in ${error.phase}: ${error.error}`);

    // Show error notification
    showErrorNotification(error);
  });

  eventSource.addEventListener('done', (event) => {
    const done: DoneEvent = JSON.parse(event.data);
    console.log(`Launch complete: ${done.status}`);

    // Close the connection
    eventSource.close();

    // Show completion notification
    if (done.status === 'active') {
      showSuccessNotification(`Node ready at ${done.endpoint}`);
    } else {
      showErrorNotification('Node launch failed');
    }
  });

  eventSource.onerror = (error) => {
    console.error('EventSource error:', error);
    eventSource.close();
  };

  return eventSource;
}

// Usage
const eventSource = streamNodeLogs('550e8400-e29b-41d4-a716-446655440000', 'your-admin-token');

// Stop streaming
// eventSource.close();
```

### Using Fetch API for Historical Logs

```typescript
async function getNodeLogs(nodeId: string, adminToken: string, tail: number = 100) {
  const response = await fetch(
    `https://api.crosslogic.ai/admin/nodes/${nodeId}/logs?tail=${tail}`,
    {
      headers: {
        'X-Admin-Token': adminToken
      }
    }
  );

  if (!response.ok) {
    throw new Error(`Failed to fetch logs: ${response.statusText}`);
  }

  const data = await response.json();
  return data.logs as LogEntry[];
}

// Usage
const logs = await getNodeLogs('550e8400-e29b-41d4-a716-446655440000', 'your-admin-token');
logs.forEach(log => {
  console.log(`[${log.timestamp}] ${log.message}`);
});
```

## Python Example

```python
import requests
import json
import sseclient  # pip install sseclient-py

def stream_node_logs(node_id: str, admin_token: str):
    """Stream node logs using SSE"""
    url = f"https://api.crosslogic.ai/admin/nodes/{node_id}/logs/stream?follow=true"
    headers = {
        'X-Admin-Token': admin_token,
        'Accept': 'text/event-stream'
    }

    response = requests.get(url, headers=headers, stream=True)
    client = sseclient.SSEClient(response)

    for event in client.events():
        if event.event == 'log':
            log = json.loads(event.data)
            print(f"[{log['level'].upper()}] {log['message']}")

        elif event.event == 'status':
            status = json.loads(event.data)
            print(f"Status: {status['phase']} - {status['progress']}%")

        elif event.event == 'error':
            error = json.loads(event.data)
            print(f"ERROR in {error['phase']}: {error['error']}")

        elif event.event == 'done':
            done = json.loads(event.data)
            print(f"Launch complete: {done['status']}")
            if done.get('endpoint'):
                print(f"Endpoint: {done['endpoint']}")
            break

def get_node_logs(node_id: str, admin_token: str, tail: int = 100):
    """Get historical node logs"""
    url = f"https://api.crosslogic.ai/admin/nodes/{node_id}/logs?tail={tail}"
    headers = {'X-Admin-Token': admin_token}

    response = requests.get(url, headers=headers)
    response.raise_for_status()

    data = response.json()
    return data['logs']

# Usage
if __name__ == '__main__':
    NODE_ID = '550e8400-e29b-41d4-a716-446655440000'
    ADMIN_TOKEN = 'your-admin-token'

    # Stream logs in real-time
    print("Streaming logs...")
    stream_node_logs(NODE_ID, ADMIN_TOKEN)

    # Or get historical logs
    print("\nFetching historical logs...")
    logs = get_node_logs(NODE_ID, ADMIN_TOKEN)
    for log in logs:
        print(f"[{log['timestamp']}] {log['message']}")
```

## Architecture

### Storage Backend

Logs are stored in Redis using the following structure:

- **Key:** `node_logs:{node_id}`
- **Type:** Redis List (RPUSH for append, LRANGE for retrieval)
- **TTL:** 24 hours (logs expire after 24 hours)

### Log Flow

1. **Node Launch:** When a node is launched via `/admin/nodes/launch`, the orchestrator starts logging
2. **Log Capture:** The SkyPilot orchestrator captures key events and progress updates
3. **Redis Storage:** Logs are appended to Redis in real-time
4. **SSE Streaming:** The SSE endpoint polls Redis every 500ms for new logs
5. **Client Consumption:** Clients receive logs as SSE events

### Performance Characteristics

- **Storage:** ~1KB per log entry, ~100 entries per node launch = ~100KB per node
- **Latency:** <1 second from log generation to client delivery
- **Polling Interval:** 500ms for SSE streaming
- **Retention:** 24 hours (configurable)
- **Concurrency:** Supports unlimited concurrent clients per node

## Error Handling

### Client-Side Errors

| Scenario                  | HTTP Status | Response                                          |
|---------------------------|-------------|---------------------------------------------------|
| Missing admin token       | 401         | `{"error": {"message": "missing admin token"}}`   |
| Invalid admin token       | 401         | `{"error": {"message": "invalid admin token"}}`   |
| Node not found            | 404         | `{"error": {"message": "node not found"}}`        |
| Invalid timestamp format  | 400         | `{"error": {"message": "invalid 'since' timestamp format"}}` |

### Server-Side Errors

SSE errors are sent as error events:

```
event: error
data: {"error":"Failed to stream logs","details":"Redis connection lost"}
```

## Best Practices

### For Administrators

1. **Use tail parameter:** Start with `tail=50` to avoid overwhelming the UI with too many logs
2. **Implement auto-reconnect:** SSE connections can drop; implement exponential backoff retry
3. **Monitor stream duration:** Node launches typically take 3-8 minutes; timeout after 15 minutes
4. **Handle terminal states:** Close the connection after receiving `done` event
5. **Log filtering:** Filter by log level on the client side if needed

### For Developers

1. **Connection cleanup:** Always close EventSource when component unmounts
2. **Error handling:** Implement proper error handling for network failures
3. **Progress visualization:** Use the `progress` field to show visual progress bars
4. **Phase tracking:** Use the `phase` field to show current launch stage
5. **Endpoint extraction:** Extract the endpoint URL from the `done` event for quick access

## Troubleshooting

### No logs appearing

1. Verify the node ID is correct
2. Check that the node exists in the database
3. Ensure Redis is running and accessible
4. Check server logs for errors

### Connection drops frequently

1. Check network stability
2. Increase SSE timeout on the server (default: 30 minutes)
3. Implement client-side auto-reconnect

### Logs delayed

1. Check Redis performance and latency
2. Verify polling interval (default: 500ms)
3. Monitor server load

## Security Considerations

1. **Authentication:** All endpoints require admin authentication via `X-Admin-Token`
2. **Authorization:** Only platform administrators can access node logs
3. **Rate Limiting:** Consider implementing rate limits on log streaming endpoints
4. **Data Retention:** Logs auto-expire after 24 hours
5. **Sensitive Data:** Logs do not contain credentials or sensitive configuration

## Future Enhancements

- [ ] Support filtering by log level
- [ ] Add full-text search across logs
- [ ] Implement log export (CSV, JSON)
- [ ] Add webhook notifications for critical events
- [ ] Support log aggregation across multiple nodes
- [ ] Add structured query language for advanced filtering
