#!/bin/bash

echo "=== Running Session Manager Benchmarks ==="
echo ""
echo "Installing dependencies..."
go get -u github.com/stretchr/testify/assert
go get -u github.com/labstack/echo/v4
go get -u github.com/labstack/echo-contrib/session
go get -u github.com/gorilla/sessions

echo ""
echo "Running tests first..."
go test -v -run Test

echo ""
echo "=== BENCHMARK RESULTS ==="
echo ""
echo "1. READ OPERATIONS (Higher is better)"
echo "-------------------------------------"
go test -bench="Read" -benchmem -benchtime=10s | grep -E "Benchmark|ns/op"

echo ""
echo "2. WRITE OPERATIONS (Higher is better)"
echo "--------------------------------------"
go test -bench="Write" -benchmem -benchtime=10s | grep -E "Benchmark|ns/op"

echo ""
echo "3. MIXED OPERATIONS (80% read, 20% write)"
echo "-----------------------------------------"
go test -bench="Mixed" -benchmem -benchtime=10s | grep -E "Benchmark|ns/op"

echo ""
echo "4. HIGH CONCURRENCY (100 goroutines)"
echo "------------------------------------"
go test -bench="HighConcurrency" -benchmem -benchtime=10s | grep -E "Benchmark|ns/op"

echo ""
echo "=== SUMMARY ==="
echo ""
echo "Performance comparison:"
echo "- SimpleMutex: Uses a single mutex for all operations"
echo "- SessionManager_NoCache: Uses RWMutex without caching"
echo "- SessionManager_WithCache: Uses RWMutex with in-memory cache"
echo ""
echo "Key metrics to look at:"
echo "- ns/op: nanoseconds per operation (lower is better)"
echo "- allocs/op: memory allocations per operation (lower is better)"
echo "- B/op: bytes allocated per operation (lower is better)"