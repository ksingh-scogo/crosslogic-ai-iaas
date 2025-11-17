Great — I will deliver the Detailed Control Plane Architecture in 4–5 messages, each one a clean, well-structured section of the full document.

This allows me to produce a deep, engineering-grade architecture without token truncation, while keeping it readable and reference-ready.

⸻

✅ CONTROL PLANE ARCHITECTURE DOCUMENT — PART 1/5

Introduction, Principles, Responsibilities, High-Level Architecture

⸻

1. Introduction

The Control Plane is the brain of CrossLogic.AI’s inference platform.
It orchestrates:
	•	multi-region routing
	•	rate limiting
	•	token accounting
	•	GPU node scheduling
	•	spot interruption handling
	•	model metadata
	•	billing reporting
	•	org / environment isolation
	•	hybrid on-prem integration

The Data Plane (vLLM/SGLang GPU workers) does only inference.
Everything else happens in the Control Plane.

This document defines the final state-of-the-art architecture, while also describing the MVP-friendly version that you can build as a solo engineer for the first ~100 customers.

All components are written in Go, using:
	•	pgx for Postgres
	•	go-redis for Redis
	•	grpc-go for node RPC
	•	chi or fiber for HTTP internal APIs

⸻

2. Control Plane Goals

Primary Goals
	1.	Multi-tenancy
	2.	Region-aware scheduling
	3.	Token-per-second enforcement
	4.	Billing accuracy
	5.	Per-org & per-env isolation
	6.	High reliability despite spot nodes
	7.	Realtime usage tracking
	8.	Low-latency routing
	9.	On-prem hybrid support

⸻

Secondary Goals
	1.	Horizontally scalable
	2.	Pluggable model registry
	3.	Ready for >10,000 orgs
	4.	Monitorable & observable
	5.	Cloud → On-Prem control channel

⸻

3. Architecture Principles

3.1 Separation of Control Plane & Data Plane
	•	Data plane = GPU inference
	•	Control plane = everything else

3.2 Stateless Frontend, Stateful Control Plane
	•	API Gateway is stateless
	•	Control plane manages state (Redis + Postgres)

3.3 Redis for speed, Postgres for truth

Redis = counters, limits, TTL
Postgres = metadata, billing, models, orgs, environments

3.4 Never trust a GPU Node

GPU nodes are:
	•	spot
	•	unpredictable
	•	replaceable
	•	ephemeral

The Control Plane must treat them as cattle, not pets.

3.5 Environment Isolation

org_id + env_id + region + model form the routing key.

⸻

4. Control Plane – High-Level Responsibilities

The Control Plane must handle ten categories of work.

1. Authentication & Authorization
	•	Validate API keys
	•	Resolve org/env
	•	Enforce org-wide disable
	•	Enforce key suspension

2. Rate Limiting
	•	Per-org
	•	Per-env
	•	Per-key
	•	Per-region
	•	Global backpressure

3. Scheduling
	•	Choose which GPU node gets request
	•	Respect region pinning
	•	Weigh load
	•	Respect reserved capacity CUs
	•	Retry/rebalance

4. Node Registry
	•	Keep track of live nodes
	•	GPU type, VRAM, throughput
	•	Model list
	•	Health checks
	•	Spot eviction notifications

5. Model Registry
	•	Supported models
	•	Model metadata (VRAM needs, speed)
	•	Region availability
	•	Pricing metadata

6. Region Routing
	•	Geographical routing
	•	Failover rules
	•	Sticky routing

7. Token Accounting & Usage Logging
	•	Count input tokens
	•	Count output tokens
	•	Store usage
	•	Store per-key counters

8. Billing Reporting
	•	Push usage → Stripe usage records
	•	Handle custom pricing
	•	Handle region-based pricing

9. Control Interfaces
	•	API Gateway → Control Plane
	•	Control Plane → GPU nodes
	•	Control Plane → Dashboard

10. Hybrid On-Prem Integration
	•	Node agent in enterprise DC
	•	Secure gRPC connection
	•	Node “belongs” to org
	•	No cross-org leakage

⸻

5. High-Level Architecture Diagram

                     ┌──────────────────────────────┐
                     │           API Gateway        │
                     │ (OpenAI-compatible wrapper)  │
                     └──────────────┬───────────────┘
                                    │
                      Validate Key  │  Apply Rate Limits
                                    ▼
                     ┌──────────────────────────────┐
                     │         Control Plane         │
                     │     (Go + Redis + PG)         │
                     └────────┬─────────┬────────────┘
                              │         │
                              │         │
         ┌────────────────────┘         └──────────────────────┐
         ▼                                                    ▼
