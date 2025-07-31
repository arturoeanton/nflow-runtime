# Rate Limiting Documentation

## Overview

nFlow Runtime includes a configurable IP-based rate limiting system to protect your API from abuse and ensure fair usage. The rate limiter uses a token bucket algorithm and supports both in-memory and Redis backends for distributed deployments.

## Features

- **IP-based rate limiting**: Limits requests per IP address
- **Token bucket algorithm**: Allows burst traffic while maintaining average rate
- **Multiple backends**: In-memory (single instance) or Redis (distributed)
- **Path exclusions**: Exempt specific paths like health checks
- **IP exclusions**: Whitelist trusted IPs or IP ranges
- **Configurable responses**: Custom error messages and Retry-After headers
- **Zero performance impact when disabled**: No overhead when rate limiting is off

## Configuration

Rate limiting is configured in the `config.toml` file:

```toml
[rate_limit]
enabled = false                   # Enable rate limiting (default: false)

# IP rate limiting configuration
ip_rate_limit = 100              # Requests per IP per window
ip_window_minutes = 1            # Time window in minutes
ip_burst_size = 10               # Burst size for IP limiting

# Storage backend
backend = "memory"               # "memory" or "redis"
cleanup_interval = 10            # Cleanup interval in minutes (memory backend)

# Response configuration
retry_after_header = true        # Include Retry-After header
error_message = "Rate limit exceeded. Please try again later."

# Exclusions (comma-separated)
excluded_ips = ""                # e.g., "127.0.0.1,192.168.1.0/24"
excluded_paths = "/health,/metrics"  # Paths to exclude from rate limiting
```

### Configuration Parameters

#### Basic Settings

- **`enabled`**: Master switch for rate limiting. Set to `true` to enable.
- **`ip_rate_limit`**: Maximum number of requests allowed per IP in the time window
- **`ip_window_minutes`**: Time window duration in minutes
- **`ip_burst_size`**: Additional requests allowed for handling traffic bursts

#### Backend Selection

- **`backend`**: Choose storage backend
  - `"memory"`: Uses in-memory storage (default). Best for single-instance deployments.
  - `"redis"`: Uses Redis for distributed rate limiting across multiple instances.
- **`cleanup_interval`**: For memory backend, how often to clean up expired entries (in minutes)

#### Response Configuration

- **`retry_after_header`**: When `true`, includes `Retry-After` header in 429 responses
- **`error_message`**: Custom error message returned when rate limit is exceeded

#### Exclusions

- **`excluded_ips`**: Comma-separated list of IPs or CIDR ranges to exclude from rate limiting
  - Examples: `"127.0.0.1"`, `"192.168.1.0/24"`, `"10.0.0.0/8,172.16.0.0/12"`
- **`excluded_paths`**: Comma-separated list of path prefixes to exclude
  - Examples: `"/health"`, `"/metrics"`, `"/health,/metrics,/api/public"`

## How It Works

### Token Bucket Algorithm

The rate limiter uses a token bucket algorithm:

1. Each IP address gets a bucket with `ip_rate_limit` tokens
2. Each request consumes one token
3. Tokens are refilled at a rate of `ip_rate_limit` per `ip_window_minutes`
4. The bucket can hold up to `ip_rate_limit + ip_burst_size` tokens
5. If no tokens are available, the request is rejected with HTTP 429

### Example Scenarios

**Configuration:**
```toml
ip_rate_limit = 60
ip_window_minutes = 1
ip_burst_size = 10
```

This allows:
- 60 requests per minute on average
- Up to 70 requests in a burst (60 + 10)
- After a burst, the client must wait for tokens to refill

## Response Headers

When rate limiting is active, the following headers are included:

### Successful Requests
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1672531260
```

### Rate Limited Requests (HTTP 429)
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1672531260
Retry-After: 30
```

Response body:
```json
{
  "error": "Rate limit exceeded. Please try again later.",
  "retry_after": 30
}
```

