package syncsession

import (
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// EJEMPLO DE MIGRACIÓN
// Este archivo muestra cómo migrar del código actual al Session Manager

// ============================================
// ANTES: Usando mutex simple
// ============================================

func OldSetSessionValue(name, key, value string, c echo.Context) {
	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()
	s, _ := session.Get(name, c)
	s.Values[key] = value
	s.Save(c.Request(), c.Response())
}

func OldGetSessionValue(name, key string, c echo.Context) interface{} {
	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()
	s, _ := session.Get(name, c)
	return s.Values[key]
}

func OldDeleteSession(name string, c echo.Context) {
	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()
	s, _ := session.Get(name, c)
	for k := range s.Values {
		delete(s.Values, k)
	}
	s.Save(c.Request(), c.Response())
}

// ============================================
// DESPUÉS: Usando Session Manager
// ============================================

func NewSetSessionValue(name, key string, value interface{}, c echo.Context) {
	Manager.SetValue(name, key, value, c)
}

func NewGetSessionValue(name, key string, c echo.Context) interface{} {
	val, _ := Manager.GetValue(name, key, c)
	return val
}

func NewDeleteSession(name string, c echo.Context) {
	Manager.DeleteSession(name, c)
}

// ============================================
// FUNCIONES DE AYUDA PARA MIGRACIÓN
// ============================================

// MigrateSetSession es un wrapper que mantiene la compatibilidad
// pero usa el Session Manager internamente
func MigrateSetSession(name, key string, value interface{}, c echo.Context, useNewImplementation bool) {
	if useNewImplementation {
		Manager.SetValue(name, key, value, c)
	} else {
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		s.Values[key] = value
		s.Save(c.Request(), c.Response())
	}
}

// MigrateGetSession permite cambiar entre implementaciones con un flag
func MigrateGetSession(name, key string, c echo.Context, useNewImplementation bool) interface{} {
	if useNewImplementation {
		val, _ := Manager.GetValue(name, key, c)
		return val
	} else {
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		return s.Values[key]
	}
}

// ============================================
// EJEMPLO DE USO EN PLAYBOOK
// ============================================

// Ejemplo de cómo actualizar feature_session.go gradualmente
func ExampleFeatureSessionMigration(useNewImpl bool) {
	// En el código real, esto vendría del contexto
	var c echo.Context

	// Ejemplo 1: Migrar set_session
	if useNewImpl {
		// Nueva implementación
		Manager.SetValue("session-name", "key", "value", c)
	} else {
		// Implementación actual
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()
		s, _ := session.Get("session-name", c)
		s.Values["key"] = "value"
		s.Save(c.Request(), c.Response())
	}

	// Ejemplo 2: Migrar operaciones múltiples
	if useNewImpl {
		// Con Session Manager - una sola operación atómica
		values := map[string]interface{}{
			"user_id": "123",
			"role":    "admin",
			"token":   "abc123",
		}
		Manager.SetMultipleValues("auth-session", values, c)
	} else {
		// Implementación actual - múltiples locks
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()
		s, _ := session.Get("auth-session", c)
		s.Values["user_id"] = "123"
		s.Values["role"] = "admin"
		s.Values["token"] = "abc123"
		s.Save(c.Request(), c.Response())
	}
}

// ============================================
// PLAN DE MIGRACIÓN RECOMENDADO
// ============================================

/*
FASE 1: Preparación (1-2 días)
1. Instalar el Session Manager en paralelo con el código actual
2. Agregar flags de feature para controlar qué implementación usar
3. Crear tests que verifiquen que ambas implementaciones dan los mismos resultados

FASE 2: Migración Gradual (1-2 semanas)
1. Empezar con módulos de bajo riesgo (ej: logging)
2. Migrar módulo por módulo usando flags de feature
3. Monitorear métricas de performance y errores

FASE 3: Validación (3-5 días)
1. Ejecutar ambas implementaciones en paralelo
2. Comparar resultados
3. Hacer pruebas de carga

FASE 4: Finalización
1. Remover código antiguo
2. Eliminar flags de feature
3. Optimizar configuración del Session Manager

EJEMPLO DE FLAG DE FEATURE:
*/

type Config struct {
	UseSessionManager bool
	CacheTTL          int // minutos
	CleanupInterval   int // minutos
}

var MigrationConfig = Config{
	UseSessionManager: false, // Cambiar a true para activar
	CacheTTL:          5,
	CleanupInterval:   10,
}
