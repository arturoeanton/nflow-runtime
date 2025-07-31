package endpoints

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/labstack/echo/v4"
)

// Metrics collector for Prometheus
type MetricsCollector struct {
	// Request metrics
	requestsTotal    uint64
	requestsDuration uint64 // in microseconds
	requestsErrors   uint64
	activeRequests   int64

	// Workflow metrics
	workflowsTotal    uint64
	workflowsDuration uint64
	workflowsErrors   uint64

	// Database metrics
	dbConnectionsActive int64
	dbConnectionsIdle   int64
	dbQueriesTotal      uint64
	dbQueryDuration     uint64

	// Process metrics
	processesActive int64
	processesTotal  uint64

	// Cache metrics
	cacheHits   uint64
	cacheMisses uint64

	// Start time for uptime calculation
	startTime time.Time

	mu sync.RWMutex
}

var metrics = &MetricsCollector{
	startTime: time.Now(),
}

// Middleware to collect metrics
func metricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip metrics endpoints to avoid recursion
			if c.Path() == "/metrics" || c.Path() == "/health" {
				return next(c)
			}

			start := time.Now()
			atomic.AddInt64(&metrics.activeRequests, 1)
			atomic.AddUint64(&metrics.requestsTotal, 1)

			err := next(c)

			duration := time.Since(start)
			atomic.AddUint64(&metrics.requestsDuration, uint64(duration.Microseconds()))
			atomic.AddInt64(&metrics.activeRequests, -1)

			if err != nil || c.Response().Status >= 400 {
				atomic.AddUint64(&metrics.requestsErrors, 1)
			}

			return err
		}
	}
}

// RegisterMonitoringEndpoints registers health and metrics endpoints
func RegisterMonitoringEndpoints(e *echo.Echo, config *engine.ConfigWorkspace) {
	if !config.MonitorConfig.Enabled {
		logger.Info("Monitoring endpoints are disabled")
		return
	}

	logger.Info("Registering monitoring endpoints")

	// Health check endpoint
	healthPath := config.MonitorConfig.HealthCheckPath
	if healthPath == "" {
		healthPath = "/health"
	}
	e.GET(healthPath, handleHealthCheck(config))
	e.HEAD(healthPath, handleHealthCheck(config))

	// Prometheus metrics endpoint
	metricsPath := config.MonitorConfig.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	// If separate metrics port is configured, start a new server
	if config.MonitorConfig.MetricsPort != "" {
		go startMetricsServer(config.MonitorConfig.MetricsPort, metricsPath, config)
	} else {
		e.GET(metricsPath, handleMetrics(config))
	}

	// Add metrics middleware
	e.Use(metricsMiddleware())
}

// startMetricsServer starts a separate HTTP server for metrics
func startMetricsServer(port, path string, config *engine.ConfigWorkspace) {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		c := echo.New().NewContext(r, &echoResponseWriter{w})
		handleMetrics(config)(c)
	})

	logger.Infof("Starting metrics server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("Failed to start metrics server:", err)
	}
}

// echoResponseWriter adapts http.ResponseWriter to echo.Response
type echoResponseWriter struct {
	http.ResponseWriter
}

