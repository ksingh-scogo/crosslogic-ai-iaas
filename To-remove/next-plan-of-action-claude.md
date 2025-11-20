# CrossLogic AI IaaS: Production Strategy for 99.99% Uptime SLA

## Executive Summary

This document outlines a comprehensive production strategy for CrossLogic AI Infrastructure to achieve 99.99% uptime SLA (52.6 minutes downtime/year) while maintaining cost-effective LLM inference operations. The strategy addresses cluster architecture, multi-cloud deployment, state management, and spot instance lifecycle automation.

## 1. Spot Price Discovery and Availability Checking

### Native SkyPilot Tools for Resource Optimization

#### Real-time Price and Availability Scanning
```bash
# Check GPU availability across all providers
sky show-gpus --all-regions --cloud aws,gcp,azure,lambda

# Get detailed pricing for specific GPU type
sky show-gpus --gpu-name H100 --all-regions --show-spot

# Check specific provider availability
sky check --clouds aws,gcp,azure
```

#### Automated Resource Selection Algorithm
```go
type SpotPriceDiscovery struct {
    providers []CloudProvider
    cache     *PriceCache
    interval  time.Duration
}

func (s *SpotPriceDiscovery) FindBestResource(requirements ResourceReq) (*Instance, error) {
    // Step 1: Query all providers in parallel
    results := make(chan PriceResult, len(s.providers))
    for _, provider := range s.providers {
        go s.queryProvider(provider, requirements, results)
    }

    // Step 2: Collect and rank results
    var options []PriceResult
    for i := 0; i < len(s.providers); i++ {
        if result := <-results; result.Available {
            options = append(options, result)
        }
    }

    // Step 3: Apply selection criteria
    best := s.selectOptimal(options, SelectionCriteria{
        MaxPrice:           requirements.MaxPrice,
        PreferSpot:        true,
        MinAvailability:   0.8,  // 80% spot availability threshold
        RegionPreference:  requirements.PreferredRegions,
    })

    return s.launchInstance(best)
}

type PriceResult struct {
    Provider     string
    Region       string
    InstanceType string
    SpotPrice    float64
    OnDemandPrice float64
    Availability float64  // Spot availability percentage
    Interruption float64  // Interruption rate last 30 days
}
```

#### Continuous Price Monitoring
```yaml
price_monitor:
  scan_interval: 5 minutes

  thresholds:
    spot_premium_alert: 0.7  # Alert if spot > 70% of on-demand
    availability_warning: 0.5  # Warn if availability < 50%

  actions:
    price_spike:
      - notify: slack
      - fallback: on-demand
      - migrate: cheaper_region

    low_availability:
      - pre_warm: on-demand_instances
      - notify: ops_team
```

### Implementation with SkyPilot CLI Integration
```python
import subprocess
import json
from typing import List, Dict

class SkyPilotPriceScanner:
    def scan_best_options(self, gpu_type: str, count: int) -> List[Dict]:
        """
        Scan all providers for best GPU options
        """
        # Use sky show-gpus to get current pricing
        cmd = f"sky show-gpus --gpu-name {gpu_type} --all-regions --format json"
        result = subprocess.run(cmd, shell=True, capture_output=True)

        options = json.loads(result.stdout)

        # Filter and rank options
        ranked = []
        for opt in options:
            score = self.calculate_score(opt)
            ranked.append({
                'provider': opt['cloud'],
                'region': opt['region'],
                'spot_price': opt['spot_price'],
                'on_demand_price': opt['on_demand_price'],
                'availability': opt['availability'],
                'score': score
            })

        return sorted(ranked, key=lambda x: x['score'], reverse=True)[:5]

    def calculate_score(self, option):
        # Weighted scoring: 50% price, 30% availability, 20% region preference
        price_score = 1.0 / (option['spot_price'] + 0.01)
        avail_score = option['availability']
        region_score = 1.0 if option['region'] in PREFERRED_REGIONS else 0.5

        return (price_score * 0.5 + avail_score * 0.3 + region_score * 0.2)
```

## 2. Cluster Architecture Clarification

### Current Understanding
- **Cluster Definition**: A logical grouping of compute resources managed by SkyPilot
- **Recommended Approach**: Single-provider, single-region clusters
- **Multi-cloud Strategy**: Achieved through multiple independent clusters, not mixed nodes

