package engine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestVMMemoryLimit(t *testing.T) {
	limits := VMResourceLimits{
		MaxMemoryBytes:   10 * 1024 * 1024, // 10MB
		MaxExecutionTime: 5 * time.Second,
		MaxOperations:    1000000,
		CheckInterval:    100,
	}

	vm := goja.New()
	tracker := SetupVMWithLimits(vm, limits)
	defer tracker.Stop()

	// Script que intenta usar mucha memoria
	script := `
		var arr = [];
		for (var i = 0; i < 10000000; i++) {
			arr.push("This is a long string that will consume memory " + i);
		}
	`

	_, err := vm.RunString(script)

	// Debería fallar por límite de memoria
	if err == nil {
		t.Error("Expected memory limit error, got nil")
	}

	if !strings.Contains(err.Error(), "execution terminated") {
		t.Errorf("Expected execution terminated error, got: %v", err)
	}

	stats := tracker.GetStats()
	if !stats.Interrupted {
		t.Error("Expected tracker to be interrupted")
	}
}

func TestVMTimeLimit(t *testing.T) {
	limits := VMResourceLimits{
		MaxMemoryBytes:   128 * 1024 * 1024,
		MaxExecutionTime: 100 * time.Millisecond, // 100ms
		MaxOperations:    10000000,
		CheckInterval:    100,
	}

	vm := goja.New()
	tracker := SetupVMWithLimits(vm, limits)
	defer tracker.Stop()

	// Script con loop infinito
	script := `
		var i = 0;
		while (true) {
			i++;
		}
	`

	start := time.Now()
	_, err := vm.RunString(script)
	elapsed := time.Since(start)

	// Debería fallar por timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// No debería tardar mucho más que el límite
	if elapsed > 200*time.Millisecond {
		t.Errorf("Execution took too long: %v", elapsed)
	}

	stats := tracker.GetStats()
	if !stats.Interrupted {
		t.Error("Expected tracker to be interrupted")
	}
}

func TestVMOperationLimit(t *testing.T) {
	limits := VMResourceLimits{
		MaxMemoryBytes:   128 * 1024 * 1024,
		MaxExecutionTime: 5 * time.Second,
		MaxOperations:    1000, // Muy bajo para testing
		CheckInterval:    10,
	}

	vm := goja.New()
	tracker := SetupVMWithLimits(vm, limits)
	defer tracker.Stop()

	// Script con muchas operaciones
	script := `
		var sum = 0;
		for (var i = 0; i < 10000; i++) {
			sum += i;
		}
	`

	_, err := vm.RunString(script)

	// Debería fallar por límite de operaciones
	if err == nil {
		t.Error("Expected operation limit error, got nil")
	}

	stats := tracker.GetStats()
	if !stats.Interrupted {
		t.Error("Expected tracker to be interrupted")
	}

	// Las operaciones deberían estar cerca del límite
	if stats.OperationCount < 900 || stats.OperationCount > 1100 {
		t.Errorf("Operation count out of expected range: %d", stats.OperationCount)
	}
}

func TestVMNoLimits(t *testing.T) {
	// Sin límites
	limits := VMResourceLimits{
		MaxMemoryBytes:   0,
		MaxExecutionTime: 0,
		MaxOperations:    0,
		CheckInterval:    1000,
	}

	vm := goja.New()
	tracker := SetupVMWithLimits(vm, limits)
	defer tracker.Stop()

	// Script normal
	script := `
		var sum = 0;
		for (var i = 0; i < 1000; i++) {
			sum += i;
		}
		sum;
	`

	result, err := vm.RunString(script)

	// No debería fallar
	if err != nil {
		t.Errorf("Expected no error with no limits, got: %v", err)
	}

	// Verificar resultado correcto
	expectedSum := 499500
	if result.ToInteger() != int64(expectedSum) {
		t.Errorf("Expected sum %d, got %v", expectedSum, result)
	}

	stats := tracker.GetStats()
	if stats.Interrupted {
		t.Error("Expected tracker not to be interrupted")
	}
}

func TestIsResourceLimitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Memory limit error",
			err:      ErrMemoryLimitExceeded,
			expected: true,
		},
		{
			name:     "Time limit error",
			err:      ErrTimeLimitExceeded,
			expected: true,
		},
		{
			name:     "Operation limit error",
			err:      ErrOperationLimitExceeded,
			expected: true,
		},
		{
			name:     "Execution interrupted",
			err:      ErrExecutionInterrupted,
			expected: true,
		},
		{
			name:     "Runtime error",
			err:      errors.New("RuntimeError: execution terminated"),
			expected: true,
		},
		{
			name:     "Other error",
			err:      errors.New("type error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResourceLimitError(tt.err)
			if result != tt.expected {
				t.Errorf("IsResourceLimitError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}
