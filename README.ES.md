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
- **Alto Rendimiento**: Maneja 5M+ requests en 8 horas
- **Thread-Safe**: Arquitectura sin condiciones de carrera usando Repository Pattern
- **Extensible**: Sistema de plugins para agregar funcionalidad personalizada
- **Logging Detallado**: Sistema de logs estructurado con modo verbose (-v)

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
# Límites de recursos (seguridad)
max_memory_mb = 128        # Memoria máxima por VM
max_execution_seconds = 30 # Tiempo máximo de ejecución
max_operations = 10000000  # Operaciones JS máximas

# Configuración de sandbox
enable_filesystem = false  # Acceso al sistema de archivos
enable_network = false     # Acceso a red
enable_process = false     # Acceso a procesos

[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587
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
│   ├── vm_limits.go    # Gestión de límites de recursos
│   ├── vm_sandbox.go   # Implementación del sandbox
│   └── config_repository.go # Patrón repository para config
├── process/            # Gestión de procesos
│   └── process_repository.go # Repository thread-safe
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

El sistema incluye métricas básicas. Con `-v` habilitado, verás:
- Tiempos de ejecución de cada nodo
- Uso de memoria de las VMs
- Operaciones por segundo
- Logs detallados de flujo de ejecución

## 🚨 Manejo de Errores

Los errores se manejan de forma consistente:
- HTTP 408: Límite de recursos excedido
- HTTP 500: Error interno del servidor
- HTTP 404: Workflow no encontrado

## 🔄 Estado del Proyecto

- **Madurez**: 4/5 ⭐ (Producción con cargas moderadas)
- **Estabilidad**: ESTABLE ✅
- **Seguridad**: BUENA ✅
- **Performance**: 5M+ requests/8h ✅

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