┌─────────────────┐                                    ┌─────────────────┐
│  Node Registry   │  <--- gRPC health, metrics --->   │ Region Router   │
└──────────────────┘                                    └─────────────────┘
         │                                                    │
         ▼                                                    ▼
┌──────────────────┐                                ┌──────────────────┐
│    Scheduler      │ ---- choose best node ----->  │ Token Accountant │
└───────────────────┘                                └──────────────────┘
         │                                                    │
         ▼                                                    ▼
         ┌───────────────────────────────┐           ┌────────────────────┐
         │       GPU Worker Nodes        │           │ Stripe Billing API │
         │  (vLLM / SGLang, SkyPilot)    │           └────────────────────┘
         └───────────────────────────────┘


⸻

6. Control Plane Subsystems

The full architecture has 10 major subsystems, broken later into 30 detailed components.
	1.	Auth Service
	2.	API Key Resolver
	3.	Tenant Configuration Manager
	4.	Environment Manager
	5.	Rate Limiter Engine
	6.	Model Registry
	7.	Node Registry
	8.	Scheduler
	9.	Token Accountant
	10.	Billing Reporter

Each subsystem contains smaller modules — described in Parts 2–5.

⸻

7. Control Plane Deployment Model

As a solo founder, MVP deployment:

Run as four processes:
	1.	control-plane-api (HTTP)
	2.	scheduler-service (gRPC/HTTP)
	3.	billing-service (cron + Stripe)
	4.	node-agent (runs inside GPU worker VM)

All run on a single VM initially.

When scaling:
	•	Split them into separate microservices
	•	Add load balancers
	•	Add region-specific replicas
	•	Add HA Redis and HA Postgres

⸻

8. Data Stores

Redis

Used for:
	•	rate limiting
	•	sliding windows
	•	token buckets
	•	node heartbeats
	•	cached routing tables

Postgres

Used for:
	•	orgs
	•	environments
	•	API keys
	•	model catalog
	•	region catalog
	•	usage logs
	•	billing events

Prometheus

For monitoring GPU nodes + control-plane latency.

✅ CONTROL PLANE ARCHITECTURE DOCUMENT — PART 2/5

Identity, Multi-Tenancy, API Keys, Environments, Rate Limiting, Quotas

This section defines the tenant model and the traffic-control layer.
This is where 90% of inference platforms fail — so we design it the right way.

⸻

Section 9 — Identity & Tenant Resolution

When a request hits:

POST /v1/chat/completions
Authorization: Bearer <api_key>

The Control Plane must map:

api_key → user_id → env_id → org_id → region → model → permissions

This resolution must be:
	•	O(1) lookup (Redis-backed cache)
	•	Deterministic
	•	Isolated
	•	Immutable (keys do not change identity)

9.1 API Key Structure

A recommended API key format:

clsk_live_6f12b33c58b94429a7b123f33a891bc7

Internally map to:
	•	org_id
	•	env_id
	•	user_id
	•	region_id
	•	key permissions
	•	key status (active / suspended / deleted)

9.2 API Key Metadata Table (Postgres)

Field	Type	Description
key_id	uuid	primary key
key_hash	text	hashed key
org_id	uuid	owner org
env_id	uuid	environment
user_id	uuid	creator
region	text	preferred region
role	enum	admin / developer / read-only
rate_limit_tokens_per_min	int	optional override
status	enum	active/inactive/suspended
created_at	ts	-

9.3 Permission Model
	•	Keys can be restricted per environment
	•	Keys cannot cross org
	•	Keys cannot cross region unless allowed
	•	Admin can disable keys instantly

Caching logic:
	•	Cache key metadata in Redis with TTL=60s
	•	Invalidate on delete
	•	Fallback to Postgres

⸻

Section 10 — Organizations & Multi-Tenancy

An org is the top-level tenant.

10.1 Org Capabilities
	•	Choose regions
	•	Create environments
	•	Create API keys
	•	View usage
	•	Select models
	•	Reserved capacity plan
	•	Billing plan
	•	Disable/enable org

10.2 Org Table (Postgres)

Field	Type
org_id	uuid
name	text
status	active / suspended
billing_plan	enum
reserved_capacity_tokens_per_sec	int
region_preferences	jsonb
created_at	timestamp


⸻

Section 11 — Environments (dev / staging / prod)

Every org may have multiple environments.
Each environment acts like an isolated subtenant.

