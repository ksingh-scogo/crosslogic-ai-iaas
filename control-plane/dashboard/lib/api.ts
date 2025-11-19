import { format } from "date-fns";

type UsageHourly = {
  id: string;
  hour: string;
  tenant_id: string;
  total_tokens: number;
  total_requests: number;
  total_cost_microdollars: number;
};

type Node = {
  id: string;
  provider: string;
  status: string;
  endpoint_url: string;
  health_score: number;
  last_heartbeat_at: string;
  cluster_name?: string;
};

type ApiKey = {
  id: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at?: string;
  status: "active" | "revoked" | "suspended";
  rate_limit?: number;
};

type CreateApiKeyResponse = {
  id: string;
  key: string;
};

type LaunchNodeRequest = {
  provider: string;
  region: string;
  gpu: string;
  model: string;
  use_spot: boolean;
};

type LaunchNodeResponse = {
  cluster_name: string;
  node_id: string;
  status: string;
};

// UI-friendly types
type UsagePoint = {
  timestamp: string;
  promptTokens: number; // Not available in hourly agg yet, using total
  completionTokens: number; // Not available in hourly agg yet, using 0
  totalTokens: number;
  totalCost: string;
};

type NodeSummary = {
  id: string;
  status: string;
  provider: string;
  endpoint: string;
  health: number;
  lastHeartbeat: string;
  clusterName?: string;
};

const baseUrl =
  process.env.CROSSLOGIC_API_BASE_URL ||
  process.env.NEXT_PUBLIC_CONTROL_PLANE_URL ||
  "http://localhost:8080";

const adminToken =
  process.env.CROSSLOGIC_ADMIN_TOKEN ||
  process.env.ADMIN_API_TOKEN ||
  process.env.NEXT_PUBLIC_ADMIN_TOKEN ||
  "";

async function adminFetch<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  if (!adminToken) {
    console.warn("ADMIN_API_TOKEN missing");
  }

  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Admin-Token": adminToken,
      ...(init?.headers || {})
    },
    cache: "no-store"
  });

  if (!response.ok) {
    throw new Error(`Admin API failed: ${response.status} ${response.statusText}`);
  }

  return response.json();
}

export async function fetchUsageHistory(
  tenantId: string
): Promise<UsagePoint[]> {
  try {
    const data = await adminFetch<UsageHourly[]>(
      `/admin/usage/${tenantId}`
    );
    
    return data.map((item) => ({
      timestamp: item.hour,
      promptTokens: 0, // TODO: Add split to backend
      completionTokens: 0, // TODO: Add split to backend
      totalTokens: item.total_tokens,
      totalCost: `$${(item.total_cost_microdollars / 1_000_000).toFixed(4)}`
    }));
  } catch (err) {
    console.warn("Falling back to mock usage data", err);
    const now = new Date();
    return Array.from({ length: 24 }).map((_, idx) => {
      const ts = new Date(now.getTime() - idx * 3600 * 1000);
      return {
        timestamp: ts.toISOString(),
        promptTokens: 0,
        completionTokens: 0,
        totalTokens: 1000 + idx * 100,
        totalCost: `$${(0.01 + idx * 0.001).toFixed(4)}`
      };
    });
  }
}

export async function fetchApiKeys(tenantId: string): Promise<ApiKey[]> {
  try {
    return await adminFetch<ApiKey[]>(`/admin/api-keys/${tenantId}`);
  } catch (err) {
    console.warn("Falling back to mock API keys", err);
    return [
      {
        id: "mock-1",
        name: "Production backend (Mock)",
        prefix: "sk-...abcd",
        created_at: new Date().toISOString(),
        status: "active"
      }
    ];
  }
}

export async function createApiKey(tenantId: string, name: string): Promise<CreateApiKeyResponse> {
  return await adminFetch<CreateApiKeyResponse>("/admin/api-keys", {
    method: "POST",
    body: JSON.stringify({ tenant_id: tenantId, name })
  });
}

export async function revokeApiKey(keyId: string): Promise<void> {
  await adminFetch(`/admin/api-keys/${keyId}`, {
    method: "DELETE"
  });
}

export async function fetchNodeSummaries(): Promise<NodeSummary[]> {
  try {
    const nodes = await adminFetch<Node[]>("/admin/nodes");
    return nodes.map((node) => ({
      id: node.id,
      status: node.status,
      provider: node.provider,
      endpoint: node.endpoint_url,
      health: node.health_score,
      lastHeartbeat: node.last_heartbeat_at,
      clusterName: node.cluster_name // Assuming backend returns this, if not we might need to update backend or use ID
    }));
  } catch (err) {
    console.warn("Using mock node data", err);
    return [
      {
        id: "node-1",
        status: "active",
        provider: "aws",
        endpoint: "https://node-1.crosslogic.ai",
        health: 98.5,
        lastHeartbeat: new Date().toISOString(),
        clusterName: "cic-node-1"
      }
    ];
  }
}

export async function launchNode(req: LaunchNodeRequest): Promise<LaunchNodeResponse> {
  return await adminFetch<LaunchNodeResponse>("/admin/nodes/launch", {
    method: "POST",
    body: JSON.stringify(req)
  });
}

export async function terminateNode(clusterName: string): Promise<void> {
  await adminFetch(`/admin/nodes/${clusterName}/terminate`, {
    method: "POST"
  });
}

export type { UsageHourly, Node, ApiKey, UsagePoint, NodeSummary, CreateApiKeyResponse, LaunchNodeRequest, LaunchNodeResponse };
