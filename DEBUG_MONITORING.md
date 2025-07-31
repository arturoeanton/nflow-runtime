# Debug and Monitoring Endpoints

This document describes the debug and monitoring endpoints available in nFlow Runtime.

## Configuration

Configure these endpoints in `config.toml`:

```toml
[debug]
enabled = false           # Enable debug endpoints (default: false)
auth_token = ""          # Optional auth token for debug endpoints
allowed_ips = ""         # Comma-separated allowed IPs (empty = all)
enable_pprof = false     # Enable Go pprof profiling endpoints

[monitor]
enabled = true                    # Enable monitoring endpoints (default: true)
health_check_path = "/health"     # Health check endpoint path
metrics_path = "/metrics"         # Prometheus metrics endpoint path
enable_detailed_metrics = false   # Include detailed metrics
metrics_port = ""                # Separate port for metrics (empty = use main port)
```

## Monitoring Endpoints

### Health Check
- **Endpoint**: `/health` (configurable)
- **Method**: GET, HEAD
- **Auth**: None required
- **Response**: JSON health status

```json
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
    "processes": {
      "status": "healthy"
    },
    "memory": {
      "status": "healthy"
    }
  }
}
```

Status codes:
- `200 OK`: System is healthy
- `503 Service Unavailable`: System is degraded

### Prometheus Metrics
- **Endpoint**: `/metrics` (configurable)
- **Method**: GET
- **Auth**: None required
- **Response**: Prometheus text format

Available metrics:
- `nflow_up`: Whether nFlow is running
- `nflow_uptime_seconds`: Uptime in seconds
- `nflow_requests_total`: Total HTTP requests
- `nflow_requests_errors_total`: Total request errors
- `nflow_requests_active`: Current active requests
- `nflow_request_duration_milliseconds`: Average request duration
- `nflow_workflows_total`: Total workflows executed
- `nflow_workflows_errors_total`: Total workflow errors
- `nflow_processes_active`: Active workflow processes
- `nflow_processes_total`: Total processes created
- `nflow_db_connections_*`: Database connection metrics
- `nflow_go_*`: Go runtime metrics
- `nflow_cache_*`: Cache hit/miss metrics

## Debug Endpoints

Debug endpoints are protected by:
1. Must be enabled in config (`debug.enabled = true`)
2. Optional auth token
3. Optional IP whitelist

### Authentication
If `auth_token` is configured, provide it via:
- Header: `X-Debug-Token: your-token`
- Query param: `?debug_token=your-token`

### Available Endpoints

#### System Information
- `GET /debug/info` - System information
- `GET /debug/config` - Current configuration (sanitized)
- `GET /debug/runtime` - Runtime statistics
- `GET /debug/goroutines` - Goroutine stack traces
- `GET /debug/memory` - Memory statistics

#### Repository Management
- `GET /debug/repositories` - Repository information
- `GET /debug/playbooks` - List all playbooks
- `GET /debug/playbook/:flow` - Get specific playbook

#### Cache Management
- `POST /debug/cache/invalidate` - Invalidate all cache
- `POST /debug/cache/invalidate/:flow` - Invalidate specific flow
- `GET /debug/cache/stats` - Cache statistics
- `GET /debug/url-cache` - URL cache contents
- `DELETE /debug/url-cache` - Clear URL cache

#### Process Management
- `GET /debug/processes` - List all processes
- `GET /debug/process/:wid` - Get specific process
- `DELETE /debug/process/:wid` - Kill specific process

#### Database
- `GET /debug/database/stats` - Database statistics
- `GET /debug/database/connections` - Connection status

#### Performance Profiling
If `enable_pprof = true`:
- `GET /debug/pprof/*` - Go pprof endpoints

## Usage Examples

### Basic health check
```bash
curl http://localhost:8080/health
```

### Get Prometheus metrics
```bash
curl http://localhost:8080/metrics
```

### Debug with auth token
```bash
curl -H "X-Debug-Token: my-secret-token" http://localhost:8080/debug/info
```

### Kill a process
```bash
curl -X DELETE -H "X-Debug-Token: my-secret-token" \
  http://localhost:8080/debug/process/uuid-here
```

### Invalidate cache
```bash
curl -X POST -H "X-Debug-Token: my-secret-token" \
  http://localhost:8080/debug/cache/invalidate
```

## Security Considerations

1. **Never enable debug endpoints in production** without proper authentication
2. Use strong auth tokens if enabling debug endpoints
3. Restrict access by IP when possible
4. Monitor access to debug endpoints
5. Consider running metrics on a separate port not exposed to the internet

## Prometheus Integration

Example Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'nflow'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Grafana Dashboard

Key metrics to monitor:
1. Request rate: `rate(nflow_requests_total[5m])`
2. Error rate: `rate(nflow_requests_errors_total[5m])`
3. Response time: `nflow_request_duration_milliseconds`
4. Active workflows: `nflow_processes_active`
5. Database connections: `nflow_db_connections_in_use`
6. Memory usage: `nflow_go_memory_alloc_bytes`

## Troubleshooting

### Debug endpoints return 404
- Check that `debug.enabled = true` in config.toml
- Restart the application after config changes

### Authentication failures
- Verify the auth token matches exactly
- Check that the header name is `X-Debug-Token`

### IP restrictions
- Ensure your IP is in the allowed list
- Use CIDR notation for IP ranges: `192.168.1.0/24`

### Metrics missing
- Verify `monitor.enabled = true`
- Check the configured paths match your requests
- Ensure no other handler is catching the metrics path