Why Envs?
	•	Separate API keys
	•	Separate quotas
	•	Separate analytics
	•	Separate regions
	•	Separate rate limits
	•	Enables CI/CD workflows

11.1 Env Table

Field	Type
env_id	uuid
org_id	uuid
name	text
region	text
model_list	jsonb
quota_tokens_per_day	int
concurrency_limit	int
created_at	timestamp


⸻

Section 12 — Region Affinity Logic

Each env picks its region:

India → Mumbai
US → Virginia
EU → Frankfurt
APAC → Singapore

Routing Rule:

if user specified region:
    use that
else:
    fallback to org-default region

Failover logic:
	•	If region down → fallback to secondary
	•	Sticky routing maintained via Redis

⸻

Section 13 — Multi-Tenant Isolation Requirements

The Control Plane must guarantee:

13.1 No tenant can see infrastructure metadata of another tenant
	•	No node IP exposure
	•	No performance leak
	•	No latency leak
	•	No error message leak
	•	Protect against prompt injection that reveals internal routing

13.2 No tenant can consume capacity of another

Enforced via:
	•	per-org rate limits
	•	per-env rate limits
	•	reserved token-per-second CUs

13.3 Strict model quotas
	•	Limit max tokens per request
	•	Limit max concurrency

⸻

Section 14 — Rate Limiting Architecture

4 Layers of Rate Limiting

This design is taken from Stripe-tier platforms.

⸻

14.1 Layer 1 — Global Rate Limit

Protects the whole system.

Redis keys:

global:tokens:minute


⸻

14.2 Layer 2 — Org-Level Limits

Configurable per org based on pricing plan.

Redis keys:

org:<org_id>:tokens:minute
org:<org_id>:tokens:second
org:<org_id>:concurrency


⸻

14.3 Layer 3 — Environment-Level Limits

Because dev/staging/prod must be isolated.

env:<env_id>:tokens:minute
env:<env_id>:concurrency


⸻

14.4 Layer 4 — API Key Limits

User-generated keys need their own limits.

key:<key_id>:tokens:minute
key:<key_id>:concurrency


⸻

Section 15 — Rate Limiting Mechanisms

Use Redis Token Bucket or Leaky Bucket algorithm.

15.1 Redis Scripts (Lua) Needed
	•	atomic token decrement
	•	atomic concurrency increment
	•	atomic sliding window
	•	atomic reset on window expiry

Why Lua?

Guarantees atomicity in Redis:
	•	No race conditions
	•	No double spending of tokens
	•	No request accepting while bucket is empty

⸻

Section 16 — Quota Management

Quotas define:
	•	max tokens/day
	•	max tokens/hour
	•	concurrency caps
	•	max request length
	•	max response length

Quota Violations

Return:

429 - Rate limit exceeded  

or

403 - Quota exceeded  


⸻

Section 17 — Reserved Capacity (Guaranteed Tokens/sec)

Reserved capacity = CUs (Capacity Units).

Example:

Customer X buys 15 tokens/sec CU for Llama 3 8B.

Control Plane Enforces:
	•	Guaranteed throughput
	•	No throttling on paid tenants
	•	Scheduler must route requests to nodes that have promised CUs

Implementation

Track in Redis:

reserved:org:<org_id>:tokens:second

Scheduler checks:
	•	reserved capacity remaining
	•	node capacity
	•	region availability

Reserved capacity tenants bypass global throttling.

⸻

Section 18 — Abuse Prevention

Abuse scenarios:
	•	Bot traffic
	•	Invalid payload spam
	•	1000 concurrent requests spike
	•	User tries to bypass rate limit with multiple keys

Controls:
	•	concurrency caps per key
	•	velocity caps
	•	fingerprinting (IP/device optional)
	•	strict payload validation

⸻

Section 19 — Key Rotation Flow

User rotates API key:
	•	Old key becomes invalid
	•	New key added to Redis
	•	Control Plane caches update
	•	No downtime

⸻

Section 20 — Suspension & Revocation

Admin can disable:
	•	org
	•	env
	•	key

Suspended tenants return:

403 - Tenant Suspended


✅ CONTROL PLANE ARCHITECTURE DOCUMENT — PART 3/5

Node Registry, GPU Node Lifecycle, Scheduler, Region Router, Spot Interruption Handling, Health Checks, Model Placement

This section describes the core intelligence of your inference platform — the part that makes CrossLogic.AI a real cloud platform rather than a thin wrapper around vLLM.

⸻

Section 21 — Node Registry (The Source of Truth for All GPU Nodes)

