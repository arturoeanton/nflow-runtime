package endpoints

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/labstack/echo/v4"
)

// URLCacheInterface defines the interface for URL cache operations
type URLCacheInterface interface {
	GetSize() int
	GetEntries() []URLCacheEntry
	Clear()
}

// URLCacheEntry represents a cached URL entry
type URLCacheEntry struct {
	URL      string
	Endpoint string
}

var (
	urlCache  URLCacheInterface
	startTime = time.Now()
)

// debugMiddleware provides authentication and IP filtering for debug endpoints
func debugMiddleware(config *engine.DebugConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check if debug endpoints are enabled
			if !config.Enabled {
				return c.JSON(http.StatusNotFound, echo.Map{
					"error": "Debug endpoints are disabled",
				})
			}

			// Check auth token if configured
			if config.AuthToken != "" {
				token := c.Request().Header.Get("X-Debug-Token")
				if token == "" {
					token = c.QueryParam("debug_token")
				}
				if token != config.AuthToken {
					return c.JSON(http.StatusUnauthorized, echo.Map{
						"error": "Invalid or missing debug token",
					})
				}
			}

			// Check IP whitelist if configured
			if config.AllowedIPs != "" {
				clientIP := getClientIP(c.Request())
				allowed := false
				for _, allowedIP := range strings.Split(config.AllowedIPs, ",") {
					allowedIP = strings.TrimSpace(allowedIP)
					if allowedIP == clientIP {
						allowed = true
						break
					}
					// Check if it's a CIDR range
					if strings.Contains(allowedIP, "/") {
						_, ipNet, err := net.ParseCIDR(allowedIP)
						if err == nil && ipNet.Contains(net.ParseIP(clientIP)) {
							allowed = true
							break
						}
					}
				}
				if !allowed {
					return c.JSON(http.StatusForbidden, echo.Map{
						"error": fmt.Sprintf("IP %s not allowed", clientIP),
					})
				}
			}

			return next(c)
		}
	}
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// RegisterDebugEndpoints registers all debug endpoints
func RegisterDebugEndpoints(e *echo.Echo, config *engine.ConfigWorkspace, appJson string, urlCacheInterface URLCacheInterface) {
	urlCache = urlCacheInterface
	if !config.DebugConfig.Enabled {
		logger.Info("Debug endpoints are disabled")
		return
	}

	logger.Info("Registering debug endpoints")
	debug := e.Group("/debug", debugMiddleware(&config.DebugConfig))

	// System information
	debug.GET("/info", handleDebugInfo)
	debug.GET("/config", handleDebugConfig(config))
	
	// Repository information
	debug.GET("/repositories", handleDebugRepositories)
	debug.GET("/playbooks", handleDebugPlaybooks(appJson))
	debug.GET("/playbook/:flow", handleDebugPlaybook(appJson))
	
	// Cache management
	debug.POST("/cache/invalidate", handleCacheInvalidate)
	debug.POST("/cache/invalidate/:flow", handleCacheInvalidateFlow)
	debug.GET("/cache/stats", handleCacheStats)
	
	// Process management
	debug.GET("/processes", handleDebugProcesses)
	debug.GET("/process/:wid", handleDebugProcess)
	debug.DELETE("/process/:wid", handleDebugKillProcess)
	
	// Session management  
	debug.GET("/sessions", handleDebugSessions)
	debug.DELETE("/sessions", handleDebugClearSessions)
	
	// VM Pool information
	debug.GET("/vm-pool", handleDebugVMPool)
	
	// Database information
	debug.GET("/database/stats", handleDebugDatabaseStats)
	debug.GET("/database/connections", handleDebugDatabaseConnections)
	
	// Runtime information
	debug.GET("/runtime", handleDebugRuntime)
	debug.GET("/goroutines", handleDebugGoroutines)
	debug.GET("/memory", handleDebugMemory)
	
	// Tracker information
	debug.GET("/tracker/stats", handleDebugTrackerStats)
	
	// URL cache information
	debug.GET("/url-cache", handleDebugURLCache)
	debug.DELETE("/url-cache", handleDebugClearURLCache)

	// Enable pprof if configured
	if config.DebugConfig.EnablePprof {
		logger.Info("Enabling pprof debug endpoints")
		debug.GET("/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}
}

// Debug handler implementations

func handleDebugInfo(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{
		"service":    "nFlow Runtime",
		"version":    "1.0.0",
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"goroutines": runtime.NumGoroutine(),
		"timestamp":  time.Now().Unix(),
		"uptime":     time.Since(startTime).String(),
	})
}

func handleDebugConfig(config *engine.ConfigWorkspace) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Sanitize sensitive information
		sanitized := echo.Map{
			"vm_pool": echo.Map{
				"max_size":              config.VMPoolConfig.MaxSize,
				"preload_size":          config.VMPoolConfig.PreloadSize,
				"idle_timeout":          config.VMPoolConfig.IdleTimeout,
				"max_memory_mb":         config.VMPoolConfig.MaxMemoryMB,
				"max_execution_seconds": config.VMPoolConfig.MaxExecutionSeconds,
				"enable_filesystem":     config.VMPoolConfig.EnableFileSystem,
				"enable_network":        config.VMPoolConfig.EnableNetwork,
			},
			"tracker": echo.Map{
				"enabled":        config.TrackerConfig.Enabled,
				"workers":        config.TrackerConfig.Workers,
				"batch_size":     config.TrackerConfig.BatchSize,
				"channel_buffer": config.TrackerConfig.ChannelBuffer,
			},
			"database": echo.Map{
				"driver": config.DatabaseNflow.Driver,
			},
			"redis": echo.Map{
				"configured": config.RedisConfig.Host != "",
			},
			"debug": echo.Map{
				"enabled":      config.DebugConfig.Enabled,
				"enable_pprof": config.DebugConfig.EnablePprof,
			},
			"monitor": echo.Map{
				"enabled":                config.MonitorConfig.Enabled,
				"health_check_path":      config.MonitorConfig.HealthCheckPath,
				"metrics_path":           config.MonitorConfig.MetricsPath,
				"enable_detailed_metrics": config.MonitorConfig.EnableDetailedMetrics,
			},
		}
		return c.JSON(http.StatusOK, sanitized)
	}
}

