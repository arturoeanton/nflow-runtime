package cache

import (
	"sync"
	"time"
)

// CacheItem representa un elemento en el caché con su tiempo de expiración
type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

// Cache es una implementación thread-safe de caché en memoria
type Cache struct {
	items map[string]*CacheItem
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewCache crea una nueva instancia de caché con el TTL especificado
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		items: make(map[string]*CacheItem),
		ttl:   ttl,
	}
	
	// Iniciar limpieza periódica
	go c.cleanupExpired()
	
	return c
}

// Get obtiene un valor del caché
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return nil, false
	}
	
	// Verificar si ha expirado
	if time.Now().After(item.Expiration) {
		return nil, false
	}
	
	return item.Value, true
}

// Set establece un valor en el caché
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL establece un valor con un TTL específico
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
}

// Delete elimina un elemento del caché
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.items, key)
}

// Clear limpia todo el caché
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*CacheItem)
}

// Size retorna el número de elementos en el caché
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.items)
}

// cleanupExpired elimina periódicamente elementos expirados
func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.Expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// GetOrCompute obtiene un valor del caché o lo calcula si no existe
func (c *Cache) GetOrCompute(key string, compute func() (interface{}, error)) (interface{}, error) {
	// Intentar obtener del caché
	if val, ok := c.Get(key); ok {
		return val, nil
	}
	
	// Calcular valor
	val, err := compute()
	if err != nil {
		return nil, err
	}
	
	// Guardar en caché
	c.Set(key, val)
	
	return val, nil
}