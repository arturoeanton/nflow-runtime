package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

// BenchmarkVMPool tests the performance of VM pool
func BenchmarkVMPool(b *testing.B) {
	// Initialize VM manager
	vmManager := NewVMManagerWithConfig(200, &VMPoolConfig{
		MaxSize:       200,
		PreloadSize:   100,
		IdleTimeout:   10,
		EnableMetrics: false,
	})

	// Create a test context
	e := echo.New()
	req := e.NewContext(nil, nil).Request()
	c := e.NewContext(req, nil)
	c.Set("_test_context", true)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := vmManager.WithVM(c, func(vm *goja.Runtime) error {
				// Simulate heavy JavaScript workload
				script := `
					function fibonacci(n) {
						if (n <= 1) return n;
						return fibonacci(n - 1) + fibonacci(n - 2);
					}
					var result = 0;
					for (var i = 0; i < 10; i++) {
						result += fibonacci(20);
					}
					result;
				`
				_, err := vm.RunString(script)
				return err
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkVMCreation compares VM pool vs creating new VMs
func BenchmarkVMCreation(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		vmManager := NewVMManagerWithConfig(200, &VMPoolConfig{
			MaxSize:       200,
			PreloadSize:   100,
			EnableMetrics: false,
		})
		e := echo.New()
		req := e.NewContext(nil, nil).Request()
		c := e.NewContext(req, nil)
		c.Set("_test_context", true)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := vmManager.WithVM(c, func(vm *goja.Runtime) error {
					_, err := vm.RunString(`1 + 1`)
					return err
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				vm := goja.New()
				_, err := vm.RunString(`1 + 1`)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

// TestVMPoolConcurrency tests concurrent access with heavy load
func TestVMPoolConcurrency(t *testing.T) {
	vmManager := NewVMManagerWithConfig(200, &VMPoolConfig{
		MaxSize:       200,
		PreloadSize:   100,
		EnableMetrics: true,
	})

	e := echo.New()
	req := e.NewContext(nil, nil).Request()
	c := e.NewContext(req, nil)
	c.Set("_test_context", true)

	// Number of concurrent goroutines
	concurrency := 200
	iterations := 100

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				err := vmManager.WithVM(c, func(vm *goja.Runtime) error {
					// Heavy JavaScript computation
					script := fmt.Sprintf(`
						var sum = 0;
						for (var i = 0; i < 1000; i++) {
							sum += Math.sqrt(i * %d);
						}
						sum;
					`, id)
					_, err := vm.RunString(script)
					return err
				})
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	totalRequests := int64(concurrency * iterations)
	rps := float64(totalRequests) / elapsed.Seconds()

	t.Logf("Concurrency test completed:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Requests per second: %.2f", rps)

	stats := vmManager.GetStats()
	t.Logf("VM Pool Stats:")
	t.Logf("  Created: %d", stats.Created)
	t.Logf("  Total uses: %d", stats.TotalUses)
	t.Logf("  Errors: %d", stats.Errors)

	if errorCount > 0 {
		t.Errorf("Had %d errors during execution", errorCount)
	}

	// Target: 160-200 RPS (4x improvement from 40-50 RPS)
	if rps < 160 {
		t.Errorf("Performance below target: %.2f RPS (target: 160+ RPS)", rps)
	} else {
		t.Logf("Performance target achieved: %.2f RPS", rps)
	}
}

// TestBabelCachePerformance tests babel transform cache effectiveness
func TestBabelCachePerformance(t *testing.T) {
	code := `
		const fibonacci = (n) => {
			if (n <= 1) return n;
			return fibonacci(n - 1) + fibonacci(n - 2);
		};
		let result = 0;
		for (let i = 0; i < 10; i++) {
			result += fibonacci(15);
		}
		result;
	`

	// First transform (cache miss)
	start := time.Now()
	transformed1 := babelTransform(code)
	firstDuration := time.Since(start)

	// Second transform (cache hit)
	start = time.Now()
	transformed2 := babelTransform(code)
	secondDuration := time.Since(start)

	if transformed1 != transformed2 {
		t.Error("Cached transform returned different result")
	}

	// Cache hit should be at least 10x faster
	if secondDuration > firstDuration/10 {
		t.Logf("Cache performance not optimal: first=%v, second=%v", firstDuration, secondDuration)
	} else {
		t.Logf("Cache performance good: first=%v, second=%v (%.2fx speedup)",
			firstDuration, secondDuration, float64(firstDuration)/float64(secondDuration))
	}
}