func handleDebugRepositories(c echo.Context) error {
	playbookRepo := engine.GetPlaybookRepository()
	if playbookRepo == nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "PlaybookRepository not available"})
	}

	info := echo.Map{
		"playbook_repository": echo.Map{
			"initialized": true,
			"cache_size":  playbookRepo.GetCacheSize(),
		},
		"process_repository": echo.Map{
			"initialized":    true,
			"active_processes": len(process.GetProcessList()),
		},
	}
	
	return c.JSON(http.StatusOK, info)
}

func handleDebugPlaybooks(appJson string) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		repo := engine.GetPlaybookRepository()
		if repo == nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Repository not available"})
		}

		playbooks, err := repo.LoadPlaybook(ctx, appJson)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		summary := echo.Map{
			"app":          appJson,
			"total_flows":  0,
			"total_nodes":  0,
			"flows":        []echo.Map{},
		}

		flows := []echo.Map{}
		totalNodes := 0

		for key, flowMap := range playbooks {
			for flowKey, pb := range flowMap {
				if pb != nil {
					nodeCount := len(*pb)
					totalNodes += nodeCount
					flows = append(flows, echo.Map{
						"key":       key,
						"flow_key":  flowKey,
						"node_count": nodeCount,
					})
				}
			}
		}

		summary["total_flows"] = len(flows)
		summary["total_nodes"] = totalNodes
		summary["flows"] = flows

		return c.JSON(http.StatusOK, summary)
	}
}

func handleDebugPlaybook(appJson string) echo.HandlerFunc {
	return func(c echo.Context) error {
		flow := c.Param("flow")
		ctx := c.Request().Context()
		
		repo := engine.GetPlaybookRepository()
		if repo == nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Repository not available"})
		}

		playbooks, err := repo.LoadPlaybook(ctx, appJson)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		// Find the specific flow
		for key, flowMap := range playbooks {
			for flowKey, pb := range flowMap {
				if flowKey == flow || key == flow {
					if pb != nil {
						return c.JSON(http.StatusOK, echo.Map{
							"flow":       flow,
							"node_count": len(*pb),
							"nodes":      *pb,
						})
					}
				}
			}
		}

		return c.JSON(http.StatusNotFound, echo.Map{"error": "Flow not found"})
	}
}

func handleCacheInvalidate(c echo.Context) error {
	repo := engine.GetPlaybookRepository()
	if repo != nil {
		repo.InvalidateAllCache()
		return c.JSON(http.StatusOK, echo.Map{"message": "All cache invalidated"})
	}
	return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Repository not available"})
}

func handleCacheInvalidateFlow(c echo.Context) error {
	flow := c.Param("flow")
	repo := engine.GetPlaybookRepository()
	if repo != nil {
		repo.InvalidateCache(flow)
		return c.JSON(http.StatusOK, echo.Map{
			"message": "Cache invalidated",
			"flow":    flow,
		})
	}
	return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Repository not available"})
}

func handleCacheStats(c echo.Context) error {
	repo := engine.GetPlaybookRepository()
	if repo == nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Repository not available"})
	}

	urlCacheSize := 0
	if urlCache != nil {
		urlCacheSize = urlCache.GetSize()
	}

	return c.JSON(http.StatusOK, echo.Map{
		"playbook_cache_size": repo.GetCacheSize(),
		"url_cache_size":      urlCacheSize,
	})
}

