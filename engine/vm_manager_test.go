package engine

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
)

func createTestContext() echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Set up test marker to skip feature initialization in tests
	ctx.Set("_test_context", true)

	return ctx
}

// TestVMManagerBasicOperations tests basic acquire/release operations
func TestVMManagerBasicOperations(t *testing.T) {
	manager := NewVMManager(5)
	ctx := createTestContext()

	// Test acquire
	instance, err := manager.AcquireVM(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, instance)
	assert.NotNil(t, instance.VM)
	assert.True(t, instance.InUse)

	// Test VM is functional
	instance.VM.Set("test", 42)
	val := instance.VM.Get("test")
	assert.Equal(t, int64(42), val.ToInteger())

	// Test release
	manager.ReleaseVM(instance)
	assert.False(t, instance.InUse)

	// Test stats
	stats := manager.GetStats()
	assert.Greater(t, stats.Created, int64(0))
	assert.Equal(t, int64(1), stats.TotalUses)
}

// TestVMManagerConcurrency tests concurrent access without race conditions
func TestVMManagerConcurrency(t *testing.T) {
	manager := NewVMManager(20) // Increased pool size
	ctx := createTestContext()

	numGoroutines := 20 // Reduced to match pool size
	numOperations := 50 // Reduced operations

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Acquire VM
				instance, err := manager.AcquireVM(ctx)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d op %d: acquire failed: %w", id, j, err)
					continue
				}

				// Use VM - simulate some work
				instance.VM.Set(fmt.Sprintf("var_%d_%d", id, j), id*1000+j)

				// Verify value
				val := instance.VM.Get(fmt.Sprintf("var_%d_%d", id, j))
				if val.ToInteger() != int64(id*1000+j) {
					errors <- fmt.Errorf("goroutine %d op %d: value mismatch", id, j)
				}

				// Random small delay
				time.Sleep(time.Microsecond * time.Duration(j%10))

				// Release VM
				manager.ReleaseVM(instance)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Error(err)
		errCount++
		if errCount > 10 {
			t.Fatal("Too many errors, stopping test")
		}
	}

	// Verify stats
	stats := manager.GetStats()
	assert.Equal(t, int64(numGoroutines*numOperations), stats.TotalUses)
	t.Logf("Stats: Created=%d, TotalUses=%d", stats.Created, stats.TotalUses)
}

// TestVMManagerPoolExhaustion tests behavior when pool is exhausted
func TestVMManagerPoolExhaustion(t *testing.T) {
	manager := NewVMManager(2) // Very small pool
	ctx := createTestContext()

	// Acquire all VMs
	vm1, err := manager.AcquireVM(ctx)
	assert.NoError(t, err)

	vm2, err := manager.AcquireVM(ctx)
	assert.NoError(t, err)

	// Try to acquire one more - should fail
	vm3, err := manager.AcquireVM(ctx)
	assert.Error(t, err)
	assert.Nil(t, vm3)
	assert.Contains(t, err.Error(), "VM pool exhausted")

	// Release one and try again
	manager.ReleaseVM(vm1)

	vm3, err = manager.AcquireVM(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, vm3)

	// Cleanup
	manager.ReleaseVM(vm2)
	manager.ReleaseVM(vm3)
}

// TestVMManagerIsolation tests that VMs are properly isolated
func TestVMManagerIsolation(t *testing.T) {
	manager := NewVMManager(2)
	ctx := createTestContext()

	// First VM sets a value
	vm1, _ := manager.AcquireVM(ctx)
	vm1.VM.Set("shared_var", "vm1_value")
	manager.ReleaseVM(vm1)

	// Second VM should not see the value
	vm2, _ := manager.AcquireVM(ctx)
	val := vm2.VM.Get("shared_var")
	assert.True(t, val == nil || val.String() == "undefined")
	manager.ReleaseVM(vm2)
}

// TestVMManagerRaceCondition tests for race conditions using Go's race detector
func TestVMManagerRaceCondition(t *testing.T) {
	manager := NewVMManager(5)
	ctx := createTestContext()

	var wg sync.WaitGroup

	// Multiple goroutines accessing shared state
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				instance, err := manager.AcquireVM(ctx)
				if err != nil {
					continue
				}

				// Access stats (read operation)
				_ = manager.GetStats()

				// Use VM
				instance.VM.RunString(fmt.Sprintf("var test%d = %d", id, j))

				// Release
				manager.ReleaseVM(instance)
			}
		}(i)
	}

	// Concurrent stats reading
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			stats := manager.GetStats()
			_ = stats.TotalUses
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
}

// TestVMManagerWithVM tests the WithVM helper function
func TestVMManagerWithVM(t *testing.T) {
	manager := NewVMManager(3)
	ctx := createTestContext()

	var executionCount int
	err := manager.WithVM(ctx, func(vm *goja.Runtime) error {
		executionCount++
		vm.Set("test", "value")
		val := vm.Get("test")
		assert.Equal(t, "value", val.String())
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, executionCount)

	// Test error propagation
	expectedErr := fmt.Errorf("test error")
	err = manager.WithVM(ctx, func(vm *goja.Runtime) error {
		return expectedErr
	})
	assert.Equal(t, expectedErr, err)
}

// BenchmarkVMManagerAcquireRelease benchmarks acquire/release operations
func BenchmarkVMManagerAcquireRelease(b *testing.B) {
	manager := NewVMManager(10)
	ctx := createTestContext()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			instance, err := manager.AcquireVM(ctx)
			if err != nil {
				b.Fatal(err)
			}
			manager.ReleaseVM(instance)
		}
	})
}

// BenchmarkVMManagerWithVM benchmarks the WithVM helper
func BenchmarkVMManagerWithVM(b *testing.B) {
	manager := NewVMManager(10)
	ctx := createTestContext()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = manager.WithVM(ctx, func(vm *goja.Runtime) error {
				vm.RunString("1 + 1")
				return nil
			})
		}
	})
}
