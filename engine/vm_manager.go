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
		config := GetConfig()
		maxSize := 50
		if config.VMPoolConfig.MaxSize > 0 {
			maxSize = config.VMPoolConfig.MaxSize
		}

		vmManager = NewVMManagerWithConfig(maxSize, &config.VMPoolConfig)

		// Start cleanup routine if configured
		if config.VMPoolConfig.CleanupInterval > 0 {
			go vmManager.Cleanup()
		}

		// Log metrics periodically if enabled
		if config.VMPoolConfig.EnableMetrics {
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

	// First attempt - try to get from pool immediately
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
		// Pool is empty, check if we can create a new VM
		m.mu.RLock()
		activeCount := len(m.activeVMs)
		poolSize := len(m.pool)
		m.mu.RUnlock()

		log.Printf("[VM Manager] Pool empty. Active: %d, Pool size: %d, Max: %d\n", activeCount, poolSize, m.maxSize)

		if activeCount >= m.maxSize {
			// Pool is at capacity, wait for a VM to become available
			log.Printf("[VM Manager] Pool at capacity, waiting for available VM...\n")

			// Wait with timeout for a VM to become available
			timeout := time.NewTimer(5 * time.Second)
			defer timeout.Stop()

			select {
			case instance := <-m.pool:
				log.Printf("[VM Manager] Got VM from pool after waiting: %s\n", instance.ID)

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

				m.resetVM(instance.VM, c)
				return instance, nil

			case <-timeout.C:
				// Log current pool state for debugging
				m.mu.RLock()
				log.Printf("[VM Manager] Timeout waiting for VM. Active VMs: %d, Pool size: %d\n",
					len(m.activeVMs), len(m.pool))
				m.mu.RUnlock()

				return nil, fmt.Errorf("VM pool exhausted: timeout waiting for available VM (max: %d)", m.maxSize)
			}
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
		log.Printf("[VM Manager] ReleaseVM called with nil instance\n")
		return
	}

	log.Printf("[VM Manager] Releasing VM: %s\n", instance.ID)

	instance.mu.Lock()
	wasInUse := instance.InUse
	instance.InUse = false
	instance.LastUsed = time.Now()
	instance.mu.Unlock()

	if !wasInUse {
		log.Printf("[VM Manager] WARNING: VM %s was not marked as in use\n", instance.ID)
	}

	m.mu.Lock()
	_, existed := m.activeVMs[instance.ID]
	delete(m.activeVMs, instance.ID)
	m.mu.Unlock()

	if !existed {
		log.Printf("[VM Manager] WARNING: VM %s was not in activeVMs map\n", instance.ID)
	}

	// Clear sensitive data from VM
	m.clearVM(instance.VM)

	// Try to return to pool
	select {
	case m.pool <- instance:
		log.Printf("[VM Manager] VM %s returned to pool\n", instance.ID)
		m.updateStats(func(s *VMStats) {
			s.InUse--
			s.Available++
		})
	default:
		// Pool is full, let GC handle it
		log.Printf("[VM Manager] WARNING: Pool is full, VM %s will be garbage collected\n", instance.ID)
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
	// Use wrapper for JS context
	SetupJSContext(vm, c)
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

	// Also verify 'c' is available
	cVal := vm.Get("c")
	if cVal == nil || cVal == goja.Undefined() || cVal == goja.Null() {
		log.Printf("[VM Reset] WARNING: 'c' is not defined in VM!\n")
	} else {
		log.Printf("[VM Reset] 'c' is defined, type: %T\n", cVal.Export())
	}

	log.Printf("[VM Reset] VM reset completed\n")
}

// clearVM removes sensitive data from VM
func (m *VMManager) clearVM(vm *goja.Runtime) {
	// Clear sensitive globals and any user-defined variables
	// NOTE: Don't clear 'c' and 'echo_context' as they need to be reset per request
	sensitiveKeys := []string{
		"form", "header", "auth_session", "profile",
		"redis_hset", "redis_hget", "redis_hdel",
		"nflow_endpoint",
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
	config := GetConfig()
	if config.VMPoolConfig.CleanupInterval > 0 {
		interval = time.Duration(config.VMPoolConfig.CleanupInterval) * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupIdleVMs()
	}
}

// logMetrics logs VM pool metrics periodically
func (m *VMManager) logMetrics() {
	ticker := time.NewTicker(30 * time.Second) // More frequent metrics
	defer ticker.Stop()

	for range ticker.C {
		m.stats.mu.RLock()
		stats := VMStats{
			Created:   m.stats.Created,
			InUse:     m.stats.InUse,
			Available: m.stats.Available,
			TotalUses: m.stats.TotalUses,
			Errors:    m.stats.Errors,
		}
		m.stats.mu.RUnlock()

		m.mu.RLock()
		activeVMs := len(m.activeVMs)
		poolSize := len(m.pool)
		m.mu.RUnlock()

		log.Printf("[VM Pool Metrics] Created: %d, InUse: %d, Available: %d, TotalUses: %d, Errors: %d | ActiveVMs: %d, PoolSize: %d, MaxSize: %d\n",
			stats.Created, stats.InUse, stats.Available, stats.TotalUses, stats.Errors,
			activeVMs, poolSize, m.maxSize)

		// Warn if pool is getting full
		if activeVMs > int(float64(m.maxSize)*0.8) {
			log.Printf("[VM Pool WARNING] Pool usage above 80%%: %d/%d VMs in use\n", activeVMs, m.maxSize)
		}
	}
}

// cleanupIdleVMs removes VMs that have been idle too long
func (m *VMManager) cleanupIdleVMs() {
	idleTimeout := 10 * time.Minute
	config := GetConfig()
	if config.VMPoolConfig.IdleTimeout > 0 {
		idleTimeout = time.Duration(config.VMPoolConfig.IdleTimeout) * time.Minute
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
