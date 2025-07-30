package syncsession

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestPerformanceComparison compara el rendimiento real entre implementaciones
func TestPerformanceComparison(t *testing.T) {
	c := setupEchoContext()

	// Configurar managers
	simpleMutex := &SimpleMutexManager{}
	sessionManager := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Número de operaciones
	numGoroutines := 50
	numOperations := 1000

	// Test 1: SimpleMutex
	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if j%5 == 0 {
					simpleMutex.SetValue("test", fmt.Sprintf("key-%d", j), "value", c)
				} else {
					simpleMutex.GetValue("test", fmt.Sprintf("key-%d", j), c)
				}
			}
		}(i)
	}
	wg.Wait()
	simpleMutexTime := time.Since(start)

	// Test 2: SessionManager con cache
	start = time.Now()

	// Pre-calentar cache
	for i := 0; i < 100; i++ {
		sessionManager.SetValue("test", fmt.Sprintf("key-%d", i), "value", c)
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if j%5 == 0 {
					sessionManager.SetValue("test", fmt.Sprintf("key-%d", j), "value", c)
				} else {
					sessionManager.GetValue("test", fmt.Sprintf("key-%d", j), c)
				}
			}
		}(i)
	}
	wg.Wait()
	sessionManagerTime := time.Since(start)

	// Resultados
	t.Logf("\n=== Performance Comparison ===")
	t.Logf("Operations: %d goroutines × %d operations = %d total",
		numGoroutines, numOperations, numGoroutines*numOperations)
	t.Logf("SimpleMutex time: %v", simpleMutexTime)
	t.Logf("SessionManager time: %v", sessionManagerTime)
	t.Logf("Performance improvement: %.2fx faster",
		float64(simpleMutexTime)/float64(sessionManagerTime))

	// El SessionManager debería ser más rápido
	if sessionManagerTime < simpleMutexTime {
		t.Logf("✓ SessionManager is faster!")
	} else {
		t.Logf("✗ SimpleMutex is faster (this might happen with low concurrency)")
	}
}

// TestCacheEffectiveness mide la efectividad del cache
func TestCacheEffectiveness(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   1 * time.Second,
	}

	// Establecer un valor
	sm.SetValue("cache-test", "key1", "value1", c)

	// Medir tiempo de acceso sin cache (primera vez)
	start := time.Now()
	val1, _ := sm.GetValue("cache-test", "key1", c)
	noCacheTime := time.Since(start)

	// Medir tiempo de acceso con cache (varias veces)
	var totalCacheTime time.Duration
	cacheHits := 100
	for i := 0; i < cacheHits; i++ {
		start = time.Now()
		val2, _ := sm.GetValue("cache-test", "key1", c)
		totalCacheTime += time.Since(start)
		_ = val2
	}
	avgCacheTime := totalCacheTime / time.Duration(cacheHits)

	t.Logf("\n=== Cache Effectiveness ===")
	t.Logf("First access (no cache): %v", noCacheTime)
	t.Logf("Avg cached access: %v", avgCacheTime)
	t.Logf("Cache speedup: %.2fx faster", float64(noCacheTime)/float64(avgCacheTime))

	// Verificar que el valor es correcto
	if val1 != "value1" {
		t.Errorf("Expected value1, got %v", val1)
	}
}

// TestMemoryUsage compara el uso de memoria
func TestMemoryUsage(t *testing.T) {
	c := setupEchoContext()

	// Forzar garbage collection antes de empezar
	runtime.GC()

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Crear SessionManager con muchas entradas en cache
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Llenar el cache
	for i := 0; i < 10000; i++ {
		sm.SetValue("memory-test", fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), c)
		sm.GetValue("memory-test", fmt.Sprintf("key-%d", i), c) // Forzar entrada en cache
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	memUsed := m2.HeapAlloc - m1.HeapAlloc
	memUsedMB := float64(memUsed) / 1024 / 1024

	t.Logf("\n=== Memory Usage ===")
	t.Logf("Cache entries: 10,000")
	t.Logf("Memory used: %.2f MB", memUsedMB)
	t.Logf("Memory per entry: %.2f KB", float64(memUsed)/10000/1024)

	// Limpiar cache
	sm.CleanupCache()

	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	t.Logf("Memory after cleanup: %.2f MB", float64(m3.HeapAlloc-m1.HeapAlloc)/1024/1024)
}
