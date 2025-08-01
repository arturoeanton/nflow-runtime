# nFlow Runtime - Complete Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Basic Configuration](#basic-configuration)
4. [Architecture Overview](#architecture-overview)
5. [Core Concepts](#core-concepts)
6. [Security Features](#security-features)
7. [Performance Optimization](#performance-optimization)
8. [Plugin Development](#plugin-development)
9. [Advanced Configuration](#advanced-configuration)
10. [Monitoring and Debugging](#monitoring-and-debugging)
11. [Production Deployment](#production-deployment)
12. [Troubleshooting](#troubleshooting)
13. [API Reference](#api-reference)

## Introduction

nFlow Runtime is a high-performance workflow execution engine designed to run workflows created in the nFlow visual designer. It provides a secure, scalable environment with extensive monitoring capabilities.

### Key Features

- **JavaScript Execution**: Secure sandboxed environment for running JavaScript code
- **High Performance**: Optimized for handling thousands of requests per second
- **Security First**: Multiple layers of security including sandboxing, resource limits, and static analysis
- **Extensible**: Plugin system for custom functionality
- **Observable**: Built-in metrics, logging, and debugging capabilities

### Use Cases

- Workflow automation
- API orchestration
- Data processing pipelines
- Business process automation
- Integration middleware

## Installation

### Prerequisites

- Go 1.19 or higher
- PostgreSQL 12+ or SQLite3
- Redis 6+ (optional, for sessions and rate limiting)
- Git

### From Source

```bash
# Clone the repository
git clone https://github.com/arturoeanton/nflow-runtime.git
cd nflow-runtime

# Build the binary
go build -o nflow-runtime .

# Run tests
go test ./...

# Run with verbose output
./nflow-runtime -v
```

### As a Go Module

```bash
go get github.com/arturoeanton/nflow-runtime
```

### Docker Installation

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

## Basic Configuration

### Minimal config.toml

```toml
[database_nflow]
driver = "sqlite3"
dsn = "app.sqlite"

[vm_pool]
max_size = 50
preload_size = 25
```

### Database Configuration

#### SQLite (Development)

```toml
[database_nflow]
driver = "sqlite3"
dsn = "app.sqlite"
```

#### PostgreSQL (Production)

```toml
[database_nflow]
driver = "postgres"
dsn = "user=nflow password=secret host=localhost port=5432 dbname=nflow sslmode=require"
query = "SELECT name,query FROM queries"
```

### Redis Configuration

```toml
[redis]
host = "localhost:6379"
password = "your-redis-password"
db = 0
```

## Architecture Overview

### Component Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│   HTTP Layer    │────▶│  Engine Core     │────▶│  VM Pool     │
│   (Echo)        │     │                  │     │  (Goja)      │
└─────────────────┘     └──────────────────┘     └──────────────┘
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│  Rate Limiter   │     │  Process Manager │     │  Security    │
│                 │     │                  │     │  Module      │
└─────────────────┘     └──────────────────┘     └──────────────┘
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│   Database      │     │     Cache        │     │   Metrics    │
│  (PostgreSQL)   │     │  (Memory/Redis)  │     │ (Prometheus) │
└─────────────────┘     └──────────────────┘     └──────────────┘
```

### Request Flow

1. **HTTP Request** arrives at Echo router
2. **Rate Limiting** checks if request should proceed
3. **Security Analysis** validates JavaScript code
4. **VM Acquisition** from pool for execution
5. **Workflow Execution** with resource limits
6. **Response Processing** with optional encryption
7. **Metrics Collection** and logging

### Directory Structure

```
nflow-runtime/
├── engine/               # Core execution engine
│   ├── engine.go        # Main workflow executor
│   ├── vm_manager.go    # VM pool management
│   ├── vm_limits.go     # Resource limit enforcement
│   └── vm_sandbox.go    # Security sandboxing
├── security/            # Security module
│   ├── analyzer/        # Static code analysis
│   ├── encryption/      # Data encryption
│   └── interceptor/     # Sensitive data detection
├── process/             # Process management
├── endpoints/           # API endpoints
├── plugins/             # Plugin system
└── main.go             # Entry point
```

## Core Concepts

### Workflows

A workflow is a JSON structure that defines:
- **Nodes**: Individual execution units
- **Connections**: How data flows between nodes
- **Data**: Input/output mappings

Example workflow structure:
```json
{
  "node1": {
    "data": {
      "type": "httpstart",
      "method": "POST",
      "path": "/api/process"
    },
    "outputs": {
      "output": {
        "connections": [{
          "node": "node2",
          "output": "input"
        }]
      }
    }
  },
  "node2": {
    "data": {
      "type": "js",
      "script": "return {result: payload.value * 2};"
    }
  }
}
```

### Nodes

Built-in node types:
- **httpstart**: HTTP endpoint trigger
- **js**: JavaScript execution
- **http**: HTTP client
- **db**: Database operations
- **mail**: Email sending
- **template**: Template rendering

### VM Pool

The VM pool manages JavaScript runtime instances:

```toml
[vm_pool]
max_size = 200              # Maximum VMs in pool
preload_size = 100          # Pre-created VMs
idle_timeout = 10           # Minutes before idle VM removal
cleanup_interval = 5        # Cleanup run interval
```

Benefits:
- Eliminates VM creation overhead
- Predictable performance
- Resource efficiency

## Security Features

### JavaScript Sandboxing

#### Resource Limits

```toml
[vm_pool]
max_memory_mb = 128         # Memory limit per VM
max_execution_seconds = 30  # Execution timeout
max_operations = 10000000   # JavaScript operation limit
```

#### Disabled Features

- `eval()` and `Function()` constructor
- File system access (configurable)
- Network access (configurable)
- Process spawning

#### Enable/Disable Features

```toml
[vm_pool]
enable_filesystem = false   # File system access
enable_network = false      # Network access
enable_process = false      # Process execution
```

### Static Code Analysis

The security module analyzes JavaScript before execution:

```toml
[security]
enable_static_analysis = true
block_on_high_severity = true
log_security_warnings = true
```

Detected patterns:
- Direct `eval()` usage
- `require('fs')` attempts
- Child process spawning
- Infinite loops
- Global scope modifications

Example blocked code:
```javascript
// This will be blocked
eval("malicious code");
require('fs').readFile('/etc/passwd');
while(true) { }
```

### Data Encryption

Automatic encryption of sensitive data:

```toml
[security]
enable_encryption = true
encryption_key = "your-32-byte-key-here"
encrypt_sensitive_data = true
```

Automatically encrypted:
- Email addresses
- Phone numbers
- Social Security Numbers
- API keys
- JWT tokens
- Credit card numbers

#### Generating Encryption Keys

```go
// Generate a secure key
key, err := encryption.GenerateKeyString()
if err != nil {
    log.Fatal(err)
}
fmt.Println("Add to config.toml:", key)
```

### Rate Limiting

IP-based rate limiting with token bucket algorithm:

```toml
[rate_limit]
enabled = true
ip_rate_limit = 100         # Requests per window
ip_window_minutes = 1       # Time window
backend = "memory"          # or "redis"
```

Advanced configuration:
```toml
[rate_limit]
burst_size = 10
cleanup_interval = 10
excluded_ips = "127.0.0.1,10.0.0.0/8"
excluded_paths = "/health,/metrics"
```

## Performance Optimization

### VM Pool Tuning

For high-traffic scenarios:

```toml
[vm_pool]
max_size = 500              # Increase pool size
preload_size = 250          # Pre-warm more VMs
idle_timeout = 5            # Aggressive cleanup
enable_metrics = true       # Monitor pool usage
```

### Caching

Multiple cache layers:

1. **Babel Cache**: ES6 transformation results
2. **Program Cache**: Compiled JavaScript programs
3. **Auth Cache**: Authentication scripts

### Database Optimization

```toml
[database_nflow]
max_open_conns = 100        # Connection pool size
max_idle_conns = 10         # Idle connections
conn_max_lifetime = 300     # Connection lifetime (seconds)
```

### Monitoring Performance

Enable detailed metrics:

```toml
[monitor]
enabled = true
enable_detailed_metrics = true
metrics_port = "9090"       # Separate metrics port
```

Key metrics to monitor:
- `nflow_vm_pool_active`: Active VMs
- `nflow_vm_pool_available`: Available VMs
- `nflow_requests_duration`: Request latency
- `nflow_workflows_total`: Workflow executions

## Plugin Development

### Creating a Custom Plugin

```go
package myplugin

import (
    "github.com/dop251/goja"
    "github.com/labstack/echo/v4"
)

type MyPlugin struct{}

func (p *MyPlugin) Name() string {
    return "myplugin"
}

func (p *MyPlugin) Initialize(vm *goja.Runtime) error {
    // Add functions to VM
    vm.Set("myFunction", func(call goja.FunctionCall) goja.Value {
        // Implementation
        return vm.ToValue("result")
    })
    return nil
}

func (p *MyPlugin) Execute(c echo.Context, vm *goja.Runtime) error {
    // Plugin logic
    return nil
}
```

### Registering Plugins

In your main.go:

```go
import "github.com/arturoeanton/nflow-runtime/plugins"

func init() {
    plugins.Register(&MyPlugin{})
}
```

### Plugin Best Practices

1. **Thread Safety**: Plugins must be thread-safe
2. **Error Handling**: Always return meaningful errors
3. **Resource Cleanup**: Use defer for cleanup
4. **Documentation**: Document all exposed functions
5. **Testing**: Include comprehensive tests

## Advanced Configuration

### Complete config.toml Example

```toml
# Database configuration
[database_nflow]
driver = "postgres"
dsn = "host=localhost user=nflow password=secret dbname=nflow sslmode=require"
max_open_conns = 100
max_idle_conns = 10
conn_max_lifetime = 300

# Redis for sessions and caching
[redis]
host = "localhost:6379"
password = "redis-password"
db = 0
max_retries = 3
pool_size = 10

# VM Pool configuration
[vm_pool]
max_size = 200
preload_size = 100
idle_timeout = 10
cleanup_interval = 5
enable_metrics = true

# Resource limits
max_memory_mb = 128
max_execution_seconds = 30
max_operations = 10000000

# Sandbox settings
enable_filesystem = false
enable_network = true
enable_process = false

# Tracking and monitoring
[tracker]
enabled = true
workers = 8
batch_size = 1000
flush_interval = 500
channel_buffer = 100000
verbose_logging = false

# Debug endpoints
[debug]
enabled = false
auth_token = "debug-token-12345"
allowed_ips = "10.0.0.0/8,172.16.0.0/12"
enable_pprof = false

# Monitoring
[monitor]
enabled = true
health_check_path = "/health"
metrics_path = "/metrics"
enable_detailed_metrics = true
metrics_port = "9090"

# Rate limiting
[rate_limit]
enabled = true
ip_rate_limit = 1000
ip_window_minutes = 1
ip_burst_size = 50
backend = "redis"
cleanup_interval = 10
retry_after_header = true
error_message = "Too many requests. Please try again later."
excluded_ips = "10.0.0.0/8,172.16.0.0/12"
excluded_paths = "/health,/metrics,/debug"

# Security
[security]
enable_static_analysis = true
block_on_high_severity = true
log_security_warnings = true
cache_analysis_results = true
cache_ttl_minutes = 5

enable_encryption = true
encryption_key = "your-base64-encoded-32-byte-key"
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

# Email configuration
[mail]
enabled = true
smtp_host = "smtp.gmail.com"
smtp_port = 587
username = "nflow@example.com"
password = "smtp-password"
from = "nFlow Runtime <nflow@example.com>"
```

### Environment Variables

Override config with environment variables:

```bash
export NFLOW_DATABASE_DSN="postgres://user:pass@host/db"
export NFLOW_REDIS_HOST="redis.example.com:6379"
export NFLOW_VM_POOL_MAX_SIZE="500"
export NFLOW_SECURITY_ENCRYPTION_KEY="your-secure-key"
```

## Monitoring and Debugging

### Health Checks

Default endpoint: `GET /health`

Response:
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

### Prometheus Metrics

Default endpoint: `GET /metrics`

Key metrics:
```
# HTTP metrics
nflow_requests_total{method="POST",path="/api/*",status="200"} 12345
nflow_requests_duration_seconds{method="POST",quantile="0.99"} 0.125

# Workflow metrics
nflow_workflows_total{status="success"} 10000
nflow_workflows_duration_seconds{quantile="0.99"} 2.5

# VM Pool metrics
nflow_vm_pool_active 45
nflow_vm_pool_available 155
nflow_vm_pool_created_total 200

# Security metrics
nflow_security_scripts_analyzed_total 5000
nflow_security_scripts_blocked_total 23
nflow_security_data_encrypted_total 1500
```

### Debug Endpoints

Enable debug endpoints:

```toml
[debug]
enabled = true
auth_token = "your-secret-token"
allowed_ips = "10.0.0.0/8"
```

Available endpoints:
- `GET /debug/info` - System information
- `GET /debug/config` - Current configuration
- `GET /debug/processes` - Active processes
- `GET /debug/cache/stats` - Cache statistics
- `GET /debug/vm/pool` - VM pool status

Example request:
```bash
curl -H "X-Debug-Token: your-secret-token" http://localhost:8080/debug/info
```

### Logging

#### Log Levels

Run with verbose logging:
```bash
./nflow-runtime -v
```

Log format:
```
2024-01-15 10:30:45 [INFO] Started nFlow Runtime
2024-01-15 10:30:45 [DEBUG] VM Pool: Created 100 VMs
2024-01-15 10:30:46 [WARN] Security: Blocked script with eval()
2024-01-15 10:30:47 [ERROR] Database connection failed: timeout
```

#### Structured Logging

Configure structured logging:

```go
logger.SetFormatter(&logger.JSONFormatter{})
logger.SetLevel(logger.DebugLevel)
```

Output:
```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "info",
  "msg": "Workflow executed",
  "workflow_id": "abc123",
  "duration_ms": 125,
  "status": "success"
}
```

## Production Deployment

### System Requirements

#### Minimum Requirements
- CPU: 2 cores
- RAM: 4GB
- Disk: 10GB SSD
- Network: 100Mbps

#### Recommended for Production
- CPU: 8+ cores
- RAM: 16GB+
- Disk: 100GB+ SSD
- Network: 1Gbps

### Deployment Checklist

1. **Security**
   - [ ] Change all default passwords
   - [ ] Enable HTTPS/TLS
   - [ ] Configure firewall rules
   - [ ] Enable rate limiting
   - [ ] Enable security module
   - [ ] Rotate encryption keys

2. **Database**
   - [ ] Use PostgreSQL for production
   - [ ] Configure connection pooling
   - [ ] Set up regular backups
   - [ ] Enable SSL for connections

3. **Monitoring**
   - [ ] Configure Prometheus
   - [ ] Set up Grafana dashboards
   - [ ] Configure alerting rules
   - [ ] Enable health checks

4. **Performance**
   - [ ] Tune VM pool size
   - [ ] Configure caching
   - [ ] Enable compression
   - [ ] Set up CDN for static assets

### Kubernetes Deployment

Example deployment:

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

### Scaling Strategies

#### Horizontal Scaling
- Add more instances behind load balancer
- Use Redis for shared state
- Configure session affinity if needed

#### Vertical Scaling
- Increase VM pool size
- Add more CPU/RAM
- Tune garbage collection

#### Database Scaling
- Read replicas for queries
- Connection pooling
- Query optimization

## Troubleshooting

### Common Issues

#### High Memory Usage

Symptoms:
- OOM errors
- Slow performance
- System instability

Solutions:
1. Reduce VM pool size
2. Lower memory limits
3. Enable aggressive GC
4. Check for memory leaks

#### Performance Degradation

Symptoms:
- Increased latency
- Timeout errors
- Queue buildup

Solutions:
1. Check VM pool metrics
2. Analyze slow queries
3. Review workflow complexity
4. Enable caching

#### Security Blocks

Symptoms:
- Scripts rejected
- "Dangerous pattern" errors

Solutions:
1. Review security logs
2. Whitelist safe patterns
3. Refactor problematic code
4. Adjust severity levels

### Debug Commands

Check system status:
```bash
# VM pool status
curl http://localhost:8080/debug/vm/pool

# Active processes
curl http://localhost:8080/debug/processes

# Cache statistics
curl http://localhost:8080/debug/cache/stats
```

### Performance Profiling

Enable pprof:
```toml
[debug]
enable_pprof = true
```

Profile CPU:
```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

Profile memory:
```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```

## API Reference

### Workflow Execution

#### Execute Workflow

```
POST /api/workflow/{workflow_id}
```

Headers:
- `Content-Type: application/json`
- `Authorization: Bearer {token}`

Request body:
```json
{
  "input": {
    "key": "value"
  }
}
```

Response:
```json
{
  "success": true,
  "output": {
    "result": "processed"
  },
  "execution_time": 125
}
```

### Administrative Endpoints

#### List Workflows

```
GET /api/admin/workflows
```

#### Update Workflow

```
PUT /api/admin/workflows/{workflow_id}
```

#### Delete Workflow

```
DELETE /api/admin/workflows/{workflow_id}
```

### Plugin APIs

Plugins can expose custom endpoints:

```
POST /api/plugin/{plugin_name}/{action}
```

## Best Practices

### Security

1. **Never disable sandboxing** in production
2. **Rotate encryption keys** regularly
3. **Monitor security metrics** for anomalies
4. **Review blocked scripts** for false positives
5. **Keep dependencies updated**

### Performance

1. **Right-size VM pool** based on load
2. **Use caching** for repeated operations
3. **Monitor resource usage** continuously
4. **Optimize database queries**
5. **Profile before optimizing**

### Operations

1. **Automate deployments** with CI/CD
2. **Use infrastructure as code**
3. **Implement proper logging**
4. **Set up alerting thresholds**
5. **Document runbooks**

## Appendix

### Glossary

- **VM**: Virtual Machine (JavaScript runtime instance)
- **Workflow**: Executable flow definition
- **Node**: Single execution unit in a workflow
- **Sandbox**: Isolated execution environment
- **Pool**: Collection of pre-initialized VMs

### References

- [Goja Documentation](https://github.com/dop251/goja)
- [Echo Framework](https://echo.labstack.com/)
- [Prometheus Metrics](https://prometheus.io/)
- [nFlow Designer](https://github.com/arturoeanton/nflow)

### Version History

- v1.0.0 - Initial release
- v1.1.0 - Added VM pooling
- v1.2.0 - Security module
- v1.3.0 - Rate limiting
- v1.4.0 - Advanced monitoring

---

For more information, visit [https://github.com/arturoeanton/nflow-runtime](https://github.com/arturoeanton/nflow-runtime)