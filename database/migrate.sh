#!/bin/bash
# Database Migration Script for CrossLogic AI IaaS
# Usage: ./migrate.sh [up|down|status]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database connection from environment
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-crosslogic}
DB_PASSWORD=${DB_PASSWORD:-}
DB_NAME=${DB_NAME:-crosslogic_iaas}
DB_SSL_MODE=${DB_SSL_MODE:-disable}

# Construct connection string
if [ -z "$DATABASE_URL" ]; then
    if [ -z "$DB_PASSWORD" ]; then
        echo -e "${RED}Error: DB_PASSWORD environment variable is required${NC}"
        echo "Set it with: export DB_PASSWORD=your_password"
        exit 1
    fi
    DATABASE_URL="postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMAS_DIR="${SCRIPT_DIR}/schemas"

echo -e "${GREEN}CrossLogic AI IaaS - Database Migration${NC}"
echo "========================================"
echo "Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "User: ${DB_USER}"
echo ""

# Function to check if database exists
check_database() {
    psql "${DATABASE_URL}" -c "SELECT 1;" > /dev/null 2>&1
    return $?
}

# Function to apply migrations
migrate_up() {
    echo -e "${YELLOW}Applying migrations...${NC}"

    # Check if database is accessible
    if ! check_database; then
        echo -e "${RED}Error: Cannot connect to database${NC}"
        echo "Make sure PostgreSQL is running and credentials are correct"
        exit 1
    fi

    # Apply migrations in order
    migrations=(
        "01_core_tables.sql"
        "02_deployments.sql"
        "03_notifications.sql"
    )

    for migration in "${migrations[@]}"; do
        migration_file="${SCHEMAS_DIR}/${migration}"

        if [ ! -f "$migration_file" ]; then
            echo -e "${RED}Error: Migration file not found: ${migration}${NC}"
            exit 1
        fi

        echo -e "${YELLOW}Applying: ${migration}${NC}"

        if psql "${DATABASE_URL}" -f "${migration_file}" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ ${migration} applied successfully${NC}"
        else
            echo -e "${RED}✗ ${migration} failed${NC}"
            echo "Run manually to see error:"
            echo "psql \"${DATABASE_URL}\" -f \"${migration_file}\""
            exit 1
        fi
    done

    echo ""
    echo -e "${GREEN}✓ All migrations applied successfully!${NC}"
}

# Function to show migration status
show_status() {
    echo -e "${YELLOW}Database Status:${NC}"
    echo ""

    if ! check_database; then
        echo -e "${RED}✗ Database not accessible${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ Database accessible${NC}"
    echo ""

    # Check for key tables
    tables=(
        "tenants"
        "environments"
        "api_keys"
        "nodes"
        "deployments"
        "notification_deliveries"
    )

    echo "Tables:"
    for table in "${tables[@]}"; do
        if psql "${DATABASE_URL}" -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='${table}'" | grep -q 1; then
            echo -e "  ${GREEN}✓${NC} ${table}"
        else
            echo -e "  ${RED}✗${NC} ${table} (missing)"
        fi
    done

    echo ""

    # Show table counts
    echo "Record counts:"
    for table in "${tables[@]}"; do
        if psql "${DATABASE_URL}" -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='${table}'" | grep -q 1; then
            count=$(psql "${DATABASE_URL}" -tAc "SELECT COUNT(*) FROM ${table}")
            echo "  ${table}: ${count}"
        fi
    done

    echo ""

    # Show database size
    size=$(psql "${DATABASE_URL}" -tAc "SELECT pg_size_pretty(pg_database_size('${DB_NAME}'))")
    echo "Database size: ${size}"
}

# Function to rollback (drop all tables)
migrate_down() {
    echo -e "${RED}WARNING: This will drop all tables and data!${NC}"
    read -p "Are you sure? Type 'yes' to continue: " confirm

    if [ "$confirm" != "yes" ]; then
        echo "Rollback cancelled"
        exit 0
    fi

    echo -e "${YELLOW}Rolling back migrations...${NC}"

    # Drop tables in reverse order (respecting foreign keys)
    psql "${DATABASE_URL}" <<EOF
        DROP TABLE IF EXISTS deployment_nodes CASCADE;
        DROP TABLE IF EXISTS notification_deliveries CASCADE;
        DROP TABLE IF EXISTS notification_config CASCADE;
        DROP TABLE IF EXISTS webhook_events CASCADE;
        DROP TABLE IF EXISTS health_checks CASCADE;
        DROP TABLE IF EXISTS audit_logs CASCADE;
        DROP TABLE IF EXISTS reservations CASCADE;
        DROP TABLE IF EXISTS credits CASCADE;
        DROP TABLE IF EXISTS billing_events CASCADE;
        DROP TABLE IF EXISTS usage_hourly CASCADE;
        DROP TABLE IF EXISTS usage_records CASCADE;
        DROP TABLE IF EXISTS nodes CASCADE;
        DROP TABLE IF EXISTS deployments CASCADE;
        DROP TABLE IF EXISTS models CASCADE;
        DROP TABLE IF EXISTS regions CASCADE;
        DROP TABLE IF EXISTS api_keys CASCADE;
        DROP TABLE IF EXISTS environments CASCADE;
        DROP TABLE IF EXISTS tenants CASCADE;
        DROP FUNCTION IF EXISTS update_updated_at_column CASCADE;
        DROP FUNCTION IF EXISTS update_notification_config_updated_at CASCADE;
EOF

    echo -e "${GREEN}✓ All tables dropped${NC}"
}

# Main command handler
case "${1:-up}" in
    up)
        migrate_up
        ;;
    down)
        migrate_down
        ;;
    status)
        show_status
        ;;
    *)
        echo "Usage: $0 [up|down|status]"
        echo ""
        echo "Commands:"
        echo "  up      - Apply all migrations (default)"
        echo "  down    - Rollback all migrations (drops all tables)"
        echo "  status  - Show current database status"
        exit 1
        ;;
esac