func handleDebugProcesses(c echo.Context) error {
	processes := process.GetProcessList()
	
	summary := echo.Map{
		"total":     len(processes),
		"processes": processes,
	}
	
	return c.JSON(http.StatusOK, summary)
}

func handleDebugProcess(c echo.Context) error {
	wid := c.Param("wid")
	proc, exists := process.GetProcessID(wid)
	
	if !exists {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Process not found"})
	}
	
	return c.JSON(http.StatusOK, proc)
}

func handleDebugKillProcess(c echo.Context) error {
	wid := c.Param("wid")
	process.WKill(wid)
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Process killed",
		"wid":     wid,
	})
}

func handleDebugSessions(c echo.Context) error {
	// This would need implementation in syncsession package
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Session information not yet implemented",
	})
}

func handleDebugClearSessions(c echo.Context) error {
	// This would need implementation in syncsession package
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Session clearing not yet implemented",
	})
}

func handleDebugVMPool(c echo.Context) error {
	// Since VM pooling is disabled, return appropriate message
	return c.JSON(http.StatusOK, echo.Map{
		"status":  "disabled",
		"message": "VM pooling is currently disabled - creating fresh VM per request",
	})
}

func handleDebugDatabaseStats(c echo.Context) error {
	db, err := engine.GetDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	stats := db.Stats()
	return c.JSON(http.StatusOK, echo.Map{
		"open_connections":      stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	})
}

func handleDebugDatabaseConnections(c echo.Context) error {
	db, err := engine.GetDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// Test database connection
	ctx := c.Request().Context()
	err = db.PingContext(ctx)
	
	return c.JSON(http.StatusOK, echo.Map{
		"connected": err == nil,
		"error":     formatError(err),
		"driver":    getDBDriver(db),
	})
}

func handleDebugRuntime(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return c.JSON(http.StatusOK, echo.Map{
		"goroutines":    runtime.NumGoroutine(),
		"cpus":          runtime.NumCPU(),
		"memory": echo.Map{
			"alloc":         m.Alloc,
			"total_alloc":   m.TotalAlloc,
			"sys":           m.Sys,
			"heap_alloc":    m.HeapAlloc,
			"heap_sys":      m.HeapSys,
			"heap_idle":     m.HeapIdle,
			"heap_in_use":   m.HeapInuse,
			"heap_released": m.HeapReleased,
			"heap_objects":  m.HeapObjects,
			"gc_runs":       m.NumGC,
			"gc_pause_ns":   m.PauseNs[(m.NumGC+255)%256],
		},
	})
}

func handleDebugGoroutines(c echo.Context) error {
	buf := make([]byte, 1<<16)
	stackSize := runtime.Stack(buf, true)
	
	return c.String(http.StatusOK, string(buf[:stackSize]))
}

func handleDebugMemory(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return c.JSON(http.StatusOK, echo.Map{
		"alloc_mb":        m.Alloc / 1024 / 1024,
		"total_alloc_mb":  m.TotalAlloc / 1024 / 1024,
		"sys_mb":          m.Sys / 1024 / 1024,
		"heap_alloc_mb":   m.HeapAlloc / 1024 / 1024,
		"heap_sys_mb":     m.HeapSys / 1024 / 1024,
		"heap_idle_mb":    m.HeapIdle / 1024 / 1024,
		"heap_inuse_mb":   m.HeapInuse / 1024 / 1024,
		"heap_released_mb": m.HeapReleased / 1024 / 1024,
		"heap_objects":    m.HeapObjects,
		"gc_runs":         m.NumGC,
		"gc_cpu_fraction": m.GCCPUFraction,
	})
}

func handleDebugTrackerStats(c echo.Context) error {
	// This would need to be implemented in the tracker
	return c.JSON(http.StatusOK, echo.Map{
		"message": "Tracker stats not yet implemented",
	})
}

func handleDebugURLCache(c echo.Context) error {
	if urlCache == nil {
		return c.JSON(http.StatusOK, echo.Map{
			"size":    0,
			"entries": []echo.Map{},
		})
	}

	entries := urlCache.GetEntries()
	result := make([]echo.Map, 0, len(entries))
	
	for _, entry := range entries {
		result = append(result, echo.Map{
			"url":      entry.URL,
			"endpoint": entry.Endpoint,
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"size":    len(entries),
		"entries": result,
	})
}

func handleDebugClearURLCache(c echo.Context) error {
	if urlCache != nil {
		urlCache.Clear()
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "URL cache cleared",
	})
}

// Helper functions

func formatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func getDBDriver(db *sql.DB) string {
	// This is a simple implementation
	// In production, you might want to store this information
	return "unknown"
}

