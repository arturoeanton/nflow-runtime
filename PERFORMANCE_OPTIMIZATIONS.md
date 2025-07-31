# Performance Optimizations for nFlow Runtime

## Objetivo
Mejorar el rendimiento de 40-50 RPS a 160-200 RPS (4x) para cargas con JavaScript pesado.

## Optimizaciones Implementadas

### 1. VM Pool Management (engine/engine.go)
- **Antes**: Se creaba una nueva VM de Goja para cada request
- **Después**: Reutilización de VMs desde un pool pre-inicializado
- **Impacto**: Elimina el overhead de crear/destruir VMs (~50-100ms por request)

### 2. Configuración Optimizada (config.toml)
```toml
[vm_pool]
max_size = 200        # Antes: 50
preload_size = 100    # Antes: 25
```

### 3. Semáforo Dinámico (engine/step_goja.go)
- **Antes**: Semáforo hardcodeado a 50 concurrent requests
- **Después**: Semáforo dinámico basado en configuración (200)
- **Código**:
```go
semVMOnce.Do(func() {
    config := GetConfig()
    maxSize := config.VMPoolConfig.MaxSize
    if maxSize <= 0 {
        maxSize = 200
    }
    semVM = make(chan int, maxSize)
})
```

### 4. Cache de Babel Transform
- **Implementado**: Cache en memoria para transformaciones ES6
- **Beneficio**: Evita re-transpilar el mismo código
- **Cache limit**: 1000 entradas con auto-limpieza

### 5. Cache de Programas Compilados
- **Implementado**: Pre-compilación y cache de programas JavaScript
- **Beneficio**: Usa `vm.RunProgram()` en lugar de `vm.RunString()`
- **Cache limit**: 500 programas compilados

## Resultados Esperados

### Métricas de Rendimiento
- **Baseline**: 40-50 RPS
- **Target**: 160-200 RPS
- **Mejora**: 4x

### Beneficios Adicionales
1. **Menor latencia**: VMs pre-inicializadas reducen tiempo de respuesta
2. **Mayor estabilidad**: Pool management evita picos de creación de recursos
3. **Mejor uso de CPU**: Reutilización reduce garbage collection
4. **Escalabilidad**: Configuración ajustable según recursos disponibles

## Testing

### Ejecutar Benchmarks
```bash
cd engine
./run_performance_test.sh
```

### Tests Incluidos
1. **BenchmarkVMPool**: Rendimiento del pool de VMs
2. **BenchmarkVMCreation**: Comparación pool vs creación nueva
3. **TestVMPoolConcurrency**: Test de carga con 200 workers concurrentes
4. **TestBabelCachePerformance**: Efectividad del cache de Babel

## Monitoreo

El VM pool incluye métricas automáticas cuando `enable_metrics = true`:
- VMs creadas
- VMs en uso
- VMs disponibles
- Total de usos
- Errores

Los logs muestran:
```
[VM Pool Metrics] Created: 100, InUse: 45, Available: 55, TotalUses: 15234, Errors: 0
```

## Notas de Implementación

1. Las VMs del pool se resetean entre usos para evitar contaminación de estado
2. El pool tiene cleanup automático de VMs idle (configurable)
3. Los límites de seguridad se aplican a cada VM del pool
4. La variable `c` (contexto Echo) ahora se establece correctamente en VMs reutilizadas