Every vLLM/SGLang GPU worker must register itself with the Control Plane.

Node Registry maintains a list of all active GPU nodes, their capabilities, and their health.

21.1 Node Metadata Stored

Attribute	Description
node_id	Unique UUID
region	India, US, EU, etc.
cloud	AWS, GCP, Azure, OCI, Yotta
spot	true/false
gpu_type	A100, L40S, H100, RTX4090
vram_total	e.g., 48 GB
vram_free	dynamic
throughput_tokens_per_sec	calculated
model_list	which models loaded
endpoint_url	RPC endpoint
tailnet_ip	internal IP on Tailscale
status	active / draining / dead
last_heartbeat_at	timestamp
created_at	timestamp

21.2 Node Registration Flow
	1.	SkyPilot launches VM
	2.	Cloud-init starts node agent
	3.	Agent installs vLLM/SGLang
	4.	Node joins Tailscale
	5.	Node agent sends registration RPC:

POST /control-plane/nodes/register

Payload includes:
	•	node hardware info
	•	model loaded
	•	RPC endpoint
	•	throughput benchmark
	•	GPU temperature metrics
	•	spot instance metadata

21.3 Node De-registration Flow

Occurs on:
	•	shutdown
	•	spot eviction
	•	health failures

Control Plane marks node → draining → dead.

⸻

Section 22 — Node Health Monitoring

The Control Plane continuously checks health.

22.1 Health Check Mechanism

Node agent exposes:

GET /healthz
GET /metrics

Control Plane checks:
	•	heartbeat every 5s
	•	full metrics every 15s

22.2 Mark Node as Unhealthy If:
	•	no response for 15 seconds
	•	GPU temperature > threshold
	•	VRAM fragmentation > 80%
	•	OOM events spike
	•	latency spikes > 3× baseline

22.3 Draining

When node is marked draining:
	•	new requests not scheduled
	•	existing requests finish
	•	node removed from active pool after TTL

⸻

Section 23 — Spot Interruption Detection & Recovery

Spot nodes will die suddenly. Recovery must be:
	•	fast
	•	correct
	•	zero customer-visible failure

23.1 Spot Interrupt Signals

Different clouds send:
	•	AWS: IMDS metadata “instance termination” notice
	•	GCP: shutdown signal
	•	Azure: Preemption notice

Node agent sends:

POST /control-plane/nodes/spot-warning

Control Plane actions:
	1.	Mark node as draining
	2.	Scheduler stops routing traffic to this node
	3.	Re-route pending requests
	4.	Launch new SkyPilot node (optional auto-scale)

23.2 Spot Eviction Immediate Termination

If node dies without warning:
	•	heartbeat timeout
	•	mark dead
	•	scheduler flushes node from pool

Control Plane must assume:
Nodes are unreliable — design accordingly.

⸻

Section 24 — Model Placement Strategy

Each GPU node runs one model at a time in vLLM (best for performance & simplicity).

24.1 Node Startup Logic

Node agent:
	•	loads model from remote storage (S3/GCS/Azure Blob)
	•	warms up prefill and decode kernels
	•	runs benchmark (prefill TPS, decode TPS)
	•	reports capacity to Control Plane

24.2 Multi-Model Support

For MVP:
	•	Single model per node
	•	Control Plane partitions nodes by model

For future:
	•	Add separate vLLM processes (multi-model node)
	•	Requires careful VRAM isolation
	•	Not recommended for v1

⸻

Section 25 — Scheduler Overview

THE most important component.

The Scheduler decides:
	•	which GPU node handles which request
	•	how to balance load
	•	how to enforce reserved capacity
	•	how to ensure region affinity
	•	how to recover from node failures

25.1 Inputs to Scheduler

Input	Source
org_id	API Gateway
env_id	API Gateway
region	env config
model	request
tokens_requested	request
key concurrency	rate limiter
node pool	Node Registry
reserved CUs	Billing Plan

25.2 Output

A single GPU node endpoint:

node.tailnet.ip:port


⸻

Section 26 — Scheduler Algorithm (Token-Aware Routing)

Your platform needs two scheduling paths:

⸻

26.1 Path A — Serverless Scheduling (Most Users)

Algorithm:
	1.	Get node pool for (region + model)
	2.	Filter nodes: status = active
	3.	Sort by (load / capacity)
	4.	Pick least loaded
	5.	Apply token bucket enforcement
	6.	Route request

This balances load while respecting quotas.

⸻

26.2 Path B — Reserved Capacity Scheduling

