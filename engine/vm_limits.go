// Package engine provides the core workflow execution engine for nFlow Runtime.
// This file contains resource limit management for JavaScript VMs to prevent
// resource exhaustion attacks and ensure stable operation.
package engine

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// VMResourceLimits defines resource limits for a VM
type VMResourceLimits struct {
	MaxMemoryBytes   int64         // Maximum memory in bytes (0 = no limit)
	MaxExecutionTime time.Duration // Maximum execution time (0 = no limit)
	MaxOperations    int64         // Maximum JS operations (0 = no limit)
	MaxStackDepth    int           // Maximum stack depth (0 = no limit)
	CheckInterval    int64         // How many operations between limit checks
}

// DefaultVMResourceLimits returns safe default limits
func DefaultVMResourceLimits() VMResourceLimits {
	return VMResourceLimits{
		MaxMemoryBytes:   128 * 1024 * 1024, // 128MB
		MaxExecutionTime: 30 * time.Second,  // 30 seconds
		MaxOperations:    10_000_000,        // 10M operations
		MaxStackDepth:    1000,              // 1000 recursion levels
		CheckInterval:    1000,              // Check every 1000 operations
	}
}

// VMResourceTracker tracks resource usage of a VM
type VMResourceTracker struct {
	startTime      time.Time
	operationCount int64
	checkInterval  int64
	limits         VMResourceLimits
	ctx            context.Context
	cancel         context.CancelFunc
	memoryBaseline uint64
	interrupted    atomic.Bool
}

// Limit errors
var (
	ErrMemoryLimitExceeded    = errors.New("memory limit exceeded")
	ErrTimeLimitExceeded      = errors.New("execution time limit exceeded")
	ErrOperationLimitExceeded = errors.New("operation limit exceeded")
	ErrExecutionInterrupted   = errors.New("execution interrupted")
)

// IsResourceLimitError checks if an error is due to resource limits
func IsResourceLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "memory limit exceeded" ||
		errStr == "execution time limit exceeded" ||
		errStr == "operation limit exceeded" ||
		errStr == "execution interrupted" ||
		errStr == "RuntimeError: execution terminated"
}

// NewVMResourceTracker creates a new resource tracker
func NewVMResourceTracker(limits VMResourceLimits) *VMResourceTracker {
	ctx, cancel := context.WithCancel(context.Background())

	// Capture baseline memory
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	tracker := &VMResourceTracker{
		startTime:      time.Now(),
		limits:         limits,
		checkInterval:  limits.CheckInterval,
		ctx:            ctx,
		cancel:         cancel,
		memoryBaseline: m.Alloc,
	}

	// If there's a time limit, configure timeout
	if limits.MaxExecutionTime > 0 {
		go tracker.watchTimeout()
	}

	return tracker
}

// SetupVMWithLimits configures a VM with resource limits
func SetupVMWithLimits(vm *goja.Runtime, limits VMResourceLimits) *VMResourceTracker {
	tracker := NewVMResourceTracker(limits)

	// Start goroutine to check limits periodically
	go tracker.monitorLimits(vm)

	return tracker
}

// monitorLimits monitors limits and interrupts the VM if necessary
func (t *VMResourceTracker) monitorLimits(vm *goja.Runtime) {
	ticker := time.NewTicker(10 * time.Millisecond) // Check every 10ms
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if t.CheckLimits() {
				vm.Interrupt("resource limit exceeded")
				return
			}
		case <-t.ctx.Done():
			return
		}
	}
}

// CheckLimits checks if limits have been exceeded
func (t *VMResourceTracker) CheckLimits() bool {
	// If already interrupted, return immediately
	if t.interrupted.Load() {
		return true
	}

	// Increment operation counter
	ops := atomic.AddInt64(&t.operationCount, 1)

	// Check operation limit immediately if exceeded
	if t.limits.MaxOperations > 0 && ops > t.limits.MaxOperations {
		t.interrupted.Store(true)
		return true
	}

	// Only check memory every N operations to avoid overhead
	if ops%t.checkInterval == 0 {
		// Check memory limit
		if t.limits.MaxMemoryBytes > 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			memoryUsed := int64(m.Alloc - t.memoryBaseline)

			if memoryUsed > t.limits.MaxMemoryBytes {
				t.interrupted.Store(true)
				return true
			}
		}
	}

	// Check if time exceeded
	if t.limits.MaxExecutionTime > 0 {
		if time.Since(t.startTime) > t.limits.MaxExecutionTime {
			t.interrupted.Store(true)
			return true
		}
	}

	// Check if cancelled
	select {
	case <-t.ctx.Done():
		t.interrupted.Store(true)
		return true
	default:
		return false
	}
}

// watchTimeout monitors execution time
func (t *VMResourceTracker) watchTimeout() {
	timer := time.NewTimer(t.limits.MaxExecutionTime)
	defer timer.Stop()

	select {
	case <-timer.C:
		t.interrupted.Store(true)
		t.cancel()
	case <-t.ctx.Done():
		return
	}
}

// GetStats returns current statistics
func (t *VMResourceTracker) GetStats() VMResourceStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return VMResourceStats{
		ElapsedTime:    time.Since(t.startTime),
		OperationCount: atomic.LoadInt64(&t.operationCount),
		MemoryUsed:     int64(m.Alloc - t.memoryBaseline),
		Interrupted:    t.interrupted.Load(),
	}
}

// Stop stops the tracker and releases resources
func (t *VMResourceTracker) Stop() {
	t.cancel()
}

// VMResourceStats contains usage statistics
type VMResourceStats struct {
	ElapsedTime    time.Duration
	OperationCount int64
	MemoryUsed     int64
	Interrupted    bool
}

// GetLimitsFromConfig gets limits from configuration
func GetLimitsFromConfig() VMResourceLimits {
	config := GetConfig()
	limits := DefaultVMResourceLimits()

	// Override with configuration values if they exist
	if config.VMPoolConfig.MaxMemoryMB > 0 {
		limits.MaxMemoryBytes = int64(config.VMPoolConfig.MaxMemoryMB) * 1024 * 1024
	}

	if config.VMPoolConfig.MaxExecutionSeconds > 0 {
		limits.MaxExecutionTime = time.Duration(config.VMPoolConfig.MaxExecutionSeconds) * time.Second
	}

	if config.VMPoolConfig.MaxOperations > 0 {
		limits.MaxOperations = config.VMPoolConfig.MaxOperations
	}

	return limits
}
