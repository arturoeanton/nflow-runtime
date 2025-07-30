package cache

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	// Test set y get básico
	c.Set("key1", "value1")
	
	val, ok := c.Get("key1")
	if !ok {
		t.Error("Expected to find key1")
	}
	
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}
	
	// Test key no existente
	_, ok = c.Get("nonexistent")
	if ok {
		t.Error("Should not find nonexistent key")
	}
}

func TestCache_Expiration(t *testing.T) {
	c := NewCache(100 * time.Millisecond)
	
	c.Set("expire", "value")
	
	// Debería existir inmediatamente
	_, ok := c.Get("expire")
	if !ok {
		t.Error("Key should exist immediately after setting")
	}
	
	// Esperar a que expire
	time.Sleep(150 * time.Millisecond)
	
	// No debería existir después de expirar
	_, ok = c.Get("expire")
	if ok {
		t.Error("Key should have expired")
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	c := NewCache(1 * time.Hour) // TTL por defecto largo
	
	// Establecer con TTL corto
	c.SetWithTTL("shortlived", "value", 100*time.Millisecond)
	
	// Debería existir
	_, ok := c.Get("shortlived")
	if !ok {
		t.Error("Key should exist")
	}
	
	// Esperar expiración
	time.Sleep(150 * time.Millisecond)
	
	// Debería haber expirado
	_, ok = c.Get("shortlived")
	if ok {
		t.Error("Key should have expired with custom TTL")
	}
}

func TestCache_Delete(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	c.Set("delete_me", "value")
	
	// Verificar que existe
	_, ok := c.Get("delete_me")
	if !ok {
		t.Error("Key should exist before deletion")
	}
	
	// Eliminar
	c.Delete("delete_me")
	
	// Verificar que no existe
	_, ok = c.Get("delete_me")
	if ok {
		t.Error("Key should not exist after deletion")
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	// Agregar múltiples elementos
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")
	
	if c.Size() != 3 {
		t.Errorf("Expected size 3, got %d", c.Size())
	}
	
	// Limpiar caché
	c.Clear()
	
	if c.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", c.Size())
	}
	
	// Verificar que los elementos no existen
	_, ok := c.Get("key1")
	if ok {
		t.Error("Cache should be empty after clear")
	}
}

func TestCache_GetOrCompute(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	computeCount := 0
	compute := func() (interface{}, error) {
		computeCount++
		return "computed_value", nil
	}
	
	// Primera llamada - debería computar
	val, err := c.GetOrCompute("compute_key", compute)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("Expected computed_value, got %v", val)
	}
	if computeCount != 1 {
		t.Errorf("Expected compute to be called once, called %d times", computeCount)
	}
	
	// Segunda llamada - debería venir del caché
	val, err = c.GetOrCompute("compute_key", compute)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("Expected computed_value, got %v", val)
	}
	if computeCount != 1 {
		t.Errorf("Expected compute to still be called once, called %d times", computeCount)
	}
}

func TestCache_GetOrComputeError(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	compute := func() (interface{}, error) {
		return nil, errors.New("compute error")
	}
	
	// Debería retornar el error
	_, err := c.GetOrCompute("error_key", compute)
	if err == nil {
		t.Error("Expected error from compute function")
	}
	
	// No debería haber guardado nada en caché
	_, ok := c.Get("error_key")
	if ok {
		t.Error("Should not cache on error")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(5 * time.Minute)
	
	var wg sync.WaitGroup
	goroutines := 100
	iterations := 1000
	
	// Múltiples goroutines escribiendo y leyendo
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				
				c.Set(key, value)
				
				if val, ok := c.Get(key); ok {
					if val != value {
						t.Errorf("Concurrent access: expected %s, got %v", value, val)
					}
				}
				
				if j%10 == 0 {
					c.Delete(key)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func TestCache_CleanupExpired(t *testing.T) {
	// Test con cleanup más frecuente para testing
	c := &Cache{
		items: make(map[string]*CacheItem),
		ttl:   100 * time.Millisecond,
	}
	
	// Iniciar cleanup manual con frecuencia más alta
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		
		for i := 0; i < 5; i++ {
			<-ticker.C
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.items {
				if now.After(item.Expiration) {
					delete(c.items, key)
				}
			}
			c.mu.Unlock()
		}
	}()
	
	// Agregar elementos con diferentes expiraciones
	c.SetWithTTL("expire1", "value1", 50*time.Millisecond)
	c.SetWithTTL("expire2", "value2", 150*time.Millisecond)
	c.SetWithTTL("expire3", "value3", 250*time.Millisecond)
	
	// Verificar estado inicial
	if c.Size() != 3 {
		t.Errorf("Expected 3 items, got %d", c.Size())
	}
	
	// Esperar primera limpieza
	time.Sleep(100 * time.Millisecond)
	
	// expire1 debería haber sido eliminado
	if _, ok := c.Get("expire1"); ok {
		t.Error("expire1 should have been cleaned up")
	}
	
	// expire2 y expire3 deberían existir todavía
	if _, ok := c.Get("expire2"); !ok {
		t.Error("expire2 should still exist")
	}
	if _, ok := c.Get("expire3"); !ok {
		t.Error("expire3 should still exist")
	}
}

func BenchmarkCache_Set(b *testing.B) {
	c := NewCache(5 * time.Minute)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	c := NewCache(5 * time.Minute)
	
	// Pre-poblar caché
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("key%d", i%1000))
	}
}

func BenchmarkCache_ConcurrentAccess(b *testing.B) {
	c := NewCache(5 * time.Minute)
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%1000)
			if i%2 == 0 {
				c.Set(key, i)
			} else {
				c.Get(key)
			}
			i++
		}
	})
}