### Production Architecture
```
┌─────────────────────────────────────────────────────────┐
│                   Control Plane (HA)                     │
│         ┌─────────────┐     ┌─────────────┐            │
│         │  Primary    │────│   Standby    │            │
│         │  us-east-1  │     │  us-west-2  │            │
│         └─────────────┘     └─────────────┘            │
└────────────────────┬───────────────────────────────────┘
                     │
        ┌────────────┼────────────┬────────────┐
        ▼            ▼            ▼            ▼
   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
   │  AWS    │  │  GCP    │  │  Azure  │  │ RunPod  │
   │ Cluster │  │ Cluster │  │ Cluster │  │ Cluster │
   └─────────┘  └─────────┘  └─────────┘  └─────────┘
   Single-region clusters with isolated failure domains
```

### Key Design Decisions
- **No Cross-Provider Clusters**: Each cluster operates within a single cloud provider boundary
- **Regional Isolation**: Clusters are region-specific to minimize latency and network costs
- **Independent Failure Domains**: One cluster failure doesn't cascade to others
- **Simplified Networking**: No complex cross-cloud VPN/peering requirements

## 3. Enhanced Naming Convention

### Format
```
cic-{provider}-{region}-{gpu_type}-{pricing}-{timestamp}-{sequence}
```

### Examples
```
# Production H100 cluster on AWS
cic-aws-us-west-2-h100-ondemand-20250119-001

# Cost-optimized A100 cluster on GCP
cic-gcp-us-central1-a100-spot-20250119-002

# Individual nodes within cluster
cic-aws-us-west-2-h100-ondemand-20250119-001-node001
cic-aws-us-west-2-h100-ondemand-20250119-001-node002
```

### Implementation
```go
type ClusterNameGenerator struct {
    Provider  string // aws, gcp, azure, runpod
    Region    string // us-west-2, us-central1
    GPUType   string // h100, a100, l40s
    Pricing   string // spot, ondemand
    Timestamp string // YYYYMMDD
    Sequence  int    // incremental counter
}

func (c *ClusterNameGenerator) Generate() string {
    return fmt.Sprintf("cic-%s-%s-%s-%s-%s-%03d",
        c.Provider, c.Region, c.GPUType, c.Pricing,
        c.Timestamp, c.Sequence)
}
```

## 4. Multi-GPU Configuration for Large Models

### DeepSeek V3 (685B) on 8xH100 Configuration

#### vLLM Launch Command
```bash
vllm serve deepseek-ai/DeepSeek-V3 \
    --tensor-parallel-size 8 \
    --gpu-memory-utilization 0.95 \
    --max-model-len 32768 \
    --enable-prefix-caching \
    --enable-chunked-prefill \
    --max-num-batched-tokens 32768 \
    --disable-log-requests \
    --kv-cache-dtype fp8 \
    --quantization fp8 \
    --distributed-executor-backend ray \
    --engine-use-ray \
    --ray-workers-use-nsight
```

#### Memory Calculation
```
Model Size: 685B parameters × 2 bytes (FP16) = 1,370 GB
With FP8 Quantization: ~685 GB
Available VRAM: 8 × 80GB = 640 GB
KV Cache per Token: ~1.5 GB for 32K context
Total Memory Usage: ~95% utilization safe
```

#### Performance Targets
- **Throughput**: 50-100 tokens/second per user
- **Latency**: P50 < 50ms, P95 < 200ms per token
- **Batch Size**: Dynamic batching up to 256 concurrent requests
- **Context Window**: 32K tokens standard, 128K on request

### Scaling Strategy
```yaml
model_configs:
  small_models:  # < 70B parameters
    tensor_parallel: 1
    pipeline_parallel: 1
    instances: multiple

  medium_models: # 70B - 200B parameters
    tensor_parallel: 2-4
    pipeline_parallel: 1
    instances: 2-4

  large_models:  # 200B+ parameters
    tensor_parallel: 8
    pipeline_parallel: 1-2
    instances: 1-2
```

## 5. Model Replica Management for High Availability

### Multi-Replica Architecture
```yaml
model_deployment:
  name: meta-llama-7b
  replicas: 2  # Admin-configurable
  strategy:
    type: RollingUpdate
    load_balancing: round-robin
    health_check_interval: 10s
    failover_threshold: 3  # consecutive failures

  placement:
    anti_affinity: true  # Don't place replicas on same physical host
    spread_across:
      - availability_zones
      - providers  # Optional: cross-provider for ultimate HA

  scaling:
    min_replicas: 2
    max_replicas: 10
    metrics:
      - type: request_rate
        target: 100  # requests per second
      - type: latency_p95
        target: 200  # milliseconds
```

