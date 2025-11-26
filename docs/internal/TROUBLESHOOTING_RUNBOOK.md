# CrossLogic AI IaaS - Troubleshooting Runbook

**CONFIDENTIAL - Internal Use Only**

This runbook provides step-by-step procedures for diagnosing and resolving common issues.

---

## Quick Reference

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| 401 errors | Invalid API key | Check key status in DB |
| 429 errors | Rate limit exceeded | Check Redis rate limit keys |
| 502/503 errors | No healthy nodes | Check node status, launch new nodes |
| High latency | Overloaded nodes | Scale up, check load balancer |
| Missing usage data | Billing job failed | Check billing service logs |

---

## 1. Authentication Issues

### 1.1 Customer Reports "Invalid API Key" (401)

**Diagnosis:**

```bash
# Check if key exists in database
psql -d crosslogic -c "
  SELECT id, key_prefix, status, tenant_id, expires_at
  FROM api_keys
  WHERE key_prefix = 'clsk_live_xx'  -- First 12 chars of customer's key
"

# Check tenant status
psql -d crosslogic -c "
  SELECT id, name, status
  FROM tenants
  WHERE id = 'tenant-uuid-here'
"

# Check environment status
psql -d crosslogic -c "
  SELECT id, name, status
  FROM environments
  WHERE tenant_id = 'tenant-uuid-here'
"
```

**Common Causes:**
1. Key was revoked (`status != 'active'`)
2. Key expired (`expires_at < NOW()`)
3. Tenant suspended (`tenants.status != 'active'`)
4. Environment disabled (`environments.status != 'active'`)

**Resolution:**

```bash
# Reactivate key
psql -d crosslogic -c "
  UPDATE api_keys SET status = 'active' WHERE id = 'key-uuid'
"

# Reactivate tenant
psql -d crosslogic -c "
  UPDATE tenants SET status = 'active' WHERE id = 'tenant-uuid'
"

# Clear cache to force re-validation
redis-cli DEL "api_key:$(echo -n 'full-api-key' | sha256sum | cut -d' ' -f1)"
```

### 1.2 Admin Token Not Working

**Diagnosis:**

```bash
# Check configured admin token
grep ADMIN_API_TOKEN /etc/crosslogic/.env

# Test with curl
curl -v -H "X-Admin-Token: YOUR_TOKEN" http://localhost:8080/admin/nodes
```

**Resolution:**
- Verify token in environment matches what's being sent
- Check for whitespace in token
- Restart control plane if token was changed

---

## 2. Rate Limiting Issues

### 2.1 Customer Reports 429 Errors

**Diagnosis:**

```bash
# Check current rate limit counters in Redis
redis-cli

# Per-key counters
KEYS "ratelimit:key:*"
GET "ratelimit:key:{key-id}:minute:2024-01-15T10:30"
GET "ratelimit:key:{key-id}:concurrency"

# Per-environment counters
GET "ratelimit:env:{env-id}:minute:2024-01-15T10:30"

# Per-tenant counters
GET "ratelimit:tenant:{tenant-id}:minute:2024-01-15T10:30"
```

**Check configured limits:**

```bash
psql -d crosslogic -c "
  SELECT
    k.id,
    k.key_prefix,
    k.rate_limit_requests_per_min,
    k.rate_limit_tokens_per_min,
    k.concurrency_limit
  FROM api_keys k
  WHERE k.key_prefix = 'clsk_live_xx'
"
```

**Resolution:**

```bash
# Increase key rate limit
psql -d crosslogic -c "
  UPDATE api_keys
  SET rate_limit_requests_per_min = 120
  WHERE id = 'key-uuid'
"

# Clear rate limit cache (temporary relief)
redis-cli DEL "ratelimit:key:{key-id}:minute:$(date +%Y-%m-%dT%H:%M)"

# Clear concurrency counter
redis-cli DEL "ratelimit:key:{key-id}:concurrency"
```

---

