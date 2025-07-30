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

### 2. **Configuración hardcodeada**
- **Problema**: Algunos valores están hardcodeados (timeouts, límites)
- **Impacto**: Requiere recompilación para ajustes
- **Solución propuesta**: Mover todos los valores a configuración

### 3. **Falta de métricas detalladas**
- **Problema**: Solo hay métricas básicas del VM pool
- **Impacto**: Visibilidad limitada del comportamiento en producción
- **Solución propuesta**: Implementar métricas con Prometheus

### 4. **Gestión de memoria en VMs**
- **Problema**: Las VMs no tienen límites de memoria configurables
- **Impacto**: Un script malicioso puede consumir toda la memoria
- **Solución propuesta**: Implementar límites de recursos por VM

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
| Media     | 4        | 3-4 semanas       |
| Menor     | 4        | 1-2 semanas       |

## 🎯 Prioridades Recomendadas

1. **Inmediato**: Arreglar tests de syncsession
2. **Corto plazo**: Implementar tests de integración y métricas
3. **Mediano plazo**: Mejorar manejo de errores y configuración
4. **Largo plazo**: Documentación completa y limpieza de código