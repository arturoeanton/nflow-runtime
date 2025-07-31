#!/bin/bash
# Test script for debug and monitoring endpoints

echo "Testing nFlow Runtime endpoints..."
echo "================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

# Function to test endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local expected_status=$3
    local description=$4
    
    echo -n "Testing $description... "
    
    response=$(curl -s -w "\n%{http_code}" -X $method "$BASE_URL$endpoint")
    status_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}PASS${NC} (Status: $status_code)"
        if [ ! -z "$body" ]; then
            echo "  Response preview: $(echo "$body" | head -1 | cut -c1-80)..."
        fi
    else
        echo -e "${RED}FAIL${NC} (Expected: $expected_status, Got: $status_code)"
        echo "  Response: $body"
    fi
    echo
}

echo "1. Testing Monitoring Endpoints"
echo "------------------------------"
test_endpoint "GET" "/health" "200" "Health check endpoint"
test_endpoint "GET" "/metrics" "200" "Prometheus metrics endpoint"

echo "2. Testing Debug Endpoints (should fail if disabled)"
echo "---------------------------------------------------"
test_endpoint "GET" "/debug/info" "404" "Debug info (disabled)"
test_endpoint "GET" "/debug/config" "404" "Debug config (disabled)"

echo "3. Testing Legacy Debug Endpoints"
echo "---------------------------------"
test_endpoint "GET" "/debug/invalidate-cache" "404" "Cache invalidation (disabled)"
test_endpoint "GET" "/debug/clean-json" "404" "Clean JSON (disabled)"
test_endpoint "GET" "/debug/starters" "404" "Starters (disabled)"

echo "Test completed!"
echo
echo "To enable debug endpoints, set debug.enabled = true in config.toml"
echo "To test with auth token, use: curl -H 'X-Debug-Token: your-token' ..."