## 3. Node/Inference Issues

### 3.1 No Healthy Nodes Available (503)

**Diagnosis:**

```bash
# Check node status
psql -d crosslogic -c "
  SELECT
    id, cluster_name, provider, status, health_score,
    model_name, endpoint_url, last_heartbeat_at
  FROM nodes
  WHERE status IN ('active', 'draining')
  ORDER BY created_at DESC
  LIMIT 20
"

# Check for stale heartbeats
psql -d crosslogic -c "
  SELECT id, cluster_name, status, last_heartbeat_at
  FROM nodes
  WHERE status = 'active'
    AND last_heartbeat_at < NOW() - INTERVAL '2 minutes'
"
```

**Check logs:**

```bash
# Control plane logs
kubectl logs -l app=control-plane --tail=100 | grep -i "node\|heartbeat\|health"

# Check specific node agent (if accessible)
ssh node-ip "journalctl -u node-agent --since '1 hour ago'"
```

**Resolution:**

```bash
# Mark stale nodes as draining
psql -d crosslogic -c "
  UPDATE nodes
  SET status = 'draining', status_message = 'stale_heartbeat'
  WHERE status = 'active'
    AND last_heartbeat_at < NOW() - INTERVAL '2 minutes'
"

# Launch new node manually
curl -X POST http://localhost:8080/admin/nodes/launch \
  -H "X-Admin-Token: TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "meta-llama/Llama-3.1-8B-Instruct",
    "provider": "aws",
    "region": "us-east-1",
    "instance_type": "g5.2xlarge",
    "spot": true
  }'
```

### 3.2 Node Registration Failing

**Diagnosis:**

```bash
# Check node agent logs
ssh node-ip "journalctl -u node-agent --since '30 minutes ago'"

# Check control plane for registration attempts
kubectl logs -l app=control-plane --tail=100 | grep "register"

# Verify network connectivity
ssh node-ip "curl -v http://control-plane:8080/health"
```

**Common Causes:**
1. Network connectivity issues
2. Control plane not reachable
3. Invalid registration payload
4. Node agent crash

**Resolution:**

```bash
# Restart node agent
ssh node-ip "systemctl restart node-agent"

# Force re-registration
ssh node-ip "rm /var/lib/node-agent/registered && systemctl restart node-agent"
```

### 3.3 High Inference Latency

**Diagnosis:**

```bash
# Check Prometheus metrics
curl -s http://localhost:9090/api/v1/query?query=loadbalancer_latency_seconds

# Check node health scores
psql -d crosslogic -c "
  SELECT cluster_name, health_score, model_name
  FROM nodes WHERE status = 'active'
  ORDER BY health_score ASC
"

# Check load distribution
curl -s http://localhost:9090/api/v1/query?query=loadbalancer_requests_total
```

**Resolution:**

```bash
# Scale up by launching more nodes
# Or switch to least-latency routing strategy

# Check if specific node is slow
ssh node-ip "nvidia-smi"  # Check GPU utilization
ssh node-ip "curl localhost:8000/health"  # Check vLLM health
```

---

## 4. Database Issues

### 4.1 Connection Pool Exhausted

**Symptoms:**
- "connection pool exhausted" errors in logs
- Requests timing out

**Diagnosis:**

```bash
# Check current connections
psql -d crosslogic -c "
  SELECT count(*) as connections,
         state,
         wait_event_type
  FROM pg_stat_activity
  WHERE datname = 'crosslogic'
  GROUP BY state, wait_event_type
"

# Check for long-running queries
psql -d crosslogic -c "
  SELECT pid, now() - pg_stat_activity.query_start AS duration, query
  FROM pg_stat_activity
  WHERE state = 'active'
    AND now() - pg_stat_activity.query_start > interval '1 minute'
"
```

**Resolution:**

