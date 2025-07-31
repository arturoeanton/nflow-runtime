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
- **Complete Monitoring**: Prometheus metrics and health checks
- **Advanced Debugging**: Debug endpoints with authentication
- **Optimized**: Smart caching and highly optimized code
- **Rate Limiting**: IP-based rate limiting with configurable backends

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

[tracker]
enabled = false            # Execution tracking (performance impact)
verbose_logging = false    # Detailed tracker logs

[monitor]
enabled = true             # Monitoring endpoints
health_check_path = "/health"
metrics_path = "/metrics"

[debug]
enabled = false            # Debug endpoints (development only)
auth_token = ""           # Authentication token
allowed_ips = ""          # Allowed IPs (e.g., "192.168.1.0/24")

[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587

[rate_limit]
enabled = false            # IP-based rate limiting
ip_rate_limit = 100       # Requests per IP per window
ip_window_minutes = 1     # Time window in minutes
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
├── endpoints/          # API endpoints
│   ├── debug_endpoints.go    # Debug endpoints
│   └── monitor_endpoints.go  # Health & metrics
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

### Monitoring Endpoints

- **Health Check**: `GET /health` - System health status
- **Prometheus Metrics**: `GET /metrics` - All metrics in Prometheus format

### Available Metrics

- `nflow_requests_total`: Total HTTP requests
- `nflow_workflows_total`: Total workflows executed
- `nflow_processes_active`: Active processes
- `nflow_db_connections_*`: Database connection metrics
- `nflow_go_memory_*`: Memory usage
- `nflow_cache_hits/misses`: Cache statistics

### Debug Endpoints (when enabled)

- `/debug/info`: System information
- `/debug/config`: Current configuration
- `/debug/processes`: Active process list
- `/debug/cache/stats`: Cache statistics
- `/debug/database/stats`: Database metrics

See [DEBUG_MONITORING.md](DEBUG_MONITORING.md) for complete documentation.

## 🛡️ Rate Limiting

nFlow Runtime includes IP-based rate limiting to protect against abuse:

- Token bucket algorithm for flexible rate control
- Memory and Redis backends for different deployment scenarios
- Configurable exclusions for IPs and paths
- Detailed headers for client integration

See [RATE_LIMITING.md](RATE_LIMITING.md) for complete documentation.

## 🚨 Error Handling

Errors are handled consistently:
- HTTP 408: Resource limit exceeded
- HTTP 500: Internal server error
- HTTP 404: Workflow not found

## 🔄 Project Status

- **Maturity**: 4.8/5 ⭐ (Production ready)
- **Stability**: STABLE ✅
- **Security**: VERY GOOD ✅
- **Performance**: 5M+ requests/8h ✅
- **Observability**: COMPLETE ✅
- **Production Ready**: 90% ✅

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