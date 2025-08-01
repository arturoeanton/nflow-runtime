# nFlow Runtime

Motor de ejecución de workflows para [nFlow](https://github.com/arturoeanton/nflow). Este proyecto ejecuta los flujos de trabajo creados en el diseñador visual nFlow, proporcionando un entorno seguro con límites de recursos y sandboxing.

## 🚀 Instalación

```bash
go get github.com/arturoeanton/nflow-runtime
```

## 📋 Requisitos

- Go 1.19 o superior
- PostgreSQL o SQLite3
- Redis (opcional, para sesiones)
- Configuración en `config.toml`

## 🎯 Características

- **Ejecución Segura**: Sandboxing de JavaScript con límites de recursos configurables
- **Alto Rendimiento**: 3,396 RPS con workflows JavaScript intensivos (1M+ requests sin errores)
- **Thread-Safe**: Arquitectura sin condiciones de carrera usando Repository Pattern
- **Extensible**: Sistema de plugins para agregar funcionalidad personalizada
- **Logging Detallado**: Sistema de logs estructurado con modo verbose (-v)
- **Monitoreo Completo**: Métricas Prometheus y health checks
- **Debug Avanzado**: Endpoints de debugging con autenticación
- **Optimizado**: Pool de VMs, cache multinivel y código altamente optimizado
- **Rate Limiting**: Limitación de tasa basada en IP con backends configurables
- **Análisis de Seguridad**: Análisis estático de JavaScript antes de ejecución
- **Encriptación Automática**: Detección y encriptación de datos sensibles

## 🔧 Configuración

### config.toml

```toml
[database_nflow]
driver = "postgres"
dsn = "user=postgres dbname=nflow sslmode=disable"

[redis]
host = "localhost:6379"
password = ""

[vm_pool]
# Pool de VMs para alto rendimiento
max_size = 200             # Máximo de VMs en pool (aumentado para 4x performance)
preload_size = 100         # VMs pre-cargadas al inicio

# Límites de recursos (seguridad)
max_memory_mb = 128        # Memoria máxima por VM
max_execution_seconds = 30 # Tiempo máximo de ejecución
max_operations = 10000000  # Operaciones JS máximas

# Configuración de sandbox
enable_filesystem = false  # Acceso al sistema de archivos
enable_network = false     # Acceso a red
enable_process = false     # Acceso a procesos

[tracker]
enabled = false            # Tracking de ejecución (impacto en performance)
verbose_logging = false    # Logs detallados del tracker

[monitor]
enabled = true             # Endpoints de monitoreo
health_check_path = "/health"
metrics_path = "/metrics"

[debug]
enabled = false            # Endpoints de debug (solo desarrollo)
auth_token = ""           # Token de autenticación
allowed_ips = ""          # IPs permitidas (ej: "192.168.1.0/24")

[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587

[rate_limit]
enabled = false            # Limitación de tasa por IP
ip_rate_limit = 100       # Solicitudes por IP por ventana
ip_window_minutes = 1     # Ventana de tiempo en minutos

[security]
# Análisis estático de JavaScript
enable_static_analysis = false    # Detecta patrones peligrosos antes de ejecución
block_on_high_severity = true     # Bloquea scripts con problemas graves

# Encriptación de datos sensibles
enable_encryption = false         # Encripta automáticamente datos sensibles
encryption_key = ""              # Clave de 32 bytes para AES-256
encrypt_sensitive_data = true    # Detecta y encripta emails, SSN, API keys, etc.
```

## 🏃‍♂️ Uso Básico

### Como Servidor Standalone

```bash
# Modo normal
./nflow-runtime

# Modo verbose (logging detallado)
./nflow-runtime -v
```

El servidor estará disponible en `http://localhost:8080`

### Como Librería

```go
import (
    "github.com/arturoeanton/nflow-runtime/engine"
    "github.com/arturoeanton/nflow-runtime/process"
)

func main() {
    // Inicializar configuración
    configRepo := engine.GetConfigRepository()
    config := engine.ConfigWorkspace{
        // ... configuración
    }
    configRepo.SetConfig(config)
    
    // Inicializar base de datos
    db, err := engine.GetDB()
    if err != nil {
        log.Fatal(err)
    }
    engine.InitializePlaybookRepository(db)
    
    // Inicializar gestor de procesos
    process.InitializeRepository()
    
    // Crear servidor Echo
    e := echo.New()
    e.Any("/*", run)
    e.Start(":8080")
}
```

## 🛡️ Seguridad

### Límites de Recursos

Cada VM tiene límites configurables para prevenir ataques DoS:
- **Memoria**: 128MB por defecto
- **Tiempo**: 30 segundos máximo
- **Operaciones**: 10M operaciones JavaScript

### Sandboxing

JavaScript ejecuta en un entorno restringido:
- ❌ `eval()` bloqueado
- ❌ `Function` constructor bloqueado
- ❌ Acceso a filesystem deshabilitado por defecto
- ❌ Acceso a red deshabilitado por defecto
- ✅ Solo módulos en whitelist disponibles

## 🔌 Plugins Disponibles

- **goja**: Motor JavaScript principal
- **mail**: Envío de correos electrónicos
- **template**: Procesamiento de plantillas
- **ianflow**: Integración con IA (OpenAI, Gemini, Ollama)
- **http**: Cliente HTTP para llamadas a APIs
- **db**: Operaciones de base de datos
- **babel**: Transpilación de código ES6+

## 📊 Arquitectura

```
nflow-runtime/
├── engine/             # Motor de ejecución principal
│   ├── engine.go       # Lógica de ejecución de workflows
│   ├── vm_manager.go   # Pool de VMs para alto rendimiento
│   ├── vm_limits.go    # Gestión de límites de recursos
│   ├── vm_sandbox.go   # Implementación del sandbox
│   ├── js_context_wrapper.go # Wrapper de contexto Echo para JS
│   └── config_repository.go # Patrón repository para config
├── process/            # Gestión de procesos
│   └── process_repository.go # Repository thread-safe
├── endpoints/          # Endpoints de API
│   ├── debug_endpoints.go    # Endpoints de debugging
│   └── monitor_endpoints.go  # Health y métricas
├── logger/             # Sistema de logging
│   └── logger.go       # Logger estructurado con niveles
├── syncsession/        # Gestión de sesiones optimizada
├── plugins/            # Plugins del sistema
└── main.go            # Punto de entrada del servidor
```

## 🧩 Steps Personalizados

Puedes crear tus propios tipos de nodos:

```go
type MyCustomStep struct{}

func (s *MyCustomStep) Run(
    cc *model.Controller, 
    actor *model.Node, 
    c echo.Context,
    vm *goja.Runtime, 
    connection_next string, 
    vars model.Vars,
    currentProcess *process.Process, 
    payload goja.Value,
) (string, goja.Value, error) {
    // Tu implementación aquí
    return nextNode, payload, nil
}

// Registrar el step
engine.RegisterStep("my-custom-step", &MyCustomStep{})
```

## 📈 Métricas y Monitoreo

### Endpoints de Monitoreo

- **Health Check**: `GET /health` - Estado de salud del sistema
- **Métricas Prometheus**: `GET /metrics` - Todas las métricas en formato Prometheus

### Métricas Disponibles

- `nflow_requests_total`: Total de requests HTTP
- `nflow_workflows_total`: Total de workflows ejecutados
- `nflow_processes_active`: Procesos activos
- `nflow_db_connections_*`: Métricas de conexiones DB
- `nflow_go_memory_*`: Uso de memoria
- `nflow_cache_hits/misses`: Estadísticas de caché

### Endpoints de Debug (cuando están habilitados)

- `/debug/info`: Información del sistema
- `/debug/config`: Configuración actual
- `/debug/processes`: Lista de procesos activos
- `/debug/cache/stats`: Estadísticas de caché
- `/debug/database/stats`: Métricas de base de datos

Ver [DEBUG_MONITORING.md](DEBUG_MONITORING.md) para documentación completa.

## 🛡️ Limitación de Tasa

nFlow Runtime incluye limitación de tasa basada en IP para proteger contra el abuso:

- Algoritmo token bucket para control flexible de tasa
- Backends de memoria y Redis para diferentes escenarios de implementación
- Exclusiones configurables para IPs y rutas
- Headers detallados para integración con clientes

Ver [RATE_LIMITING.ES.md](RATE_LIMITING.ES.md) para documentación completa.

## 🚀 Optimizaciones de Rendimiento

nFlow Runtime ha sido optimizado para manejar cargas pesadas de JavaScript:

### Pool de VMs
- Reutilización de VMs Goja mediante pool configurable
- Pre-carga de VMs al inicio para disponibilidad inmediata
- Gestión inteligente con timeout de espera de 5 segundos
- Métricas detalladas del estado del pool

### Sistema de Cache
- **Cache de Babel**: Transformaciones ES6 en memoria
- **Cache de programas**: JavaScript pre-compilado
- **Cache de auth.js**: Evita lectura repetitiva de archivos

### Resultados de Pruebas JMeter
- **Workflow probado**: httpstart → js-JsonRender con 1000 cálculos matemáticos
- **Throughput demostrado**: 3,396 req/s (~3.4 millones de cálculos/segundo)
- **Confiabilidad**: 1,007,399 requests procesados con 0% de errores
- **Latencia promedio**: 860ms (incluye compilación JS + 1000 operaciones)
- **Tiempos de respuesta**: Mínimo 25ms, máximo 2,488ms
- **Desviación estándar**: 87.36ms (comportamiento predecible)
- **Transferencia**: 5,265.98 KB/s de capacidad

## 🚨 Manejo de Errores

Los errores se manejan de forma consistente:
- HTTP 408: Límite de recursos excedido
- HTTP 500: Error interno del servidor
- HTTP 404: Workflow no encontrado

## 🔄 Estado del Proyecto

- **Madurez**: 4.9/5 ⭐ (Listo para producción)
- **Estabilidad**: ESTABLE ✅
- **Seguridad**: MUY BUENA ✅
- **Performance**: 3,396 RPS con JavaScript intensivo (0% errores) ✅
- **Observabilidad**: COMPLETA ✅
- **Preparación Producción**: 95% ✅

Ver [STATUS.md](STATUS.md) para más detalles.

## 🐛 Problemas Conocidos

Ver [DEUDA.md](DEUDA.md) para la lista completa de deuda técnica.

## 🤝 Contribuir

1. Fork el proyecto
2. Crea tu rama de feature (`git checkout -b feature/AmazingFeature`)
3. Commit tus cambios (`git commit -m 'Add some AmazingFeature'`)
4. Push a la rama (`git push origin feature/AmazingFeature`)
5. Abre un Pull Request

## 📝 Licencia

MIT - ver archivo LICENSE para detalles.

## 🙏 Agradecimientos

- [Goja](https://github.com/dop251/goja) - Motor JavaScript en Go
- [Echo](https://echo.labstack.com/) - Framework web
- [nFlow](https://github.com/arturoeanton/nflow) - Diseñador visual de workflows