```bash
# Kill long-running queries
psql -d crosslogic -c "SELECT pg_terminate_backend(PID)"

# Increase pool size (requires restart)
# Update DB_MAX_CONNECTIONS in environment

# Emergency: restart control plane
kubectl rollout restart deployment/control-plane
```

### 4.2 Slow Queries

**Diagnosis:**

```bash
# Enable slow query logging (temporarily)
psql -d crosslogic -c "SET log_min_duration_statement = 1000;"

# Check for missing indexes
psql -d crosslogic -c "
  SELECT schemaname, tablename, indexname
  FROM pg_indexes
  WHERE tablename IN ('api_keys', 'usage_records', 'nodes')
"
```

**Resolution:**

```bash
# Add missing indexes
psql -d crosslogic -c "
  CREATE INDEX CONCURRENTLY idx_usage_tenant_time
  ON usage_records(tenant_id, timestamp)
"

# Vacuum and analyze
psql -d crosslogic -c "VACUUM ANALYZE usage_records"
```

---

## 5. Redis Issues

### 5.1 Redis Connection Failures

**Diagnosis:**

```bash
# Check Redis status
redis-cli ping

# Check memory usage
redis-cli info memory

# Check connected clients
redis-cli info clients
```

**Resolution:**

```bash
# If memory exhausted
redis-cli FLUSHDB  # WARNING: Clears all rate limit state

# Restart Redis
kubectl rollout restart deployment/redis
```

### 5.2 Rate Limit Data Corruption

**Symptoms:**
- Incorrect rate limit values
- Negative counters

**Resolution:**

```bash
# Clear rate limit keys for specific key
redis-cli KEYS "ratelimit:key:{key-id}:*" | xargs redis-cli DEL

# Clear all rate limit keys (emergency)
redis-cli KEYS "ratelimit:*" | xargs redis-cli DEL
```

---

## 6. Billing Issues

### 6.1 Usage Records Not Being Created

**Diagnosis:**

```bash
# Check recent usage records
psql -d crosslogic -c "
  SELECT * FROM usage_records
  ORDER BY timestamp DESC
  LIMIT 10
"

# Check for errors in logs
kubectl logs -l app=control-plane --tail=100 | grep -i "usage\|billing"
```

**Resolution:**

```bash
# Check async worker is running
# Usage recording is async - check for goroutine issues

# Manual backfill (if needed)
psql -d crosslogic -c "
  INSERT INTO usage_records (...)
  SELECT ... FROM request_logs WHERE ...
"
```

### 6.2 Stripe Integration Failures

**Diagnosis:**

```bash
# Check Stripe webhook events
psql -d crosslogic -c "
  SELECT * FROM billing_events
  ORDER BY created_at DESC
  LIMIT 10
"

# Check webhook signature
# Verify STRIPE_WEBHOOK_SECRET is correct

# Test Stripe API
curl -u sk_live_xxx: https://api.stripe.com/v1/customers
```

**Resolution:**

```bash
# Re-process failed billing events
psql -d crosslogic -c "
  UPDATE billing_events
  SET processed = false, retry_count = 0
  WHERE processed = false AND error IS NOT NULL
"

# Manually push usage to Stripe
curl -X POST https://api.stripe.com/v1/subscription_items/{si_xxx}/usage_records \
  -u sk_live_xxx: \
  -d quantity=1000 \
  -d timestamp=$(date +%s)
```

---

## 7. Deployment Issues

### 7.1 Control Plane Not Starting

**Diagnosis:**

```bash
# Check pod status
kubectl get pods -l app=control-plane

# Check pod events
kubectl describe pod control-plane-xxx

# Check logs
kubectl logs control-plane-xxx --previous
```

**Common Causes:**
1. Database connection failure
2. Redis connection failure
3. Missing environment variables
4. Invalid configuration

**Resolution:**

```bash
# Check secrets exist
kubectl get secrets

# Verify environment variables
kubectl exec control-plane-xxx -- env | grep -E "DATABASE|REDIS"

# Force restart
kubectl delete pod control-plane-xxx
```

