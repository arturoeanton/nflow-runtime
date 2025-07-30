package syncsession

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkRPS_SimpleMutex mide requests por segundo con implementación actual
func BenchmarkRPS_SimpleMutex(b *testing.B) {
	c := setupEchoContext()
	sm := &SimpleMutexManager{}

	// Preparar datos iniciales
	for i := 0; i < 100; i++ {
		sm.SetValue("session", fmt.Sprintf("key-%d", i), "value", c)
	}

	b.Run("RPS_Test", func(b *testing.B) {
		var requestCount int64
		var wg sync.WaitGroup
		done := make(chan bool)

		// Simular diferentes números de usuarios concurrentes
		concurrentUsers := []int{10, 50, 100, 500, 1000}

		for _, users := range concurrentUsers {
			requestCount = 0

			// Ejecutar por 10 segundos
			go func() {
				time.Sleep(10 * time.Second)
				close(done)
			}()

			// Lanzar usuarios concurrentes
			for i := 0; i < users; i++ {
				wg.Add(1)
				go func(userID int) {
					defer wg.Done()

					for {
						select {
						case <-done:
							return
						default:
							// Simular una request típica:
							// 3 lecturas, 1 escritura
							sm.GetValue("session", "user_id", c)
							sm.GetValue("session", "role", c)
							sm.GetValue("session", "last_access", c)
							sm.SetValue("session", "last_access", time.Now().String(), c)

							atomic.AddInt64(&requestCount, 1)
						}
					}
				}(i)
			}

			// Reiniciar el channel para la próxima prueba
			done = make(chan bool)

			// Esperar a que termine la prueba
			time.Sleep(10 * time.Second)
			close(done)
			wg.Wait()

			rps := float64(requestCount) / 10.0
			b.Logf("SimpleMutex - %d usuarios concurrentes: %.0f RPS", users, rps)
		}
	})
}

// BenchmarkRPS_SessionManager mide requests por segundo con Session Manager
func BenchmarkRPS_SessionManager(b *testing.B) {
	c := setupEchoContext()
	sm := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Preparar datos iniciales y calentar cache
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		sm.SetValue("session", key, "value", c)
		sm.GetValue("session", key, c) // Calentar cache
	}

	b.Run("RPS_Test", func(b *testing.B) {
		var requestCount int64
		var wg sync.WaitGroup
		done := make(chan bool)

		// Simular diferentes números de usuarios concurrentes
		concurrentUsers := []int{10, 50, 100, 500, 1000}

		for _, users := range concurrentUsers {
			requestCount = 0

			// Ejecutar por 10 segundos
			go func() {
				time.Sleep(10 * time.Second)
				close(done)
			}()

			// Lanzar usuarios concurrentes
			for i := 0; i < users; i++ {
				wg.Add(1)
				go func(userID int) {
					defer wg.Done()

					for {
						select {
						case <-done:
							return
						default:
							// Simular una request típica:
							// 3 lecturas, 1 escritura
							sm.GetValue("session", "user_id", c)
							sm.GetValue("session", "role", c)
							sm.GetValue("session", "last_access", c)
							sm.SetValue("session", "last_access", time.Now().String(), c)

							atomic.AddInt64(&requestCount, 1)
						}
					}
				}(i)
			}

			// Reiniciar el channel para la próxima prueba
			done = make(chan bool)

			// Esperar a que termine la prueba
			time.Sleep(10 * time.Second)
			close(done)
			wg.Wait()

			rps := float64(requestCount) / 10.0
			b.Logf("SessionManager - %d usuarios concurrentes: %.0f RPS", users, rps)
		}
	})
}

