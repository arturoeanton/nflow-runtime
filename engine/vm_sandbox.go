package engine

import (
	"errors"
	"log"

	"github.com/dop251/goja"
)

// SandboxConfig define la configuración del sandbox
type SandboxConfig struct {
	AllowedGlobals   map[string]bool
	AllowedModules   map[string]bool
	BlockedFunctions []string
	EnableFileSystem bool
	EnableNetwork    bool
	EnableProcess    bool
}

// DefaultSandboxConfig retorna una configuración segura por defecto
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
			// Bloqueados por defecto: fs, net, child_process, cluster, dgram, dns, http, https, os, process, tls
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

// VMSandbox gestiona el sandbox de una VM
type VMSandbox struct {
	config SandboxConfig
	vm     *goja.Runtime
}

// NewVMSandbox crea un nuevo sandbox
func NewVMSandbox(vm *goja.Runtime, config SandboxConfig) *VMSandbox {
	return &VMSandbox{
		config: config,
		vm:     vm,
	}
}

// Apply aplica las restricciones del sandbox a la VM
func (s *VMSandbox) Apply() error {
	// Remover funciones peligrosas
	for _, fn := range s.config.BlockedFunctions {
		s.vm.Set(fn, goja.Undefined())
	}
	
	// Configurar require seguro
	s.vm.Set("require", s.sandboxedRequire)
	
	// Remover globales no permitidos
	s.removeDisallowedGlobals()
	
	// Agregar console seguro
	s.addSafeConsole()
	
	// Bloquear acceso a process si está deshabilitado
	if !s.config.EnableProcess {
		s.vm.Set("process", goja.Undefined())
	}
	
	return nil
}

// sandboxedRequire es una versión segura de require
func (s *VMSandbox) sandboxedRequire(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		panic(s.vm.NewTypeError("require expects 1 argument"))
	}
	
	moduleName := call.Arguments[0].String()
	
	// Verificar si el módulo está permitido
	if !s.config.AllowedModules[moduleName] {
		panic(s.vm.NewGoError(errors.New("module '" + moduleName + "' is not allowed in sandbox")))
	}
	
	// Si llegamos aquí, permitir el require normal
	// Nota: En producción, deberías interceptar y proveer versiones seguras de los módulos
	// Por ahora, simplemente retornamos un objeto vacío para módulos permitidos
	return s.vm.NewObject()
}

// removeDisallowedGlobals remueve globales no permitidos
func (s *VMSandbox) removeDisallowedGlobals() {
	globalObj := s.vm.GlobalObject()
	
	// Obtener todas las propiedades del objeto global
	for _, key := range globalObj.Keys() {
		if !s.config.AllowedGlobals[key] {
			// Si no está en la whitelist, remover
			globalObj.Delete(key)
		}
	}
}

// addSafeConsole agrega una versión segura de console
func (s *VMSandbox) addSafeConsole() {
	console := s.vm.NewObject()
	
	// Log seguro que no expone información sensible
	console.Set("log", func(args ...interface{}) {
		// Sanitizar output antes de loguear
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		log.Println(safeArgs...)
	})
	
	console.Set("error", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Error]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		log.Println(safeArgs...)
	})
	
	console.Set("warn", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Warn]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		log.Println(safeArgs...)
	})
	
	console.Set("info", func(args ...interface{}) {
		safeArgs := make([]interface{}, len(args)+1)
		safeArgs[0] = "[Sandbox Info]"
		for i, arg := range args {
			safeArgs[i+1] = s.sanitizeOutput(arg)
		}
		log.Println(safeArgs...)
	})
	
	s.vm.Set("console", console)
}

// sanitizeOutput sanitiza el output para evitar leaks de información
func (s *VMSandbox) sanitizeOutput(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		// Remover paths del sistema
		if len(val) > 1000 {
			return val[:1000] + "...(truncated)"
		}
		return val
	case error:
		// No exponer stack traces completos
		return "Error: " + val.Error()
	default:
		return v
	}
}

// CreateSecureVM crea una VM con sandbox y límites aplicados
func CreateSecureVM(limits VMResourceLimits, sandboxConfig SandboxConfig) (*goja.Runtime, *VMResourceTracker, error) {
	vm := goja.New()
	
	// Aplicar límites de recursos
	tracker := SetupVMWithLimits(vm, limits)
	
	// Aplicar sandbox
	sandbox := NewVMSandbox(vm, sandboxConfig)
	if err := sandbox.Apply(); err != nil {
		tracker.Stop()
		return nil, nil, err
	}
	
	return vm, tracker, nil
}

// GetSandboxConfigFromConfig obtiene la configuración del sandbox desde config
func GetSandboxConfigFromConfig() SandboxConfig {
	config := GetConfig()
	sandboxConfig := DefaultSandboxConfig()
	
	// Aquí podrías sobrescribir con valores de configuración
	// Por ejemplo:
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