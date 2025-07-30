package syncsession

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// Implementación con mutex simple (como está actualmente)
type SimpleMutexManager struct {
	mu sync.Mutex
}

func (sm *SimpleMutexManager) GetValue(sessionName, key string, c echo.Context) (interface{}, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return nil, err
	}
	return s.Values[key], nil
}

func (sm *SimpleMutexManager) SetValue(sessionName, key string, value interface{}, c echo.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return err
	}
	s.Values[key] = value
	return s.Save(c.Request(), c.Response())
}

// Benchmarks para operaciones de lectura
func BenchmarkSimpleMutex_Read(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}
	
	// Preparar datos
	sm.SetValue("bench-session", "test-key", "test-value", c)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sm.GetValue("bench-session", "test-key", c)
		}
	})
}

func BenchmarkSessionManager_Read_NoCache(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   0, // Sin cache
	}
	
	// Preparar datos
	sm.SetValue("bench-session", "test-key", "test-value", c)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sm.GetValue("bench-session", "test-key", c)
		}
	})
}

func BenchmarkSessionManager_Read_WithCache(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}
	
	// Preparar datos y calentar cache
	sm.SetValue("bench-session", "test-key", "test-value", c)
	sm.GetValue("bench-session", "test-key", c) // Calentar cache
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sm.GetValue("bench-session", "test-key", c)
		}
	})
}

// Benchmarks para operaciones de escritura
func BenchmarkSimpleMutex_Write(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			sm.SetValue("bench-session", key, "value", c)
			i++
		}
	})
}

func BenchmarkSessionManager_Write(b *testing.B) {
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
			sm.SetValue("bench-session", key, "value", c)
			i++
		}
	})
}

// Benchmarks para operaciones mixtas (80% lectura, 20% escritura)
func BenchmarkSimpleMutex_Mixed(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}
	
	// Preparar datos
	for i := 0; i < 100; i++ {
		sm.SetValue("bench-session", fmt.Sprintf("key-%d", i), "value", c)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%5 == 0 { // 20% escritura
				sm.SetValue("bench-session", fmt.Sprintf("key-%d", i%100), "new-value", c)
			} else { // 80% lectura
				sm.GetValue("bench-session", fmt.Sprintf("key-%d", i%100), c)
			}
			i++
		}
	})
}

func BenchmarkSessionManager_Mixed(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}
	
	// Preparar datos
	for i := 0; i < 100; i++ {
		sm.SetValue("bench-session", fmt.Sprintf("key-%d", i), "value", c)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%5 == 0 { // 20% escritura
				sm.SetValue("bench-session", fmt.Sprintf("key-%d", i%100), "new-value", c)
			} else { // 80% lectura
				sm.GetValue("bench-session", fmt.Sprintf("key-%d", i%100), c)
			}
			i++
		}
	})
}

// Benchmark para escrituras múltiples
func BenchmarkSimpleMutex_MultipleWrites(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simular escritura de 10 valores
		for j := 0; j < 10; j++ {
			sm.SetValue("bench-session", fmt.Sprintf("key-%d", j), fmt.Sprintf("value-%d", i), c)
		}
	}
}

func BenchmarkSessionManager_MultipleWrites(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Usar SetMultipleValues para escribir 10 valores de una vez
		values := make(map[string]interface{})
		for j := 0; j < 10; j++ {
			values[fmt.Sprintf("key-%d", j)] = fmt.Sprintf("value-%d", i)
		}
		sm.SetMultipleValues("bench-session", values, c)
	}
}

// Benchmark de concurrencia alta
func BenchmarkSimpleMutex_HighConcurrency(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}
	
	// Preparar datos
	for i := 0; i < 1000; i++ {
		sm.SetValue("bench-session", fmt.Sprintf("key-%d", i), "value", c)
	}
	
	b.ResetTimer()
	b.SetParallelism(100) // 100 goroutines concurrentes
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sm.GetValue("bench-session", fmt.Sprintf("key-%d", i%1000), c)
			i++
		}
	})
}

func BenchmarkSessionManager_HighConcurrency(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}
	
	// Preparar datos y calentar cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		sm.SetValue("bench-session", key, "value", c)
		sm.GetValue("bench-session", key, c) // Calentar cache
	}
	
	b.ResetTimer()
	b.SetParallelism(100) // 100 goroutines concurrentes
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sm.GetValue("bench-session", fmt.Sprintf("key-%d", i%1000), c)
			i++
		}
	})
}