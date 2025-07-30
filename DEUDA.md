# Deuda Técnica - nFlow Runtime

## 🔴 Deuda Crítica

### 1. **Tests de syncsession con deadlock**
- **Problema**: Los tests de race conditions en syncsession causan deadlock
- **Impacto**: No se pueden ejecutar todos los tests con -race
- **Solución propuesta**: Refactorizar los tests para evitar contención excesiva de mutex

### 2. **Falta de tests de integración completos**
- **Problema**: No hay tests end-to-end que simulen flujos completos
- **Impacto**: Cambios pueden romper funcionalidad sin ser detectados
- **Solución propuesta**: Crear suite de tests de integración con workflows reales

### 3. **Documentación de API incompleta**
- **Problema**: No hay documentación formal de las APIs REST
- **Impacto**: Dificulta la integración con sistemas externos
- **Solución propuesta**: Generar documentación OpenAPI/Swagger

## 🟡 Deuda Media

### 1. **Manejo de errores inconsistente**
- **Problema**: Algunos errores se loguean, otros se ignoran silenciosamente
- **Impacto**: Dificulta el debugging en producción
- **Solución propuesta**: Implementar sistema centralizado de manejo de errores

### 2. ~~**Configuración hardcodeada**~~ ✅ RESUELTO
- ~~**Problema**: Algunos valores están hardcodeados (timeouts, límites)~~
- ~~**Impacto**: Requiere recompilación para ajustes~~
- **Solución implementada**: Límites de recursos y sandboxing ahora configurables en config.toml

### 3. **Falta de métricas detalladas**
- **Problema**: Solo hay métricas básicas del VM pool
- **Impacto**: Visibilidad limitada del comportamiento en producción
- **Solución propuesta**: Implementar métricas con Prometheus (ahora incluir métricas de seguridad)

### 4. ~~**Gestión de memoria en VMs**~~ ✅ RESUELTO
- ~~**Problema**: Las VMs no tienen límites de memoria configurables~~
- ~~**Impacto**: Un script malicioso puede consumir toda la memoria~~
- **Solución implementada**: Sistema completo de límites (memoria, tiempo, operaciones) + sandboxing

## 🟢 Deuda Menor

### 1. **Logging verboso en producción**
- **Problema**: Muchos logs de debug activos siempre
- **Impacto**: Performance y ruido en logs
- **Solución propuesta**: Implementar niveles de log configurables

### 2. **Código comentado no eliminado**
- **Problema**: Hay código comentado en varios archivos
- **Impacto**: Reduce legibilidad
- **Solución propuesta**: Limpieza de código

### 3. **Nombres de variables en inglés/español**
- **Problema**: Mezcla de idiomas en el código
- **Impacto**: Inconsistencia y confusión
- **Solución propuesta**: Estandarizar a inglés

### 4. **Falta de benchmarks automatizados**
- **Problema**: No hay CI/CD que ejecute benchmarks
- **Impacto**: Regresiones de performance no detectadas
- **Solución propuesta**: Integrar benchmarks en pipeline

## 📊 Resumen de Impacto

| Categoría | Cantidad | Esfuerzo Estimado |
|-----------|----------|-------------------|
| Crítica   | 3        | 2-3 semanas       |
| Media     | 2 (-2)   | 2-3 semanas       |
| Menor     | 4        | 1-2 semanas       |

## ✅ Deuda Resuelta Recientemente

1. **Límites de recursos en VMs** - Implementado sistema completo con configuración
2. **Sandboxing de JavaScript** - Whitelist de funciones y módulos seguros
3. **Configuración hardcodeada** - Ahora todo configurable en config.toml

## 🎯 Prioridades Recomendadas (Actualizado)

1. **Inmediato**: Arreglar tests de syncsession
2. **Corto plazo**: 
   - Tests de integración con workflows reales
   - Métricas detalladas (incluyendo seguridad)
   - Rate limiting por usuario
3. **Mediano plazo**: 
   - Manejo de errores centralizado
   - Documentación OpenAPI
   - Health check endpoint
4. **Largo plazo**: 
   - Limpieza de código
   - Migración completa a inglés