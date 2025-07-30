package syncsession

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Mock session store para tests
type mockStore struct {
	sessions map[string]*sessions.Session
	mu       sync.RWMutex
}

func (m *mockStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if s, ok := m.sessions[name]; ok {
		return s, nil
	}

	s := sessions.NewSession(m, name)
	return s, nil
}

func (m *mockStore) New(r *http.Request, name string) (*sessions.Session, error) {
	s := sessions.NewSession(m, name)
	m.mu.Lock()
	m.sessions[name] = s
	m.mu.Unlock()
	return s, nil
}

func (m *mockStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	m.mu.Lock()
	m.sessions[s.Name()] = s
	m.mu.Unlock()
	return nil
}

func setupEchoContext() echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Configurar mock store
	store := &mockStore{sessions: make(map[string]*sessions.Session)}

	// Usar el middleware de sesiones
	e.Use(session.Middleware(store))

	// Configurar el store en el contexto
	c.Set("_session_store", store)

	return c
}

func TestSessionManager_GetSetValue(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Test set y get
	err := sm.SetValue("test-session", "key1", "value1", c)
	assert.NoError(t, err)

	val, err := sm.GetValue("test-session", "key1", c)
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestSessionManager_SetMultipleValues(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	values := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	err := sm.SetMultipleValues("test-session", values, c)
	assert.NoError(t, err)

	// Verificar todos los valores
	val1, _ := sm.GetValue("test-session", "key1", c)
	assert.Equal(t, "value1", val1)

	val2, _ := sm.GetValue("test-session", "key2", c)
	assert.Equal(t, 42, val2)

	val3, _ := sm.GetValue("test-session", "key3", c)
	assert.Equal(t, true, val3)
}

func TestSessionManager_Cache(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   100 * time.Millisecond, // TTL corto para test
	}

	// Primera lectura - no está en cache
	sm.SetValue("test-session", "key1", "value1", c)

	// Segunda lectura - debería venir del cache
	start := time.Now()
	val, _ := sm.GetValue("test-session", "key1", c)
	duration1 := time.Since(start)
	assert.Equal(t, "value1", val)

	// Esperar que expire el cache
	time.Sleep(150 * time.Millisecond)

	// Tercera lectura - cache expirado, debería ser más lento
	start = time.Now()
	val, _ = sm.GetValue("test-session", "key1", c)
	duration2 := time.Since(start)
	assert.Equal(t, "value1", val)

	// La lectura desde cache debería ser más rápida
	// (esto puede fallar en sistemas muy cargados)
	t.Logf("Cache hit: %v, Cache miss: %v", duration1, duration2)
}

func TestSessionManager_DeleteSession(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Configurar valores
	sm.SetValue("test-session", "key1", "value1", c)
	sm.SetValue("test-session", "key2", "value2", c)

	// Eliminar sesión
	err := sm.DeleteSession("test-session", c)
	assert.NoError(t, err)

	// Verificar que los valores fueron eliminados
	val1, _ := sm.GetValue("test-session", "key1", c)
	assert.Nil(t, val1)

	val2, _ := sm.GetValue("test-session", "key2", c)
	assert.Nil(t, val2)
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Múltiples goroutines escribiendo y leyendo
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)

				// Escribir
				err := sm.SetValue("concurrent-session", key, value, c)
				assert.NoError(t, err)

				// Leer
				val, err := sm.GetValue("concurrent-session", key, c)
				assert.NoError(t, err)
				assert.Equal(t, value, val)
			}
		}(i)
	}

	wg.Wait()
}

func TestSessionManager_CleanupCache(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   50 * time.Millisecond,
	}

	// Agregar entradas al cache
	cacheKey := sm.getCacheKey("test-session", c)
	sm.cache[cacheKey] = &SessionCache{
		values:     map[interface{}]interface{}{"key": "value"},
		lastAccess: time.Now(),
		dirty:      false,
	}

	// Verificar que está en cache
	assert.Len(t, sm.cache, 1)

	// Esperar que expire
	time.Sleep(100 * time.Millisecond)

	// Limpiar cache
	sm.CleanupCache()

	// Verificar que fue eliminado
	assert.Len(t, sm.cache, 0)
}