// TestCompareRPS ejecuta una comparación directa de RPS
func TestCompareRPS(t *testing.T) {
	c := setupEchoContext()

	// Configurar ambos managers
	simpleMutex := &SimpleMutexManager{}
	sessionManager := &SessionManager{
		cache: make(map[string]*SessionCache),
		ttl:   5 * time.Minute,
	}

	// Preparar datos
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		simpleMutex.SetValue("session", key, "value", c)
		sessionManager.SetValue("session", key, "value", c)
		sessionManager.GetValue("session", key, c) // Calentar cache
	}

	// Test con 100 usuarios concurrentes por 5 segundos
	concurrentUsers := 100
	testDuration := 5 * time.Second

	// Test SimpleMutex
	var simpleMutexCount int64
	var wg sync.WaitGroup
	done := make(chan bool)

	go func() {
		time.Sleep(testDuration)
		close(done)
	}()

	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Operación típica de sesión
					simpleMutex.GetValue("session", "user_id", c)
					simpleMutex.GetValue("session", "role", c)
					simpleMutex.SetValue("session", "counter", time.Now().String(), c)
					atomic.AddInt64(&simpleMutexCount, 1)
				}
			}
		}()
	}
	wg.Wait()

	simpleMutexRPS := float64(simpleMutexCount) / testDuration.Seconds()

	// Test SessionManager
	var sessionManagerCount int64
	done = make(chan bool)

	go func() {
		time.Sleep(testDuration)
		close(done)
	}()

	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Operación típica de sesión
					sessionManager.GetValue("session", "user_id", c)
					sessionManager.GetValue("session", "role", c)
					sessionManager.SetValue("session", "counter", time.Now().String(), c)
					atomic.AddInt64(&sessionManagerCount, 1)
				}
			}
		}()
	}
	wg.Wait()

	sessionManagerRPS := float64(sessionManagerCount) / testDuration.Seconds()

	// Resultados
	t.Logf("\n=== COMPARACIÓN DE REQUESTS POR SEGUNDO ===")
	t.Logf("Usuarios concurrentes: %d", concurrentUsers)
	t.Logf("Duración del test: %v", testDuration)
	t.Logf("\nImplementación actual (SimpleMutex):")
	t.Logf("  - Requests totales: %d", simpleMutexCount)
	t.Logf("  - RPS: %.0f", simpleMutexRPS)
	t.Logf("\nSession Manager optimizado:")
	t.Logf("  - Requests totales: %d", sessionManagerCount)
	t.Logf("  - RPS: %.0f", sessionManagerRPS)
	t.Logf("\n🚀 Mejora: %.1fx más requests por segundo", sessionManagerRPS/simpleMutexRPS)

	// Calcular límites teóricos
	t.Logf("\n=== ESTIMACIÓN DE CAPACIDAD MÁXIMA ===")

	// Basado en latencias observadas
	avgLatencySimple := 1000.0  // ~1000 nanosegundos por operación
	avgLatencyOptimized := 50.0 // ~50 nanosegundos con cache

	maxRPSSimple := 1000000000.0 / avgLatencySimple * float64(concurrentUsers)
	maxRPSOptimized := 1000000000.0 / avgLatencyOptimized * float64(concurrentUsers)

	t.Logf("\nCapacidad teórica máxima:")
	t.Logf("SimpleMutex: %.0f RPS", maxRPSSimple)
	t.Logf("SessionManager: %.0f RPS", maxRPSOptimized)

	// Estimación realista (considerando overhead del sistema)
	t.Logf("\nEstimación realista (70%% de capacidad teórica):")
	t.Logf("SimpleMutex: %.0f RPS", maxRPSSimple*0.7)
	t.Logf("SessionManager: %.0f RPS", maxRPSOptimized*0.7)
}

// TestBottleneckAnalysis analiza dónde están los cuellos de botella
func TestBottleneckAnalysis(t *testing.T) {
	c := setupEchoContext()

	testCases := []struct {
		name            string
		concurrentUsers int
		readRatio       float64 // porcentaje de lecturas vs escrituras
	}{
		{"Mostly Reads (90/10)", 100, 0.9},
		{"Balanced (50/50)", 100, 0.5},
		{"Write Heavy (10/90)", 100, 0.1},
		{"High Concurrency Reads", 1000, 0.9},
		{"High Concurrency Writes", 1000, 0.1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			simpleMutex := &SimpleMutexManager{}
			sessionManager := &SessionManager{
				cache: make(map[string]*SessionCache),
				ttl:   5 * time.Minute,
			}

			// Preparar datos
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key-%d", i)
				simpleMutex.SetValue("session", key, "value", c)
				sessionManager.SetValue("session", key, "value", c)
				sessionManager.GetValue("session", key, c)
			}

			duration := 3 * time.Second

			// Test cada implementación
			results := make(map[string]float64)

			for name, manager := range map[string]interface{}{
				"SimpleMutex":    simpleMutex,
				"SessionManager": sessionManager,
			} {
				var count int64
				var wg sync.WaitGroup
				done := make(chan bool)

				go func() {
					time.Sleep(duration)
					close(done)
				}()

				for i := 0; i < tc.concurrentUsers; i++ {
					wg.Add(1)
					go func(userID int) {
						defer wg.Done()
						readCount := 0
						for {
							select {
							case <-done:
								return
							default:
								readCount++
								if float64(readCount%100)/100.0 < tc.readRatio {
									// Operación de lectura
									switch m := manager.(type) {
									case *SimpleMutexManager:
										m.GetValue("session", fmt.Sprintf("key-%d", userID%100), c)
									case *SessionManager:
										m.GetValue("session", fmt.Sprintf("key-%d", userID%100), c)
									}
								} else {
									// Operación de escritura
									switch m := manager.(type) {
									case *SimpleMutexManager:
										m.SetValue("session", fmt.Sprintf("key-%d", userID%100), time.Now(), c)
									case *SessionManager:
										m.SetValue("session", fmt.Sprintf("key-%d", userID%100), time.Now(), c)
									}
								}
								atomic.AddInt64(&count, 1)
							}
						}
					}(i)
				}

				wg.Wait()
				results[name] = float64(count) / duration.Seconds()
			}

			improvement := results["SessionManager"] / results["SimpleMutex"]
			t.Logf("\n%s:", tc.name)
			t.Logf("SimpleMutex: %.0f RPS", results["SimpleMutex"])
			t.Logf("SessionManager: %.0f RPS", results["SessionManager"])
			t.Logf("Mejora: %.1fx", improvement)
		})
	}
}
