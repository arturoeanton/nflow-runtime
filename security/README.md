# Security Module for nFlow Runtime

This module provides transparent security features for nFlow Runtime without modifying the core engine. It includes static analysis of JavaScript code and automatic encryption of sensitive data.

## Features

### 1. Static Code Analysis
- Detects dangerous patterns in JavaScript before execution
- Configurable severity levels (high, medium, low)
- Can block execution based on security policies
- Patterns detected:
  - `eval()` and `Function()` constructor usage
  - File system access attempts
  - Process spawning attempts
  - Network access
  - Infinite loops
  - Global scope modifications
  - Potentially dangerous regex patterns

### 2. Data Encryption
- AES-256-GCM encryption for sensitive data
- Automatic detection of sensitive information
- Two modes:
  - **In-place encryption**: Replaces sensitive values directly
  - **Metadata mode**: Adds encryption metadata without modifying structure
- Built-in patterns for:
  - Email addresses
  - Phone numbers
  - Social Security Numbers (SSN)
  - Credit card numbers
  - API keys and tokens
  - JWT tokens
- Support for custom patterns

### 3. Performance Optimizations
- Thread-safe implementation
- Buffer pooling for encryption
- Concurrent operation support
- Minimal overhead when disabled

## Configuration

Add to your `config.toml`:

```toml
[security]
# Static Analysis
enable_static_analysis = true
block_on_high_severity = true
log_security_warnings = true

# Encryption
enable_encryption = true
encryption_key = "your-32-byte-key-here"
encrypt_sensitive_data = true
encrypt_in_place = true

# Always encrypt these fields
always_encrypt_fields = ["password", "token", "secret"]

# Custom patterns
[security.custom_patterns]
employee_id = "EMP\\d{6}"
```

## Usage

### Basic Integration

```go
// Create security middleware
config := &security.Config{
    EnableStaticAnalysis: true,
    EnableEncryption: true,
    EncryptionKey: "your-secure-key",
}

sm, err := security.NewSecurityMiddleware(config)
if err != nil {
    log.Fatal(err)
}

// Analyze JavaScript before execution
err = sm.AnalyzeScript(jsCode, "script-id")
if err != nil {
    // Script blocked due to security issues
    return err
}

// Process response data
secureData, err := sm.ProcessResponse(responseData)
if err != nil {
    return err
}
```

### Generating Encryption Keys

```go
// Generate a secure key
key, err := encryption.GenerateKeyString()
if err != nil {
    log.Fatal(err)
}
fmt.Println("Add this to config.toml:", key)
```

## Performance

Benchmark results on MacBook Pro M1:

```
BenchmarkAnalyzer_SafeScript-8          50000    25483 ns/op
BenchmarkAnalyzer_DangerousScript-8     30000    45632 ns/op
BenchmarkEncryption_Small-8            300000     4521 ns/op
BenchmarkEncryption_Large-8             10000   125634 ns/op
BenchmarkInterceptor_MediumData-8       20000    65432 ns/op
```

## Security Considerations

1. **Encryption Keys**: 
   - Store encryption keys securely (use environment variables or key management systems)
   - Rotate keys periodically
   - Never commit keys to version control

2. **False Positives**:
   - Use `allowed_patterns` to whitelist safe patterns
   - Adjust severity levels based on your security requirements

3. **Performance**:
   - Enable only the features you need
   - Use caching for static analysis results
   - Consider using metadata mode for large responses

## Testing

Run all tests:
```bash
go test ./security/...
```

Run benchmarks:
```bash
go test -bench=. ./security/...
```

Run with race detection:
```bash
go test -race ./security/...
```

## Contributing

When adding new features:
1. Maintain backward compatibility
2. Add comprehensive tests
3. Include benchmarks for performance-critical code
4. Update documentation
5. Ensure thread-safety

## License

Same as nFlow Runtime (MIT)