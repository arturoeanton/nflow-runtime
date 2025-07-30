#!/bin/bash

# Test concurrent requests to trigger race conditions
echo "Testing concurrent requests..."

# Function to make concurrent requests
make_requests() {
    for i in {1..100}; do
        curl -s http://localhost:8080/pepe11 > /dev/null &
    done
}

# Run multiple batches concurrently
for batch in {1..5}; do
    echo "Batch $batch..."
    make_requests
    sleep 0.5
done

wait
echo "Done!"