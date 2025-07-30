# Session Manager - Optimización de Performance

## Resumen

El Session Manager es una implementación optimizada para el manejo de sesiones que resuelve los problemas de race conditions y mejora significativamente el performance en aplicaciones con alta concurrencia.

## Comparación de Implementaciones

### Implementación Actual (SimpleMutex)
- Usa `sync.Mutex` que bloquea todas las operaciones
- Sin cache - cada operación accede al session store
- Todas las goroutines esperan incluso para lecturas

### Session Manager (Optimizado)
- Usa `sync.RWMutex` permitiendo múltiples lecturas simultáneas
- Cache en memoria con TTL configurable
- Operaciones batch para múltiples valores
- Limpieza automática de cache

## Benchmarks de Performance

Ejecutar benchmarks:
```bash
cd pkg/syncsession
./run_benchmarks.sh
```

### Resultados Esperados

| Operación | SimpleMutex | SessionManager | Mejora |
|-----------|-------------|----------------|---------|
| Lectura (con cache) | ~1000 ns/op | ~50 ns/op | **20x más rápido** |
| Lectura (sin cache) | ~1000 ns/op | ~800 ns/op | 1.25x más rápido |
| Escritura | ~1500 ns/op | ~1400 ns/op | Similar |
| Mixto (80/20) | ~1200 ns/op | ~400 ns/op | **3x más rápido** |
| Alta concurrencia | ~5000 ns/op | ~100 ns/op | **50x más rápido** |

## Uso

### API Básica

```go
import "github.com/arturoeanton/nFlow/pkg/syncsession"

// Obtener un valor
value, err := syncsession.Manager.GetValue("session-name", "key", c)

// Establecer un valor
err := syncsession.Manager.SetValue("session-name", "key", "value", c)

// Establecer múltiples valores (más eficiente)
values := map[string]interface{}{
    "key1": "value1",
    "key2": 42,
    "key3": true,
}
err := syncsession.Manager.SetMultipleValues("session-name", values, c)

// Eliminar una sesión
err := syncsession.Manager.DeleteSession("session-name", c)
```

### Migración Gradual

Para migrar sin riesgos, usa flags de feature:

```go
// En tu configuración
useNewSessionManager := os.Getenv("USE_SESSION_MANAGER") == "true"

// En tu código
if useNewSessionManager {
    syncsession.Manager.SetValue(name, key, value, c)
} else {
    // Código actual con mutex
    syncsession.EchoSessionsMutex.Lock()
    defer syncsession.EchoSessionsMutex.Unlock()
    s, _ := session.Get(name, c)
    s.Values[key] = value
    s.Save(c.Request(), c.Response())
}
```

## Tests

### Tests Unitarios
```bash
go test -v -run Test
```

### Tests de Integración
```bash
go test -v -run TestPerformanceComparison
go test -v -run TestCacheEffectiveness
go test -v -run TestMemoryUsage
```

## Configuración

### Opciones del Session Manager

```go
// Personalizar configuración
sm := &SessionManager{
    cache: make(map[string]*SessionCache),
    ttl:   10 * time.Minute,  // Tiempo de vida del cache
}

// Iniciar limpieza automática
sm.StartCleanupRoutine()
```

## Mejores Prácticas

1. **Usa operaciones batch**: Si necesitas establecer múltiples valores, usa `SetMultipleValues`
2. **Cache warming**: Para sesiones críticas, pre-carga el cache
3. **Monitorea el cache**: Ajusta el TTL según tus patrones de uso
4. **Limpieza periódica**: El cleanup automático previene memory leaks

## Consideraciones de Memoria

- Cada entrada en cache usa aproximadamente 1-2 KB
- Con 10,000 sesiones activas: ~10-20 MB de RAM
- El cache se limpia automáticamente cada 10 minutos

## Cuándo Usar Session Manager

✅ **Recomendado cuando:**
- Alta concurrencia (>100 requests/segundo)
- Muchas lecturas de sesión
- Sesiones que se acceden frecuentemente
- Aplicaciones con múltiples goroutines

❌ **No necesario cuando:**
- Baja concurrencia (<10 requests/segundo)
- Sesiones que se leen una sola vez
- Aplicaciones simples sin concurrencia

## Troubleshooting

### El cache no parece funcionar
- Verifica que el TTL sea mayor que el tiempo entre accesos
- Asegúrate de que StartCleanupRoutine() se ejecute

### Alto uso de memoria
- Reduce el TTL del cache
- Aumenta la frecuencia de limpieza
- Considera limitar el tamaño del cache

### Race conditions persisten
- Asegúrate de usar el Session Manager para TODAS las operaciones
- No mezcles acceso directo con Session Manager