### Replica Launch Orchestration
```go
type ModelReplicaManager struct {
    models    map[string]*ModelDeployment
    skyPilot  *SkyPilotClient
    loadBalancer *LoadBalancer
}

func (m *ModelReplicaManager) DeployModel(config ModelConfig) error {
    deployment := &ModelDeployment{
        ModelName: config.Name,
        Replicas:  config.Replicas,
        Instances: make([]*Instance, 0, config.Replicas),
    }

    // Launch replicas in parallel
    var wg sync.WaitGroup
    errors := make(chan error, config.Replicas)

    for i := 0; i < config.Replicas; i++ {
        wg.Add(1)
        go func(replicaID int) {
            defer wg.Done()

            // Find best available resource for this replica
            resource := m.findOptimalResource(config, replicaID)

            // Launch with anti-affinity rules
            instance, err := m.skyPilot.Launch(SkyPilotConfig{
                Name:     fmt.Sprintf("%s-replica-%d", config.Name, replicaID),
                Resource: resource,
                AntiAffinity: deployment.GetOtherReplicas(),
                Script:   m.generateLaunchScript(config),
            })

            if err != nil {
                errors <- err
                return
            }

            deployment.Instances = append(deployment.Instances, instance)
        }(i)
    }

    wg.Wait()
    close(errors)

    // Check for launch failures
    var launchErrors []error
    for err := range errors {
        launchErrors = append(launchErrors, err)
    }

    // Ensure minimum replicas are running
    if len(deployment.Instances) < config.MinReplicas {
        return fmt.Errorf("failed to launch minimum replicas: %d/%d successful",
            len(deployment.Instances), config.MinReplicas)
    }

    // Register with load balancer
    for _, instance := range deployment.Instances {
        m.loadBalancer.AddEndpoint(instance.Endpoint, LoadBalancerConfig{
            Weight:       1,
            HealthCheck:  "/health",
            MaxFails:     3,
        })
    }

    m.models[config.Name] = deployment
    return nil
}
```

### Intelligent Request Routing
```python
class InferenceLoadBalancer:
    def __init__(self, replicas: List[Replica]):
        self.replicas = replicas
        self.health_checker = HealthChecker()
        self.metrics = MetricsCollector()

    def route_request(self, request: InferenceRequest) -> Replica:
        """
        Route to the best available replica based on:
        1. Health status
        2. Current load
        3. Latency metrics
        4. Affinity preferences
        """
        healthy_replicas = self.get_healthy_replicas()

        if not healthy_replicas:
            # All replicas down - trigger emergency launch
            self.trigger_emergency_replica()
            raise ServiceUnavailableError()

        # Select replica with lowest latency and load
        best_replica = min(healthy_replicas,
            key=lambda r: r.current_latency * 0.5 + r.queue_depth * 0.5)

        return best_replica

    def handle_replica_failure(self, failed_replica: Replica):
        """
        Handle replica failure with automatic recovery
        """
        # Mark as unhealthy
        failed_replica.healthy = False

        # Remove from load balancer pool
        self.remove_from_pool(failed_replica)

        # Launch replacement if below minimum
        if len(self.healthy_replicas) < self.min_replicas:
            self.launch_replacement_replica()

        # Redistribute load
        self.rebalance_load()
```

## 6. Fast Model Loading via CDN/Cache

### Model Storage Architecture
```yaml
model_storage:
  primary:
    type: distributed_cache
    providers:
      - cloudflare_r2  # Global CDN with S3 API
      - aws_efs        # Regional persistent cache
      - gcp_filestore  # GCP equivalent

  cache_hierarchy:
    L1_edge:
      location: CDN edge nodes
      capacity: 10TB per region
      models: frequently_used  # Top 20%

    L2_regional:
      location: Cloud provider storage (EFS/Filestore)
      capacity: 100TB per region
      models: all_active  # All deployed models

    L3_archive:
      location: S3/GCS
      capacity: unlimited
      models: all  # Complete model library
```

### Mount-Based Model Loading
```bash
#!/bin/bash
# VM startup script for instant model access

# Option 1: AWS EFS Mount (Pre-cached models)
mkdir -p /models
mount -t efs ${EFS_ID}:/ /models

# Option 2: GCP Filestore
mount -t nfs ${FILESTORE_IP}:/models /models

# Option 3: Cloudflare R2 via S3 FUSE
s3fs ${R2_BUCKET} /models \
  -o url=https://s3.r2.cloudflarestorage.com \
  -o use_cache=/tmp/s3fs_cache \
  -o kernel_cache \
  -o max_background=1000 \
  -o max_stat_cache_size=100000

# Start vLLM with mounted model directory
vllm serve /models/${MODEL_NAME} \
  --model-loader-extra-config '{"cache_dir": "/models"}' \
  --download-dir /models \
  --load-format safetensors
```

