# nFlow Runtime

Motor de ejecución de workflows extraído de [nFlow](https://github.com/arturoeanton/nflow).

## Instalación

```bash
go get github.com/arturoeanton/nflow-runtime
```

## Uso Básico

```go
import runtime "github.com/arturoeanton/nflow-runtime"

// Configurar el motor
config := &runtime.Config{
    MaxVMPool:      50,
    MaxConcurrency: 100,
    EnableBabel:    true,
    DatabaseConfig: &runtime.DatabaseConfig{
        Driver: "sqlite3",
        DSN:    "./nflow.db",
    },
}

// Crear motor
engine := runtime.NewEngine(config)

// Ejecutar workflow
err := engine.Execute(controller, context, vars, "", "/api/endpoint", processID, nil)
```

## Configuración de Dependencias

El runtime necesita que se configuren ciertas dependencias externas:

```go
engine.SetDependencies(&engine.ExternalDependencies{
    GetDB: func() (*sql.DB, error) {
        // Retornar conexión a base de datos
    },
    AddFeatureSession: func(vm *goja.Runtime, c echo.Context) {
        // Agregar funciones de sesión al contexto JS
    },
    // ... otras dependencias
})
```

## Steps Disponibles

- `js`: Ejecuta código JavaScript
- `gorutine`: Ejecuta en goroutine paralela
- `dromedary`: Plugin genérico
- `dromedary_callback`: Plugin con callback asíncrono

## Registrar Steps Personalizados

```go
type MyStep struct{}

func (s *MyStep) Run(cc *model.Controller, actor *model.Node, c echo.Context, 
    vm *goja.Runtime, connection_next string, vars model.Vars, 
    currentProcess *process.Process, payload goja.Value) (string, goja.Value, error) {
    // Implementación
    return nextNode, payload, nil
}

engine.RegisterStep("my-step", &MyStep{})
```

## Estructura del Proyecto

```
nflow-runtime/
├── api.go              # API pública principal
├── engine/             # Motor de ejecución
│   ├── engine.go       # Lógica principal
│   ├── interfaces.go   # Interfaces y dependencias
│   ├── session.go      # Manejo de sesiones
│   └── step_*.go       # Implementaciones de steps
├── model/              # Estructuras de datos
│   └── model.go        # Node, Playbook, Controller
├── process/            # Gestión de procesos
│   └── process.go      # Process manager
└── example/            # Ejemplo de uso
    └── main.go
```

## Ejemplo Completo

Ver `example/main.go` para un ejemplo completo de cómo usar la librería.

## Licencia

MIT