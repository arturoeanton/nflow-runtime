# Análisis de Capacidad - 5M Workflows en 8 Horas

## Objetivo
Procesar 5,000,000 workflows en 8 horas (173.6 req/seg)

## Capacidad del Sistema

### VM Pool Performance
- Latencia VM acquire/release: ~16μs
- Throughput del pool: ~60,000 ops/seg
- **Conclusión**: ✅ El VM pool NO es el cuello de botella

### Factores Limitantes Reales

1. **Complejidad del Workflow**
   - JS simple (1-10ms): ✅ Soporta >1,000 req/seg
   - JS complejo (10-100ms): ✅ Soporta 100-1,000 req/seg
   - JS muy complejo (>100ms): ⚠️ Soporta <100 req/seg

2. **Operaciones I/O**
   - DB queries
   - HTTP calls a servicios externos
   - File operations
   - Session management

3. **Recursos del Sistema**
   - CPU cores disponibles
   - Memoria RAM
   - Conexiones a BD
   - Ancho de banda

## Configuración Recomendada

### 1. VM Pool (config.toml)
```toml
[vm_pool]
max_size = 200           # Pool grande para alta concurrencia
preload_size = 100       # Pre-calentar 100 VMs
idle_timeout = 15        # Mantener VMs más tiempo
cleanup_interval = 10    # Limpieza menos frecuente
enable_metrics = true    # Monitoreo activo
```

### 2. Base de Datos
```toml
[database_nflow]
# Usar PostgreSQL para producción
driver = "postgres"
dsn = "postgres://user:pass@localhost/nflow?sslmode=disable&pool_max_conns=100"
```

### 3. Redis para Caché
```toml
[redis]
host = "localhost:6379"
maxconnectionpool = 200
```

## Arquitectura para 5M Requests

### Opción 1: Servidor Único Potente
- **Specs mínimos**:
  - 16-32 CPU cores
  - 32-64 GB RAM
  - SSD NVMe
  - 10 Gbps network

- **Capacidad estimada**:
  - Workflows simples: ✅ 500-1,000 req/seg
  - Workflows medianos: ✅ 200-500 req/seg
  - Workflows complejos: ⚠️ 50-200 req/seg

### Opción 2: Cluster con Load Balancer (Recomendado)
```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │   (HAProxy)     │
                    └────────┬────────┘
                             │
          ┌─────────────┬────┴────┬─────────────┐
          │             │         │             │
    ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
    │ nFlow #1  │ │ nFlow #2  │ │ nFlow #3  │ │ nFlow #4  │
    │ 8 cores   │ │ 8 cores   │ │ 8 cores   │ │ 8 cores   │
    │ 16GB RAM  │ │ 16GB RAM  │ │ 16GB RAM  │ │ 16GB RAM  │
    └───────────┘ └───────────┘ └───────────┘ └───────────┘
          │             │         │             │
          └─────────────┴────┬────┴─────────────┘
                             │
                    ┌────────▼────────┐
                    │   PostgreSQL    │
                    │   (Primary)     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │     Redis       │
                    │    Cluster      │
                    └─────────────────┘
```

**Capacidad del cluster**:
- 4 nodos × 250 req/seg = 1,000 req/seg
- **5M en 8 horas**: ✅ Fácilmente alcanzable

## Métricas de Monitoreo

### Indicadores Clave
1. **VM Pool**
   - Pool utilization < 80%
   - Zero pool exhaustion errors
   - Avg acquire time < 100μs

2. **Sistema**
   - CPU usage < 70%
   - Memory usage < 80%
   - DB connection pool < 80%

3. **Application**
   - p95 latency < 100ms
   - Error rate < 0.1%
   - Throughput > 200 req/seg/node

## Script de Test de Carga

```bash
#!/bin/bash
# load_test.sh

# Instalar vegeta si no está instalado
# brew install vegeta (macOS)
# apt-get install vegeta (Linux)

# Test incremental
echo "GET http://localhost:8080/api/workflow" | \
  vegeta attack -duration=1m -rate=50/s | \
  vegeta report

echo "GET http://localhost:8080/api/workflow" | \
  vegeta attack -duration=1m -rate=100/s | \
  vegeta report

echo "GET http://localhost:8080/api/workflow" | \
  vegeta attack -duration=1m -rate=200/s | \
  vegeta report

# Test sostenido para 8 horas
echo "GET http://localhost:8080/api/workflow" | \
  vegeta attack -duration=8h -rate=174/s | \
  vegeta report
```

## Optimizaciones Adicionales

### 1. Caché de Resultados
```go
// En plugins que hacen llamadas costosas
var resultCache = cache.New(5*time.Minute, 10*time.Minute)

func expensiveOperation(key string) (interface{}, error) {
    if cached, found := resultCache.Get(key); found {
        return cached, nil
    }
    
    result, err := doExpensiveWork()
    if err == nil {
        resultCache.Set(key, result, cache.DefaultExpiration)
    }
    return result, err
}
```

### 2. Batch Processing
Para workflows que no requieren respuesta inmediata:
```go
type BatchProcessor struct {
    queue    chan WorkflowRequest
    workers  int
    interval time.Duration
}

func (bp *BatchProcessor) ProcessBatch() {
    batch := make([]WorkflowRequest, 0, 100)
    ticker := time.NewTicker(bp.interval)
    
    for {
        select {
        case req := <-bp.queue:
            batch = append(batch, req)
            if len(batch) >= 100 {
                bp.executeBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                bp.executeBatch(batch)
                batch = batch[:0]
            }
        }
    }
}
```

### 3. Connection Pooling
```go
// HTTP Client pool
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
    Timeout: 30 * time.Second,
}
```

## Conclusión

✅ **SÍ, es posible procesar 5M workflows en 8 horas** con:

1. **Configuración correcta** del VM pool y recursos
2. **Hardware adecuado** (1 servidor potente o cluster de 4 nodos)
3. **Optimizaciones** según la complejidad de los workflows
4. **Monitoreo activo** para ajustar en tiempo real

**Recomendación**: Empezar con pruebas de carga incrementales para determinar la configuración óptima para tus workflows específicos.