package engine

import (
	"sync"
)

// SessionManager interfaz para manejo de sesiones
type SessionManager interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// defaultSessionManager implementación por defecto
type defaultSessionManager struct {
	mu sync.RWMutex
}

func (m *defaultSessionManager) Lock()    { m.mu.Lock() }
func (m *defaultSessionManager) Unlock()  { m.mu.Unlock() }
func (m *defaultSessionManager) RLock()   { m.mu.RLock() }
func (m *defaultSessionManager) RUnlock() { m.mu.RUnlock() }

// EchoSessionsMutex para compatibilidad
var EchoSessionsMutex = &defaultSessionManager{}

// PayloadSessionMutex para compatibilidad  
var PayloadSessionMutex = &defaultSessionManager{}