Reserved tenants must always get:
	•	guaranteed tokens/sec
	•	guaranteed serving capacity

Algorithm:
	1.	Check tenant reserved CUs
	2.	Ensure upcoming request fits within CU
	3.	Update reserved capacity window
	4.	Select nodes dedicated to that org (if applicable)
	5.	If shared pool → consider model-specific lanes
	6.	Route to node even if overall system is overloaded

Reserved > Serverless.

⸻

26.3 Load Calculation

Node load score:

score = (current_tokens_per_sec / max_tokens_per_sec)

Sort ascending → lower score = less busy.

⸻

26.4 Failover Logic

If node fails:
	1.	Remove node from pool
	2.	Retry scheduling (max 3 tries)
	3.	If region empty → fallback to nearest region
	4.	Mark request as failover-invoked

Failover safety throttle

If cloud region down → route 20% of traffic to closest region
Prevents global overload.

⸻

Section 27 — Region Router

This component makes sure that requests go to the correct geography.

27.1 Routing Modes
	1.	Strict Region Pinning (preferred)
	•	dev/staging/prod all pinned to region
	•	consistent latency
	•	predictable billing
	2.	Latency-Based Routing (future)
	•	measure ping to regions
	•	select closest region
	•	limited usefulness for India use case

27.2 Region Table

Stores:
	•	countries supported
	•	cities
	•	cloud providers per region
	•	cost multipliers
	•	node pools

27.3 Fallback Rules

India → Singapore → Europe → USA
Europe → USA → Singapore
USA → Europe

Always route back to preferred region when recovered.

⸻

Section 28 — Load Shedding

When the system is overloaded:
	•	global limit reached
	•	region CPU/GPU saturated
	•	network congestion
	•	node fragmentation

Control Plane returns:

429 Too Busy

Only serverless traffic gets shed.
Reserved capacity always survives.

⸻

Section 29 — Node-Agent → Control Plane Communication (gRPC)

Define gRPC services:

NodeAgent.RegisterNode()
	•	registers metadata

NodeAgent.SendHeartbeat()
	•	every 5s

NodeAgent.SendMetrics()
	•	GPU load
	•	VRAM fragmentation
	•	throughput

NodeAgent.NotifySpotEviction()
	•	preemption warning

NodeAgent.Shutdown()
	•	graceful drain

All traffic is mutual TLS to prevent MITM in Tailscale.

⸻

Section 30 — Model Version Upgrades (No Version Pinning)

Your requirement:

“No model version pinning, I will upgrade backend models.”

Model Upgrade Flow
	1.	Deploy new model version to storage
	2.	Launch N new SkyPilot nodes
	3.	Register new nodes
	4.	Drain old nodes
	5.	Disable routing to old nodes
	6.	Delete old nodes

This is blue/green deployment for inference.

Tenants experience:
	•	zero downtime
	•	no version selection
	•	consistent state

⸻

Section 31 — Node Draining Logic

When node marked draining:
	•	scheduler stops assigning traffic
	•	running requests finish
	•	after TTL node is removed

Useful for:
	•	spot eviction
	•	model upgrades
	•	maintenance
	•	performance degradation

⸻

✅ CONTROL PLANE ARCHITECTURE DOCUMENT — PART 4/5

Token Accounting, Usage Logging, Billing Engine, Stripe Integration, Aggregation Jobs, Pricing Model, Region Multipliers

This section defines the full Billing & Usage pipeline, from token counting → metering → billing → invoicing.

This is one of the most critical components of the system — if billing is wrong, nothing else matters.

⸻

Section 32 — Token Accounting (Real-Time Token Metering)

The Control Plane must compute tokens per request, per key, per env, per org.

32.1 Token Types
	•	prompt_tokens (input)
	•	completion_tokens (output)
	•	total_tokens (sum)

32.2 Token Counting Flow
	1.	Request arrives
	2.	API Gateway extracts prompt → validates
	3.	Scheduler routes request
	4.	Node executes inference
	5.	Node sends back token usage metadata (from vLLM/SGLang)
	6.	Control Plane increments:
	•	API key counters
	•	Environment counters
	•	Org counters
	•	Global counters
	7.	Logs usage to Postgres

Token counting MUST be:
	•	Accurate
	•	Atomic
	•	Streaming-aware

⸻

Section 33 — Real-Time Token Counters (Redis)

To enforce limits, Redis stores:

Per-Key Counters

key:<key_id>:tokens:minute
key:<key_id>:tokens:day
key:<key_id>:concurrency

Per-Env Counters

