type UsagePoint = {
  timestamp: string;
  promptTokens: number;
  completionTokens: number;
  totalCost: string;
};

type ApiKey = {
  id: string;
  name: string;
  prefix: string;
  createdAt: string;
  status: "active" | "revoked" | "suspended";
};

type NodeSummary = {
  id: string;
  status: string;
  model: string;
  region: string;
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
    throw new Error("ADMIN_API_TOKEN missing");
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
    throw new Error(`Admin API failed: ${response.status}`);
  }

  return response.json();
}

export async function fetchUsageHistory(
  tenantId: string
): Promise<UsagePoint[]> {
  try {
    const data = await adminFetch<{ usage: UsagePoint[] }>(
      `/admin/usage/${tenantId}`
    );
    return data.usage;
  } catch (err) {
    console.warn("Falling back to mock usage data", err);
    const now = new Date();
    return Array.from({ length: 5 }).map((_, idx) => {
      const ts = new Date(now.getTime() - idx * 3600 * 1000);
      return {
        timestamp: ts.toISOString(),
        promptTokens: 2000 + idx * 150,
        completionTokens: 1200 + idx * 90,
        totalCost: `$${(0.34 + idx * 0.02).toFixed(2)}`
      };
    });
  }
}

export async function fetchApiKeys(): Promise<ApiKey[]> {
  try {
    const data = await adminFetch<{ keys: ApiKey[] }>("/admin/api-keys");
    return data.keys;
  } catch (err) {
    console.warn("Using mock API key data", err);
    return [
      {
        id: "mock-1",
        name: "Production backend",
        prefix: "clsk_live_abcd",
        createdAt: new Date().toISOString(),
        status: "active"
      },
      {
        id: "mock-2",
        name: "Data science team",
        prefix: "clsk_live_efgh",
        createdAt: new Date(Date.now() - 86400000).toISOString(),
        status: "revoked"
      }
    ];
  }
}

export async function fetchNodeSummaries(): Promise<NodeSummary[]> {
  try {
    const data = await adminFetch<{ nodes: NodeSummary[] }>("/admin/nodes");
    return data.nodes;
  } catch (err) {
    console.warn("Using mock node data", err);
    return [
      {
        id: "node-1",
        status: "active",
        model: "llama-3-70b",
        region: "us-east"
      },
      {
        id: "node-2",
        status: "initializing",
        model: "mistral-7b",
        region: "eu-west"
      }
    ];
  }
}

export type { UsagePoint, ApiKey, NodeSummary };

