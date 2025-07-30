package syncsession

import (
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// SessionManager maneja sesiones con concurrencia optimizada
type SessionManager struct {
	mu    sync.RWMutex
	cache map[string]*SessionCache
	ttl   time.Duration
}

type SessionCache struct {
	values     map[interface{}]interface{}
	lastAccess time.Time
	dirty      bool
}

var Manager = &SessionManager{
	cache: make(map[string]*SessionCache),
	ttl:   5 * time.Minute, // Cache por 5 minutos
}

// GetValue obtiene un valor de la sesión (operación de lectura)
func (sm *SessionManager) GetValue(sessionName, key string, c echo.Context) (interface{}, error) {
	cacheKey := sm.getCacheKey(sessionName, c)
	
	// Primero intentar desde cache
	sm.mu.RLock()
	if cached, ok := sm.cache[cacheKey]; ok && time.Since(cached.lastAccess) < sm.ttl {
		value := cached.values[key]
		sm.mu.RUnlock()
		return value, nil
	}
	sm.mu.RUnlock()

	// Si no está en cache, obtener de la sesión
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return nil, err
	}
	
	// Actualizar cache
	sm.cache[cacheKey] = &SessionCache{
		values:     s.Values,
		lastAccess: time.Now(),
		dirty:      false,
	}
	
	return s.Values[key], nil
}

// SetValue establece un valor en la sesión (operación de escritura)
func (sm *SessionManager) SetValue(sessionName, key string, value interface{}, c echo.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return err
	}
	
	s.Values[key] = value
	err = s.Save(c.Request(), c.Response())
	
	// Invalidar cache
	cacheKey := sm.getCacheKey(sessionName, c)
	delete(sm.cache, cacheKey)
	
	return err
}

// SetMultipleValues establece múltiples valores de forma atómica
func (sm *SessionManager) SetMultipleValues(sessionName string, values map[string]interface{}, c echo.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return err
	}
	
	for k, v := range values {
		s.Values[k] = v
	}
	
	err = s.Save(c.Request(), c.Response())
	
	// Invalidar cache
	cacheKey := sm.getCacheKey(sessionName, c)
	delete(sm.cache, cacheKey)
	
	return err
}

// GetSession obtiene toda la sesión (para operaciones complejas)
func (sm *SessionManager) GetSession(sessionName string, c echo.Context) (*sessions.Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return session.Get(sessionName, c)
}

// SaveSession guarda la sesión después de modificaciones
func (sm *SessionManager) SaveSession(sessionName string, c echo.Context, s *sessions.Session) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	err := s.Save(c.Request(), c.Response())
	
	// Invalidar cache
	cacheKey := sm.getCacheKey(sessionName, c)
	delete(sm.cache, cacheKey)
	
	return err
}

// DeleteSession elimina todos los valores de una sesión
func (sm *SessionManager) DeleteSession(sessionName string, c echo.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	s, err := session.Get(sessionName, c)
	if err != nil {
		return err
	}
	
	// Limpiar valores
	for k := range s.Values {
		delete(s.Values, k)
	}
	
	err = s.Save(c.Request(), c.Response())
	
	// Eliminar de cache
	cacheKey := sm.getCacheKey(sessionName, c)
	delete(sm.cache, cacheKey)
	
	return err
}

// CleanupCache limpia entradas antiguas del cache
func (sm *SessionManager) CleanupCache() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	for key, cached := range sm.cache {
		if now.Sub(cached.lastAccess) > sm.ttl {
			delete(sm.cache, key)
		}
	}
}

func (sm *SessionManager) getCacheKey(sessionName string, c echo.Context) string {
	// Usar session ID si está disponible, sino usar IP
	if cookie, err := c.Cookie("session"); err == nil {
		return sessionName + ":" + cookie.Value
	}
	return sessionName + ":" + c.RealIP()
}

// StartCleanupRoutine inicia una rutina de limpieza periódica
func (sm *SessionManager) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			sm.CleanupCache()
		}
	}()
}