### Pre-warming and Caching Strategy
```go
type ModelCacheManager struct {
    cdn      *CloudflareCDN
    efs      *AWSEFSClient
    models   map[string]*ModelMetadata
}

func (m *ModelCacheManager) PrewarmModel(modelName string, regions []string) error {
    model := m.models[modelName]

    // Step 1: Upload to CDN if not present
    if !m.cdn.Has(model.Path) {
        m.cdn.Upload(model.Path, model.Files)
    }

    // Step 2: Pre-cache in regional storage
    for _, region := range regions {
        go m.prewarmRegion(model, region)
    }

    // Step 3: Set cache headers for optimal performance
    m.cdn.SetCacheControl(model.Path, CacheControl{
        MaxAge:       86400,  // 24 hours
        Immutable:    true,   // Models don't change
        EdgeTTL:      604800, // 7 days at edge
    })

    return nil
}

func (m *ModelCacheManager) GetModelPath(modelName string, region string) string {
    // Priority order for model loading
    paths := []string{
        fmt.Sprintf("/efs/%s/%s", region, modelName),     // Local EFS
        fmt.Sprintf("/r2-cache/%s", modelName),           // CDN cache
        fmt.Sprintf("s3://models/%s", modelName),         // Fallback to S3
    }

    for _, path := range paths {
        if m.isAccessible(path) {
            return path
        }
    }

    return ""
}
```

### Zero-Download Model Loading
```python
class FastModelLoader:
    """
    Eliminates model download time by using mounted storage
    """
    def __init__(self):
        self.mount_points = {
            'efs': '/models/efs',
            'r2': '/models/r2',
            'filestore': '/models/gcp'
        }

    def load_model(self, model_name: str) -> str:
        """
        Returns the path to the model without downloading
        """
        # Check each mount point for model availability
        for mount_type, mount_path in self.mount_points.items():
            model_path = f"{mount_path}/{model_name}"
            if os.path.exists(model_path):
                print(f"Model found at {mount_type}: {model_path}")

                # Verify model integrity
                if self.verify_model(model_path):
                    return model_path

        # Fallback: trigger background cache population
        self.populate_cache(model_name)
        raise ModelNotCachedError(f"Model {model_name} not in cache")

    def verify_model(self, path: str) -> bool:
        """
        Quick integrity check without loading entire model
        """
        manifest = f"{path}/model.safetensors.index.json"
        if os.path.exists(manifest):
            with open(manifest) as f:
                index = json.load(f)
                # Verify all shards are present
                for shard in index.get('weight_map', {}).values():
                    if not os.path.exists(f"{path}/{shard}"):
                        return False
        return True
```

### Performance Comparison
```yaml
loading_performance:
  traditional_download:
    steps:
      - download_from_s3: 5-10 minutes (100GB model)
      - extract_files: 1-2 minutes
      - load_to_gpu: 30-60 seconds
    total: 6-12 minutes

  cdn_mount_approach:
    steps:
      - mount_verification: 1-2 seconds
      - load_to_gpu: 30-60 seconds
    total: 31-62 seconds

  improvement: 10-20x faster model loading
```

## 7. Documentation Requirements

### Essential Documentation Sections

#### 7.1 Provisioning Lifecycle
```mermaid
graph LR
    A[Request Received] --> B[Capacity Check]
    B --> C{Spot Available?}
    C -->|Yes| D[Launch Spot Instance]
    C -->|No| E[Launch On-Demand]
    D --> F[Configure vLLM]
    E --> F
    F --> G[Health Check]
    G --> H[Register with LB]
    H --> I[Ready for Traffic]
```

#### 7.2 Model Deployment Guide
- Optimal shard configurations per model size
- Memory requirements calculator
- Batch size optimization strategies
- Quantization trade-offs (FP8 vs FP16 vs INT8)

#### 7.3 Disaster Recovery Playbook
- Control plane failover procedures (60-second RTO)
- Data recovery from S3/GCS snapshots
- Model checkpoint restoration
- Traffic rerouting strategies

#### 7.4 Cost Optimization Guide
- Spot vs on-demand decision matrix
- Reserved capacity planning
- Off-peak scheduling strategies
- Model routing based on cost/performance

