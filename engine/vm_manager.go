package engine

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/util"
	"github.com/labstack/echo/v4"
)

// VMManager manages a pool of Goja VMs with proper synchronization
type VMManager struct {
	mu        sync.RWMutex
	pool      chan *VMInstance
	factory   VMFactory
	maxSize   int
	activeVMs map[string]*VMInstance
	stats     VMStats
	registry  *require.Registry
}

// VMInstance represents a VM with metadata
type VMInstance struct {
	VM       *goja.Runtime
	ID       string
	InUse    bool
	LastUsed time.Time
	UseCount int64
	mu       sync.Mutex
}

// VMFactory creates new VM instances with proper initialization
type VMFactory func() (*goja.Runtime, error)

// VMStats tracks usage statistics
type VMStats struct {
	mu        sync.RWMutex
	Created   int64
	InUse     int64
	Available int64
	TotalUses int64
	Errors    int64
}

var (
	vmManager     *VMManager
	vmManagerOnce sync.Once
)

// GetVMManager returns the singleton VM manager
func GetVMManager() *VMManager {
	vmManagerOnce.Do(func() {
		// Use config or defaults
		maxSize := 50
		if Config.VMPoolConfig.MaxSize > 0 {
			maxSize = Config.VMPoolConfig.MaxSize
		}

		vmManager = NewVMManagerWithConfig(maxSize, &Config.VMPoolConfig)

		// Start cleanup routine if configured
		if Config.VMPoolConfig.CleanupInterval > 0 {
			go vmManager.Cleanup()
		}

		// Log metrics periodically if enabled
		if Config.VMPoolConfig.EnableMetrics {
			go vmManager.logMetrics()
		}
	})
	return vmManager
}

// NewVMManager creates a new VM manager with specified pool size
func NewVMManager(maxSize int) *VMManager {
	return NewVMManagerWithConfig(maxSize, nil)
}

// NewVMManagerWithConfig creates a new VM manager with configuration
func NewVMManagerWithConfig(maxSize int, config *VMPoolConfig) *VMManager {
	if maxSize <= 0 {
		maxSize = 10
	}

	manager := &VMManager{
		pool:      make(chan *VMInstance, maxSize),
		maxSize:   maxSize,
		activeVMs: make(map[string]*VMInstance),
		registry:  new(require.Registry),
	}

	// Setup registry once
	manager.registry.RegisterNativeModule("console", console.Require)
	manager.registry.RegisterNativeModule("util", util.Require)

	// Set factory function
	manager.factory = manager.createVM

	// Determine preload size
	preloadSize := maxSize / 2
	if config != nil && config.PreloadSize > 0 {
		preloadSize = config.PreloadSize
	}

	// Pre-populate pool
	for i := 0; i < preloadSize; i++ {
		if vm, err := manager.factory(); err == nil {
			instance := &VMInstance{
				VM:       vm,
				ID:       fmt.Sprintf("vm-%d-%d", time.Now().UnixNano(), i),
				LastUsed: time.Now(),
			}
			select {
			case manager.pool <- instance:
				manager.updateStats(func(s *VMStats) {
					s.Created++
					s.Available++
				})
			default:
				// Pool is full
			}
		}
	}

	return manager
}

