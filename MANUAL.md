# nFlow Runtime Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Getting Started](#getting-started)
3. [Basic Concepts](#basic-concepts)
4. [Configuration Guide](#configuration-guide)
5. [Workflow Development](#workflow-development)
6. [Security Features](#security-features)
7. [Performance Tuning](#performance-tuning)
8. [Monitoring & Debugging](#monitoring--debugging)
9. [Plugin Development](#plugin-development)
10. [Production Deployment](#production-deployment)
11. [Troubleshooting](#troubleshooting)
12. [Advanced Topics](#advanced-topics)

## Introduction

nFlow Runtime is a high-performance workflow execution engine designed to run workflows created in the nFlow visual designer. It provides a secure, scalable, and observable platform for executing JavaScript-based workflows with enterprise-grade features.

### Key Features

- **Secure Execution**: Sandboxed JavaScript environment with resource limits
- **High Performance**: 3,396+ RPS with compute-intensive workflows
- **Enterprise Security**: Static analysis, encryption, and log sanitization
- **Complete Observability**: Health checks, Prometheus metrics, debug endpoints
- **Flexible Architecture**: Plugin system for extensibility

## Getting Started

### Prerequisites

- Go 1.19 or higher
- PostgreSQL 9.6+ or SQLite3
- Redis 5.0+ (optional, for session management)
- Git

### Installation

#### From Source

```bash
# Clone the repository
git clone https://github.com/arturoeanton/nflow-runtime.git
cd nflow-runtime

# Build the binary
go build -o nflow-runtime .

# Run the server
./nflow-runtime
```

#### As Go Module

```bash
go get github.com/arturoeanton/nflow-runtime
```

### Quick Start

1. Create a basic `config.toml`:

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

2. Run the server:

```bash
./nflow-runtime -v  # Verbose mode for detailed logs
```

3. Verify the server is running:

```bash
curl http://localhost:8080/health
```

## Basic Concepts

### Workflows

A workflow is a series of connected nodes (steps) that execute in sequence or parallel. Each node performs a specific action and can pass data to subsequent nodes.

### Nodes/Steps

Nodes are the building blocks of workflows:
- **httpstart**: Entry point for HTTP-triggered workflows
- **js**: Execute JavaScript code
- **http**: Make HTTP requests
- **db**: Database operations
- **mail**: Send emails
- **template**: Process templates

### Process

A process is an instance of a workflow execution. Each process has:
- Unique ID
- Status (running, completed, failed)
- Variables/state
- Execution history

### Payload

Data passed between nodes. Can be any JSON-serializable value.

## Configuration Guide

### Complete Configuration Reference

```toml
# Database configuration
[database_nflow]
driver = "postgres"  # postgres, mysql, sqlite3
dsn = "host=localhost user=postgres password=secret dbname=nflow sslmode=disable"
query = "SELECT name,query FROM queries"  # Custom query table

# Redis configuration (optional)
[redis]
host = "localhost:6379"
password = ""
db = 0

# Session storage
[pg_session]
url = "postgres://user:pass@localhost/sessions?sslmode=disable"

# VM Pool configuration
[vm_pool]
max_size = 200              # Maximum VMs in pool
preload_size = 100          # VMs to create at startup
idle_timeout = 10           # Minutes before removing idle VMs
cleanup_interval = 5        # Minutes between cleanup runs
enable_metrics = true       # Log pool metrics

# Resource limits per VM
max_memory_mb = 128         # Max memory per VM (MB)
max_execution_seconds = 30  # Max execution time (seconds)
max_operations = 10000000   # Max JS operations

# Sandbox settings
enable_filesystem = false   # Allow filesystem access
enable_network = false      # Allow network access
enable_process = false      # Allow process spawning

# Execution tracking
[tracker]
enabled = false             # Enable execution tracking
workers = 4                 # Number of worker goroutines
batch_size = 100           # Batch size for DB inserts
flush_interval = 250       # Flush interval (ms)
channel_buffer = 100000    # Channel buffer size
verbose_logging = false    # Enable verbose logging
stats_interval = 300       # Stats reporting interval (seconds)

# Debug endpoints
[debug]
enabled = false            # Enable debug endpoints
auth_token = "secret"      # Authentication token
allowed_ips = "127.0.0.1,192.168.1.0/24"  # Allowed IPs
enable_pprof = false       # Enable Go pprof profiling

# Monitoring
[monitor]
enabled = true                    # Enable monitoring endpoints
health_check_path = "/health"     # Health check endpoint
metrics_path = "/metrics"         # Prometheus metrics endpoint
enable_detailed_metrics = true    # Include detailed metrics
metrics_port = "9090"            # Separate port for metrics (optional)

# Rate limiting
[rate_limit]
enabled = true                    # Enable rate limiting
ip_rate_limit = 100              # Requests per IP per window
ip_window_minutes = 1            # Time window in minutes
ip_burst_size = 10               # Burst size for IP limiting
backend = "memory"               # "memory" or "redis"
cleanup_interval = 10            # Cleanup interval in minutes
retry_after_header = true        # Include Retry-After header
error_message = "Rate limit exceeded. Please try again later."
excluded_ips = "127.0.0.1,10.0.0.0/8"     # Excluded IPs
excluded_paths = "/health,/metrics"        # Excluded paths

# Security configuration
[security]
# Static analysis
enable_static_analysis = true     # Enable JavaScript static analysis
block_on_high_severity = true     # Block high severity issues
log_security_warnings = true      # Log security warnings
cache_analysis_results = true     # Cache analysis results
cache_ttl_minutes = 5            # Cache TTL in minutes
allowed_patterns = []            # Whitelisted patterns

# Encryption
enable_encryption = true          # Enable data encryption
encryption_key = ""              # 32-byte key (base64 or hex)
encrypt_sensitive_data = true    # Auto-encrypt sensitive data
encrypt_in_place = true          # Replace values in-place
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

# Custom patterns for sensitive data
[security.custom_patterns]
employee_id = "EMP\\d{6}"
account_number = "ACC-\\d{4}-\\d{4}-\\d{4}"

# Log sanitization
enable_log_sanitization = true    # Enable log sanitization
log_masking_char = "*"           # Masking character
log_preserve_length = false      # Preserve original length
log_show_type = true            # Show data type in replacement

# Custom patterns for log sanitization
[security.log_custom_patterns]
session_id = "sess_[a-zA-Z0-9]{32}"
internal_id = "INT-\\d{8}"

# Mail configuration
[mail]
enabled = false
smtp_host = "smtp.gmail.com"
smtp_port = 587
smtp_user = "user@example.com"
smtp_password = "password"
smtp_from = "noreply@example.com"
use_tls = true

# Environment variables
[env]
scim_base = "https://localhost:8443"
openid_base = "https://localhost:8443"
custom_var = "value"
```

### Environment-Specific Configurations

#### Development

```toml
[debug]
enabled = true
auth_token = "dev-token"

[vm_pool]
max_size = 10
preload_size = 2

[security]
enable_static_analysis = true
block_on_high_severity = false  # Warn but don't block
```

#### Production

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

## Workflow Development

### Creating Your First Workflow

1. **Design in nFlow**: Use the visual designer at https://github.com/arturoeanton/nflow

2. **Basic JavaScript Node**:

```javascript
// Access input data
const input = payload;

// Process data
const result = {
    message: "Hello, " + input.name,
    timestamp: new Date().toISOString()
};

// Return data for next node
return result;
```

3. **Using Context Methods**:

```javascript
// Get HTTP request data
const body = c.request.body;
const headers = c.request.headers;
const query = c.request.query;

// Set response
c.response.status = 200;
c.response.set("Content-Type", "application/json");

// Access environment variables
const apiKey = env.API_KEY;

// Log messages (will be sanitized if enabled)
console.log("Processing user:", body.email);
```

### Working with Variables

```javascript
// Set workflow variables
vars.set("userId", 12345);
vars.set("userName", "John Doe");

// Get variables
const userId = vars.get("userId");

// Check if variable exists
if (vars.has("userId")) {
    // Process...
}

// Get all variables
const allVars = vars.getAll();
```

### HTTP Requests

```javascript
// Using the http plugin
const response = await http.get("https://api.example.com/data", {
    headers: {
        "Authorization": "Bearer " + vars.get("token")
    }
});

// POST request
const result = await http.post("https://api.example.com/users", {
    body: {
        name: "John Doe",
        email: "john@example.com"
    },
    headers: {
        "Content-Type": "application/json"
    }
});
```

### Database Operations

```javascript
// Query database
const users = await db.query("SELECT * FROM users WHERE active = $1", [true]);

// Insert data
const result = await db.exec(
    "INSERT INTO logs (message, created_at) VALUES ($1, $2)",
    ["User logged in", new Date()]
);

// Transaction
await db.transaction(async (tx) => {
    await tx.exec("UPDATE users SET credits = credits - 10 WHERE id = $1", [userId]);
    await tx.exec("INSERT INTO transactions (user_id, amount) VALUES ($1, $2)", [userId, -10]);
});
```

### Error Handling

```javascript
try {
    // Risky operation
    const data = await http.get("https://api.example.com/data");
    return data;
} catch (error) {
    console.error("API call failed:", error.message);
    
    // Return error response
    c.response.status = 500;
    return {
        error: "Failed to fetch data",
        details: error.message
    };
}
```

## Security Features

### Static Analysis

The static analyzer checks JavaScript code before execution:

```javascript
// These will be blocked:
eval("console.log('dangerous')");  // ❌ eval usage
new Function("return 1");           // ❌ Function constructor
require('fs');                      // ❌ Filesystem access
require('child_process');           // ❌ Process spawning

// These are allowed:
console.log("Safe operation");      // ✅
Math.random();                      // ✅
JSON.parse('{"key": "value"}');     // ✅
```

### Encryption

Sensitive data is automatically encrypted:

```javascript
// This will be automatically encrypted in responses
const userData = {
    email: "user@example.com",      // Detected as email
    phone: "555-123-4567",          // Detected as phone
    apiKey: "sk_test_1234567890",   // Detected as API key
    safe: "This stays as plain text"
};

// Manual encryption
const encrypted = security.encrypt("sensitive data");
const decrypted = security.decrypt(encrypted);
```

### Log Sanitization

Logs are automatically sanitized:

```javascript
// Original log
console.log("User email: john@example.com, SSN: 123-45-6789");

// Sanitized output
// User email: [REDACTED:email], SSN: [REDACTED:ssn]
```

### Resource Limits

Each script runs with limits:

```javascript
// This will timeout after 30 seconds (configurable)
while (true) {
    // Infinite loop protection
}

// This will fail if memory exceeds 128MB
const bigArray = new Array(100000000);

// This will fail after 10M operations
for (let i = 0; i < 100000000; i++) {
    // CPU protection
}
```

## Performance Tuning

### VM Pool Optimization

```toml
[vm_pool]
# For high-throughput scenarios
max_size = 500              # Increase pool size
preload_size = 250          # Pre-warm more VMs
idle_timeout = 30           # Keep VMs longer
cleanup_interval = 15       # Less frequent cleanup

# For memory-constrained environments
max_size = 50
preload_size = 10
idle_timeout = 5
max_memory_mb = 64          # Reduce per-VM memory
```

### Caching Strategies

1. **Enable Analysis Caching**:
```toml
[security]
cache_analysis_results = true
cache_ttl_minutes = 10
```

2. **Use Redis for Distributed Caching**:
```toml
[redis]
host = "redis-cluster:6379"
```

3. **Optimize Database Queries**:
```javascript
// Use prepared statements
const stmt = db.prepare("SELECT * FROM users WHERE id = $1");
const user = await stmt.query(userId);
```

### Monitoring Performance

```bash
# Check VM pool status
curl http://localhost:8080/debug/vm/pool

# Get detailed metrics
curl http://localhost:8080/metrics | grep nflow_

# Profile CPU usage (when pprof enabled)
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

## Monitoring & Debugging

### Health Checks

```bash
# Basic health check
curl http://localhost:8080/health

# Response
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

### Prometheus Metrics

Key metrics to monitor:

```prometheus
# Request rate
rate(nflow_requests_total[5m])

# Error rate
rate(nflow_requests_errors_total[5m]) / rate(nflow_requests_total[5m])

# Workflow execution time
histogram_quantile(0.95, nflow_workflow_duration_seconds)

# Active processes
nflow_processes_active

# VM pool utilization
nflow_vm_pool_active / nflow_vm_pool_size

# Memory usage
nflow_go_memory_alloc_bytes
```

### Debug Endpoints

```bash
# Get system information
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/info

# List active processes
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/processes

# View current configuration
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/config

# Cache statistics
curl -H "Authorization: Bearer secret" http://localhost:8080/debug/cache/stats
```

### Logging

```bash
# Run with verbose logging
./nflow-runtime -v

# Log format
# 2024/01/02 15:04:05 [INFO] Starting nFlow Runtime
# 2024/01/02 15:04:05 [DEBUG] Loading configuration from config.toml
# 2024/01/02 15:04:05 [WARN] Redis not configured, using in-memory sessions
```

## Plugin Development

### Creating a Custom Plugin

1. **Define the Plugin Structure**:

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

2. **Implement Plugin Methods**:

```go
func (p *MyPlugin) DoSomething(data string) (string, error) {
    // Your implementation
    return "Processed: " + data, nil
}

func (p *MyPlugin) RegisterVM(vm *goja.Runtime) {
    // Create plugin object
    obj := vm.NewObject()
    
    // Add methods
    obj.Set("doSomething", p.DoSomething)
    
    // Register globally
    vm.Set("myPlugin", obj)
}
```

3. **Register in Engine**:

```go
// In your main.go or initialization code
import "github.com/arturoeanton/nflow-runtime/plugins/myplugin"

func init() {
    plugin := myplugin.New(config)
    engine.RegisterPlugin("myPlugin", plugin)
}
```

4. **Use in Workflows**:

```javascript
// In your workflow JavaScript
const result = myPlugin.doSomething("test data");
console.log(result); // "Processed: test data"
```

### Plugin Best Practices

1. **Error Handling**:
```go
func (p *MyPlugin) RiskyOperation() (interface{}, error) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("Plugin panic:", r)
        }
    }()
    
    // Your code
}
```

2. **Resource Management**:
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

3. **Configuration Validation**:
```go
func New(config map[string]interface{}) (*MyPlugin, error) {
    // Validate required fields
    if _, ok := config["apiKey"]; !ok {
        return nil, errors.New("apiKey is required")
    }
    
    return &MyPlugin{config: config}, nil
}
```

## Production Deployment

### Pre-Production Checklist

- [ ] Configure production database
- [ ] Set up Redis for sessions/caching
- [ ] Generate secure encryption key
- [ ] Configure rate limiting
- [ ] Enable security features
- [ ] Set up monitoring
- [ ] Configure log aggregation
- [ ] Plan backup strategy
- [ ] Document runbooks
- [ ] Load test the system

### Docker Deployment

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

### Kubernetes Deployment

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
        image: your-registry/nflow-runtime:latest
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

### Monitoring Setup

#### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'nflow-runtime'
    static_configs:
      - targets: ['nflow-runtime:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

#### Grafana Dashboard

Import the nFlow Runtime dashboard (JSON available in repo) or create custom panels:

```json
{
  "dashboard": {
    "title": "nFlow Runtime",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "rate(nflow_requests_total[5m])"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "rate(nflow_requests_errors_total[5m])"
        }]
      },
      {
        "title": "Active Workflows",
        "targets": [{
          "expr": "nflow_processes_active"
        }]
      }
    ]
  }
}
```

## Troubleshooting

### Common Issues

#### High Memory Usage

**Symptoms**: OOM errors, slow performance

**Solutions**:
1. Reduce VM pool size
2. Lower per-VM memory limit
3. Enable more aggressive GC:
```go
GOGC=50 ./nflow-runtime
```

#### Workflow Timeouts

**Symptoms**: 408 errors, incomplete executions

**Solutions**:
1. Increase execution timeout:
```toml
max_execution_seconds = 60
```
2. Optimize JavaScript code
3. Use async operations where possible

#### Database Connection Issues

**Symptoms**: "too many connections" errors

**Solutions**:
1. Configure connection pooling:
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```
2. Use connection pooler (PgBouncer)

#### Rate Limiting False Positives

**Symptoms**: Legitimate users blocked

**Solutions**:
1. Exclude trusted IPs:
```toml
excluded_ips = "10.0.0.0/8,172.16.0.0/12"
```
2. Increase rate limits
3. Use Redis backend for distributed limiting

### Debug Techniques

#### Enable Verbose Logging

```bash
./nflow-runtime -v
```

#### Trace Specific Workflow

```javascript
// Add debug logs in workflow
console.log("[DEBUG] Starting process:", vars.get("processId"));
console.log("[DEBUG] Payload:", JSON.stringify(payload));
```

#### Use Debug Endpoints

```bash
# Check process status
curl -H "Authorization: Bearer secret" \
  http://localhost:8080/debug/process/12345

# Force garbage collection
curl -X POST -H "Authorization: Bearer secret" \
  http://localhost:8080/debug/gc
```

#### Profile Performance

```bash
# CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Memory profile
curl http://localhost:8080/debug/pprof/heap > mem.prof
go tool pprof mem.prof

# Goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

## Advanced Topics

### Custom Authentication

Implement custom authentication middleware:

```go
func AuthMiddleware(config *Config) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Extract token
            token := c.Request().Header.Get("Authorization")
            
            // Validate token
            if !isValidToken(token) {
                return echo.ErrUnauthorized
            }
            
            // Set user context
            c.Set("user", getUserFromToken(token))
            
            return next(c)
        }
    }
}
```

### Workflow Versioning

Implement workflow versioning:

```javascript
// In workflow
const version = vars.get("workflowVersion") || "1.0";

switch(version) {
    case "1.0":
        // Original logic
        break;
    case "2.0":
        // New logic
        break;
    default:
        throw new Error("Unknown version: " + version);
}
```

### Distributed Tracing

Integrate with OpenTelemetry:

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

### Webhook Integration

Handle webhooks in workflows:

```javascript
// Verify webhook signature
const signature = c.request.headers["x-webhook-signature"];
const payload = c.request.body;

const expectedSignature = crypto
    .createHmac('sha256', env.WEBHOOK_SECRET)
    .update(JSON.stringify(payload))
    .digest('hex');

if (signature !== expectedSignature) {
    c.response.status = 401;
    return { error: "Invalid signature" };
}

// Process webhook
switch(payload.event) {
    case "user.created":
        // Handle user creation
        break;
    case "payment.completed":
        // Handle payment
        break;
}
```

### Event Streaming

Implement Server-Sent Events:

```javascript
// In workflow
c.response.set("Content-Type", "text/event-stream");
c.response.set("Cache-Control", "no-cache");
c.response.set("Connection", "keep-alive");

// Send events
for (let i = 0; i < 10; i++) {
    const event = {
        id: i,
        data: { message: "Update " + i },
        timestamp: new Date()
    };
    
    c.response.write(`data: ${JSON.stringify(event)}\n\n`);
    await sleep(1000); // Custom sleep function
}

c.response.end();
```

### Multi-Tenancy

Implement tenant isolation:

```go
type TenantMiddleware struct {
    tenantResolver func(c echo.Context) string
}

func (tm *TenantMiddleware) Process(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        tenantID := tm.tenantResolver(c)
        
        // Set tenant context
        c.Set("tenantID", tenantID)
        
        // Configure tenant-specific database
        db := getDBForTenant(tenantID)
        c.Set("db", db)
        
        return next(c)
    }
}
```

## Best Practices

### Workflow Design

1. **Keep workflows simple**: Break complex logic into multiple nodes
2. **Handle errors gracefully**: Always include error handling
3. **Use descriptive names**: Name nodes and variables clearly
4. **Document complex logic**: Add comments in JavaScript nodes
5. **Test edge cases**: Include test data for various scenarios

### Security

1. **Never hardcode secrets**: Use environment variables
2. **Validate all inputs**: Don't trust external data
3. **Use encryption**: Enable encryption for sensitive data
4. **Regular updates**: Keep dependencies updated
5. **Audit logs**: Enable comprehensive logging

### Performance

1. **Reuse connections**: Use connection pooling
2. **Batch operations**: Group database operations
3. **Async when possible**: Use promises for I/O operations
4. **Cache results**: Cache expensive computations
5. **Monitor metrics**: Watch for performance degradation

### Operations

1. **Automate deployment**: Use CI/CD pipelines
2. **Monitor everything**: Set up comprehensive monitoring
3. **Plan for failure**: Implement circuit breakers
4. **Document runbooks**: Create operational procedures
5. **Regular backups**: Implement backup strategy

## Conclusion

nFlow Runtime provides a robust platform for executing workflows with enterprise-grade features. By following this manual and best practices, you can build secure, scalable, and maintainable workflow solutions.

For additional support:
- GitHub Issues: https://github.com/arturoeanton/nflow-runtime/issues
- Documentation: https://github.com/arturoeanton/nflow-runtime/docs
- Community: Join our Discord/Slack channel

Remember to always test thoroughly in a staging environment before deploying to production.