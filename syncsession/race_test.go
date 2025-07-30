// +build race

package syncsession

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestRaceCondition_BasicOperations prueba operaciones básicas con detector de race
func TestRaceCondition_BasicOperations(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	var wg sync.WaitGroup

	// Escritor 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.SetValue("race-test", "shared-key", fmt.Sprintf("writer1-%d", i), c)
		}
	}()

	// Escritor 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.SetValue("race-test", "shared-key", fmt.Sprintf("writer2-%d", i), c)
		}
	}()

	// Lector 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.GetValue("race-test", "shared-key", c)
		}
	}()

	// Lector 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			sm.GetValue("race-test", "shared-key", c)
		}
	}()

	// Limpiador
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			sm.CleanupCache()
		}
	}()

	wg.Wait()
}

// TestRaceCondition_CacheOperations prueba race conditions específicas del cache
func TestRaceCondition_CacheOperations(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   50 * time.Millisecond, // TTL corto para forzar expiración
	}

	var wg sync.WaitGroup
	numWorkers := 20
	
	// Workers que continuamente leen/escriben/limpian
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 500; j++ {
				operation := rand.Intn(4)
				key := fmt.Sprintf("key-%d", rand.Intn(10))
				
				switch operation {
				case 0: // Get
					sm.GetValue("race-cache", key, c)
				case 1: // Set
					sm.SetValue("race-cache", key, fmt.Sprintf("value-%d-%d", id, j), c)
				case 2: // SetMultiple
					values := map[string]interface{}{
						key + "-1": "value1",
						key + "-2": "value2",
					}
					sm.SetMultipleValues("race-cache", values, c)
				case 3: // Cleanup
					sm.CleanupCache()
				}
				
				// Pequeña pausa aleatoria
				if rand.Intn(10) == 0 {
					time.Sleep(time.Microsecond * time.Duration(rand.Intn(100)))
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestRaceCondition_SessionDelete prueba race conditions al eliminar sesiones
func TestRaceCondition_SessionDelete(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	var wg sync.WaitGroup

	// Escritores
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sm.SetValue("delete-test", fmt.Sprintf("key-%d", j), "value", c)
			}
		}(i)
	}

	// Lectores
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sm.GetValue("delete-test", fmt.Sprintf("key-%d", j), c)
			}
		}(i)
	}

	// Eliminadores
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				time.Sleep(5 * time.Millisecond)
				sm.DeleteSession("delete-test", c)
			}
		}(i)
	}

	wg.Wait()
}

// TestRaceCondition_GetSession prueba race conditions con GetSession/SaveSession
func TestRaceCondition_GetSession(t *testing.T) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	var wg sync.WaitGroup

	// Workers que obtienen y modifican sesiones completas
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 100; j++ {
				// GetSession
				sess, err := sm.GetSession("full-session", c)
				if err != nil {
					continue
				}
				
				// Modificar valores
				sess.Values[fmt.Sprintf("worker-%d", id)] = j
				
				// SaveSession
				sm.SaveSession("full-session", c, sess)
			}
		}(i)
	}

	// Workers que usan operaciones individuales
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 100; j++ {
				sm.SetValue("full-session", fmt.Sprintf("individual-%d", id), j, c)
				sm.GetValue("full-session", fmt.Sprintf("worker-%d", id), c)
			}
		}(i)
	}

	wg.Wait()
}

// TestRaceCondition_CacheKeyGeneration prueba race conditions en getCacheKey
func TestRaceCondition_CacheKeyGeneration(t *testing.T) {
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	var wg sync.WaitGroup

	// Múltiples contextos concurrentes
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Crear contexto único para cada goroutine
			c := setupEchoContext()
			
			// Si es par, añadir cookie
			if id%2 == 0 {
				c.Request().AddCookie(&http.Cookie{
					Name:  "session",
					Value: fmt.Sprintf("session-%d", id),
				})
			}
			
			for j := 0; j < 100; j++ {
				// Operaciones que usan getCacheKey internamente
				sm.SetValue("key-test", "key", fmt.Sprintf("value-%d-%d", id, j), c)
				sm.GetValue("key-test", "key", c)
				
				if j%10 == 0 {
					sm.CleanupCache()
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestRaceCondition_ComplexScenario prueba un escenario complejo y realista
func TestRaceCondition_ComplexScenario(t *testing.T) {
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   100 * time.Millisecond,
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Simulación de usuarios concurrentes
	numUsers := 50
	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			
			// Cada usuario tiene su propio contexto
			c := setupEchoContext()
			c.Request().AddCookie(&http.Cookie{
				Name:  "session",
				Value: fmt.Sprintf("user-%d", userID),
			})
			
			// Simular actividad del usuario
			for {
				select {
				case <-stopChan:
					return
				default:
					// Operación aleatoria
					switch rand.Intn(5) {
					case 0: // Login
						sm.SetValue("auth-session", "userID", userID, c)
						sm.SetValue("auth-session", "loginTime", time.Now(), c)
					case 1: // Update profile
						profile := map[string]interface{}{
							"name":  fmt.Sprintf("User %d", userID),
							"email": fmt.Sprintf("user%d@example.com", userID),
						}
						sm.SetMultipleValues("user-profile", profile, c)
					case 2: // Read profile
						sm.GetValue("user-profile", "name", c)
						sm.GetValue("user-profile", "email", c)
					case 3: // Update form data
						sm.SetValue("nflow_form", fmt.Sprintf("field-%d", rand.Intn(10)), "value", c)
					case 4: // Logout
						sm.DeleteSession("auth-session", c)
					}
					
					// Pausa aleatoria entre operaciones
					time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				}
			}
		}(i)
	}

	// Rutina de limpieza agresiva
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopChan:
				return
			default:
				sm.CleanupCache()
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()

	// Ejecutar por 5 segundos
	time.Sleep(5 * time.Second)
	close(stopChan)
	wg.Wait()
}