## 8. SkyPilot State Management

### PostgreSQL-Based State Store
```sql
-- Core state management tables
CREATE TABLE skypilot_clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_name VARCHAR(255) UNIQUE NOT NULL,
    provider VARCHAR(50) NOT NULL,
    region VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL, -- provisioning, running, terminating, terminated
    config JSONB NOT NULL, -- Full cluster configuration
    nodes JSONB DEFAULT '[]'::jsonb, -- Array of node details
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_health_check TIMESTAMP WITH TIME ZONE
);

CREATE TABLE skypilot_state_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID REFERENCES skypilot_clusters(id),
    snapshot_data JSONB NOT NULL, -- Complete state snapshot
    s3_backup_url TEXT, -- S3 location of backup
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- State synchronization
CREATE TABLE skypilot_sync_status (
    cluster_name VARCHAR(255) PRIMARY KEY,
    local_state_hash VARCHAR(64),
    remote_state_hash VARCHAR(64),
    last_sync TIMESTAMP WITH TIME ZONE,
    sync_status VARCHAR(50) -- in_sync, diverged, syncing
);
```

### State Synchronization Strategy
```go
type StateManager struct {
    db       *sql.DB
    s3Client *s3.Client
    interval time.Duration
}

func (sm *StateManager) SyncLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            sm.syncState(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (sm *StateManager) syncState(ctx context.Context) error {
    // 1. Get SkyPilot local state
    localState := skyutil.GetClusterState()

    // 2. Compare with PostgreSQL
    dbState := sm.getDBState(ctx)

    // 3. Reconcile differences
    if !reflect.DeepEqual(localState, dbState) {
        sm.reconcile(ctx, localState, dbState)
    }

    // 4. Backup to S3 every 5 minutes
    if time.Since(lastBackup) > 5*time.Minute {
        sm.backupToS3(ctx, localState)
    }

    return nil
}
```

## 9. Control Plane Resilience

### High Availability Architecture
```yaml
control_plane:
  primary:
    region: us-east-1
    components:
      - api_server: 3 instances behind ALB
      - postgresql: Primary with streaming replication
      - redis: Cluster mode with 3 masters
      - prometheus: With remote write to S3

  standby:
    region: us-west-2
    components:
      - api_server: 2 instances (warm standby)
      - postgresql: Read replica (promotable)
      - redis: Replica of primary cluster
      - prometheus: Independent instance

  failover:
    trigger_conditions:
      - api_unavailable: 3 consecutive health check failures
      - database_unreachable: Connection timeout > 5 seconds
      - manual_trigger: Via admin API

    procedure:
      1. Route53 health check detects failure
      2. DNS failover to standby region (30 seconds)
      3. Promote PostgreSQL replica (15 seconds)
      4. Activate standby API servers (15 seconds)
      5. Total RTO: 60 seconds
```

### Health Monitoring
```go
type HealthMonitor struct {
    checks []HealthCheck
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
    Critical() bool // If true, triggers failover
}

// Critical health checks
var criticalChecks = []HealthCheck{
    &DatabaseHealthCheck{},
    &RedisHealthCheck{},
    &APIHealthCheck{},
    &SkyPilotControllerCheck{},
}

// Non-critical checks (alerts only)
var warningChecks = []HealthCheck{
    &PrometheusHealthCheck{},
    &LoggingHealthCheck{},
    &BackupHealthCheck{},
}
```

## 10. Multi-Cloud Scaling Strategy

### Tiered Cloud Provider Model

#### Tier 1: Primary Production (AWS + GCP)
- **Use Case**: Mission-critical, latency-sensitive workloads
- **SLA**: 99.99% uptime guarantee
- **Capacity**: 60% of total inference capacity
- **Instance Types**: On-demand H100, A100
- **Cost**: $2-4 per GPU-hour

#### Tier 2: Overflow Capacity (Azure + Lambda Labs)
- **Use Case**: Burst traffic, failover capacity
- **SLA**: 99.9% uptime
- **Capacity**: 25% of total inference capacity
- **Instance Types**: Mix of spot and on-demand
- **Cost**: $1.5-3 per GPU-hour

#### Tier 3: Cost Optimization (RunPod + Vast.ai + Together.ai)
- **Use Case**: Batch processing, non-critical workloads
- **SLA**: 99% uptime
- **Capacity**: 15% of total inference capacity
- **Instance Types**: Primarily spot instances
- **Cost**: $0.5-2 per GPU-hour