func (w *echoResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *echoResponseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func (w *echoResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

// Health check response structure
type HealthStatus struct {
	Status     string                     `json:"status"`
	Timestamp  int64                      `json:"timestamp"`
	Uptime     string                     `json:"uptime"`
	Version    string                     `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
	Details    map[string]interface{}     `json:"details,omitempty"`
}

type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// handleHealthCheck provides comprehensive health status
func handleHealthCheck(config *engine.ConfigWorkspace) echo.HandlerFunc {
	return func(c echo.Context) error {
		health := HealthStatus{
			Status:     "healthy",
			Timestamp:  time.Now().Unix(),
			Uptime:     time.Since(metrics.startTime).String(),
			Version:    "1.0.0",
			Components: make(map[string]ComponentHealth),
		}

		// Check database health
		dbHealth := checkDatabaseHealth()
		health.Components["database"] = dbHealth
		if dbHealth.Status != "healthy" {
			health.Status = "degraded"
		}

		// Check Redis health if configured
		if config.RedisConfig.Host != "" {
			redisHealth := checkRedisHealth(config)
			health.Components["redis"] = redisHealth
			if redisHealth.Status != "healthy" {
				health.Status = "degraded"
			}
		}

		// Check process repository
		processHealth := checkProcessHealth()
		health.Components["processes"] = processHealth

		// Check memory usage
		memoryHealth := checkMemoryHealth()
		health.Components["memory"] = memoryHealth
		if memoryHealth.Status != "healthy" {
			health.Status = "degraded"
		}

		// Add detailed metrics if enabled
		if config.MonitorConfig.EnableDetailedMetrics {
			health.Details = getDetailedMetrics()
		}

		// Return appropriate status code
		statusCode := http.StatusOK
		if health.Status != "healthy" {
			statusCode = http.StatusServiceUnavailable
		}

		return c.JSON(statusCode, health)
	}
}

// handleMetrics provides Prometheus-compatible metrics
func handleMetrics(config *engine.ConfigWorkspace) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/plain; version=0.0.4")

		output := ""

		// Basic metrics
		output += fmt.Sprintf("# HELP nflow_up Whether the nFlow Runtime is up\n")
		output += fmt.Sprintf("# TYPE nflow_up gauge\n")
		output += fmt.Sprintf("nflow_up 1\n\n")

		output += fmt.Sprintf("# HELP nflow_uptime_seconds Number of seconds since nFlow Runtime started\n")
		output += fmt.Sprintf("# TYPE nflow_uptime_seconds counter\n")
		output += fmt.Sprintf("nflow_uptime_seconds %f\n\n", time.Since(metrics.startTime).Seconds())

		// Request metrics
		output += fmt.Sprintf("# HELP nflow_requests_total Total number of HTTP requests\n")
		output += fmt.Sprintf("# TYPE nflow_requests_total counter\n")
		output += fmt.Sprintf("nflow_requests_total %d\n\n", atomic.LoadUint64(&metrics.requestsTotal))

		output += fmt.Sprintf("# HELP nflow_requests_errors_total Total number of HTTP request errors\n")
		output += fmt.Sprintf("# TYPE nflow_requests_errors_total counter\n")
		output += fmt.Sprintf("nflow_requests_errors_total %d\n\n", atomic.LoadUint64(&metrics.requestsErrors))

		output += fmt.Sprintf("# HELP nflow_requests_active Number of active HTTP requests\n")
		output += fmt.Sprintf("# TYPE nflow_requests_active gauge\n")
		output += fmt.Sprintf("nflow_requests_active %d\n\n", atomic.LoadInt64(&metrics.activeRequests))

		// Calculate average request duration
		totalRequests := atomic.LoadUint64(&metrics.requestsTotal)
		if totalRequests > 0 {
			avgDuration := float64(atomic.LoadUint64(&metrics.requestsDuration)) / float64(totalRequests) / 1000.0 // Convert to milliseconds
			output += fmt.Sprintf("# HELP nflow_request_duration_milliseconds Average HTTP request duration\n")
			output += fmt.Sprintf("# TYPE nflow_request_duration_milliseconds gauge\n")
			output += fmt.Sprintf("nflow_request_duration_milliseconds %f\n\n", avgDuration)
		}

		// Workflow metrics
		output += fmt.Sprintf("# HELP nflow_workflows_total Total number of workflows executed\n")
		output += fmt.Sprintf("# TYPE nflow_workflows_total counter\n")
		output += fmt.Sprintf("nflow_workflows_total %d\n\n", atomic.LoadUint64(&metrics.workflowsTotal))

		output += fmt.Sprintf("# HELP nflow_workflows_errors_total Total number of workflow errors\n")
		output += fmt.Sprintf("# TYPE nflow_workflows_errors_total counter\n")
		output += fmt.Sprintf("nflow_workflows_errors_total %d\n\n", atomic.LoadUint64(&metrics.workflowsErrors))

		// Process metrics
		activeProcesses := int64(len(process.GetProcessList()))
		output += fmt.Sprintf("# HELP nflow_processes_active Number of active workflow processes\n")
		output += fmt.Sprintf("# TYPE nflow_processes_active gauge\n")
		output += fmt.Sprintf("nflow_processes_active %d\n\n", activeProcesses)

		output += fmt.Sprintf("# HELP nflow_processes_total Total number of workflow processes created\n")
		output += fmt.Sprintf("# TYPE nflow_processes_total counter\n")
		output += fmt.Sprintf("nflow_processes_total %d\n\n", atomic.LoadUint64(&metrics.processesTotal))

		// Database metrics
		if db, err := engine.GetDB(); err == nil {
			stats := db.Stats()
			output += fmt.Sprintf("# HELP nflow_db_connections_open Number of open database connections\n")
			output += fmt.Sprintf("# TYPE nflow_db_connections_open gauge\n")
			output += fmt.Sprintf("nflow_db_connections_open %d\n\n", stats.OpenConnections)

			output += fmt.Sprintf("# HELP nflow_db_connections_in_use Number of database connections in use\n")
			output += fmt.Sprintf("# TYPE nflow_db_connections_in_use gauge\n")
			output += fmt.Sprintf("nflow_db_connections_in_use %d\n\n", stats.InUse)

			output += fmt.Sprintf("# HELP nflow_db_connections_idle Number of idle database connections\n")
			output += fmt.Sprintf("# TYPE nflow_db_connections_idle gauge\n")
			output += fmt.Sprintf("nflow_db_connections_idle %d\n\n", stats.Idle)
		}

		// Go runtime metrics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		output += fmt.Sprintf("# HELP nflow_go_goroutines Number of goroutines\n")
		output += fmt.Sprintf("# TYPE nflow_go_goroutines gauge\n")
		output += fmt.Sprintf("nflow_go_goroutines %d\n\n", runtime.NumGoroutine())

		output += fmt.Sprintf("# HELP nflow_go_memory_alloc_bytes Current memory allocation\n")
		output += fmt.Sprintf("# TYPE nflow_go_memory_alloc_bytes gauge\n")
		output += fmt.Sprintf("nflow_go_memory_alloc_bytes %d\n\n", m.Alloc)

		output += fmt.Sprintf("# HELP nflow_go_memory_sys_bytes Total memory obtained from system\n")
		output += fmt.Sprintf("# TYPE nflow_go_memory_sys_bytes gauge\n")
		output += fmt.Sprintf("nflow_go_memory_sys_bytes %d\n\n", m.Sys)

		output += fmt.Sprintf("# HELP nflow_go_gc_runs_total Number of GC runs\n")
		output += fmt.Sprintf("# TYPE nflow_go_gc_runs_total counter\n")
		output += fmt.Sprintf("nflow_go_gc_runs_total %d\n\n", m.NumGC)

		// Cache metrics
		output += fmt.Sprintf("# HELP nflow_cache_hits_total Total number of cache hits\n")
		output += fmt.Sprintf("# TYPE nflow_cache_hits_total counter\n")
		output += fmt.Sprintf("nflow_cache_hits_total %d\n\n", atomic.LoadUint64(&metrics.cacheHits))

		output += fmt.Sprintf("# HELP nflow_cache_misses_total Total number of cache misses\n")
		output += fmt.Sprintf("# TYPE nflow_cache_misses_total counter\n")
		output += fmt.Sprintf("nflow_cache_misses_total %d\n\n", atomic.LoadUint64(&metrics.cacheMisses))

		// Detailed metrics if enabled
		if config.MonitorConfig.EnableDetailedMetrics {
			output += getDetailedPrometheusMetrics()
		}

		return c.String(http.StatusOK, output)
	}
}

// Health check helper functions

func checkDatabaseHealth() ComponentHealth {
	db, err := engine.GetDB()
	if err != nil {
		return ComponentHealth{
			Status:  "unhealthy",
			Message: fmt.Sprintf("Failed to get database: %v", err),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return ComponentHealth{
			Status:  "unhealthy",
			Message: fmt.Sprintf("Database ping failed: %v", err),
		}
	}

	return ComponentHealth{Status: "healthy"}
}

func checkRedisHealth(config *engine.ConfigWorkspace) ComponentHealth {
	// This would need actual Redis client check
	return ComponentHealth{
		Status:  "healthy",
		Message: "Redis health check not implemented",
	}
}

func checkProcessHealth() ComponentHealth {
	processes := len(process.GetProcessList())
	if processes > 1000 {
		return ComponentHealth{
			Status:  "warning",
			Message: fmt.Sprintf("High number of active processes: %d", processes),
		}
	}
	return ComponentHealth{Status: "healthy"}
}

func checkMemoryHealth() ComponentHealth {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Alert if memory usage is above 1GB
	if m.Alloc > 1024*1024*1024 {
		return ComponentHealth{
			Status:  "warning",
			Message: fmt.Sprintf("High memory usage: %d MB", m.Alloc/1024/1024),
		}
	}

	return ComponentHealth{Status: "healthy"}
}

// getDetailedMetrics returns detailed metrics for health check
func getDetailedMetrics() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	db, _ := engine.GetDB()
	var dbStats sql.DBStats
	if db != nil {
		dbStats = db.Stats()
	}

	return map[string]interface{}{
		"requests": map[string]interface{}{
			"total":  atomic.LoadUint64(&metrics.requestsTotal),
			"errors": atomic.LoadUint64(&metrics.requestsErrors),
			"active": atomic.LoadInt64(&metrics.activeRequests),
		},
		"workflows": map[string]interface{}{
			"total":  atomic.LoadUint64(&metrics.workflowsTotal),
			"errors": atomic.LoadUint64(&metrics.workflowsErrors),
		},
		"processes": map[string]interface{}{
			"active": len(process.GetProcessList()),
			"total":  atomic.LoadUint64(&metrics.processesTotal),
		},
		"database": map[string]interface{}{
			"connections_open":   dbStats.OpenConnections,
			"connections_in_use": dbStats.InUse,
			"connections_idle":   dbStats.Idle,
		},
		"memory": map[string]interface{}{
			"alloc_mb":      m.Alloc / 1024 / 1024,
			"sys_mb":        m.Sys / 1024 / 1024,
			"heap_alloc_mb": m.HeapAlloc / 1024 / 1024,
			"gc_runs":       m.NumGC,
		},
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"cpus":       runtime.NumCPU(),
		},
	}
}

// getDetailedPrometheusMetrics returns additional detailed metrics
func getDetailedPrometheusMetrics() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	output := ""

	// Detailed memory metrics
	output += fmt.Sprintf("# HELP nflow_go_memory_heap_alloc_bytes Heap allocation\n")
	output += fmt.Sprintf("# TYPE nflow_go_memory_heap_alloc_bytes gauge\n")
	output += fmt.Sprintf("nflow_go_memory_heap_alloc_bytes %d\n\n", m.HeapAlloc)

	output += fmt.Sprintf("# HELP nflow_go_memory_heap_sys_bytes Heap system memory\n")
	output += fmt.Sprintf("# TYPE nflow_go_memory_heap_sys_bytes gauge\n")
	output += fmt.Sprintf("nflow_go_memory_heap_sys_bytes %d\n\n", m.HeapSys)

	output += fmt.Sprintf("# HELP nflow_go_memory_heap_idle_bytes Heap idle memory\n")
	output += fmt.Sprintf("# TYPE nflow_go_memory_heap_idle_bytes gauge\n")
	output += fmt.Sprintf("nflow_go_memory_heap_idle_bytes %d\n\n", m.HeapIdle)

	output += fmt.Sprintf("# HELP nflow_go_memory_heap_inuse_bytes Heap in-use memory\n")
	output += fmt.Sprintf("# TYPE nflow_go_memory_heap_inuse_bytes gauge\n")
	output += fmt.Sprintf("nflow_go_memory_heap_inuse_bytes %d\n\n", m.HeapInuse)

	output += fmt.Sprintf("# HELP nflow_go_memory_heap_objects Number of heap objects\n")
	output += fmt.Sprintf("# TYPE nflow_go_memory_heap_objects gauge\n")
	output += fmt.Sprintf("nflow_go_memory_heap_objects %d\n\n", m.HeapObjects)

	// GC metrics
	output += fmt.Sprintf("# HELP nflow_go_gc_cpu_fraction GC CPU fraction\n")
	output += fmt.Sprintf("# TYPE nflow_go_gc_cpu_fraction gauge\n")
	output += fmt.Sprintf("nflow_go_gc_cpu_fraction %f\n\n", m.GCCPUFraction)

	return output
}

// UpdateMetrics provides methods to update metrics from other parts of the application
func UpdateWorkflowMetrics(success bool, duration time.Duration) {
	atomic.AddUint64(&metrics.workflowsTotal, 1)
	atomic.AddUint64(&metrics.workflowsDuration, uint64(duration.Microseconds()))
	if !success {
		atomic.AddUint64(&metrics.workflowsErrors, 1)
	}
}

func UpdateProcessMetrics(created bool) {
	if created {
		atomic.AddUint64(&metrics.processesTotal, 1)
	}
}

func UpdateCacheMetrics(hit bool) {
	if hit {
		atomic.AddUint64(&metrics.cacheHits, 1)
	} else {
		atomic.AddUint64(&metrics.cacheMisses, 1)
	}
}

func UpdateDatabaseMetrics(duration time.Duration) {
	atomic.AddUint64(&metrics.dbQueriesTotal, 1)
	atomic.AddUint64(&metrics.dbQueryDuration, uint64(duration.Microseconds()))
}
