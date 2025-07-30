package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/util"
	"github.com/labstack/echo/v4"
)

// VMPoolV2 is an improved VM pool that preserves base functionality
type VMPoolV2 struct {
	mu       sync.Mutex
	pool     chan *VMInstanceV2
	maxSize  int
	registry *require.Registry
}

// VMInstanceV2 represents a VM with better state management
type VMInstanceV2 struct {
	VM           *goja.Runtime
	ID           string
	LastUsed     time.Time
	UseCount     int64
	Initialized  bool
}

var (
	vmPoolV2     *VMPoolV2
	vmPoolV2Once sync.Once
)

// GetVMPoolV2 returns the singleton VM pool
func GetVMPoolV2() *VMPoolV2 {
	vmPoolV2Once.Do(func() {
		size := 10 // Default size
		if Config.VMPoolConfig.MaxSize > 0 {
			size = Config.VMPoolConfig.MaxSize
		}
		vmPoolV2 = NewVMPoolV2(size)
	})
	return vmPoolV2
}

// NewVMPoolV2 creates a new VM pool
func NewVMPoolV2(maxSize int) *VMPoolV2 {
	pool := &VMPoolV2{
		pool:     make(chan *VMInstanceV2, maxSize),
		maxSize:  maxSize,
		registry: new(require.Registry),
	}
	
	// Setup registry once
	pool.registry.RegisterNativeModule("console", console.Require)
	pool.registry.RegisterNativeModule("util", util.Require)
	
	// Pre-create some VMs
	for i := 0; i < maxSize/2; i++ {
		instance := pool.createVM()
		select {
		case pool.pool <- instance:
		default:
			// Pool is full
		}
	}
	
	return pool
}

// createVM creates a new VM with base initialization
func (p *VMPoolV2) createVM() *VMInstanceV2 {
	vm := goja.New()
	
	// Enable base modules
	p.registry.Enable(vm)
	console.Enable(vm)
	
	// Set field mapper
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	
	instance := &VMInstanceV2{
		VM:          vm,
		ID:          fmt.Sprintf("vm-%d", time.Now().UnixNano()),
		LastUsed:    time.Now(),
		Initialized: false,
	}
	
	return instance
}

// AcquireVM gets a VM from pool and prepares it for use
func (p *VMPoolV2) AcquireVM(c echo.Context) (*goja.Runtime, func()) {
	var instance *VMInstanceV2
	
	// Try to get from pool
	select {
	case instance = <-p.pool:
		instance.UseCount++
		instance.LastUsed = time.Now()
	default:
		// Create new if pool is empty
		instance = p.createVM()
	}
	
	// Always reinitialize features to ensure proper context binding
	// This is necessary because Echo context changes between requests
	p.initializeFeatures(instance.VM, c)
	
	// Set request-specific data
	p.setRequestData(instance.VM, c)
	
	// Return VM and release function
	return instance.VM, func() {
		p.releaseVM(instance)
	}
}

// initializeFeatures adds all the base features that don't change between requests
func (p *VMPoolV2) initializeFeatures(vm *goja.Runtime, c echo.Context) {
	// Add session features
	if syncsession.Manager != nil {
		AddFeatureSessionOptimized(vm, c)
	} else {
		AddFeatureSession(vm, c)
	}
	
	// Add other features
	AddFeatureUsers(vm, c)
	AddFeatureToken(vm, c)
	AddFeatureTemplate(vm, c)
	AddGlobals(vm, c)
	
	// Add plugin features (these are functions, not data)
	for _, plugin := range Plugins {
		for key, fx := range plugin.AddFeatureJS() {
			vm.Set(key, fx)
		}
	}
}

// setRequestData sets request-specific data that changes each time
func (p *VMPoolV2) setRequestData(vm *goja.Runtime, c echo.Context) {
	// Set Echo context
	vm.Set("c", c)
	vm.Set("echo_context", c)
	
	// Set headers
	header := make(map[string][]string)
	if c.Request().Header != nil {
		header = (map[string][]string)(c.Request().Header)
	}
	vm.Set("header", header)
	
	// Set form data
	form, err := c.FormParams()
	if err != nil {
		vm.Set("form", make(map[string][]string))
	} else {
		vm.Set("form", (map[string][]string)(form))
	}
	
	// Clear any previous request-specific data
	requestKeys := []string{
		"nflow_endpoint", "vars", "path_vars", "wid",
		"current_box", "prev_box", "post_data",
		"profile", "next", "auth_flag", "url_access",
	}
	
	for _, key := range requestKeys {
		vm.Set(key, goja.Undefined())
	}
}

// releaseVM returns VM to pool
func (p *VMPoolV2) releaseVM(instance *VMInstanceV2) {
	// Note: We don't clear data here because the VM might still be in use
	// The clearing will happen when the VM is acquired again for a new request
	
	// Try to return to pool
	select {
	case p.pool <- instance:
		// Successfully returned to pool
	default:
		// Pool is full, let GC handle it
	}
}

// clearRequestData clears only request-specific data
func (p *VMPoolV2) clearRequestData(vm *goja.Runtime) {
	// Clear sensitive request data but NOT the echo context
	// which is needed during the entire request
	sensitiveKeys := []string{
		"header", "form", "post_data",
		"nflow_endpoint", "vars", "path_vars", "wid",
		"current_box", "prev_box", "profile", "auth_session",
		"payload", "next", "auth_flag", "url_access",
	}
	
	for _, key := range sensitiveKeys {
		vm.Set(key, goja.Undefined())
	}
	
	// Note: We don't clear 'c' or 'echo_context' here because
	// they are needed throughout the request lifecycle
}

// GetPoolStats returns current pool statistics
func (p *VMPoolV2) GetPoolStats() map[string]int {
	return map[string]int{
		"available": len(p.pool),
		"max_size":  p.maxSize,
	}
}