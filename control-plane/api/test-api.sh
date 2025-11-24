#!/bin/bash

# Model Management API Test Script
# Tests all CRUD operations for the CrossLogic AI IaaS Model Management API

set -e

# Configuration
BASE_URL="${API_BASE_URL:-http://localhost:8080}"
ADMIN_TOKEN="${ADMIN_TOKEN:-dev-admin-token-12345}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_header() {
    echo -e "\n${BLUE}=== $1 ===${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}→ $1${NC}"
}

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Test variables
TEST_MODEL_ID=""
TEST_MODEL_NAME="meta-llama/Llama-3.1-8B-Instruct-TEST-$(date +%s)"

# Test 1: List Models
print_header "Test 1: List All Models"
print_info "GET $BASE_URL/api/v1/admin/models"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models?limit=10" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "List models returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "List models failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 2: Create Model
print_header "Test 2: Create New Model"
print_info "POST $BASE_URL/api/v1/admin/models"

CREATE_PAYLOAD=$(cat <<EOF
{
  "name": "$TEST_MODEL_NAME",
  "family": "llama",
  "size": "8B",
  "type": "chat",
  "context_length": 8192,
  "vram_required_gb": 16,
  "price_input_per_million": 0.15,
  "price_output_per_million": 0.60,
  "tokens_per_second_capacity": 500,
  "status": "active",
  "metadata": {
    "storage": {
      "provider": "cloudflare-r2",
      "bucket": "test-bucket",
      "path": "test/path"
    },
    "capabilities": ["chat", "test"],
    "test": true
  }
}
EOF
)

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/api/v1/admin/models" \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$CREATE_PAYLOAD")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "201" ]; then
    print_success "Create model returned 201 Created"
    TEST_MODEL_ID=$(echo "$BODY" | jq -r '.id' 2>/dev/null || echo "")
    if [ -n "$TEST_MODEL_ID" ] && [ "$TEST_MODEL_ID" != "null" ]; then
        print_success "Model created with ID: $TEST_MODEL_ID"
        echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        print_error "Failed to extract model ID from response"
        echo "$BODY"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
else
    print_error "Create model failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Only continue if we have a valid model ID
if [ -z "$TEST_MODEL_ID" ] || [ "$TEST_MODEL_ID" = "null" ]; then
    print_error "Cannot continue tests without valid model ID"
    exit 1
fi

# Test 3: Get Model by ID
print_header "Test 3: Get Model by ID"
print_info "GET $BASE_URL/api/v1/admin/models/$TEST_MODEL_ID"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models/$TEST_MODEL_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Get model by ID returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Get model by ID failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 4: Update Model (PATCH - Partial Update)
print_header "Test 4: Partial Update Model (PATCH)"
print_info "PATCH $BASE_URL/api/v1/admin/models/$TEST_MODEL_ID"

PATCH_PAYLOAD=$(cat <<EOF
{
  "price_input_per_million": 0.12,
  "price_output_per_million": 0.48,
  "status": "beta"
}
EOF
)

RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE_URL/api/v1/admin/models/$TEST_MODEL_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$PATCH_PAYLOAD")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Patch model returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Patch model failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 5: Update Model (PUT - Full Update)
print_header "Test 5: Full Update Model (PUT)"
print_info "PUT $BASE_URL/api/v1/admin/models/$TEST_MODEL_ID"

PUT_PAYLOAD=$(cat <<EOF
{
  "name": "$TEST_MODEL_NAME",
  "family": "llama",
  "size": "8B",
  "type": "chat",
  "context_length": 16384,
  "vram_required_gb": 18,
  "price_input_per_million": 0.10,
  "price_output_per_million": 0.40,
  "tokens_per_second_capacity": 600,
  "status": "active",
  "metadata": {
    "storage": {
      "provider": "aws-s3",
      "bucket": "updated-bucket",
      "path": "updated/path"
    },
    "capabilities": ["chat", "updated"],
    "updated": true
  }
}
EOF
)

RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/api/v1/admin/models/$TEST_MODEL_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$PUT_PAYLOAD")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Put model returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Put model failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 6: Search Models
print_header "Test 6: Search Models"
print_info "GET $BASE_URL/api/v1/admin/models/search?q=test&families=llama"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models/search?q=TEST&families=llama" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Search models returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Search models failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 7: Filter by Family
print_header "Test 7: Filter Models by Family"
print_info "GET $BASE_URL/api/v1/admin/models?family=llama&limit=5"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models?family=llama&limit=5" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Filter by family returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Filter by family failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 8: Sort Models
print_header "Test 8: Sort Models"
print_info "GET $BASE_URL/api/v1/admin/models?sort_by=created_at&sort_order=desc&limit=5"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models?sort_by=created_at&sort_order=desc&limit=5" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    print_success "Sort models returned 200 OK"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Sort models failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 9: Invalid UUID (Error Handling)
print_header "Test 9: Error Handling - Invalid UUID"
print_info "GET $BASE_URL/api/v1/admin/models/invalid-uuid"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models/invalid-uuid" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "400" ]; then
    print_success "Invalid UUID correctly returned 400 Bad Request"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Invalid UUID handling failed (expected 400, got $HTTP_CODE)"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 10: Unauthorized Access
print_header "Test 10: Error Handling - Unauthorized"
print_info "GET $BASE_URL/api/v1/admin/models (without token)"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "401" ]; then
    print_success "Unauthorized request correctly returned 401"
    echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Unauthorized handling failed (expected 401, got $HTTP_CODE)"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 11: Delete Model
print_header "Test 11: Delete Model"
print_info "DELETE $BASE_URL/api/v1/admin/models/$TEST_MODEL_ID"

RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE "$BASE_URL/api/v1/admin/models/$TEST_MODEL_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "204" ]; then
    print_success "Delete model returned 204 No Content"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Delete model failed with status $HTTP_CODE"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 12: Verify Deletion (404)
print_header "Test 12: Verify Model Deleted"
print_info "GET $BASE_URL/api/v1/admin/models/$TEST_MODEL_ID"

RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/api/v1/admin/models/$TEST_MODEL_ID" \
  -H "X-Admin-Token: $ADMIN_TOKEN")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "404" ]; then
    print_success "Deleted model correctly returns 404 Not Found"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    print_error "Deleted model check failed (expected 404, got $HTTP_CODE)"
    echo "$BODY"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Summary
print_header "Test Summary"
TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo -e "Total: $TOTAL_TESTS"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}All tests passed! ✓${NC}\n"
    exit 0
else
    echo -e "\n${RED}Some tests failed ✗${NC}\n"
    exit 1
fi