env:<env_id>:tokens:minute
env:<env_id>:tokens:day

Per-Org Counters

org:<org_id>:tokens:minute
org:<org_id>:tokens:day
org:<org_id>:reserved_tps

Global Counters

global:tokens:minute

Why Redis?
	•	Millisecond latency
	•	Atomic Lua scripts
	•	TTL windows
	•	Handles sliding counters with no DB load

⸻

Section 34 — Usage Logging (Long-Term Storage in Postgres)

Every request generates a Usage Record in Postgres.

Field	Type	Description
usage_id	uuid	primary key
timestamp	ts	time of request
org_id	uuid	tenant
env_id	uuid	environment
key_id	uuid	API key
region	text	region served
model	text	model used
prompt_tokens	int	input tokens
completion_tokens	int	output tokens
total_tokens	int	total
latency_ms	int	p50 measurement
cost_microdollars	bigint	cost * 1e6
billed	bool	has been exported to Stripe?

34.1 Why log to Postgres?
	•	Reliability
	•	Auditing
	•	Dispute resolution
	•	Replay protection
	•	Regulatory requirements
	•	Enterprise customer transparency

⸻

Section 35 — The Billing Engine (Core Component)

Billing Engine = the system that converts usage → revenue.

It has 5 responsibilities:

⸻

35.1 Responsibility 1 — Price Lookup

Every token must be priced by:
	•	model
	•	region
	•	pricing tier
	•	reserved capacity plan
	•	discount plan

Price Table Example:

Model	Input ($/1M)	Output ($/1M)	Region Multiplier
Llama 3 8B	$0.05	$0.05	India 0.7x
Llama 3 70B	$0.60	$0.60	India 0.7x
Qwen 2.5	$0.04	$0.04	India 0.7x
Gemma 7B	$0.03	$0.03	India 0.7x

35.1.1 Region Multipliers

You said:

“Yes, custom pricing per region.”

Example:

USA = 1.0x
EU = 1.1x
India = 0.7x
APAC = 0.9x

Control Plane computes:

effective_price = base_price * region_multiplier


⸻

35.2 Responsibility 2 — Apply Free Tier

Example:
	•	25k tokens free per month per org
	•	Only for serverless plan
	•	Not applied to reserved capacity tenants

⸻

35.3 Responsibility 3 — Compute Cost

cost = total_tokens * (price_per_token / 1e6)

Pricing may differ for:
	•	embeddings vs completions
	•	model category
	•	fine-grained model sizes

⸻

35.4 Responsibility 4 — Export to Stripe (Metered Billing)

Stripe provides usage-based billing via:
Usage Records API

35.4.1 Aggregation Interval

You should push usage every:
	•	1 minute (recommended)
	•	or 5-minute buckets

35.4.2 Stripe Usage Record Payload

{
  "timestamp": 1736947200,
  "quantity": 2048,
  "action": "increment"
}

35.4.3 Stripe Subscription Item

Each org has:
	•	one Stripe customer
	•	recurring subscription
	•	multiple “usage meters” per model-category

⸻

35.5 Responsibility 5 — Reserved Capacity Billing

Capacity Units (CU):

1 CU = 1 token/sec guaranteed

Billing:
	•	Monthly subscription
	•	Flat rate
	•	Token usage still tracked but not billed
	•	Overages optionally billed
	•	Scheduler guarantees capacity

⸻

Section 36 — Billing Reporter (Background Job)

A cron-like job (Go + goroutines):

Runs every:
	•	1 minute
	•	5 minute
	•	1 hour
	•	daily

Tasks:
	1.	Read unbilled usage rows from Postgres
	2.	Group by org, env, model
	3.	Compute totals
	4.	Convert to Stripe usage records
	5.	Mark usage as billed
	6.	Retry failed exports

⸻

Section 37 — Billing Error Handling

Failures with Stripe must not break user experience.

Retry Strategy:
	•	exponential backoff
	•	durable queue
	•	local fallback logs

Mark records:

billed = false
billing_failed = true
retry_count = X

Retries every 15 minutes.

⸻

Section 38 — Reconciliation Engine

To guarantee 99.999% billing accuracy, you need:
	1.	Daily rollback audit
	2.	Compare Stripe totals vs internal totals
	3.	Regenerate missing usage
	4.	Operator dashboard for disputes

⸻

Section 39 — Metric Export (Billing + Monitoring)

Expose Prometheus metrics:

billing_tokens_total
billing_cost_total
billing_records_pushed_total
billing_records_failed_total

