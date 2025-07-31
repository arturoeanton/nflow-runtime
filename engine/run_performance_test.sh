#!/bin/bash

echo "Running VM Pool Performance Tests"
echo "================================="

# Run benchmarks
echo -e "\n1. Running VM Pool Benchmarks..."
go test -bench=BenchmarkVM -benchtime=10s -cpu=4,8 ./engine

# Run concurrency test
echo -e "\n2. Running Concurrency Test (200 concurrent workers)..."
go test -run=TestVMPoolConcurrency -v ./engine

# Run babel cache test
echo -e "\n3. Running Babel Cache Performance Test..."
go test -run=TestBabelCachePerformance -v ./engine

# Run with race detector for safety
echo -e "\n4. Running with Race Detector..."
go test -race -run=TestVMPoolConcurrency ./engine

echo -e "\nPerformance test completed!"
echo "Target: 160-200 RPS (4x improvement from 40-50 RPS baseline)"