### Intelligent Routing Algorithm
```python
class CloudRouter:
    def route_request(self, request):
        # Priority 1: Latency requirements
        if request.latency_sla < 100:  # ms
            return self.tier1_providers

        # Priority 2: Model requirements
        if request.model_size > 200_000_000_000:  # 200B+
            return self.get_providers_with_gpu("H100")

        # Priority 3: Cost optimization
        if request.priority == "batch":
            return self.tier3_providers

        # Priority 4: Availability
        return self.get_available_providers()
```

### Capacity Planning
```yaml
capacity_allocation:
  baseline:  # Always running
    aws: 10 x H100 nodes (on-demand)
    gcp: 10 x A100 nodes (committed use)
    total: 20 nodes (30% of peak capacity)

  elastic:   # Auto-scaling
    aws: 0-20 x H100 nodes (spot)
    gcp: 0-15 x A100 nodes (spot)
    azure: 0-10 x A100 nodes (spot)
    runpod: 0-20 x A100/H100 (spot)
    total: 0-65 nodes (70% of peak capacity)

  surge:     # Emergency capacity
    lambda_labs: On-demand H100 (uncapped)
    together_ai: API fallback
```

## 11. Spot Instance Lifecycle Automation with Dual Safety

### Enhanced Termination Detection with Dual Safety Mechanisms

#### Push-Pull Architecture for Maximum Reliability
```go
type DualSafetyMonitor struct {
    // Push mechanism: Node agent sends heartbeats
    heartbeatReceiver *HeartbeatServer

    // Pull mechanism: Controller actively polls
    activePoller      *NodePoller

    // Coordination
    stateManager      *StateManager
    alertManager      *AlertManager
}

// Node Agent Side - Push Mechanism
type NodeAgent struct {
    controllerURL string
    instanceID    string
    heartbeatInterval time.Duration
}

func (n *NodeAgent) Start(ctx context.Context) {
    // Regular heartbeat
    heartbeatTicker := time.NewTicker(n.heartbeatInterval)

    // Spot termination watcher
    go n.watchSpotTermination(ctx)

    for {
        select {
        case <-heartbeatTicker.C:
            n.sendHeartbeat()
        case <-ctx.Done():
            return
        }
    }
}

func (n *NodeAgent) watchSpotTermination(ctx context.Context) {
    // AWS spot termination notice
    awsWatcher := &AWSSpotWatcher{
        MetadataURL: "http://169.254.169.254/latest/meta-data/spot/termination-time",
        CheckInterval: 5 * time.Second,
    }

    // GCP preemption notice
    gcpWatcher := &GCPPreemptionWatcher{
        MetadataURL: "http://metadata.google.internal/computeMetadata/v1/instance/preempted",
        CheckInterval: 5 * time.Second,
    }

    for {
        if termination := awsWatcher.Check(); termination != nil {
            n.sendTerminationWarning(termination)
        }

        if preemption := gcpWatcher.Check(); preemption != nil {
            n.sendTerminationWarning(preemption)
        }

        time.Sleep(5 * time.Second)
    }
}

func (n *NodeAgent) sendHeartbeat() error {
    payload := HeartbeatPayload{
        InstanceID: n.instanceID,
        Timestamp:  time.Now(),
        Health: HealthStatus{
            CPUUsage:    getCPUUsage(),
            MemoryUsage: getMemoryUsage(),
            GPUStatus:   getGPUStatus(),
            ModelStatus: getModelStatus(),
        },
    }

    return n.post("/heartbeat", payload)
}

func (n *NodeAgent) sendTerminationWarning(warning *TerminationWarning) error {
    return n.post("/termination-warning", warning)
}
```

#### Controller Side - Pull Mechanism
```go
type NodePoller struct {
    nodes        map[string]*Node
    pollInterval time.Duration
    timeout      time.Duration
}

func (p *NodePoller) Start(ctx context.Context) {
    ticker := time.NewTicker(p.pollInterval)

    for {
        select {
        case <-ticker.C:
            p.pollAllNodes(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (p *NodePoller) pollAllNodes(ctx context.Context) {
    var wg sync.WaitGroup

    for _, node := range p.nodes {
        wg.Add(1)
        go func(n *Node) {
            defer wg.Done()

            // Try multiple health check methods
            healthy := false

            // Method 1: HTTP health endpoint
            if p.checkHTTPHealth(n) {
                healthy = true
            } else if p.checkSSHHealth(n) { // Method 2: SSH connectivity
                healthy = true
            } else if p.checkCloudAPIHealth(n) { // Method 3: Cloud provider API
                healthy = true
            }

            if !healthy {
                p.handleUnhealthyNode(n)
            }
        }(node)
    }

    wg.Wait()
}

func (p *NodePoller) handleUnhealthyNode(node *Node) {
    node.FailureCount++

    if node.FailureCount >= 3 {
        // Node is confirmed down
        log.Error("Node confirmed down after 3 failures",
            "nodeID", node.ID,
            "lastSeen", node.LastSeen)

        // Trigger replacement
        p.replaceNode(node)
    }
}
```

