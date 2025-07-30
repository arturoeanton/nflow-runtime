# nFlow Runtime

Workflow execution engine for [nFlow](https://github.com/arturoeanton/nflow). This project executes workflows created in the nFlow visual designer, providing a secure environment with resource limits and sandboxing.

## 🚀 Installation

```bash
go get github.com/arturoeanton/nflow-runtime
```

## 📋 Requirements

- Go 1.19 or higher
- PostgreSQL or SQLite3
- Redis (optional, for sessions)
- Configuration in `config.toml`

## 🎯 Features

- **Secure Execution**: JavaScript sandboxing with configurable resource limits
- **High Performance**: Handles 5M+ requests in 8 hours
- **Thread-Safe**: Race condition-free architecture using Repository Pattern
- **Extensible**: Plugin system for custom functionality
- **Detailed Logging**: Structured logging system with verbose mode (-v)

## 🔧 Configuration

### config.toml

```toml
[database_nflow]
driver = "postgres"
dsn = "user=postgres dbname=nflow sslmode=disable"

[redis]
host = "localhost:6379"
password = ""

[vm_pool]
# Resource limits (security)
max_memory_mb = 128        # Maximum memory per VM
max_execution_seconds = 30 # Maximum execution time
max_operations = 10000000  # Maximum JS operations

# Sandbox settings
enable_filesystem = false  # Filesystem access
enable_network = false     # Network access
enable_process = false     # Process access

[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587
```

## 🏃‍♂️ Basic Usage

### As Standalone Server

```bash
# Normal mode
./nflow-runtime

# Verbose mode (detailed logging)
./nflow-runtime -v
```

Server will be available at `http://localhost:8080`

### As Library

```go
import (
    "github.com/arturoeanton/nflow-runtime/engine"
    "github.com/arturoeanton/nflow-runtime/process"
)

func main() {
    // Initialize configuration
    configRepo := engine.GetConfigRepository()
    config := engine.ConfigWorkspace{
        // ... configuration
    }
    configRepo.SetConfig(config)
    
    // Initialize database
    db, err := engine.GetDB()
    if err != nil {
        log.Fatal(err)
    }
    engine.InitializePlaybookRepository(db)
    
    // Initialize process manager
    process.InitializeRepository()
    
    // Create Echo server
    e := echo.New()
    e.Any("/*", run)
    e.Start(":8080")
}
```

## 🛡️ Security

### Resource Limits

Each VM has configurable limits to prevent DoS attacks:
- **Memory**: 128MB by default
- **Time**: 30 seconds maximum
- **Operations**: 10M JavaScript operations

### Sandboxing

JavaScript executes in a restricted environment:
- ❌ `eval()` blocked
- ❌ `Function` constructor blocked
- ❌ Filesystem access disabled by default
- ❌ Network access disabled by default
- ✅ Only whitelisted modules available

## 🔌 Available Plugins

- **goja**: Main JavaScript engine
- **mail**: Email sending
- **template**: Template processing
- **ianflow**: AI integration (OpenAI, Gemini, Ollama)
- **http**: HTTP client for API calls
- **db**: Database operations
- **babel**: ES6+ code transpilation

## 📊 Architecture

```
nflow-runtime/
├── engine/             # Main execution engine
│   ├── engine.go       # Workflow execution logic
│   ├── vm_limits.go    # Resource limit management
│   ├── vm_sandbox.go   # Sandbox implementation
│   └── config_repository.go # Repository pattern for config
├── process/            # Process management
│   └── process_repository.go # Thread-safe repository
├── logger/             # Logging system
│   └── logger.go       # Structured logger with levels
├── syncsession/        # Optimized session management
├── plugins/            # System plugins
└── main.go            # Server entry point
```

## 🧩 Custom Steps

You can create your own node types:

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
    // Your implementation here
    return nextNode, payload, nil
}

// Register the step
engine.RegisterStep("my-custom-step", &MyCustomStep{})
```

## 📈 Metrics and Monitoring

The system includes basic metrics. With `-v` enabled, you'll see:
- Execution time for each node
- VM memory usage
- Operations per second
- Detailed execution flow logs

## 🚨 Error Handling

Errors are handled consistently:
- HTTP 408: Resource limit exceeded
- HTTP 500: Internal server error
- HTTP 404: Workflow not found

## 🔄 Project Status

- **Maturity**: 4/5 ⭐ (Production ready for moderate loads)
- **Stability**: STABLE ✅
- **Security**: GOOD ✅
- **Performance**: 5M+ requests/8h ✅

See [STATUS.md](STATUS.md) for more details.

## 🐛 Known Issues

See [DEUDA.md](DEUDA.md) for the complete technical debt list.

## 🤝 Contributing

1. Fork the project
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📝 License

MIT - see LICENSE file for details.

## 🙏 Acknowledgments

- [Goja](https://github.com/dop251/goja) - JavaScript engine in Go
- [Echo](https://echo.labstack.com/) - Web framework
- [nFlow](https://github.com/arturoeanton/nflow) - Visual workflow designer