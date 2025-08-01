# Manual de nFlow Runtime

## Tabla de Contenidos

1. [Introducción](#introducción)
2. [Comenzando](#comenzando)
3. [Conceptos Básicos](#conceptos-básicos)
4. [Guía de Configuración](#guía-de-configuración)
5. [Desarrollo de Workflows](#desarrollo-de-workflows)
6. [Características de Seguridad](#características-de-seguridad)
7. [Optimización de Rendimiento](#optimización-de-rendimiento)
8. [Monitoreo y Depuración](#monitoreo-y-depuración)
9. [Desarrollo de Plugins](#desarrollo-de-plugins)
10. [Despliegue en Producción](#despliegue-en-producción)
11. [Solución de Problemas](#solución-de-problemas)
12. [Temas Avanzados](#temas-avanzados)

## Introducción

nFlow Runtime es un motor de ejecución de workflows de alto rendimiento diseñado para ejecutar flujos de trabajo creados en el diseñador visual nFlow. Proporciona una plataforma segura, escalable y observable para ejecutar workflows basados en JavaScript con características de nivel empresarial.

### Características Principales

- **Ejecución Segura**: Entorno JavaScript sandboxed con límites de recursos
- **Alto Rendimiento**: 3,396+ RPS con workflows intensivos en cómputo
- **Seguridad Empresarial**: Análisis estático, encriptación y sanitización de logs
- **Observabilidad Completa**: Health checks, métricas Prometheus, endpoints de debug
- **Arquitectura Flexible**: Sistema de plugins para extensibilidad

## Comenzando

### Prerrequisitos

- Go 1.19 o superior
- PostgreSQL 9.6+ o SQLite3
- Redis 5.0+ (opcional, para gestión de sesiones)
- Git

### Instalación

#### Desde el Código Fuente

```bash
# Clonar el repositorio
git clone https://github.com/arturoeanton/nflow-runtime.git
cd nflow-runtime

# Compilar el binario
go build -o nflow-runtime .

# Ejecutar el servidor
./nflow-runtime
```

#### Como Módulo Go

```bash
go get github.com/arturoeanton/nflow-runtime
```

### Inicio Rápido

1. Crear un `config.toml` básico:

```toml
[database_nflow]
driver = "sqlite3"
dsn = "./nflow.db"

[vm_pool]
max_size = 50
preload_size = 10

[monitor]
enabled = true
```

2. Ejecutar el servidor:

```bash
./nflow-runtime -v  # Modo verbose para logs detallados
```

3. Verificar que el servidor esté funcionando:

```bash
curl http://localhost:8080/health
```

## Conceptos Básicos

### Workflows

Un workflow es una serie de nodos (pasos) conectados que se ejecutan en secuencia o en paralelo. Cada nodo realiza una acción específica y puede pasar datos a los nodos siguientes.

### Nodos/Pasos

Los nodos son los bloques de construcción de los workflows:
- **httpstart**: Punto de entrada para workflows activados por HTTP
- **js**: Ejecutar código JavaScript
- **http**: Realizar peticiones HTTP
- **db**: Operaciones de base de datos
- **mail**: Enviar correos electrónicos
- **template**: Procesar plantillas

### Proceso

Un proceso es una instancia de ejecución de un workflow. Cada proceso tiene:
- ID único
- Estado (en ejecución, completado, fallido)
- Variables/estado
- Historial de ejecución

### Payload

Datos pasados entre nodos. Puede ser cualquier valor serializable en JSON.

## Guía de Configuración

### Referencia Completa de Configuración

```toml
# Configuración de base de datos
[database_nflow]
driver = "postgres"  # postgres, mysql, sqlite3
dsn = "host=localhost user=postgres password=secret dbname=nflow sslmode=disable"
query = "SELECT name,query FROM queries"  # Tabla de consultas personalizada

# Configuración de Redis (opcional)
[redis]
host = "localhost:6379"
password = ""
db = 0

# Almacenamiento de sesiones
[pg_session]
url = "postgres://user:pass@localhost/sessions?sslmode=disable"

# Configuración del pool de VMs
[vm_pool]
max_size = 200              # Máximo de VMs en el pool
preload_size = 100          # VMs a crear al inicio
idle_timeout = 10           # Minutos antes de eliminar VMs inactivas
cleanup_interval = 5        # Minutos entre limpiezas
enable_metrics = true       # Registrar métricas del pool

# Límites de recursos por VM
max_memory_mb = 128         # Memoria máxima por VM (MB)
max_execution_seconds = 30  # Tiempo máximo de ejecución (segundos)
max_operations = 10000000   # Operaciones JS máximas

# Configuración del sandbox
enable_filesystem = false   # Permitir acceso al sistema de archivos
enable_network = false      # Permitir acceso a la red
enable_process = false      # Permitir creación de procesos

# Seguimiento de ejecución
[tracker]
enabled = false             # Habilitar seguimiento de ejecución
workers = 4                 # Número de goroutines trabajadoras
batch_size = 100           # Tamaño del lote para inserciones DB
flush_interval = 250       # Intervalo de flush (ms)
channel_buffer = 100000    # Tamaño del buffer del canal
verbose_logging = false    # Habilitar logging verbose
stats_interval = 300       # Intervalo de reporte de estadísticas (segundos)

# Endpoints de depuración
[debug]
enabled = false            # Habilitar endpoints de debug
auth_token = "secret"      # Token de autenticación
allowed_ips = "127.0.0.1,192.168.1.0/24"  # IPs permitidas
enable_pprof = false       # Habilitar profiling con pprof

# Monitoreo
[monitor]
enabled = true                    # Habilitar endpoints de monitoreo
health_check_path = "/health"     # Endpoint de health check
metrics_path = "/metrics"         # Endpoint de métricas Prometheus
enable_detailed_metrics = true    # Incluir métricas detalladas
metrics_port = "9090"            # Puerto separado para métricas (opcional)

# Limitación de tasa
[rate_limit]
enabled = true                    # Habilitar limitación de tasa
ip_rate_limit = 100              # Solicitudes por IP por ventana
ip_window_minutes = 1            # Ventana de tiempo en minutos
ip_burst_size = 10               # Tamaño de ráfaga para límite IP
backend = "memory"               # "memory" o "redis"
cleanup_interval = 10            # Intervalo de limpieza en minutos
retry_after_header = true        # Incluir header Retry-After
error_message = "Límite de tasa excedido. Intente nuevamente más tarde."
excluded_ips = "127.0.0.1,10.0.0.0/8"     # IPs excluidas
excluded_paths = "/health,/metrics"        # Rutas excluidas

# Configuración de seguridad
[security]
# Análisis estático
enable_static_analysis = true     # Habilitar análisis estático de JavaScript
block_on_high_severity = true     # Bloquear problemas de alta severidad
log_security_warnings = true      # Registrar advertencias de seguridad
cache_analysis_results = true     # Cachear resultados del análisis
cache_ttl_minutes = 5            # TTL del cache en minutos
allowed_patterns = []            # Patrones en lista blanca

# Encriptación
enable_encryption = true          # Habilitar encriptación de datos
encryption_key = ""              # Clave de 32 bytes (base64 o hex)
encrypt_sensitive_data = true    # Auto-encriptar datos sensibles
encrypt_in_place = true          # Reemplazar valores en el lugar
always_encrypt_fields = [
    "password",
    "token",
    "secret",
    "api_key"
]
sensitive_patterns = [
    "email",
    "phone",
    "ssn",
    "credit_card",
    "api_key",
    "jwt"
]

# Patrones personalizados para datos sensibles
[security.custom_patterns]
employee_id = "EMP\\d{6}"
account_number = "ACC-\\d{4}-\\d{4}-\\d{4}"

# Sanitización de logs
enable_log_sanitization = true    # Habilitar sanitización de logs
log_masking_char = "*"           # Carácter de enmascaramiento
log_preserve_length = false      # Preservar longitud original
log_show_type = true            # Mostrar tipo de dato en reemplazo

# Patrones personalizados para sanitización de logs
[security.log_custom_patterns]
session_id = "sess_[a-zA-Z0-9]{32}"
internal_id = "INT-\\d{8}"

# Configuración de correo
[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587
smtp_user = "user@example.com"
smtp_password = "password"
smtp_from = "noreply@example.com"
use_tls = true

# Variables de entorno
[env]
scim_base = "https://localhost:8443"
openid_base = "https://localhost:8443"
custom_var = "value"
```

### Configuraciones Específicas por Entorno

#### Desarrollo

```toml
[debug]
enabled = true
auth_token = "dev-token"

[vm_pool]
max_size = 10
preload_size = 2

[security]
enable_static_analysis = true
block_on_high_severity = false  # Advertir pero no bloquear
```

#### Producción

```toml
[debug]
enabled = false

[vm_pool]
max_size = 200
preload_size = 100

[security]
enable_static_analysis = true
block_on_high_severity = true
enable_encryption = true
enable_log_sanitization = true

[rate_limit]
enabled = true
backend = "redis"
```

## Desarrollo de Workflows

### Creando Tu Primer Workflow

1. **Diseñar en nFlow**: Usa el diseñador visual en https://github.com/arturoeanton/nflow

2. **Nodo JavaScript Básico**:

```javascript
// Acceder a datos de entrada
const input = payload;

// Procesar datos
const result = {
    message: "Hola, " + input.name,
    timestamp: new Date().toISOString()
};

// Retornar datos para el siguiente nodo
return result;
```

3. **Usando Métodos del Contexto**:

```javascript
// Obtener datos de la petición HTTP
const body = c.request.body;
const headers = c.request.headers;
const query = c.request.query;

// Establecer respuesta
c.response.status = 200;
c.response.set("Content-Type", "application/json");

// Acceder a variables de entorno
const apiKey = env.API_KEY;

// Registrar mensajes (serán sanitizados si está habilitado)
console.log("Procesando usuario:", body.email);
```

### Trabajando con Variables

```javascript
// Establecer variables del workflow
vars.set("userId", 12345);
vars.set("userName", "Juan Pérez");

// Obtener variables
const userId = vars.get("userId");

// Verificar si existe una variable
if (vars.has("userId")) {
    // Procesar...
}

// Obtener todas las variables
const allVars = vars.getAll();
```

### Peticiones HTTP

```javascript
// Usando el plugin http
const response = await http.get("https://api.example.com/data", {
    headers: {
        "Authorization": "Bearer " + vars.get("token")
    }
});

// Petición POST
const result = await http.post("https://api.example.com/users", {
    body: {
        name: "Juan Pérez",
        email: "juan@example.com"
    },
    headers: {
        "Content-Type": "application/json"
    }
});
```

### Operaciones de Base de Datos

```javascript
// Consultar base de datos
const users = await db.query("SELECT * FROM users WHERE active = $1", [true]);

// Insertar datos
const result = await db.exec(
    "INSERT INTO logs (message, created_at) VALUES ($1, $2)",
    ["Usuario conectado", new Date()]
);

// Transacción
await db.transaction(async (tx) => {
    await tx.exec("UPDATE users SET credits = credits - 10 WHERE id = $1", [userId]);
    await tx.exec("INSERT INTO transactions (user_id, amount) VALUES ($1, $2)", [userId, -10]);
});
```

### Manejo de Errores

```javascript
try {
    // Operación riesgosa
    const data = await http.get("https://api.example.com/data");
    return data;
} catch (error) {
    console.error("Falló la llamada API:", error.message);
    
    // Retornar respuesta de error
    c.response.status = 500;
    return {
        error: "Fallo al obtener datos",
        details: error.message
    };
}
```

## Características de Seguridad

### Análisis Estático

El analizador estático verifica el código JavaScript antes de la ejecución:

```javascript
// Estos serán bloqueados:
eval("console.log('peligroso')");  // ❌ uso de eval
new Function("return 1");           // ❌ constructor Function
require('fs');                      // ❌ acceso al sistema de archivos
require('child_process');           // ❌ creación de procesos

// Estos están permitidos:
console.log("Operación segura");    // ✅
Math.random();                      // ✅
JSON.parse('{"key": "value"}');     // ✅
```

### Encriptación

Los datos sensibles se encriptan automáticamente:

```javascript
// Esto será encriptado automáticamente en las respuestas
const userData = {
    email: "usuario@example.com",    // Detectado como email
    phone: "555-123-4567",          // Detectado como teléfono
    apiKey: "sk_test_1234567890",   // Detectado como API key
    safe: "Esto permanece como texto plano"
};

// Encriptación manual
const encrypted = security.encrypt("datos sensibles");
const decrypted = security.decrypt(encrypted);
```

### Sanitización de Logs

Los logs se sanitizan automáticamente:

```javascript
// Log original
console.log("Email del usuario: juan@example.com, SSN: 123-45-6789");

// Salida sanitizada
// Email del usuario: [REDACTED:email], SSN: [REDACTED:ssn]
```

### Límites de Recursos

Cada script se ejecuta con límites:

```javascript
// Esto expirará después de 30 segundos (configurable)
while (true) {
    // Protección contra bucles infinitos
}

// Esto fallará si la memoria excede 128MB
const bigArray = new Array(100000000);

// Esto fallará después de 10M operaciones
for (let i = 0; i < 100000000; i++) {
    // Protección de CPU
}
```

## Optimización de Rendimiento

### Optimización del Pool de VMs

```toml
[vm_pool]
# Para escenarios de alto throughput
max_size = 500              # Aumentar tamaño del pool
preload_size = 250          # Pre-calentar más VMs
idle_timeout = 30           # Mantener VMs más tiempo
cleanup_interval = 15       # Limpieza menos frecuente

# Para entornos con memoria limitada
max_size = 50
preload_size = 10
idle_timeout = 5
max_memory_mb = 64          # Reducir memoria por VM
```

### Estrategias de Cache

1. **Habilitar Cache de Análisis**:
```toml
[security]
cache_analysis_results = true
cache_ttl_minutes = 10
```

2. **Usar Redis para Cache Distribuido**:
```toml
[redis]
host = "redis-cluster:6379"
```

3. **Optimizar Consultas de Base de Datos**:
```javascript
// Usar sentencias preparadas
const stmt = db.prepare("SELECT * FROM users WHERE id = $1");
const user = await stmt.query(userId);
```

### Monitoreo del Rendimiento

```bash
# Verificar estado del pool de VMs
curl http://localhost:8080/debug/vm/pool

# Obtener métricas detalladas
curl http://localhost:8080/metrics | grep nflow_

# Perfilar uso de CPU (cuando pprof está habilitado)
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

## Monitoreo y Depuración

### Health Checks

```bash
# Health check básico
curl http://localhost:8080/health

# Respuesta
{
    "status": "healthy",
    "timestamp": 1643723400,
    "uptime": "2h30m15s",
    "version": "1.0.0",
    "components": {
        "database": {
            "status": "healthy"
        },
        "redis": {
            "status": "healthy"
        },
        "memory": {
            "status": "healthy"
        }
    }
}
```

### Métricas Prometheus

Métricas clave para monitorear:

```prometheus
# Tasa de requests
rate(nflow_requests_total[5m])

# Tasa de errores
rate(nflow_requests_errors_total[5m]) / rate(nflow_requests_total[5m])

# Tiempo de ejecución de workflows
histogram_quantile(0.95, nflow_workflow_duration_seconds)

# Procesos activos
nflow_processes_active

# Utilización del pool de VMs
nflow_vm_pool_active / nflow_vm_pool_size

# Uso de memoria
nflow_go_memory_alloc_bytes
```

### Endpoints de Debug

```bash
# Obtener información del sistema
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/info

# Listar procesos activos
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/processes

# Ver configuración actual
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/config

# Estadísticas de cache
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/cache/stats
```

### Logging

```bash
# Ejecutar con logging verbose
./nflow-runtime -v

# Formato de logs
# 2024/01/02 15:04:05 [INFO] Iniciando nFlow Runtime
# 2024/01/02 15:04:05 [DEBUG] Cargando configuración desde config.toml
# 2024/01/02 15:04:05 [WARN] Redis no configurado, usando sesiones en memoria
```

## Desarrollo de Plugins

### Creando un Plugin Personalizado

1. **Definir la Estructura del Plugin**:

```go
package myplugin

import (
    "github.com/dop251/goja"
    "github.com/labstack/echo/v4"
)

type MyPlugin struct {
    config map[string]interface{}
}

func New(config map[string]interface{}) *MyPlugin {
    return &MyPlugin{
        config: config,
    }
}
```

2. **Implementar Métodos del Plugin**:

```go
func (p *MyPlugin) DoSomething(data string) (string, error) {
    // Tu implementación
    return "Procesado: " + data, nil
}

func (p *MyPlugin) RegisterVM(vm *goja.Runtime) {
    // Crear objeto del plugin
    obj := vm.NewObject()
    
    // Agregar métodos
    obj.Set("doSomething", p.DoSomething)
    
    // Registrar globalmente
    vm.Set("myPlugin", obj)
}
```

3. **Registrar en el Motor**:

```go
// En tu main.go o código de inicialización
import "github.com/arturoeanton/nflow-runtime/plugins/myplugin"

func init() {
    plugin := myplugin.New(config)
    engine.RegisterPlugin("myPlugin", plugin)
}
```

4. **Usar en Workflows**:

```javascript
// En tu JavaScript del workflow
const result = myPlugin.doSomething("datos de prueba");
console.log(result); // "Procesado: datos de prueba"
```

### Mejores Prácticas para Plugins

1. **Manejo de Errores**:
```go
func (p *MyPlugin) RiskyOperation() (interface{}, error) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("Plugin panic:", r)
        }
    }()
    
    // Tu código
}
```

2. **Gestión de Recursos**:
```go
type MyPlugin struct {
    pool sync.Pool
    mu   sync.RWMutex
}

func (p *MyPlugin) GetResource() *Resource {
    if r := p.pool.Get(); r != nil {
        return r.(*Resource)
    }
    return newResource()
}
```

3. **Validación de Configuración**:
```go
func New(config map[string]interface{}) (*MyPlugin, error) {
    // Validar campos requeridos
    if _, ok := config["apiKey"]; !ok {
        return nil, errors.New("apiKey es requerido")
    }
    
    return &MyPlugin{config: config}, nil
}
```

## Despliegue en Producción

### Checklist Pre-Producción

- [ ] Configurar base de datos de producción
- [ ] Configurar Redis para sesiones/cache
- [ ] Generar clave de encriptación segura
- [ ] Configurar limitación de tasa
- [ ] Habilitar características de seguridad
- [ ] Configurar monitoreo
- [ ] Configurar agregación de logs
- [ ] Planificar estrategia de backup
- [ ] Documentar runbooks
- [ ] Hacer pruebas de carga

### Despliegue con Docker

```dockerfile
# Dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o nflow-runtime .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/nflow-runtime .
COPY config.toml .
EXPOSE 8080
CMD ["./nflow-runtime"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  nflow:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis
    volumes:
      - ./config.toml:/root/config.toml
    
  postgres:
    image: postgres:14
    environment:
      POSTGRES_DB: nflow
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Despliegue en Kubernetes

```yaml
# deployment.yaml
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
        image: tu-registro/nflow-runtime:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          value: postgres-service
        - name: REDIS_HOST
          value: redis-service
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
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
        volumeMounts:
        - name: config
          mountPath: /app/config.toml
          subPath: config.toml
      volumes:
      - name: config
        configMap:
          name: nflow-config
```

### Configuración de Monitoreo

#### Configuración de Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'nflow-runtime'
    static_configs:
      - targets: ['nflow-runtime:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

#### Dashboard de Grafana

Importa el dashboard de nFlow Runtime (JSON disponible en el repo) o crea paneles personalizados:

```json
{
  "dashboard": {
    "title": "nFlow Runtime",
    "panels": [
      {
        "title": "Tasa de Requests",
        "targets": [{
          "expr": "rate(nflow_requests_total[5m])"
        }]
      },
      {
        "title": "Tasa de Errores",
        "targets": [{
          "expr": "rate(nflow_requests_errors_total[5m])"
        }]
      },
      {
        "title": "Workflows Activos",
        "targets": [{
          "expr": "nflow_processes_active"
        }]
      }
    ]
  }
}
```

## Solución de Problemas

### Problemas Comunes

#### Alto Uso de Memoria

**Síntomas**: Errores OOM, rendimiento lento

**Soluciones**:
1. Reducir tamaño del pool de VMs
2. Bajar límite de memoria por VM
3. Habilitar GC más agresivo:
```go
GOGC=50 ./nflow-runtime
```

#### Timeouts en Workflows

**Síntomas**: Errores 408, ejecuciones incompletas

**Soluciones**:
1. Aumentar timeout de ejecución:
```toml
max_execution_seconds = 60
```
2. Optimizar código JavaScript
3. Usar operaciones asíncronas donde sea posible

#### Problemas de Conexión a Base de Datos

**Síntomas**: Errores "too many connections"

**Soluciones**:
1. Configurar pooling de conexiones:
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```
2. Usar un pooler de conexiones (PgBouncer)

#### Falsos Positivos en Rate Limiting

**Síntomas**: Usuarios legítimos bloqueados

**Soluciones**:
1. Excluir IPs confiables:
```toml
excluded_ips = "10.0.0.0/8,172.16.0.0/12"
```
2. Aumentar límites de tasa
3. Usar backend Redis para límites distribuidos

### Técnicas de Depuración

#### Habilitar Logging Verbose

```bash
./nflow-runtime -v
```

#### Rastrear Workflow Específico

```javascript
// Agregar logs de debug en el workflow
console.log("[DEBUG] Iniciando proceso:", vars.get("processId"));
console.log("[DEBUG] Payload:", JSON.stringify(payload));
```

#### Usar Endpoints de Debug

```bash
# Verificar estado del proceso
curl -H "Authorization: Bearer secret" \
  http://localhost:8080/debug/process/12345

# Forzar recolección de basura
curl -X POST -H "Authorization: Bearer secret" \
  http://localhost:8080/debug/gc
```

#### Perfilar Rendimiento

```bash
# Perfil de CPU
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Perfil de memoria
curl http://localhost:8080/debug/pprof/heap > mem.prof
go tool pprof mem.prof

# Dump de goroutines
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

## Temas Avanzados

### Autenticación Personalizada

Implementar middleware de autenticación personalizado:

```go
func AuthMiddleware(config *Config) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Extraer token
            token := c.Request().Header.Get("Authorization")
            
            // Validar token
            if !isValidToken(token) {
                return echo.ErrUnauthorized
            }
            
            // Establecer contexto de usuario
            c.Set("user", getUserFromToken(token))
            
            return next(c)
        }
    }
}
```

### Versionado de Workflows

Implementar versionado de workflows:

```javascript
// En el workflow
const version = vars.get("workflowVersion") || "1.0";

switch(version) {
    case "1.0":
        // Lógica original
        break;
    case "2.0":
        // Nueva lógica
        break;
    default:
        throw new Error("Versión desconocida: " + version);
}
```

### Trazado Distribuido

Integrar con OpenTelemetry:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func TraceMiddleware() echo.MiddlewareFunc {
    tracer := otel.Tracer("nflow-runtime")
    
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            ctx, span := tracer.Start(c.Request().Context(), "workflow")
            defer span.End()
            
            c.SetRequest(c.Request().WithContext(ctx))
            return next(c)
        }
    }
}
```

### Integración de Webhooks

Manejar webhooks en workflows:

```javascript
// Verificar firma del webhook
const signature = c.request.headers["x-webhook-signature"];
const payload = c.request.body;

const expectedSignature = crypto
    .createHmac('sha256', env.WEBHOOK_SECRET)
    .update(JSON.stringify(payload))
    .digest('hex');

if (signature !== expectedSignature) {
    c.response.status = 401;
    return { error: "Firma inválida" };
}

// Procesar webhook
switch(payload.event) {
    case "user.created":
        // Manejar creación de usuario
        break;
    case "payment.completed":
        // Manejar pago
        break;
}
```

### Streaming de Eventos

Implementar Server-Sent Events:

```javascript
// En el workflow
c.response.set("Content-Type", "text/event-stream");
c.response.set("Cache-Control", "no-cache");
c.response.set("Connection", "keep-alive");

// Enviar eventos
for (let i = 0; i < 10; i++) {
    const event = {
        id: i,
        data: { message: "Actualización " + i },
        timestamp: new Date()
    };
    
    c.response.write(`data: ${JSON.stringify(event)}\n\n`);
    await sleep(1000); // Función sleep personalizada
}

c.response.end();
```

### Multi-Tenancy

Implementar aislamiento de inquilinos:

```go
type TenantMiddleware struct {
    tenantResolver func(c echo.Context) string
}

func (tm *TenantMiddleware) Process(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        tenantID := tm.tenantResolver(c)
        
        // Establecer contexto del inquilino
        c.Set("tenantID", tenantID)
        
        // Configurar base de datos específica del inquilino
        db := getDBForTenant(tenantID)
        c.Set("db", db)
        
        return next(c)
    }
}
```

## Mejores Prácticas

### Diseño de Workflows

1. **Mantener workflows simples**: Dividir lógica compleja en múltiples nodos
2. **Manejar errores graciosamente**: Siempre incluir manejo de errores
3. **Usar nombres descriptivos**: Nombrar nodos y variables claramente
4. **Documentar lógica compleja**: Agregar comentarios en nodos JavaScript
5. **Probar casos límite**: Incluir datos de prueba para varios escenarios

### Seguridad

1. **Nunca hardcodear secretos**: Usar variables de entorno
2. **Validar todas las entradas**: No confiar en datos externos
3. **Usar encriptación**: Habilitar encriptación para datos sensibles
4. **Actualizaciones regulares**: Mantener dependencias actualizadas
5. **Logs de auditoría**: Habilitar logging comprensivo

### Rendimiento

1. **Reutilizar conexiones**: Usar pooling de conexiones
2. **Operaciones en lote**: Agrupar operaciones de base de datos
3. **Asíncrono cuando sea posible**: Usar promesas para operaciones I/O
4. **Cachear resultados**: Cachear cálculos costosos
5. **Monitorear métricas**: Vigilar degradación del rendimiento

### Operaciones

1. **Automatizar despliegue**: Usar pipelines CI/CD
2. **Monitorear todo**: Configurar monitoreo comprensivo
3. **Planificar para fallos**: Implementar circuit breakers
4. **Documentar runbooks**: Crear procedimientos operacionales
5. **Backups regulares**: Implementar estrategia de backup

## Conclusión

nFlow Runtime proporciona una plataforma robusta para ejecutar workflows con características de nivel empresarial. Siguiendo este manual y las mejores prácticas, puedes construir soluciones de workflow seguras, escalables y mantenibles.

Para soporte adicional:
- GitHub Issues: https://github.com/arturoeanton/nflow-runtime/issues
- Documentación: https://github.com/arturoeanton/nflow-runtime/docs
- Comunidad: Únete a nuestro canal Discord/Slack

Recuerda siempre probar exhaustivamente en un entorno de staging antes de desplegar a producción.