## Backend Comparison

### Memory Backend

**Pros:**
- Zero configuration
- Very fast (microsecond latency)
- No external dependencies

**Cons:**
- Not suitable for distributed deployments
- Rate limits are per-instance, not global
- Data lost on restart

**Use when:**
- Running a single instance
- Don't need persistent rate limit data
- Want maximum performance

### Redis Backend

**Pros:**
- Works across multiple instances
- Persistent rate limit data
- True distributed rate limiting

**Cons:**
- Requires Redis setup
- Slightly higher latency (milliseconds)
- Additional infrastructure dependency

**Use when:**
- Running multiple instances
- Need consistent rate limiting across all instances
- Already using Redis for sessions

## Examples

### Basic Setup (Single Instance)

```toml
[rate_limit]
enabled = true
ip_rate_limit = 100
ip_window_minutes = 1
backend = "memory"
```

### Production Setup (Multiple Instances)

```toml
[rate_limit]
enabled = true
ip_rate_limit = 1000
ip_window_minutes = 1
ip_burst_size = 50
backend = "redis"
excluded_ips = "10.0.0.0/8"  # Internal network
excluded_paths = "/health,/metrics"
```

### Strict API Protection

```toml
[rate_limit]
enabled = true
ip_rate_limit = 10
ip_window_minutes = 1
ip_burst_size = 0  # No burst allowed
retry_after_header = true
error_message = "API rate limit exceeded. Maximum 10 requests per minute."
```

## Client IP Detection

The rate limiter detects client IPs in the following order:

1. `X-Real-IP` header (set by reverse proxies)
2. `X-Forwarded-For` header (leftmost IP if multiple)
3. `RemoteAddr` from the connection

### Behind a Reverse Proxy

Ensure your reverse proxy sets the appropriate headers:

**Nginx:**
```nginx
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

**Apache:**
```apache
RequestHeader set X-Real-IP "%{REMOTE_ADDR}s"
RequestHeader set X-Forwarded-For "%{REMOTE_ADDR}s"
```

## Monitoring

### Logs

When verbose logging is enabled (`-v` flag), rate limit events are logged:

```
Rate limit exceeded for IP: 192.168.1.100, path: /api/workflow
```

### Metrics

If Prometheus metrics are enabled, you can monitor:
- Total rate limited requests
- Rate limit hits by IP
- Current bucket states

## Troubleshooting

### Rate Limiting Not Working

1. Check `enabled = true` in config.toml
2. Verify the configuration is loaded (check startup logs)
3. Ensure you're not hitting excluded paths or IPs

### All Requests Being Limited

1. Check if `ip_rate_limit` is too low
2. Verify time windows are appropriate
3. Check client IP detection (might be seeing proxy IP)

### Redis Backend Issues

1. Verify Redis connection in logs
2. Check Redis is accessible from the application
3. Ensure Redis has enough memory for rate limit keys

## Best Practices

1. **Start Conservative**: Begin with higher limits and reduce as needed
2. **Monitor Impact**: Watch your metrics after enabling
3. **Exclude Health Checks**: Always exclude monitoring endpoints
4. **Use Burst for APIs**: Allow some burst to handle legitimate traffic spikes
5. **Different Limits for Different Paths**: Consider using a reverse proxy for path-specific limits

## Security Considerations

1. **IP Spoofing**: In production, ensure proper header validation at your edge proxy
2. **Distributed Attacks**: Consider additional DDoS protection at the network level
3. **Resource Exhaustion**: Monitor memory usage with memory backend under attack conditions

## Performance Impact

- **Disabled**: Zero overhead
- **Memory Backend**: ~1-2 microseconds per request
- **Redis Backend**: ~1-5 milliseconds per request (depends on Redis latency)

The rate limiter is designed to have minimal impact on legitimate traffic while effectively preventing abuse.