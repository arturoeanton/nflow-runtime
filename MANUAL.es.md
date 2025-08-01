# nFlow Runtime - Manual Completo

## Tabla de Contenidos

1. [Introducción](#introducción)
2. [Instalación](#instalación)
3. [Configuración Básica](#configuración-básica)
4. [Visión General de la Arquitectura](#visión-general-de-la-arquitectura)
5. [Conceptos Fundamentales](#conceptos-fundamentales)
6. [Características de Seguridad](#características-de-seguridad)
7. [Optimización del Rendimiento](#optimización-del-rendimiento)
8. [Desarrollo de Plugins](#desarrollo-de-plugins)
9. [Configuración Avanzada](#configuración-avanzada)
10. [Monitoreo y Depuración](#monitoreo-y-depuración)
11. [Despliegue en Producción](#despliegue-en-producción)
12. [Solución de Problemas](#solución-de-problemas)
13. [Referencia de API](#referencia-de-api)

## Introducción

nFlow Runtime es un motor de ejecución de workflows de alto rendimiento diseñado para ejecutar flujos de trabajo creados en el diseñador visual nFlow. Proporciona un entorno seguro y escalable con amplias capacidades de monitoreo.

### Características Principales

- **Ejecución de JavaScript**: Entorno sandboxed seguro para ejecutar código JavaScript
- **Alto Rendimiento**: Optimizado para manejar miles de solicitudes por segundo
- **Seguridad Primero**: Múltiples capas de seguridad incluyendo sandboxing, límites de recursos y análisis estático
- **Extensible**: Sistema de plugins para funcionalidad personalizada
- **Observable**: Métricas integradas, logging y capacidades de depuración

### Casos de Uso

- Automatización de workflows
- Orquestación de APIs
- Pipelines de procesamiento de datos
- Automatización de procesos de negocio
- Middleware de integración

## Instalación

### Prerrequisitos

- Go 1.19 o superior
- PostgreSQL 12+ o SQLite3
- Redis 6+ (opcional, para sesiones y rate limiting)
- Git

### Desde el Código Fuente

```bash
# Clonar el repositorio
git clone https://github.com/arturoeanton/nflow-runtime.git
cd nflow-runtime

# Compilar el binario
go build -o nflow-runtime .

# Ejecutar las pruebas
go test ./...

# Ejecutar con salida detallada
./nflow-runtime -v
```

### Como Módulo de Go

```bash
go get github.com/arturoeanton/nflow-runtime
```

### Instalación con Docker

```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o nflow-runtime .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/nflow-runtime .
COPY config.toml .
EXPOSE 8080
CMD ["./nflow-runtime"]
```

## Configuración Básica

### config.toml Mínimo

```toml
[database_nflow]
driver = "sqlite3"
dsn = "app.sqlite"

[vm_pool]
max_size = 50
preload_size = 25
```

### Configuración de Base de Datos

#### SQLite (Desarrollo)

```toml
[database_nflow]
driver = "sqlite3"
dsn = "app.sqlite"
```

#### PostgreSQL (Producción)

```toml
[database_nflow]
driver = "postgres"
dsn = "user=nflow password=secret host=localhost port=5432 dbname=nflow sslmode=require"
query = "SELECT name,query FROM queries"
```

### Configuración de Redis

```toml
[redis]
host = "localhost:6379"
password = "tu-contraseña-redis"
db = 0
```

## Visión General de la Arquitectura

### Arquitectura de Componentes

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│   Capa HTTP     │────▶│  Motor Principal │────▶│  Pool de VMs │
│   (Echo)        │     │                  │     │  (Goja)      │
└─────────────────┘     └──────────────────┘     └──────────────┘
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│ Limitador Tasa  │     │ Gestor Procesos  │     │  Módulo      │
│                 │     │                  │     │  Seguridad   │
└─────────────────┘     └──────────────────┘     └──────────────┘
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│ Base de Datos   │     │      Caché       │     │   Métricas   │
│  (PostgreSQL)   │     │  (Memoria/Redis) │     │ (Prometheus) │
└─────────────────┘     └──────────────────┘     └──────────────┘
```

### Flujo de Solicitudes

1. **Solicitud HTTP** llega al router Echo
2. **Limitación de Tasa** verifica si la solicitud debe proceder
3. **Análisis de Seguridad** valida el código JavaScript
4. **Adquisición de VM** del pool para ejecución
5. **Ejecución del Workflow** con límites de recursos
6. **Procesamiento de Respuesta** con encriptación opcional
7. **Recolección de Métricas** y logging

### Estructura de Directorios

```
nflow-runtime/
├── engine/               # Motor de ejecución principal
│   ├── engine.go        # Ejecutor principal de workflows
│   ├── vm_manager.go    # Gestión del pool de VMs
│   ├── vm_limits.go     # Aplicación de límites de recursos
│   └── vm_sandbox.go    # Sandboxing de seguridad
├── security/            # Módulo de seguridad
│   ├── analyzer/        # Análisis estático de código
│   ├── encryption/      # Encriptación de datos
│   └── interceptor/     # Detección de datos sensibles
├── process/             # Gestión de procesos
├── endpoints/           # Endpoints de API
├── plugins/             # Sistema de plugins
└── main.go             # Punto de entrada
```

## Conceptos Fundamentales

### Workflows

Un workflow es una estructura JSON que define:
- **Nodos**: Unidades individuales de ejecución
- **Conexiones**: Cómo fluyen los datos entre nodos
- **Datos**: Mapeos de entrada/salida

Ejemplo de estructura de workflow:
```json
{
  "nodo1": {
    "data": {
      "type": "httpstart",
      "method": "POST",
      "path": "/api/procesar"
    },
    "outputs": {
      "output": {
        "connections": [{
          "node": "nodo2",
          "output": "input"
        }]
      }
    }
  },
  "nodo2": {
    "data": {
      "type": "js",
      "script": "return {resultado: payload.valor * 2};"
    }
  }
}
```

### Nodos

Tipos de nodos integrados:
- **httpstart**: Disparador de endpoint HTTP
- **js**: Ejecución de JavaScript
- **http**: Cliente HTTP
- **db**: Operaciones de base de datos
- **mail**: Envío de correos
- **template**: Renderizado de plantillas

### Pool de VMs

El pool de VMs gestiona las instancias de runtime JavaScript:

```toml
[vm_pool]
max_size = 200              # Máximo de VMs en el pool
preload_size = 100          # VMs pre-creadas
idle_timeout = 10           # Minutos antes de eliminar VM inactiva
cleanup_interval = 5        # Intervalo de limpieza
```

Beneficios:
- Elimina el overhead de creación de VMs
- Rendimiento predecible
- Eficiencia de recursos

## Características de Seguridad

### Sandboxing de JavaScript

#### Límites de Recursos

```toml
[vm_pool]
max_memory_mb = 128         # Límite de memoria por VM
max_execution_seconds = 30  # Timeout de ejecución
max_operations = 10000000   # Límite de operaciones JavaScript
```

#### Características Deshabilitadas

- Constructores `eval()` y `Function()`
- Acceso al sistema de archivos (configurable)
- Acceso a la red (configurable)
- Spawning de procesos

#### Habilitar/Deshabilitar Características

```toml
[vm_pool]
enable_filesystem = false   # Acceso al sistema de archivos
enable_network = false      # Acceso a la red
enable_process = false      # Ejecución de procesos
```

### Análisis Estático de Código

El módulo de seguridad analiza JavaScript antes de la ejecución:

```toml
[security]
enable_static_analysis = true
block_on_high_severity = true
log_security_warnings = true
```

Patrones detectados:
- Uso directo de `eval()`
- Intentos de `require('fs')`
- Spawning de procesos hijos
- Loops infinitos
- Modificaciones del scope global

Ejemplo de código bloqueado:
```javascript
// Esto será bloqueado
eval("código malicioso");
require('fs').readFile('/etc/passwd');
while(true) { }
```

### Encriptación de Datos

Encriptación automática de datos sensibles:

```toml
[security]
enable_encryption = true
encryption_key = "tu-clave-de-32-bytes-aquí"
encrypt_sensitive_data = true
```

Encriptados automáticamente:
- Direcciones de email
- Números de teléfono
- Números de Seguro Social
- Claves API
- Tokens JWT
- Números de tarjetas de crédito

#### Generación de Claves de Encriptación

```go
// Generar una clave segura
key, err := encryption.GenerateKeyString()
if err != nil {
    log.Fatal(err)
}
fmt.Println("Agregar a config.toml:", key)
```

### Limitación de Tasa

Limitación de tasa basada en IP con algoritmo token bucket:

```toml
[rate_limit]
enabled = true
ip_rate_limit = 100         # Solicitudes por ventana
ip_window_minutes = 1       # Ventana de tiempo
backend = "memory"          # o "redis"
```

Configuración avanzada:
```toml
[rate_limit]
burst_size = 10
cleanup_interval = 10
excluded_ips = "127.0.0.1,10.0.0.0/8"
excluded_paths = "/health,/metrics"
```

## Optimización del Rendimiento

### Ajuste del Pool de VMs

Para escenarios de alto tráfico:

```toml
[vm_pool]
max_size = 500              # Aumentar tamaño del pool
preload_size = 250          # Pre-calentar más VMs
idle_timeout = 5            # Limpieza agresiva
enable_metrics = true       # Monitorear uso del pool
```

### Caché

Múltiples capas de caché:

1. **Caché Babel**: Resultados de transformación ES6
2. **Caché de Programas**: Programas JavaScript compilados
3. **Caché de Auth**: Scripts de autenticación

### Optimización de Base de Datos

```toml
[database_nflow]
max_open_conns = 100        # Tamaño del pool de conexiones
max_idle_conns = 10         # Conexiones inactivas
conn_max_lifetime = 300     # Tiempo de vida de conexión (segundos)
```

### Monitoreo del Rendimiento

Habilitar métricas detalladas:

```toml
[monitor]
enabled = true
enable_detailed_metrics = true
metrics_port = "9090"       # Puerto separado para métricas
```

Métricas clave a monitorear:
- `nflow_vm_pool_active`: VMs activas
- `nflow_vm_pool_available`: VMs disponibles
- `nflow_requests_duration`: Latencia de solicitudes
- `nflow_workflows_total`: Ejecuciones de workflows

## Desarrollo de Plugins

### Creación de un Plugin Personalizado

```go
package miplugin

import (
    "github.com/dop251/goja"
    "github.com/labstack/echo/v4"
)

type MiPlugin struct{}

func (p *MiPlugin) Name() string {
    return "miplugin"
}

func (p *MiPlugin) Initialize(vm *goja.Runtime) error {
    // Agregar funciones a la VM
    vm.Set("miFuncion", func(call goja.FunctionCall) goja.Value {
        // Implementación
        return vm.ToValue("resultado")
    })
    return nil
}

func (p *MiPlugin) Execute(c echo.Context, vm *goja.Runtime) error {
    // Lógica del plugin
    return nil
}
```

### Registro de Plugins

En tu main.go:

```go
import "github.com/arturoeanton/nflow-runtime/plugins"

func init() {
    plugins.Register(&MiPlugin{})
}
```

### Mejores Prácticas para Plugins

1. **Thread Safety**: Los plugins deben ser thread-safe
2. **Manejo de Errores**: Siempre retornar errores significativos
3. **Limpieza de Recursos**: Usar defer para limpieza
4. **Documentación**: Documentar todas las funciones expuestas
5. **Testing**: Incluir tests comprehensivos

## Configuración Avanzada

### Ejemplo Completo de config.toml

```toml
# Configuración de base de datos
[database_nflow]
driver = "postgres"
dsn = "host=localhost user=nflow password=secret dbname=nflow sslmode=require"
max_open_conns = 100
max_idle_conns = 10
conn_max_lifetime = 300

# Redis para sesiones y caché
[redis]
host = "localhost:6379"
password = "contraseña-redis"
db = 0
max_retries = 3
pool_size = 10

# Configuración del Pool de VMs
[vm_pool]
max_size = 200
preload_size = 100
idle_timeout = 10
cleanup_interval = 5
enable_metrics = true

# Límites de recursos
max_memory_mb = 128
max_execution_seconds = 30
max_operations = 10000000

# Configuración del sandbox
enable_filesystem = false
enable_network = true
enable_process = false

# Tracking y monitoreo
[tracker]
enabled = true
workers = 8
batch_size = 1000
flush_interval = 500
channel_buffer = 100000
verbose_logging = false

# Endpoints de debug
[debug]
enabled = false
auth_token = "debug-token-12345"
allowed_ips = "10.0.0.0/8,172.16.0.0/12"
enable_pprof = false

# Monitoreo
[monitor]
enabled = true
health_check_path = "/health"
metrics_path = "/metrics"
enable_detailed_metrics = true
metrics_port = "9090"

# Limitación de tasa
[rate_limit]
enabled = true
ip_rate_limit = 1000
ip_window_minutes = 1
ip_burst_size = 50
backend = "redis"
cleanup_interval = 10
retry_after_header = true
error_message = "Demasiadas solicitudes. Por favor intente más tarde."
excluded_ips = "10.0.0.0/8,172.16.0.0/12"
excluded_paths = "/health,/metrics,/debug"

# Seguridad
[security]
enable_static_analysis = true
block_on_high_severity = true
log_security_warnings = true
cache_analysis_results = true
cache_ttl_minutes = 5

enable_encryption = true
encryption_key = "tu-clave-codificada-base64-de-32-bytes"
encrypt_sensitive_data = true
encrypt_in_place = true

always_encrypt_fields = [
    "password",
    "token",
    "secret",
    "api_key",
    "private_key"
]

[security.custom_patterns]
employee_id = "EMP\\d{6}"
internal_api = "int_api_[a-zA-Z0-9]{32}"

# Configuración de email
[mail]
enabled = true
smtp_host = "smtp.gmail.com"
smtp_port = 587
username = "nflow@ejemplo.com"
password = "contraseña-smtp"
from = "nFlow Runtime <nflow@ejemplo.com>"
```

### Variables de Entorno

Sobrescribir configuración con variables de entorno:

```bash
export NFLOW_DATABASE_DSN="postgres://user:pass@host/db"
export NFLOW_REDIS_HOST="redis.ejemplo.com:6379"
export NFLOW_VM_POOL_MAX_SIZE="500"
export NFLOW_SECURITY_ENCRYPTION_KEY="tu-clave-segura"
```

## Monitoreo y Depuración

### Health Checks

Endpoint por defecto: `GET /health`

Respuesta:
```json
{
  "status": "healthy",
  "timestamp": 1627849200,
  "uptime": "24h15m30s",
  "version": "1.0.0",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "vm_pool": "ok"
  }
}
```

### Métricas Prometheus

Endpoint por defecto: `GET /metrics`

Métricas principales:
```
# Métricas HTTP
nflow_requests_total{method="POST",path="/api/*",status="200"} 12345
nflow_requests_duration_seconds{method="POST",quantile="0.99"} 0.125

# Métricas de workflows
nflow_workflows_total{status="success"} 10000
nflow_workflows_duration_seconds{quantile="0.99"} 2.5

# Métricas del Pool de VMs
nflow_vm_pool_active 45
nflow_vm_pool_available 155
nflow_vm_pool_created_total 200

# Métricas de seguridad
nflow_security_scripts_analyzed_total 5000
nflow_security_scripts_blocked_total 23
nflow_security_data_encrypted_total 1500
```

### Endpoints de Debug

Habilitar endpoints de debug:

```toml
[debug]
enabled = true
auth_token = "tu-token-secreto"
allowed_ips = "10.0.0.0/8"
```

Endpoints disponibles:
- `GET /debug/info` - Información del sistema
- `GET /debug/config` - Configuración actual
- `GET /debug/processes` - Procesos activos
- `GET /debug/cache/stats` - Estadísticas de caché
- `GET /debug/vm/pool` - Estado del pool de VMs

Ejemplo de solicitud:
```bash
curl -H "X-Debug-Token: tu-token-secreto" http://localhost:8080/debug/info
```

### Logging

#### Niveles de Log

Ejecutar con logging detallado:
```bash
./nflow-runtime -v
```

Formato de log:
```
2024-01-15 10:30:45 [INFO] Iniciado nFlow Runtime
2024-01-15 10:30:45 [DEBUG] Pool de VMs: Creadas 100 VMs
2024-01-15 10:30:46 [WARN] Seguridad: Script bloqueado con eval()
2024-01-15 10:30:47 [ERROR] Conexión a base de datos falló: timeout
```

#### Logging Estructurado

Configurar logging estructurado:

```go
logger.SetFormatter(&logger.JSONFormatter{})
logger.SetLevel(logger.DebugLevel)
```

Salida:
```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "info",
  "msg": "Workflow ejecutado",
  "workflow_id": "abc123",
  "duration_ms": 125,
  "status": "success"
}
```

## Despliegue en Producción

### Requisitos del Sistema

#### Requisitos Mínimos
- CPU: 2 cores
- RAM: 4GB
- Disco: 10GB SSD
- Red: 100Mbps

#### Recomendado para Producción
- CPU: 8+ cores
- RAM: 16GB+
- Disco: 100GB+ SSD
- Red: 1Gbps

### Checklist de Despliegue

1. **Seguridad**
   - [ ] Cambiar todas las contraseñas por defecto
   - [ ] Habilitar HTTPS/TLS
   - [ ] Configurar reglas de firewall
   - [ ] Habilitar limitación de tasa
   - [ ] Habilitar módulo de seguridad
   - [ ] Rotar claves de encriptación

2. **Base de Datos**
   - [ ] Usar PostgreSQL para producción
   - [ ] Configurar pooling de conexiones
   - [ ] Configurar backups regulares
   - [ ] Habilitar SSL para conexiones

3. **Monitoreo**
   - [ ] Configurar Prometheus
   - [ ] Configurar dashboards de Grafana
   - [ ] Configurar reglas de alertas
   - [ ] Habilitar health checks

4. **Rendimiento**
   - [ ] Ajustar tamaño del pool de VMs
   - [ ] Configurar caché
   - [ ] Habilitar compresión
   - [ ] Configurar CDN para assets estáticos

### Despliegue en Kubernetes

Ejemplo de deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nflow-runtime
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nflow-runtime
  template:
    metadata:
      labels:
        app: nflow-runtime
    spec:
      containers:
      - name: nflow-runtime
        image: nflow/runtime:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: NFLOW_DATABASE_DSN
          valueFrom:
            secretKeyRef:
              name: nflow-secrets
              key: database-dsn
        - name: NFLOW_SECURITY_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: nflow-secrets
              key: encryption-key
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Estrategias de Escalamiento

#### Escalamiento Horizontal
- Agregar más instancias detrás de un balanceador
- Usar Redis para estado compartido
- Configurar afinidad de sesión si es necesario

#### Escalamiento Vertical
- Aumentar tamaño del pool de VMs
- Agregar más CPU/RAM
- Ajustar recolección de basura

#### Escalamiento de Base de Datos
- Réplicas de lectura para consultas
- Pooling de conexiones
- Optimización de consultas

## Solución de Problemas

### Problemas Comunes

#### Alto Uso de Memoria

Síntomas:
- Errores OOM
- Rendimiento lento
- Inestabilidad del sistema

Soluciones:
1. Reducir tamaño del pool de VMs
2. Bajar límites de memoria
3. Habilitar GC agresivo
4. Buscar fugas de memoria

#### Degradación del Rendimiento

Síntomas:
- Latencia aumentada
- Errores de timeout
- Acumulación de cola

Soluciones:
1. Verificar métricas del pool de VMs
2. Analizar consultas lentas
3. Revisar complejidad de workflows
4. Habilitar caché

#### Bloqueos de Seguridad

Síntomas:
- Scripts rechazados
- Errores de "patrón peligroso"

Soluciones:
1. Revisar logs de seguridad
2. Whitelist de patrones seguros
3. Refactorizar código problemático
4. Ajustar niveles de severidad

### Comandos de Debug

Verificar estado del sistema:
```bash
# Estado del pool de VMs
curl http://localhost:8080/debug/vm/pool

# Procesos activos
curl http://localhost:8080/debug/processes

# Estadísticas de caché
curl http://localhost:8080/debug/cache/stats
```

### Profiling de Rendimiento

Habilitar pprof:
```toml
[debug]
enable_pprof = true
```

Perfilar CPU:
```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

Perfilar memoria:
```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```

## Referencia de API

### Ejecución de Workflows

#### Ejecutar Workflow

```
POST /api/workflow/{workflow_id}
```

Headers:
- `Content-Type: application/json`
- `Authorization: Bearer {token}`

Cuerpo de solicitud:
```json
{
  "input": {
    "clave": "valor"
  }
}
```

Respuesta:
```json
{
  "success": true,
  "output": {
    "resultado": "procesado"
  },
  "execution_time": 125
}
```

### Endpoints Administrativos

#### Listar Workflows

```
GET /api/admin/workflows
```

#### Actualizar Workflow

```
PUT /api/admin/workflows/{workflow_id}
```

#### Eliminar Workflow

```
DELETE /api/admin/workflows/{workflow_id}
```

### APIs de Plugins

Los plugins pueden exponer endpoints personalizados:

```
POST /api/plugin/{plugin_name}/{action}
```

## Mejores Prácticas

### Seguridad

1. **Nunca deshabilitar sandboxing** en producción
2. **Rotar claves de encriptación** regularmente
3. **Monitorear métricas de seguridad** para anomalías
4. **Revisar scripts bloqueados** para falsos positivos
5. **Mantener dependencias actualizadas**

### Rendimiento

1. **Dimensionar correctamente el pool de VMs** basado en carga
2. **Usar caché** para operaciones repetidas
3. **Monitorear uso de recursos** continuamente
4. **Optimizar consultas de base de datos**
5. **Perfilar antes de optimizar**

### Operaciones

1. **Automatizar despliegues** con CI/CD
2. **Usar infraestructura como código**
3. **Implementar logging apropiado**
4. **Configurar umbrales de alerta**
5. **Documentar runbooks**

## Apéndice

### Glosario

- **VM**: Máquina Virtual (instancia de runtime JavaScript)
- **Workflow**: Definición de flujo ejecutable
- **Nodo**: Unidad de ejecución individual en un workflow
- **Sandbox**: Entorno de ejecución aislado
- **Pool**: Colección de VMs pre-inicializadas

### Referencias

- [Documentación de Goja](https://github.com/dop251/goja)
- [Framework Echo](https://echo.labstack.com/)
- [Métricas Prometheus](https://prometheus.io/)
- [Diseñador nFlow](https://github.com/arturoeanton/nflow)

### Historial de Versiones

- v1.0.0 - Lanzamiento inicial
- v1.1.0 - Agregado pooling de VMs
- v1.2.0 - Módulo de seguridad
- v1.3.0 - Limitación de tasa
- v1.4.0 - Monitoreo avanzado

---

Para más información, visita [https://github.com/arturoeanton/nflow-runtime](https://github.com/arturoeanton/nflow-runtime)