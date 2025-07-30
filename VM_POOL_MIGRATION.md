# VM Pool Migration Guide

## Overview

The VM Pool Manager solves race condition issues with Goja VM instances by implementing a thread-safe pool pattern similar to the Session Manager. This ensures safe concurrent access to JavaScript VMs in high-traffic scenarios.

## Key Benefits

1. **Thread Safety**: Eliminates race conditions with proper synchronization
2. **Performance**: Reuses VM instances instead of creating new ones
3. **Resource Management**: Controls memory usage with configurable pool size
4. **Metrics**: Built-in monitoring for pool performance

## Configuration

Add the following to your `config.toml`:

```toml
[vm_pool]
max_size = 50              # Maximum VMs in pool (default: 50)
preload_size = 25          # VMs to create at startup (default: max_size/2)
idle_timeout = 10          # Minutes before removing idle VMs (default: 10)
cleanup_interval = 5       # Minutes between cleanup runs (default: 5)
enable_metrics = true      # Log pool metrics (default: false)
```

## Migration Steps

### 1. Update Configuration

Add the VM pool configuration to your `config.toml` as shown above.

### 2. Monitor Performance

If metrics are enabled, you'll see periodic logs:
```
[VM Pool Metrics] Created: 25, InUse: 5, Available: 20, TotalUses: 1523, Errors: 0
```

### 3. Tune Pool Size

- For low traffic: `max_size = 10-20`
- For medium traffic: `max_size = 30-50`
- For high traffic: `max_size = 50-100`

## Testing for Race Conditions

Run the race detector tests:
```bash
go test -race ./engine -run TestVMManagerRaceCondition
```

Run all VM manager tests:
```bash
go test ./engine -run TestVMManager
```

Run benchmarks:
```bash
go test -bench=BenchmarkVMManager ./engine
```

## Troubleshooting

### Pool Exhaustion
If you see "VM pool exhausted" errors:
1. Increase `max_size` in config
2. Check for VM leaks (VMs not being released)
3. Enable metrics to monitor usage patterns

### High Memory Usage
1. Decrease `max_size` and `preload_size`
2. Reduce `idle_timeout` to clean up VMs faster
3. Monitor with `enable_metrics = true`

### Performance Issues
1. Increase `preload_size` to have more VMs ready
2. Adjust `idle_timeout` based on traffic patterns
3. Use benchmarks to find optimal settings

## Architecture

The VM Pool Manager implements:
- **Pool Pattern**: Pre-allocated VMs ready for use
- **RWMutex**: Allows concurrent reads, exclusive writes
- **Lazy Loading**: Creates VMs on demand up to max_size
- **Auto Cleanup**: Removes idle VMs to save memory
- **Metrics**: Track usage for optimization

## Comparison with Direct VM Creation

### Before (Race Condition Risk):
```go
vm := goja.New()
// Shared state modifications - UNSAFE!
registry.Enable(vm)
AddFeatures(vm, c)
```

### After (Thread-Safe):
```go
vmManager := GetVMManager()
instance, err := vmManager.AcquireVM(c)
defer vmManager.ReleaseVM(instance)
vm := instance.VM
// Safe to use - isolated VM instance
```

## Best Practices

1. **Always Release VMs**: Use defer to ensure VMs return to pool
2. **Configure Based on Load**: Start conservative, increase as needed
3. **Monitor Metrics**: Enable in production to understand usage
4. **Test Thoroughly**: Use race detector in development/testing
5. **Clear Sensitive Data**: VMs are cleaned between uses automatically