These metrics help:
	•	debugging
	•	customer support
	•	anomaly detection

⸻

Section 40 — Customer Invoices

Stripe handles invoice generation.

Control Plane attaches:
	•	usage per model
	•	usage per region
	•	reserved CU charges
	•	discounts
	•	taxes (GST for India)

⸻

Section 41 — Customer Billing UI

On dashboard:
	•	Usage graph
	•	Tokens consumed
	•	Cost estimate
	•	Free tier remaining
	•	Reserved CU info
	•	Model-wise breakdown
	•	Region-wise breakdown

⸻

Section 42 — Cost Guardrails

Protect yourself from:
	•	DDoS
	•	runaway scripts
	•	malicious users
	•	infinite loops

Guardrails:
	•	request payload validation
	•	max tokens per request
	•	max requests/min
	•	max RPS per key
	•	maximum monthly bill cap
	•	auto-suspend over-limit tenants

⸻

Section 43 — Internal Cost Model (Your Profitability)

As founder, you need:
	•	cost per GPU per hour (spot/on-demand)
	•	cost per token produced
	•	margin per model
	•	revenue per region

Build:
	•	FinOps module
	•	Internal profit estimator

⸻

✅ CONTROL PLANE ARCHITECTURE DOCUMENT — PART 5/5

On-Prem Architecture, Hybrid Nodes, Security, Observability, Deployment Strategy, MVP Scope, Future Evolution & Founder Playbook

This final section completes the Control Plane architecture with enterprise-grade features, operational considerations, and roadmap guidance.

⸻

Section 44 — On-Prem Enterprise Deployment Model (Hybrid Cloud)

Enterprise customers want:
	1.	Local GPU inference inside their data centers
	2.	Cloud-hosted control plane (managed by you)
	3.	Zero trust boundary between orgs
	4.	Full audit + SSO
	5.	Air-gapped mode (for banks, telcos, government)

Your design must satisfy all scenarios.

⸻

44.1 Hybrid Mode (Recommended Default)

Control Plane: Cloud

Runs on your infrastructure (AWS/GCP/Render/Fly.io).

Data Plane: On-Prem

GPU nodes run inside enterprise DC.

Communication: Secure gRPC
	•	TLS mutual auth
	•	Each enterprise node gets org-scoped certificate
	•	Node can only act for its org

Flow
	1.	Enterprise deploys “node agent” using your installer
	2.	Agent registers with control plane using org-scoped API key
	3.	Control plane schedules only their org’s requests onto their nodes
	4.	Usage metering and billing still performed in cloud
	5.	Latency extremely low inside enterprise network

This is the enterprise monetization engine.

⸻

44.2 Full Air-Gap Mode (Regulated Industries)

For banks, defense, etc.

Requirements:
	•	No external network calls
	•	No cloud backend
	•	All components must run locally
	•	Billing optional (license-based)

Architecture:
	•	Self-hosted Control Plane
	•	Local Redis
	•	Local Postgres
	•	Local Dashboard
	•	Local Billing (flat license)
	•	No Stripe

This will be needed later for Fortune 500 deals.

⸻

Section 45 — Node Agent (On-Prem + Cloud) Responsibilities

Each GPU worker runs your Node Agent (Go binary).

Responsibilities:
	1.	Node registration
	2.	Heartbeats
	3.	Metrics reporting
	4.	Spot interruption reporting (cloud mode)
	5.	Secure gRPC
	6.	Model loading
	7.	Auto-upgrade agent
	8.	Auto-restart vLLM/SGLang runtime
	9.	Disk cleanup
	10.	Log streaming to control plane

In enterprise:
	•	No spot events
	•	No SkyPilot
	•	Static node pool

⸻

Section 46 — Security Architecture

Security is non-negotiable.
Your system touches customer data and may sit inside enterprise data centers.

⸻

46.1 Authentication
	•	API keys (hashed)
	•	Service tokens
	•	Node certificates
	•	Admin SSO (Google or Azure AD)

⸻

46.2 Authorization
	•	Org isolation enforced at key-level
	•	Node may only serve requests for org it belongs to (on-prem)
	•	Internal RBAC for employees

⸻

46.3 Network Security
	•	Tailscale Mesh (cloud workers)
	•	mTLS for all node-agent calls
	•	API Gateway rate limits
	•	Disable public node ports
	•	No inbound connections to workers

⸻

46.4 Data Security
	•	Encrypt all DB fields containing keys
	•	Store hashed keys (never plaintext)
	•	Audit logs for all admin actions

⸻

