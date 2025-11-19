#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Docker Test Runner...${NC}"

# Ensure containers are up
echo "Ensuring services are running..."
docker-compose up -d postgres redis

# Wait for DB
echo "Waiting for database..."
until docker-compose exec postgres pg_isready -U crosslogic; do
  echo "Waiting for postgres..."
  sleep 2
done

# Run Control Plane Tests
echo -e "${GREEN}Running Control Plane Tests...${NC}"
docker-compose run --rm --entrypoint go control-plane test ./... -v

# Run Node Agent Tests
echo -e "${GREEN}Running Node Agent Tests...${NC}"
# Assuming node-agent uses the same base image or has go installed
# If node-agent image is minimal, we might need a separate test container or use the control-plane one if compatible
# For now, assuming we can run tests in the control-plane container context for shared libs, 
# or we need to build a test image for node-agent.
# Let's try running in a temporary container with the node-agent source mounted.

docker run --rm \
  -v $(pwd)/node-agent:/app \
  -w /app \
  golang:1.22 \
  go test ./... -v

echo -e "${GREEN}All tests passed!${NC}"
