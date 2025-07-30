package syncsession

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionManager_RaceConditions verifica que no hay race conditions
func TestSessionManager_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	const (
		numWriters    = 50
		numReaders    = 100
		numOperations = 1000
	)

	var (
		writeErrors int32
		readErrors  int32
		wg          sync.WaitGroup
	)

	// Writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d", rand.Intn(100))
				value := fmt.Sprintf("value-%d-%d", id, j)

				if err := sm.SetValue("race-test", key, value, c); err != nil {
					atomic.AddInt32(&writeErrors, 1)
				}

				// Ocasionalmente hacer batch updates
				if j%10 == 0 {
					values := map[string]interface{}{
						key + "-1": value + "-1",
						key + "-2": value + "-2",
						key + "-3": value + "-3",
					}
					if err := sm.SetMultipleValues("race-test", values, c); err != nil {
						atomic.AddInt32(&writeErrors, 1)
					}
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d", rand.Intn(100))

				if _, err := sm.GetValue("race-test", key, c); err != nil {
					atomic.AddInt32(&readErrors, 1)
				}

				// Ocasionalmente limpiar cache
				if j%100 == 0 {
					sm.CleanupCache()
				}
			}
		}(i)
	}

	// Cleanup goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			sm.CleanupCache()
		}
	}()

	wg.Wait()

	t.Logf("Race condition test completed")
	t.Logf("Write errors: %d", writeErrors)
	t.Logf("Read errors: %d", readErrors)

	// No debería haber errores
	assert.Equal(t, int32(0), writeErrors, "Write operations should not fail")
	assert.Equal(t, int32(0), readErrors, "Read operations should not fail")
}

// TestSessionManager_CacheInvalidation verifica la invalidación correcta del cache
func TestSessionManager_CacheInvalidation(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Establecer valor inicial
	err := sm.SetValue("cache-test", "key1", "initial", c)
	require.NoError(t, err)

	// Leer para cachear
	val, err := sm.GetValue("cache-test", "key1", c)
	require.NoError(t, err)
	assert.Equal(t, "initial", val)

	// Verificar que está en cache
	cacheKey := sm.getCacheKey("cache-test", c)
	sm.mu.RLock()
	_, inCache := sm.cache[cacheKey]
	sm.mu.RUnlock()
	assert.True(t, inCache, "Value should be in cache")

	// Actualizar valor
	err = sm.SetValue("cache-test", "key1", "updated", c)
	require.NoError(t, err)

	// Verificar que el cache fue invalidado
	sm.mu.RLock()
	_, inCache = sm.cache[cacheKey]
	sm.mu.RUnlock()
	assert.False(t, inCache, "Cache should be invalidated after update")

	// Leer nuevo valor
	val, err = sm.GetValue("cache-test", "key1", c)
	require.NoError(t, err)
	assert.Equal(t, "updated", val)
}

// TestSessionManager_TTLExpiration verifica la expiración del TTL
func TestSessionManager_TTLExpiration(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   200 * time.Millisecond, // TTL corto para pruebas
	}

	// Establecer valor
	err := sm.SetValue("ttl-test", "key1", "value1", c)
	require.NoError(t, err)

	// Primera lectura - debería cachear
	val, err := sm.GetValue("ttl-test", "key1", c)
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Verificar que está en cache
	cacheKey := sm.getCacheKey("ttl-test", c)
	sm.mu.RLock()
	cached, exists := sm.cache[cacheKey]
	sm.mu.RUnlock()
	assert.True(t, exists, "Should be in cache")
	assert.NotNil(t, cached)

	// Esperar casi el TTL
	time.Sleep(150 * time.Millisecond)

	// Todavía debería estar en cache
	sm.mu.RLock()
	_, exists = sm.cache[cacheKey]
	sm.mu.RUnlock()
	assert.True(t, exists, "Should still be in cache")

	// Esperar más allá del TTL
	time.Sleep(100 * time.Millisecond)

	// Ahora no debería usar el cache
	val, err = sm.GetValue("ttl-test", "key1", c)
	require.NoError(t, err)
	assert.Equal(t, "value1", val)
}

// TestSessionManager_LargePayloads prueba con payloads grandes
func TestSessionManager_LargePayloads(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Crear payload grande
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// Guardar payload grande
	err := sm.SetValue("large-test", "bigdata", largeData, c)
	require.NoError(t, err)

	// Leer payload
	val, err := sm.GetValue("large-test", "bigdata", c)
	require.NoError(t, err)

	retrievedData, ok := val.([]byte)
	require.True(t, ok, "Should be able to cast to []byte")
	assert.Equal(t, len(largeData), len(retrievedData))
}

