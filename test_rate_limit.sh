#!/bin/bash

# Test script for rate limiting functionality

echo "Testing Rate Limiting..."
echo "========================"

# Function to make a request and show the response
make_request() {
    echo -n "Request $1: "
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}\n" http://localhost:8080/test 2>/dev/null)
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    
    if [ "$http_code" = "429" ]; then
        echo "Rate limited (429)"
        echo "$response" | grep -v "HTTP_CODE:" | jq . 2>/dev/null || echo "$response" | grep -v "HTTP_CODE:"
    else
        echo "OK ($http_code)"
    fi
}

# Make multiple requests to trigger rate limit
echo "Making 15 rapid requests (configured limit: 10 requests per minute with burst of 10)..."
echo ""

for i in {1..15}; do
    make_request $i
    sleep 0.1  # Small delay between requests
done

echo ""
echo "Waiting 60 seconds for rate limit window to reset..."
sleep 60

echo ""
echo "Making request after waiting..."
make_request "after-wait"

echo ""
echo "Test complete."