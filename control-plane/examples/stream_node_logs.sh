#!/bin/bash

# Real-Time Node Log Streaming Example
# This script demonstrates how to stream node launch logs using curl and Server-Sent Events

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:-your-admin-token}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== CrossLogic Node Log Streaming Demo ===${NC}\n"

# Function to parse SSE events
parse_sse() {
    local event_type=""
    local data=""

    while IFS= read -r line; do
        if [[ $line == event:* ]]; then
            event_type="${line#event: }"
        elif [[ $line == data:* ]]; then
            data="${line#data: }"

            # Process based on event type
            case $event_type in
                "log")
                    timestamp=$(echo "$data" | jq -r '.timestamp')
                    level=$(echo "$data" | jq -r '.level')
                    message=$(echo "$data" | jq -r '.message')
                    phase=$(echo "$data" | jq -r '.phase')
                    progress=$(echo "$data" | jq -r '.progress // 0')

                    case $level in
                        "error")
                            echo -e "${RED}[ERROR]${NC} [$phase] $message"
                            ;;
                        "warn")
                            echo -e "${YELLOW}[WARN]${NC} [$phase] $message"
                            ;;
                        "info")
                            echo -e "${GREEN}[INFO]${NC} [$phase] $message (${progress}%)"
                            ;;
                        *)
                            echo -e "[${level^^}] [$phase] $message"
                            ;;
                    esac
                    ;;

                "status")
                    phase=$(echo "$data" | jq -r '.phase')
                    progress=$(echo "$data" | jq -r '.progress')
                    message=$(echo "$data" | jq -r '.message')
                    echo -e "${BLUE}[STATUS]${NC} $phase: $message (${progress}%)"
                    ;;

                "error")
                    error=$(echo "$data" | jq -r '.error')
                    details=$(echo "$data" | jq -r '.details')
                    phase=$(echo "$data" | jq -r '.phase')
                    echo -e "${RED}[LAUNCH ERROR]${NC} $phase: $error"
                    echo -e "${RED}  Details:${NC} $details"
                    ;;

                "done")
                    status=$(echo "$data" | jq -r '.status')
                    endpoint=$(echo "$data" | jq -r '.endpoint // "N/A"')
                    message=$(echo "$data" | jq -r '.message')

                    echo -e "\n${GREEN}=== Launch Complete ===${NC}"
                    echo -e "Status: $status"
                    echo -e "Message: $message"
                    if [[ "$endpoint" != "N/A" ]]; then
                        echo -e "Endpoint: $endpoint"
                    fi
                    exit 0
                    ;;
            esac

            event_type=""
            data=""
        fi
    done
}

# Check if node ID is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <node-id> [tail] [follow]"
    echo ""
    echo "Example:"
    echo "  $0 550e8400-e29b-41d4-a716-446655440000"
    echo "  $0 550e8400-e29b-41d4-a716-446655440000 50 true"
    echo ""
    echo "Environment Variables:"
    echo "  API_BASE_URL  - API base URL (default: http://localhost:8080)"
    echo "  ADMIN_TOKEN   - Admin authentication token"
    exit 1
fi

NODE_ID="$1"
TAIL="${2:-100}"
FOLLOW="${3:-true}"

echo "Node ID: $NODE_ID"
echo "Tail: $TAIL lines"
echo "Follow: $FOLLOW"
echo ""

# Stream logs
echo -e "${BLUE}Connecting to log stream...${NC}\n"

curl -N -s \
    -H "X-Admin-Token: $ADMIN_TOKEN" \
    "${API_BASE_URL}/admin/nodes/${NODE_ID}/logs/stream?tail=${TAIL}&follow=${FOLLOW}" \
    | parse_sse

echo -e "\n${BLUE}Stream ended${NC}"