// AcquireVM gets a VM from the pool or creates a new one
func (m *VMManager) AcquireVM(c echo.Context) (*VMInstance, error) {
	log.Printf("[VM Manager] AcquireVM called\n")

	// Try to get from pool first
	select {
	case instance := <-m.pool:
		log.Printf("[VM Manager] Got VM from pool: %s\n", instance.ID)

		m.mu.Lock()
		instance.InUse = true
		instance.LastUsed = time.Now()
		instance.UseCount++
		m.activeVMs[instance.ID] = instance
		m.mu.Unlock()

		m.updateStats(func(s *VMStats) {
			s.InUse++
			s.Available--
			s.TotalUses++
		})

		// Reset VM state for new use
		log.Printf("[VM Manager] Resetting VM for new use\n")
		m.resetVM(instance.VM, c)
		return instance, nil

	default:
		// Pool is empty, create new VM if under limit
		m.mu.RLock()
		activeCount := len(m.activeVMs)
		m.mu.RUnlock()

		if activeCount >= m.maxSize {
			return nil, fmt.Errorf("VM pool exhausted: %d VMs in use", activeCount)
		}

		// Create new VM
		log.Printf("[VM Manager] Creating new VM (pool empty)\n")
		vm, err := m.factory()
		if err != nil {
			m.updateStats(func(s *VMStats) {
				s.Errors++
			})
			return nil, fmt.Errorf("failed to create VM: %w", err)
		}

		instance := &VMInstance{
			VM:       vm,
			ID:       fmt.Sprintf("vm-%d", time.Now().UnixNano()),
			InUse:    true,
			LastUsed: time.Now(),
			UseCount: 1,
		}

		m.mu.Lock()
		m.activeVMs[instance.ID] = instance
		m.mu.Unlock()

		m.updateStats(func(s *VMStats) {
			s.Created++
			s.InUse++
			s.TotalUses++
		})

		// Initialize VM with context
		log.Printf("[VM Manager] Initializing new VM with context\n")
		m.resetVM(vm, c)
		return instance, nil
	}
}

// ReleaseVM returns a VM to the pool
func (m *VMManager) ReleaseVM(instance *VMInstance) {
	if instance == nil {
		return
	}

	instance.mu.Lock()
	instance.InUse = false
	instance.LastUsed = time.Now()
	instance.mu.Unlock()

	m.mu.Lock()
	delete(m.activeVMs, instance.ID)
	m.mu.Unlock()

	// Clear sensitive data from VM
	m.clearVM(instance.VM)

	// Try to return to pool
	select {
	case m.pool <- instance:
		m.updateStats(func(s *VMStats) {
			s.InUse--
			s.Available++
		})
	default:
		// Pool is full, let GC handle it
		m.updateStats(func(s *VMStats) {
			s.InUse--
		})
	}
}

// createVM creates a new VM instance with base configuration
func (m *VMManager) createVM() (*goja.Runtime, error) {
	vm := goja.New()

	// Enable require and console
	m.registry.Enable(vm)
	console.Enable(vm)

	// Set base configuration
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	return vm, nil
}

// resetVM resets VM state for new request
func (m *VMManager) resetVM(vm *goja.Runtime, c echo.Context) {
	log.Printf("[VM Reset] Starting resetVM\n")

	// Clear any previous global state
	vm.Set("console", require.Require(vm, "console"))

	// Check if this is a test context by looking for our specific test marker
	if testMarker := c.Get("_test_context"); testMarker != nil {
		log.Printf("[VM Reset] Test context detected, skipping feature setup\n")
		// This is a test context, skip feature setup
		return
	}

	log.Printf("[VM Reset] Initializing VM features for context\n")

	// Add features - these will be called for each request
	// The actual feature functions should handle the context properly

	// Use optimized session if available, otherwise use regular session
	if syncsession.Manager != nil {
		log.Printf("[VM Reset] Using optimized session manager\n")
		AddFeatureSessionOptimized(vm, c)
	} else {
		log.Printf("[VM Reset] Using regular session\n")
		AddFeatureSession(vm, c)
	}

	log.Printf("[VM Reset] Adding other features...\n")
	AddFeatureUsers(vm, c)
	AddFeatureToken(vm, c)
	AddFeatureTemplate(vm, c)
	AddGlobals(vm, c)

	// Add plugin features
	log.Printf("[VM Reset] Adding plugin features (%d plugins)...\n", len(Plugins))
	for _, p := range Plugins {
		features := p.AddFeatureJS()
		log.Printf("[VM Reset] Plugin features: %d\n", len(features))
		for key, fx := range features {
			vm.Set(key, fx)
		}
	}

	// Verify critical functions are available
	log.Printf("[VM Reset] Verifying critical functions...\n")
	criticalFunctions := []string{"get_profile", "set_session", "get_session", "template"}
	for _, fn := range criticalFunctions {
		val := vm.Get(fn)
		if val == nil || val.String() == "undefined" {
			log.Printf("[VM Reset] WARNING: Function '%s' is not defined!\n", fn)
		} else {
			log.Printf("[VM Reset] Function '%s' is defined\n", fn)
		}
	}

	log.Printf("[VM Reset] VM reset completed\n")
}