#### Coordination Layer
```go
type HealthCoordinator struct {
    heartbeats   map[string]time.Time
    pollResults  map[string]PollResult
    mu           sync.RWMutex
}

func (h *HealthCoordinator) ReconcileHealth(nodeID string) NodeHealth {
    h.mu.RLock()
    defer h.mu.RUnlock()

    lastHeartbeat := h.heartbeats[nodeID]
    pollResult := h.pollResults[nodeID]

    // Dual verification
    heartbeatHealthy := time.Since(lastHeartbeat) < 30*time.Second
    pollHealthy := pollResult.Status == "healthy"

    if heartbeatHealthy && pollHealthy {
        return NodeHealthy
    } else if !heartbeatHealthy && !pollHealthy {
        return NodeDead
    } else {
        // Disagreement - investigate further
        return NodeDegraded
    }
}

func (h *HealthCoordinator) HandleNodeFailure(nodeID string) {
    health := h.ReconcileHealth(nodeID)

    switch health {
    case NodeDead:
        // Both mechanisms agree - node is dead
        h.immediateReplace(nodeID)

    case NodeDegraded:
        // One mechanism reports failure - investigate
        if h.investigateNode(nodeID) == NodeDead {
            h.gracefulReplace(nodeID)
        }

    case NodeHealthy:
        // False alarm - node is actually healthy
        log.Info("False alarm for node", "nodeID", nodeID)
    }
}
```

#### Recovery Procedures
```yaml
failure_scenarios:
  node_agent_crash:
    detection: No heartbeat but VM responsive to polling
    action:
      1. SSH to VM and restart node-agent service
      2. If restart fails, mark VM for replacement
      3. Migrate workload to healthy replicas

  network_partition:
    detection: No heartbeat, no poll response, but cloud API shows running
    action:
      1. Wait 60 seconds for network recovery
      2. Check alternate network paths
      3. If persistent, terminate via cloud API
      4. Launch replacement in different AZ

  spot_termination:
    detection: Termination warning received
    action:
      1. Stop accepting new requests immediately
      2. Save model checkpoint to S3
      3. Drain existing requests (max 90 seconds)
      4. Launch replacement preemptively
      5. Graceful shutdown

  vm_freeze:
    detection: VM running but not responding to any checks
    action:
      1. Force stop via cloud API
      2. Launch replacement immediately
      3. Investigate root cause from logs
```

### Original Termination Handling Pipeline
```go
type SpotLifecycleManager struct {
    checkpointInterval time.Duration
    migrationTime      time.Duration
}

func (m *SpotLifecycleManager) HandleTerminationNotice(ctx context.Context, instance Instance) {
    // Step 1: Receive 2-minute warning
    log.Info("Spot termination notice received", "instance", instance.ID)

    // Step 2: Stop accepting new requests
    instance.SetDraining(true)

    // Step 3: Save model checkpoint
    checkpoint := m.saveCheckpoint(ctx, instance)
    m.uploadToS3(ctx, checkpoint)

    // Step 4: Launch replacement
    replacement := m.launchReplacement(ctx, instance.Config)

    // Step 5: Migrate active requests
    m.migrateRequests(ctx, instance, replacement)

    // Step 6: Graceful shutdown
    instance.WaitForRequestsToComplete(60 * time.Second)
    instance.Shutdown()
}
```

### Checkpoint Management
```yaml
checkpointing:
  interval: 5 minutes
  storage:
    primary: S3 with versioning
    backup: Cross-region replication

  checkpoint_contents:
    - model_weights: Saved in SafeTensors format
    - kv_cache: Optional for stateful sessions
    - request_queue: In-flight requests
    - metrics: Performance counters

  restore_procedure:
    1. Download latest checkpoint from S3
    2. Load model weights into vLLM
    3. Warm up KV cache if available
    4. Resume request processing
    5. Total restore time: < 2 minutes
```

