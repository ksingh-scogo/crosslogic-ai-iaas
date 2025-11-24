import axios, { type AxiosInstance, type AxiosError } from 'axios'
import type {
  UsageHourly,
  UsagePoint,
  ApiKey,
  CreateApiKeyRequest,
  CreateApiKeyResponse,
  Node,
  NodeSummary,
  LaunchNodeRequest,
  LaunchNodeResponse,
  LaunchStatusResponse,
  ResolveTenantResponse,
  Model,
} from '@/types'

// Create axios instance with base configuration
const createApiClient = (): AxiosInstance => {
  const baseURL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
  const adminToken = import.meta.env.VITE_ADMIN_TOKEN || localStorage.getItem('admin_token') || ''

  const client = axios.create({
    baseURL,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Token': adminToken,
    },
  })

  // Add response interceptor for error handling
  client.interceptors.response.use(
    (response) => response,
    (error: AxiosError) => {
      if (error.response?.status === 401) {
        // Clear invalid token
        localStorage.removeItem('admin_token')
        window.location.href = '/login'
      }
      return Promise.reject(error)
    }
  )

  return client
}

const api = createApiClient()

// Update token dynamically
export const setAuthToken = (token: string) => {
  localStorage.setItem('admin_token', token)
  api.defaults.headers.common['X-Admin-Token'] = token
}

export const clearAuthToken = () => {
  localStorage.removeItem('admin_token')
  delete api.defaults.headers.common['X-Admin-Token']
}

// Usage API
export const fetchUsageHistory = async (tenantId: string): Promise<UsagePoint[]> => {
  try {
    const { data } = await api.get<UsageHourly[]>(`/admin/usage/${tenantId}`)
    return data.map((item) => ({
      timestamp: item.hour,
      promptTokens: 0, // TODO: Backend split
      completionTokens: 0, // TODO: Backend split
      totalTokens: item.total_tokens,
      totalCost: `$${(item.total_cost_microdollars / 1_000_000).toFixed(4)}`,
      requests: item.total_requests,
    }))
  } catch (error) {
    console.error('Failed to fetch usage:', error)
    throw error
  }
}

// API Keys
export const fetchApiKeys = async (tenantId: string): Promise<ApiKey[]> => {
  try {
    const { data } = await api.get<ApiKey[]>(`/admin/api-keys/${tenantId}`)
    return data
  } catch (error) {
    console.error('Failed to fetch API keys:', error)
    throw error
  }
}

export const createApiKey = async (
  request: CreateApiKeyRequest
): Promise<CreateApiKeyResponse> => {
  const { data } = await api.post<CreateApiKeyResponse>('/admin/api-keys', request)
  return data
}

export const revokeApiKey = async (keyId: string): Promise<void> => {
  await api.delete(`/admin/api-keys/${keyId}`)
}

// Nodes
export const fetchNodeSummaries = async (): Promise<NodeSummary[]> => {
  try {
    const { data } = await api.get<Node[]>('/admin/nodes')
    return data.map((node) => ({
      id: node.id,
      status: node.status,
      provider: node.provider,
      endpoint: node.endpoint_url,
      health: node.health_score,
      lastHeartbeat: node.last_heartbeat_at,
      clusterName: node.cluster_name,
      instanceType: node.instance_type,
      region: node.region,
      model: node.model,
    }))
  } catch (error) {
    console.error('Failed to fetch nodes:', error)
    throw error
  }
}

export const launchNode = async (request: LaunchNodeRequest): Promise<LaunchNodeResponse> => {
  const { data } = await api.post<LaunchNodeResponse>('/admin/nodes/launch', request)
  return data
}

export const terminateNode = async (clusterName: string): Promise<void> => {
  await api.post(`/admin/nodes/${clusterName}/terminate`)
}

// Models
export const fetchModels = async (): Promise<Model[]> => {
  try {
    const { data } = await api.get<{ models: Model[] }>('/admin/models/r2')
    return data.models || []
  } catch (error) {
    console.error('Failed to fetch models:', error)
    throw error
  }
}

// Launch Instance
export const launchInstance = async (config: {
  model_name: string
  provider: string
  region: string
  instance_type: string
  use_spot: boolean
}): Promise<LaunchNodeResponse> => {
  const { data } = await api.post<LaunchNodeResponse>('/admin/instances/launch', config)
  return data
}

export const fetchLaunchStatus = async (jobId: string): Promise<LaunchStatusResponse> => {
  const { data } = await api.get<LaunchStatusResponse>(`/admin/instances/status?job_id=${jobId}`)
  return data
}

// Tenant
export const resolveTenant = async (
  email: string,
  name: string
): Promise<ResolveTenantResponse> => {
  const { data } = await api.post<ResolveTenantResponse>('/admin/tenants/resolve', {
    email,
    name,
  })
  return data
}

export default api