### 7.2 Node Agent Not Starting

**Diagnosis:**

```bash
# SSH to node
ssh node-ip

# Check service status
systemctl status node-agent

# Check logs
journalctl -u node-agent -f
```

**Resolution:**

```bash
# Reinstall node agent
curl -fsSL https://install.crosslogic.ai/node-agent.sh | sudo bash

# Check vLLM is running
systemctl status vllm
docker logs vllm-container
```

---

## 8. Emergency Procedures

### 8.1 Complete Service Outage

**Checklist:**

1. [ ] Check load balancer health: `curl -v https://api.crosslogic.ai/health`
2. [ ] Check control plane pods: `kubectl get pods -l app=control-plane`
3. [ ] Check database: `psql -d crosslogic -c "SELECT 1"`
4. [ ] Check Redis: `redis-cli ping`
5. [ ] Check DNS: `dig api.crosslogic.ai`
6. [ ] Check TLS certificate: `openssl s_client -connect api.crosslogic.ai:443`

**Recovery Steps:**

```bash
# Step 1: Restore database connectivity
kubectl rollout restart deployment/postgres

# Step 2: Restore Redis
kubectl rollout restart deployment/redis

# Step 3: Restart control plane
kubectl rollout restart deployment/control-plane

# Step 4: Verify nodes
psql -d crosslogic -c "SELECT * FROM nodes WHERE status = 'active'"

# Step 5: Launch emergency nodes if needed
for i in 1 2 3; do
  curl -X POST http://localhost:8080/admin/nodes/launch \
    -H "X-Admin-Token: TOKEN" \
    -d '{"model_name": "meta-llama/Llama-3.1-8B-Instruct", "spot": false}'
done
```

### 8.2 Data Recovery

**PostgreSQL Recovery:**

```bash
# Find latest backup
aws s3 ls s3://crosslogic-backups/postgres/

# Download backup
aws s3 cp s3://crosslogic-backups/postgres/backup-2024-01-15.sql.gz .

# Restore
gunzip backup-2024-01-15.sql.gz
psql -d crosslogic < backup-2024-01-15.sql
```

### 8.3 Rollback Deployment

```bash
# View deployment history
kubectl rollout history deployment/control-plane

# Rollback to previous version
kubectl rollout undo deployment/control-plane

# Rollback to specific version
kubectl rollout undo deployment/control-plane --to-revision=5
```

---

## 9. Monitoring Commands

### Quick Health Check

```bash
#!/bin/bash
echo "=== API Health ==="
curl -s https://api.crosslogic.ai/health | jq .

echo "=== Control Plane Pods ==="
kubectl get pods -l app=control-plane

echo "=== Active Nodes ==="
psql -d crosslogic -c "SELECT COUNT(*) FROM nodes WHERE status = 'active'"

echo "=== Recent Errors ==="
kubectl logs -l app=control-plane --since=5m | grep -i error | tail -10

echo "=== Redis Memory ==="
redis-cli info memory | grep used_memory_human

echo "=== DB Connections ==="
psql -d crosslogic -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'crosslogic'"
```

### Performance Dashboard

```bash
# Watch request rate
watch -n 5 'curl -s localhost:9090/metrics | grep http_requests_total'

# Watch node health
watch -n 10 'psql -d crosslogic -c "SELECT cluster_name, health_score FROM nodes WHERE status = '\''active'\''"'

# Watch rate limits
watch -n 5 'redis-cli KEYS "ratelimit:*" | wc -l'
```

---

## 10. Contact Escalation

| Issue Type | First Contact | Escalation |
|------------|---------------|------------|
| Infrastructure | On-call SRE | Platform Lead |
| Database | DBA on-call | Database Lead |
| Billing | Billing team | Finance Lead |
| Security | Security on-call | Security Lead |
| Customer-facing | Support team | Customer Success |

---

*Last Updated: January 2024*
*Document Owner: Platform Engineering Team*