// clearVM removes sensitive data from VM
func (m *VMManager) clearVM(vm *goja.Runtime) {
	// Clear sensitive globals and any user-defined variables
	sensitiveKeys := []string{
		"form", "header", "auth_session", "profile",
		"redis_hset", "redis_hget", "redis_hdel",
		"c", "echo_context", "nflow_endpoint",
		"shared_var", // For tests
	}

	for _, key := range sensitiveKeys {
		vm.Set(key, goja.Undefined())
	}

	// Clear any global variables that might have been set
	// Wrapped in try-catch to avoid issues during concurrent access
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[VM Clear] Recovered from panic during cleanup: %v\n", r)
			}
		}()

		vm.RunString(`
			for (var key in this) {
				if (this.hasOwnProperty(key) && !['console', 'require', 'module', 'exports'].includes(key)) {
					delete this[key];
				}
			}
		`)
	}()
}

// updateStats safely updates statistics
func (m *VMManager) updateStats(update func(*VMStats)) {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()
	update(&m.stats)
}

// GetStats returns current pool statistics as a pointer to avoid copying the mutex
func (m *VMManager) GetStats() *VMStats {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()
	return &m.stats
}

// Cleanup performs periodic cleanup of idle VMs
func (m *VMManager) Cleanup() {
	interval := 5 * time.Minute
	if Config.VMPoolConfig.CleanupInterval > 0 {
		interval = time.Duration(Config.VMPoolConfig.CleanupInterval) * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupIdleVMs()
	}
}

// logMetrics logs VM pool metrics periodically
func (m *VMManager) logMetrics() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		stats := m.GetStats()
		log.Printf("[VM Pool Metrics] Created: %d, InUse: %d, Available: %d, TotalUses: %d, Errors: %d\n",
			stats.Created, stats.InUse, stats.Available, stats.TotalUses, stats.Errors)
	}
}

// cleanupIdleVMs removes VMs that have been idle too long
func (m *VMManager) cleanupIdleVMs() {
	idleTimeout := 10 * time.Minute
	if Config.VMPoolConfig.IdleTimeout > 0 {
		idleTimeout = time.Duration(Config.VMPoolConfig.IdleTimeout) * time.Minute
	}
	now := time.Now()

	// Collect idle VMs
	var toRemove []*VMInstance

	// Check pool for idle VMs
	poolSize := len(m.pool)

	for i := 0; i < poolSize; i++ {
		select {
		case instance := <-m.pool:
			if now.Sub(instance.LastUsed) > idleTimeout {
				toRemove = append(toRemove, instance)
			} else {
				// Put it back
				select {
				case m.pool <- instance:
				default:
					// Pool is full, VM will be garbage collected
				}
			}
		default:
			// No more VMs available in pool
			// Since channel is non-blocking, this means pool is empty
			// Exit the cleanup early
			return
		}
	}

	// Remove idle VMs
	for range toRemove {
		m.updateStats(func(s *VMStats) {
			s.Available--
		})
	}
}

// WithVM executes a function with a VM from the pool
func (m *VMManager) WithVM(c echo.Context, fn func(*goja.Runtime) error) error {
	instance, err := m.AcquireVM(c)
	if err != nil {
		return err
	}
	defer m.ReleaseVM(instance)

	return fn(instance.VM)
}
