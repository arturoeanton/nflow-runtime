package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestSandboxBlockedFunctions(t *testing.T) {
	vm := goja.New()
	config := DefaultSandboxConfig()
	sandbox := NewVMSandbox(vm, config)
	
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// Test que eval está bloqueado
	_, err = vm.RunString(`eval("1+1")`)
	if err == nil {
		t.Error("Expected eval to be blocked")
	}
	
	// Test que Function constructor está bloqueado
	_, err = vm.RunString(`new Function("return 1+1")`)
	if err == nil {
		t.Error("Expected Function constructor to be blocked")
	}
}

func TestSandboxAllowedGlobals(t *testing.T) {
	vm := goja.New()
	config := DefaultSandboxConfig()
	sandbox := NewVMSandbox(vm, config)
	
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// Estos deberían funcionar
	allowedScripts := []string{
		`JSON.stringify({a: 1})`,
		`Math.max(1, 2)`,
		`new Date().getTime()`,
		`[1,2,3].map(x => x * 2)`,
		`"hello".toUpperCase()`,
		`parseInt("123")`,
		`new Map().set("key", "value")`,
	}
	
	for _, script := range allowedScripts {
		_, err := vm.RunString(script)
		if err != nil {
			t.Errorf("Script should be allowed: %s, error: %v", script, err)
		}
	}
}

func TestSandboxBlockedModules(t *testing.T) {
	vm := goja.New()
	
	// Primero necesitamos el registry
	registry := GetRequireRegistry()
	if registry != nil {
		registry.Enable(vm)
	}
	
	config := DefaultSandboxConfig()
	sandbox := NewVMSandbox(vm, config)
	
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// fs debería estar bloqueado
	_, err = vm.RunString(`require('fs')`)
	if err == nil {
		t.Error("Expected fs module to be blocked")
	}
	if !strings.Contains(err.Error(), "not allowed in sandbox") {
		t.Errorf("Expected sandbox error, got: %v", err)
	}
	
	// net debería estar bloqueado
	_, err = vm.RunString(`require('net')`)
	if err == nil {
		t.Error("Expected net module to be blocked")
	}
	
	// child_process debería estar bloqueado
	_, err = vm.RunString(`require('child_process')`)
	if err == nil {
		t.Error("Expected child_process module to be blocked")
	}
}

func TestSandboxAllowedModules(t *testing.T) {
	vm := goja.New()
	
	config := DefaultSandboxConfig()
	sandbox := NewVMSandbox(vm, config)
	
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// Verificar que los módulos permitidos están en la whitelist
	allowedModules := []string{"crypto", "querystring", "url", "util", "path"}
	
	for _, module := range allowedModules {
		if !config.AllowedModules[module] {
			t.Errorf("Module %s should be allowed", module)
		}
	}
}

func TestSandboxConsole(t *testing.T) {
	vm := goja.New()
	config := DefaultSandboxConfig()
	sandbox := NewVMSandbox(vm, config)
	
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// Console debería funcionar
	scripts := []string{
		`console.log("test")`,
		`console.error("error")`,
		`console.warn("warning")`,
		`console.info("info")`,
	}
	
	for _, script := range scripts {
		_, err := vm.RunString(script)
		if err != nil {
			t.Errorf("Console method should work: %s, error: %v", script, err)
		}
	}
}

func TestSandboxConfigurableOptions(t *testing.T) {
	vm := goja.New()
	
	// Configuración personalizada
	config := SandboxConfig{
		AllowedGlobals: map[string]bool{
			"console": true,
			"Math":    true,
		},
		AllowedModules: map[string]bool{
			"fs": true, // Permitir fs
		},
		BlockedFunctions: []string{"eval"},
		EnableFileSystem: true,
		EnableNetwork:    false,
		EnableProcess:    false,
	}
	
	sandbox := NewVMSandbox(vm, config)
	err := sandbox.Apply()
	if err != nil {
		t.Fatalf("Failed to apply sandbox: %v", err)
	}
	
	// Math debería funcionar
	_, err = vm.RunString(`Math.max(1, 2)`)
	if err != nil {
		t.Error("Math should be allowed")
	}
	
	// JSON no debería funcionar (no está en whitelist)
	val := vm.Get("JSON")
	if val != nil && !goja.IsUndefined(val) {
		t.Error("JSON should not be available")
	}
}

func TestCreateSecureVM(t *testing.T) {
	limits := VMResourceLimits{
		MaxMemoryBytes:   64 * 1024 * 1024,
		MaxExecutionTime: 1 * time.Second,
		MaxOperations:    1000000,
		CheckInterval:    1000,
	}
	
	sandboxConfig := DefaultSandboxConfig()
	
	vm, tracker, err := CreateSecureVM(limits, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create secure VM: %v", err)
	}
	defer tracker.Stop()
	
	// Verificar que la VM funciona
	result, err := vm.RunString(`1 + 1`)
	if err != nil {
		t.Errorf("Basic operation should work: %v", err)
	}
	
	if result.ToInteger() != 2 {
		t.Errorf("Expected 2, got %v", result)
	}
	
	// Verificar que eval está bloqueado
	_, err = vm.RunString(`eval("1+1")`)
	if err == nil {
		t.Error("Expected eval to be blocked in secure VM")
	}
	
	// Verificar que el límite de tiempo funciona
	_, err = vm.RunString(`while(true) {}`)
	if err == nil {
		t.Error("Expected timeout in secure VM")
	}
}