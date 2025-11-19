# Database Setup Guide

This guide explains how to set up the PostgreSQL database for CrossLogic AI IaaS.

## Prerequisites

- PostgreSQL 14+ installed and running
- Database user with CREATE DATABASE privileges
- `psql` command-line tool available

## Quick Start

### 1. Create Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database and user
CREATE DATABASE crosslogic_iaas;
CREATE USER crosslogic WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE crosslogic_iaas TO crosslogic;
\q
```

### 2. Run Migrations

Execute the schema files in order:

```bash
# Set connection string
export DATABASE_URL="postgresql://crosslogic:your_secure_password@localhost:5432/crosslogic_iaas"

# Apply migrations in order
psql $DATABASE_URL -f database/schemas/01_core_tables.sql
psql $DATABASE_URL -f database/schemas/02_deployments.sql
psql $DATABASE_URL -f database/schemas/03_notifications.sql
```

Alternatively, use the migration script:

```bash
cd database
./migrate.sh
```

### 3. Verify Installation

```bash
psql $DATABASE_URL -c "\dt"  # List all tables
psql $DATABASE_URL -c "SELECT * FROM deployments LIMIT 1;"  # Test sample data
```

## Schema Overview

### 01_core_tables.sql
Core platform tables including:
- `tenants` - Multi-tenant organizations
- `environments` - Dev/staging/prod per tenant
- `api_keys` - Authentication and rate limiting
- `regions` - Available cloud regions
- `models` - LLM model catalog
- `nodes` - GPU node registry
- `usage_records` - Token usage tracking
- `billing_events` - Stripe integration

### 02_deployments.sql
Deployment management for 1:1 cluster-node architecture:
- `deployments` - Managed model deployments with auto-scaling
- `nodes` table updates - Adds cluster_name, model_name, deployment_id
- `deployment_nodes` - Junction table for node membership

Key features:
- **1 Cluster = 1 Node** architecture enforced
- **Auto-scaling** based on min/max replicas
- **Deployment strategies**: spread (multi-region) or packed (same region)
- **Automatic GPU selection** when gpu_type='auto'

### 03_notifications.sql
Event notification system:
- `notification_deliveries` - Webhook delivery tracking with retries
- `notification_config` - Per-tenant notification preferences

Supports:
- Discord, Slack, Email, Generic Webhooks
- Automatic retry with exponential backoff
- Event filtering per channel

## Database Configuration

### Environment Variables

```bash
# Required
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=crosslogic
export DB_PASSWORD=your_secure_password
export DB_NAME=crosslogic_iaas
export DB_SSL_MODE=require  # Use 'disable' for local dev only

# Optional (with defaults)
export DB_MAX_OPEN_CONNS=25
export DB_MAX_IDLE_CONNS=5
export DB_CONN_MAX_LIFETIME=5m
```

### Connection Pooling

The application uses pgx connection pooling with these defaults:
- Max open connections: 25
- Max idle connections: 5
- Connection max lifetime: 5 minutes

Adjust based on your workload:
- High traffic: Increase max open connections to 50-100
- Low traffic: Decrease to 10-15 to reduce resource usage

## PostgreSQL State Backend for SkyPilot

To enable SkyPilot to use PostgreSQL for state management instead of local SQLite:

### 1. Create SkyPilot State Database

```bash
# Connect to PostgreSQL
psql -U postgres

# Create separate database for SkyPilot state
CREATE DATABASE skypilot_state;
GRANT ALL PRIVILEGES ON DATABASE skypilot_state TO crosslogic;
\q
```

### 2. Configure SkyPilot

Create or update `~/.sky/config.yaml`:

```yaml
database:
  type: postgres
  host: localhost  # Or your PostgreSQL host
  port: 5432
  database: skypilot_state
  user: crosslogic
  password: your_secure_password

state:
  backend: postgres  # Instead of local sqlite
  sync_interval: 30s

cache:
  catalog: redis://localhost:6379/1
  ttl: 300s
```

### 3. Initialize SkyPilot State

```bash
# Initialize SkyPilot with PostgreSQL backend
sky status  # This will create the necessary tables

