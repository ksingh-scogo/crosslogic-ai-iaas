// API Response Types
export interface UsageHourly {
  id: string
  hour: string
  tenant_id: string
  total_tokens: number
  total_requests: number
  total_cost_microdollars: number
}

export interface UsagePoint {
  timestamp: string
  promptTokens: number
  completionTokens: number
  totalTokens: number
  totalCost: string
  requests?: number
}

export interface Node {
  id: string
  provider: string
  status: string
  endpoint_url: string
  health_score: number
  last_heartbeat_at: string
  cluster_name?: string
  instance_type?: string
  region?: string
  model?: string
  created_at?: string
}

export interface NodeSummary {
  id: string
  status: string
  provider: string
  endpoint: string
  health: number
  lastHeartbeat: string
  clusterName?: string
  instanceType?: string
  region?: string
  model?: string
}

export interface ApiKey {
  id: string
  name: string
  prefix: string
  created_at: string
  last_used_at?: string
  status: 'active' | 'revoked' | 'suspended'
  rate_limit?: number
  description?: string
}

export interface CreateApiKeyRequest {
  tenant_id: string
  name: string
  description?: string
  scopes?: string[]
  rate_limit?: number
}

export interface CreateApiKeyResponse {
  id: string
  key: string
}

export interface Model {
  id: string
  name: string
  family: string
  size: string
  type: string
  vram_required_gb: number
  description?: string
}

export interface LaunchNodeRequest {
  provider: string
  region: string
  gpu: string
  model: string
  use_spot: boolean
  instance_type?: string
}

export interface LaunchNodeResponse {
  cluster_name: string
  node_id: string
  status: string
  job_id?: string
}

export interface LaunchStatusResponse {
  job_id: string
  status: 'pending' | 'provisioning' | 'configuring' | 'loading' | 'completed' | 'failed'
  progress?: number
  stages?: string[]
  error?: string
  endpoint_url?: string
}

export interface ResolveTenantResponse {
  id: string
  status: string
  new: boolean
}

export interface InstanceSpec {
  vcpu: number
  memory_gb: number
  gpu_count: number
  gpu_vram_gb: number
  gpu_model?: string
  cost_per_hour?: number
}

// Auth Types
export interface User {
  id: string
  email: string
  name: string
  tenantId?: string
}

export interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
}