Section 47 — Observability & Monitoring

The platform MUST be observable.

Use:
	•	Prometheus for metrics
	•	Grafana dashboards
	•	Loki for logs
	•	AlertManager for alerts

⸻

47.1 Metrics to Collect

Control Plane
	•	scheduler latency
	•	routing decisions
	•	rejected requests
	•	rate limiting events
	•	billing exports
	•	token counters

GPU Nodes
	•	GPU temperature
	•	VRAM free/used
	•	VRAM fragmentation
	•	token throughput
	•	model load time
	•	spot events

⸻

47.2 Dashboards

1. Region Overview
	•	node count
	•	active capacity
	•	latency distribution

2. Model Performance
	•	TPS (tokens/sec) per model
	•	cost per region
	•	GPU utilization graphed

3. Billing Overview
	•	token usage per org
	•	billed vs unbilled
	•	Stripe integration health

4. Reliability Dashboard
	•	error rate
	•	failovers
	•	node churn

⸻

Section 48 — Deployment Strategy (For You As Solo Founder)

You asked for a version that is easy to build solo, yet can evolve into a planet-scale inference platform.

Here’s exactly what you should do:

⸻

48.1 MVP Control Plane (Weeks 1–3)

Run all components on one VM:
	•	Go API Gateway wrapper
	•	Scheduler
	•	Node Registry
	•	Token Accounting
	•	Billing Reporter
	•	Postgres
	•	Redis

Region support:
	•	Only India for week 1
	•	Add Singapore in week 2
	•	Add US in week 3

GPU nodes:
	•	SkyPilot on AWS/GCP spot

Monitoring:
	•	Basic Prometheus
	•	Basic Grafana

Billing:
	•	Single Stripe meter
	•	No discounting initially

Perfect for first 50–100 customers.

⸻

48.2 Post-MVP (Weeks 4–12)

Split into separate services:
	1.	control-plane-api
	2.	scheduler
	3.	billing-engine
	4.	node-registry-service
	5.	on-prem agent

Add:
	•	multi-region deployment
	•	region failover
	•	rate limit tiers
	•	model catalog UI

⸻

48.3 Scaling Stage (After ~500 customers)

Add:
	•	region-specific control plane replicas
	•	HA Redis + HA Postgres
	•	on-prem operator
	•	internal FinOps dashboards
	•	full observability stack

⸻

Section 49 — Future Evolution of Control Plane

49.1 Distributed Scheduler

Move from a single scheduler to:
	•	shard per region
	•	consensus via etcd or memberlist

49.2 Model Auto-Migration

AI-driven:
	•	detect overloaded regions
	•	move GPU nodes accordingly

49.3 Support Distributed Inference

For >70B models:
	•	Exo
	•	Tensor parallelism
	•	pipeline parallelism
(This is future, not MVP.)

49.4 Intelligent Cost Optimization
	•	choose cheapest cloud at moment
	•	GPU arbitrage
	•	predictive spot capacity

⸻

Section 50 — Founder Playbook (Strategic)

You must build this platform alone initially. Here is the exact strategic plan.

⸻

50.1 Focus Only On:
	•	API Gateway wrapper
	•	Control Plane core (scheduler + node registry + token accounting)
	•	Billing
	•	Dashboard
	•	SkyPilot integration
	•	India regions
	•	2–3 models (8B + 70B + embed)

Nothing else.


⸻

50.3 GO-BASED SERVICES TO BUILD FIRST

control-plane-api
scheduler-service
token-accountant
billing-reporter
node-agent (binary)



⸻

Section 51 — Risks & Mitigations

Risk 1: Spot node churn breaks latency

Mitigation:
	•	hybrid nodes (on-demand + spot)
	•	draining mechanism
	•	scheduler failover

Risk 2: Billing inaccuracy

Mitigation:
	•	two-phase accounting
	•	reconciliation engine
	•	Stripe retry logic

Risk 3: Overload in single region

Mitigation:
	•	region multipliers
	•	horizontal burst pool
	•	overflow routing

Risk 4: DevOps complexity

Mitigation:
	•	use SkyPilot
	•	simple deployment first
	•	automate later

⸻

Section 52 — Final Checklist (Production Ready)

MUST HAVE

✔ Control Plane (Go)
✔ Node Agent (Go)
✔ Redis rate limiting
✔ Postgres usage ledger
✔ Stripe metering
✔ Tailscale networking
✔ vLLM/SGLang workers
✔ SkyPilot orchestration
✔ Multi-region routing
✔ Developer dashboard