### Automated Replacement Strategy
```python
class SpotReplacementOrchestrator:
    def __init__(self):
        self.replacement_pool = []
        self.spot_fleet_target = 50  # nodes

    def maintain_fleet(self):
        while True:
            current_spots = self.get_running_spots()

            # Pre-warm replacements when spot availability drops
            if len(current_spots) < self.spot_fleet_target * 0.8:
                self.launch_on_demand_buffer(5)

            # Monitor for termination notices
            for spot in current_spots:
                if spot.has_termination_notice():
                    self.handle_termination(spot)

            time.sleep(30)

    def handle_termination(self, spot):
        # Use pre-warmed instance if available
        if self.replacement_pool:
            replacement = self.replacement_pool.pop()
        else:
            # Launch on-demand as emergency backup
            replacement = self.launch_on_demand_immediate()

        self.migrate_workload(spot, replacement)
```

## Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- [ ] Implement PostgreSQL state management tables
- [ ] Create cluster naming convention generator
- [ ] Set up basic health monitoring
- [ ] Document provisioning lifecycle

### Phase 2: High Availability (Week 3-4)
- [ ] Deploy standby control plane in us-west-2
- [ ] Configure PostgreSQL streaming replication
- [ ] Implement Route53 health checks
- [ ] Test failover procedures

### Phase 3: Multi-Cloud Integration (Week 5-6)
- [ ] Integrate RunPod and Vast.ai APIs
- [ ] Implement intelligent routing algorithm
- [ ] Set up cross-cloud monitoring
- [ ] Create cost optimization dashboard

### Phase 4: Spot Automation (Week 7-8)
- [ ] Implement termination notice handlers
- [ ] Create checkpoint management system
- [ ] Build automated replacement orchestrator
- [ ] Develop migration procedures

### Phase 5: Production Hardening (Week 9-10)
- [ ] Load testing at 10x expected capacity
- [ ] Chaos engineering exercises
- [ ] Security audit and penetration testing
- [ ] Complete documentation and runbooks

## Success Metrics

### SLA Targets
- **Uptime**: 99.99% (52.6 minutes downtime/year)
- **API Latency**: P50 < 100ms, P99 < 500ms
- **Model Inference**: P50 < 50ms/token, P95 < 200ms/token
- **Failover RTO**: < 60 seconds
- **Checkpoint RPO**: < 5 minutes

### Cost Targets
- **Spot Usage**: > 70% of total compute
- **Utilization**: > 80% GPU utilization
- **Cost per Token**: < $0.0001 for most models
- **Monthly Savings**: 40-60% vs pure on-demand

### Operational Targets
- **MTTR**: < 15 minutes for critical issues
- **Deployment Frequency**: Daily with zero downtime
- **Change Failure Rate**: < 5%
- **Alert Noise**: < 10 actionable alerts/day

## Risk Mitigation

### Identified Risks and Mitigations

1. **Spot Instance Unavailability**
   - Mitigation: Multi-cloud strategy with 30% on-demand baseline
   - Fallback: Surge capacity agreements with Lambda Labs

2. **Control Plane Failure**
   - Mitigation: Active-passive HA with 60-second failover
   - Fallback: Manual intervention procedures documented

3. **Model Checkpoint Corruption**
   - Mitigation: Versioned S3 storage with cross-region replication
   - Fallback: Multiple checkpoint retention (last 10 versions)

4. **Network Partitions**
   - Mitigation: Regional isolation, no cross-cloud dependencies
   - Fallback: Graceful degradation to available regions

5. **Cost Overruns**
   - Mitigation: Hard spending limits per cloud provider
   - Fallback: Automatic workload shedding based on priority

## Conclusion

This production strategy provides a robust foundation for CrossLogic AI IaaS to deliver 99.99% uptime SLA while optimizing costs through intelligent multi-cloud orchestration and spot instance utilization. The phased implementation approach ensures systematic risk reduction while building toward a fully automated, self-healing infrastructure.

### Next Steps
1. Review and approve strategy with stakeholders
2. Allocate engineering resources for implementation
3. Set up monitoring and alerting infrastructure
4. Begin Phase 1 implementation
5. Schedule weekly progress reviews

### Key Success Factors
- **Automation First**: Every manual process is a potential failure point
- **Observability**: You can't fix what you can't see
- **Gradual Rollout**: Test in staging, canary in production
- **Documentation**: Runbooks for every scenario
- **Team Training**: Everyone should understand the system

---
*Document Version: 1.0*
*Last Updated: January 19, 2025*
*Author: Claude (Anthropic)*
*Status: Draft - Pending Review*