// TestSessionManager_MultipleSessionTypes prueba diferentes tipos de sesiones
func TestSessionManager_MultipleSessionTypes(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Diferentes tipos de sesiones
	sessions := []string{"auth-session", "nflow_form", "user-prefs", "temp-data"}

	// Establecer valores en diferentes sesiones
	for _, sessName := range sessions {
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("%s-value-%d", sessName, i)
			err := sm.SetValue(sessName, key, value, c)
			require.NoError(t, err)
		}
	}

	// Verificar que cada sesión mantiene sus valores separados
	for _, sessName := range sessions {
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("key-%d", i)
			expectedValue := fmt.Sprintf("%s-value-%d", sessName, i)

			val, err := sm.GetValue(sessName, key, c)
			require.NoError(t, err)
			assert.Equal(t, expectedValue, val)
		}
	}
}

// TestSessionManager_ErrorHandling prueba el manejo de errores
func TestSessionManager_ErrorHandling(t *testing.T) {
	// Contexto con request/response inválidos
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// No configurar store - esto causará errores

	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Intentar operaciones que deberían fallar
	err := sm.SetValue("test", "key", "value", c)
	assert.Error(t, err, "Should error without proper session store")

	val, err := sm.GetValue("test", "key", c)
	assert.Error(t, err, "Should error without proper session store")
	assert.Nil(t, val)
}

// TestSessionManager_ConcurrentCacheCleanup prueba limpieza concurrente
func TestSessionManager_ConcurrentCacheCleanup(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   100 * time.Millisecond,
	}

	var wg sync.WaitGroup

	// Goroutine que añade entradas continuamente
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.SetValue("cleanup-test", fmt.Sprintf("key-%d", i), "value", c)
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine que limpia continuamente
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			sm.CleanupCache()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Goroutine que lee continuamente
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.GetValue("cleanup-test", fmt.Sprintf("key-%d", rand.Intn(1000)), c)
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()

	// No debe haber panic o deadlock
	t.Log("Concurrent cleanup test completed successfully")
}

// BenchmarkSessionManager_GetValue mide el rendimiento de GetValue
func BenchmarkSessionManager_GetValue(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Pre-poblar cache
	for i := 0; i < 100; i++ {
		sm.SetValue("bench", fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), c)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			sm.GetValue("bench", key, c)
			i++
		}
	})
}

// BenchmarkSessionManager_SetValue mide el rendimiento de SetValue
func BenchmarkSessionManager_SetValue(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			sm.SetValue("bench", key, value, c)
			i++
		}
	})
}

// BenchmarkSessionManager_MixedOperations mide operaciones mixtas
func BenchmarkSessionManager_MixedOperations(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Pre-poblar
	for i := 0; i < 100; i++ {
		sm.SetValue("bench", fmt.Sprintf("key-%d", i), "value", c)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				// 10% writes
				sm.SetValue("bench", fmt.Sprintf("key-%d", i%100), "new-value", c)
			} else {
				// 90% reads
				sm.GetValue("bench", fmt.Sprintf("key-%d", i%100), c)
			}
			i++
		}
	})
}

// TestSessionManager_MemoryLeaks verifica que no hay memory leaks
func TestSessionManager_MemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   100 * time.Millisecond,
	}

	// Forzar GC inicial
	runtime.GC()
	runtime.GC()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Realizar muchas operaciones
	for cycle := 0; cycle < 10; cycle++ {
		// Añadir 1000 entradas
		for i := 0; i < 1000; i++ {
			sm.SetValue("leak-test", fmt.Sprintf("key-%d-%d", cycle, i), "value", c)
		}

		// Esperar TTL
		time.Sleep(150 * time.Millisecond)

		// Limpiar
		sm.CleanupCache()

		// Forzar GC
		runtime.GC()
	}

	runtime.GC()
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Calcular crecimiento de memoria
	var memGrowth float64
	if m2.HeapAlloc > m1.HeapAlloc {
		memGrowth = float64(m2.HeapAlloc-m1.HeapAlloc) / 1024 / 1024
	} else {
		// Si el heap es menor, el GC liberó memoria
		memGrowth = 0
	}
	t.Logf("Memory growth after 10 cycles: %.2f MB", memGrowth)

	// No debería crecer significativamente
	assert.Less(t, memGrowth, 10.0, "Memory should not grow more than 10MB")
}