# Verify
psql postgresql://crosslogic:password@localhost:5432/skypilot_state -c "\dt"
```

Expected tables:
- `clusters` - SkyPilot cluster registry
- `cluster_status` - Cluster health and state
- `task_history` - Task execution history

## JuiceFS Metadata Store

JuiceFS uses Redis for metadata. Ensure Redis is configured:

```bash
# Install Redis (if not already installed)
sudo apt-get install redis-server  # Ubuntu/Debian
brew install redis                 # macOS

# Start Redis
redis-server

# Verify
redis-cli ping  # Should return PONG
```

Set environment variable:
```bash
export JUICEFS_REDIS_URL=redis://localhost:6379/1
```

## Maintenance

### Backup

```bash
# Backup all databases
pg_dump -U crosslogic crosslogic_iaas > backup_$(date +%Y%m%d).sql
pg_dump -U crosslogic skypilot_state > skypilot_backup_$(date +%Y%m%d).sql

# Backup with compression
pg_dump -U crosslogic crosslogic_iaas | gzip > backup_$(date +%Y%m%d).sql.gz
```

### Restore

```bash
# Restore from backup
psql -U crosslogic crosslogic_iaas < backup_20250119.sql

# Restore from compressed backup
gunzip -c backup_20250119.sql.gz | psql -U crosslogic crosslogic_iaas
```

### Vacuum and Analyze

```bash
# Optimize database performance
psql $DATABASE_URL -c "VACUUM ANALYZE;"

# Auto-vacuum configuration (in postgresql.conf)
autovacuum = on
autovacuum_max_workers = 3
autovacuum_naptime = 1min
```

### Monitoring Queries

```sql
-- Check table sizes
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Check index usage
SELECT
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;

-- Check slow queries
SELECT
    query,
    calls,
    total_time,
    mean_time,
    max_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;
```

## Production Considerations

### Security

1. **Use strong passwords**: Generate with `openssl rand -base64 32`
2. **Enable SSL**: Set `DB_SSL_MODE=require` in production
3. **Firewall rules**: Only allow control plane IPs to access database
4. **Rotate credentials**: Regular password rotation policy
5. **Audit logging**: Enable `log_statement = 'all'` for compliance

### High Availability

For production deployments:

1. **Streaming Replication**: Set up primary-replica topology
2. **Connection pooling**: Use PgBouncer for connection management
3. **Monitoring**: Set up alerts for:
   - Replication lag > 1 second
   - Connection pool exhaustion
   - Disk space < 20%
   - Slow query threshold > 1 second

### Performance Tuning

Key PostgreSQL parameters for GPU node workload:

```ini
# postgresql.conf
shared_buffers = 4GB              # 25% of RAM
effective_cache_size = 12GB       # 75% of RAM
maintenance_work_mem = 1GB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1            # For SSD
effective_io_concurrency = 200    # For SSD
work_mem = 64MB
min_wal_size = 2GB
max_wal_size = 8GB
max_worker_processes = 8
max_parallel_workers_per_gather = 4
max_parallel_workers = 8
```

## Troubleshooting

### Connection Issues

```bash
# Test connection
psql "postgresql://crosslogic:password@localhost:5432/crosslogic_iaas"

# Check if PostgreSQL is running
sudo systemctl status postgresql

# Check logs
sudo tail -f /var/log/postgresql/postgresql-*.log
```

### Migration Errors

```bash
# Check current schema version
psql $DATABASE_URL -c "SELECT tablename FROM pg_tables WHERE schemaname = 'public';"

# Rollback if needed (be careful!)
psql $DATABASE_URL -c "DROP TABLE IF EXISTS deployments CASCADE;"
psql $DATABASE_URL -c "ALTER TABLE nodes DROP COLUMN IF EXISTS cluster_name CASCADE;"
```

### Performance Issues

```sql
-- Find slow queries
SELECT * FROM pg_stat_activity WHERE state = 'active' AND now() - query_start > interval '1 second';

-- Kill long-running query
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid = <pid>;

-- Analyze query plan
EXPLAIN ANALYZE SELECT * FROM nodes WHERE status = 'active';
```

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [SkyPilot State Backend](https://skypilot.readthedocs.io/en/latest/reference/config.html)
- [JuiceFS Metadata Engine](https://juicefs.com/docs/community/databases_for_metadata/)
- [pgx Connection Pool](https://github.com/jackc/pgx)
