package engine

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// VMResourceLimits define los límites de recursos para una VM
type VMResourceLimits struct {
	MaxMemoryBytes    int64         // Máximo de memoria en bytes (0 = sin límite)
	MaxExecutionTime  time.Duration // Tiempo máximo de ejecución (0 = sin límite)
	MaxOperations     int64         // Máximo de operaciones JS (0 = sin límite)
	MaxStackDepth     int           // Profundidad máxima del stack (0 = sin límite)
	CheckInterval     int64         // Cada cuántas operaciones verificar límites
}

// DefaultVMResourceLimits retorna límites por defecto seguros
func DefaultVMResourceLimits() VMResourceLimits {
	return VMResourceLimits{
		MaxMemoryBytes:   128 * 1024 * 1024, // 128MB
		MaxExecutionTime: 30 * time.Second,  // 30 segundos
		MaxOperations:    10_000_000,        // 10M operaciones
		MaxStackDepth:    1000,              // 1000 niveles de recursión
		CheckInterval:    1000,              // Verificar cada 1000 operaciones
	}
}

// VMResourceTracker rastrea el uso de recursos de una VM
type VMResourceTracker struct {
	startTime        time.Time
	operationCount   int64
	checkInterval    int64
	limits           VMResourceLimits
	ctx              context.Context
	cancel           context.CancelFunc
	memoryBaseline   uint64
	interrupted      atomic.Bool
}

// Errores de límites
var (
	ErrMemoryLimitExceeded    = errors.New("memory limit exceeded")
	ErrTimeLimitExceeded      = errors.New("execution time limit exceeded")
	ErrOperationLimitExceeded = errors.New("operation limit exceeded")
	ErrExecutionInterrupted   = errors.New("execution interrupted")
)

// IsResourceLimitError verifica si un error es por límite de recursos
func IsResourceLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "memory limit exceeded" ||
		errStr == "execution time limit exceeded" ||
		errStr == "operation limit exceeded" ||
		errStr == "execution interrupted" ||
		errStr == "RuntimeError: execution terminated"
}

// NewVMResourceTracker crea un nuevo tracker de recursos
func NewVMResourceTracker(limits VMResourceLimits) *VMResourceTracker {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Capturar memoria baseline
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	tracker := &VMResourceTracker{
		startTime:      time.Now(),
		limits:         limits,
		checkInterval:  limits.CheckInterval,
		ctx:            ctx,
		cancel:         cancel,
		memoryBaseline: m.Alloc,
	}
	
	// Si hay límite de tiempo, configurar timeout
	if limits.MaxExecutionTime > 0 {
		go tracker.watchTimeout()
	}
	
	return tracker
}

// SetupVMWithLimits configura una VM con límites de recursos
func SetupVMWithLimits(vm *goja.Runtime, limits VMResourceLimits) *VMResourceTracker {
	tracker := NewVMResourceTracker(limits)
	
	// Iniciar goroutine para verificar límites periódicamente
	go tracker.monitorLimits(vm)
	
	return tracker
}

// monitorLimits monitorea los límites e interrumpe la VM si es necesario
func (t *VMResourceTracker) monitorLimits(vm *goja.Runtime) {
	ticker := time.NewTicker(10 * time.Millisecond) // Verificar cada 10ms
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if t.CheckLimits() {
				vm.Interrupt("resource limit exceeded")
				return
			}
		case <-t.ctx.Done():
			return
		}
	}
}

// CheckLimits verifica si se han excedido los límites
func (t *VMResourceTracker) CheckLimits() bool {
	// Si ya fue interrumpido, retornar inmediatamente
	if t.interrupted.Load() {
		return true
	}
	
	// Incrementar contador de operaciones
	ops := atomic.AddInt64(&t.operationCount, 1)
	
	// Verificar límite de operaciones inmediatamente si se excedió
	if t.limits.MaxOperations > 0 && ops > t.limits.MaxOperations {
		t.interrupted.Store(true)
		return true
	}
	
	// Solo verificar memoria cada N operaciones para evitar overhead
	if ops%t.checkInterval == 0 {
		// Verificar límite de memoria
		if t.limits.MaxMemoryBytes > 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			memoryUsed := int64(m.Alloc - t.memoryBaseline)
			
			if memoryUsed > t.limits.MaxMemoryBytes {
				t.interrupted.Store(true)
				return true
			}
		}
	}
	
	// Verificar si el tiempo se excedió
	if t.limits.MaxExecutionTime > 0 {
		if time.Since(t.startTime) > t.limits.MaxExecutionTime {
			t.interrupted.Store(true)
			return true
		}
	}
	
	// Verificar si fue cancelado
	select {
	case <-t.ctx.Done():
		t.interrupted.Store(true)
		return true
	default:
		return false
	}
}

// watchTimeout monitorea el tiempo de ejecución
func (t *VMResourceTracker) watchTimeout() {
	timer := time.NewTimer(t.limits.MaxExecutionTime)
	defer timer.Stop()
	
	select {
	case <-timer.C:
		t.interrupted.Store(true)
		t.cancel()
	case <-t.ctx.Done():
		return
	}
}

// GetStats retorna estadísticas actuales
func (t *VMResourceTracker) GetStats() VMResourceStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return VMResourceStats{
		ElapsedTime:    time.Since(t.startTime),
		OperationCount: atomic.LoadInt64(&t.operationCount),
		MemoryUsed:     int64(m.Alloc - t.memoryBaseline),
		Interrupted:    t.interrupted.Load(),
	}
}

// Stop detiene el tracker y libera recursos
func (t *VMResourceTracker) Stop() {
	t.cancel()
}

// VMResourceStats contiene estadísticas de uso
type VMResourceStats struct {
	ElapsedTime    time.Duration
	OperationCount int64
	MemoryUsed     int64
	Interrupted    bool
}

// GetLimitsFromConfig obtiene límites desde la configuración
func GetLimitsFromConfig() VMResourceLimits {
	config := GetConfig()
	limits := DefaultVMResourceLimits()
	
	// Sobrescribir con valores de configuración si existen
	if config.VMPoolConfig.MaxMemoryMB > 0 {
		limits.MaxMemoryBytes = int64(config.VMPoolConfig.MaxMemoryMB) * 1024 * 1024
	}
	
	if config.VMPoolConfig.MaxExecutionSeconds > 0 {
		limits.MaxExecutionTime = time.Duration(config.VMPoolConfig.MaxExecutionSeconds) * time.Second
	}
	
	if config.VMPoolConfig.MaxOperations > 0 {
		limits.MaxOperations = config.VMPoolConfig.MaxOperations
	}
	
	return limits
}