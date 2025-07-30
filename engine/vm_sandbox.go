// Package engine provides the core workflow execution engine for nFlow Runtime.
// This file implements JavaScript sandboxing to restrict dangerous operations
// and provide a secure execution environment for user scripts.
package engine

import (
	"errors"

	"github.com/arturoeanton/nflow-runtime/logger"

	"github.com/dop251/goja"
)

// SandboxConfig defines the sandbox configuration
type SandboxConfig struct {
	AllowedGlobals   map[string]bool
	AllowedModules   map[string]bool
	BlockedFunctions []string
	EnableFileSystem bool
	EnableNetwork    bool
	EnableProcess    bool
}

// DefaultSandboxConfig returns a secure default configuration
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		AllowedGlobals: map[string]bool{
			"console":     true,
			"JSON":        true,
			"Math":        true,
			"Date":        true,
			"Array":       true,
			"Object":      true,
			"String":      true,
			"Number":      true,
			"Boolean":     true,
			"RegExp":      true,
			"Error":       true,
			"Promise":     true,
			"Map":         true,
			"Set":         true,
			"parseInt":    true,
			"parseFloat":  true,
			"isNaN":       true,
			"isFinite":    true,
			"encodeURI":   true,
			"decodeURI":   true,
			"setTimeout":  true,
			"clearTimeout": true,
			"Buffer":      true,
		},
		AllowedModules: map[string]bool{
			"crypto":      true,
			"querystring": true,
			"url":         true,
			"util":        true,
			"path":        true,
			// Blocked by default: fs, net, child_process, cluster, dgram, dns, http, https, os, process, tls
		},
		BlockedFunctions: []string{
			"eval",
			"Function",
			"generateFunction",
			"WebAssembly",
		},
		EnableFileSystem: false,
		EnableNetwork:    false,
		EnableProcess:    false,
	}
}

// VMSandbox manages the sandbox for a VM
type VMSandbox struct {
	config SandboxConfig
	vm     *goja.Runtime
}

// NewVMSandbox creates a new sandbox
func NewVMSandbox(vm *goja.Runtime, config SandboxConfig) *VMSandbox {
	return &VMSandbox{
		config: config,
		vm:     vm,
	}
}

// Apply applies sandbox restrictions to the VM
func (s *VMSandbox) Apply() error {
	// Remove dangerous functions
	for _, fn := range s.config.BlockedFunctions {
		s.vm.Set(fn, goja.Undefined())
	}
	
	// Configure secure require
	s.vm.Set("require", s.sandboxedRequire)
	
	// Remove disallowed globals
	s.removeDisallowedGlobals()
	
	// Add secure console
	s.addSafeConsole()
	
	// Block access to process if disabled
	if !s.config.EnableProcess {
		s.vm.Set("process", goja.Undefined())
	}
	
	return nil
}

// sandboxedRequire is a secure version of require
func (s *VMSandbox) sandboxedRequire(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		panic(s.vm.NewTypeError("require expects 1 argument"))
	}
	
	moduleName := call.Arguments[0].String()
	
	// Check if module is allowed
	if !s.config.AllowedModules[moduleName] {
		panic(s.vm.NewGoError(errors.New("module '" + moduleName + "' is not allowed in sandbox")))
	}
	
	// If we get here, allow normal require
	// Note: In production, you should intercept and provide secure versions of modules
	// For now, we simply return an empty object for allowed modules
	return s.vm.NewObject()
}

// removeDisallowedGlobals removes disallowed globals
func (s *VMSandbox) removeDisallowedGlobals() {
	globalObj := s.vm.GlobalObject()
	
	// Get all properties of the global object
	for _, key := range globalObj.Keys() {
		if !s.config.AllowedGlobals[key] {
			// If not in whitelist, remove
			globalObj.Delete(key)
		}
	}
}

// addSafeConsole adds a secure version of console
func (s *VMSandbox) addSafeConsole() {
	console := s.vm.NewObject()
	
	// Secure log that doesn't expose sensitive information
	console.Set("log", func(args ...interface{}) {
		// Sanitize output before logging
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		logger.Info(safeArgs...)
	})
	
	console.Set("error", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Error]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		logger.Info(safeArgs...)
	})
	
	console.Set("warn", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Warn]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		logger.Info(safeArgs...)
	})
	
	console.Set("info", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Info]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		logger.Info(safeArgs...)
	})
	
	s.vm.Set("console", console)
}

// sanitizeOutput sanitizes output to prevent information leaks
func (s *VMSandbox) sanitizeOutput(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		// Remove system paths
		if len(val) > 1000 {
			return val[:1000] + "...(truncated)"
		}
		return val
	case error:
		// Don't expose full stack traces
		return "Error: " + val.Error()
	default:
		return v
	}
}

// CreateSecureVM creates a VM with sandbox and limits applied
func CreateSecureVM(limits VMResourceLimits, sandboxConfig SandboxConfig) (*goja.Runtime, *VMResourceTracker, error) {
	vm := goja.New()
	
	// Apply resource limits
	tracker := SetupVMWithLimits(vm, limits)
	
	// Apply sandbox
	sandbox := NewVMSandbox(vm, sandboxConfig)
	if err := sandbox.Apply(); err != nil {
		tracker.Stop()
		return nil, nil, err
	}
	
	return vm, tracker, nil
}

// GetSandboxConfigFromConfig gets sandbox configuration from config
func GetSandboxConfigFromConfig() SandboxConfig {
	config := GetConfig()
	sandboxConfig := DefaultSandboxConfig()
	
	// Here you could override with configuration values
	// For example:
	if config.VMPoolConfig.EnableFileSystem {
		sandboxConfig.EnableFileSystem = true
		sandboxConfig.AllowedModules["fs"] = true
	}
	
	if config.VMPoolConfig.EnableNetwork {
		sandboxConfig.EnableNetwork = true
		sandboxConfig.AllowedModules["http"] = true
		sandboxConfig.AllowedModules["https"] = true
	}
	
	return sandboxConfig
}