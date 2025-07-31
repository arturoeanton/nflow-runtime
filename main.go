// Package main implements the nFlow Runtime server.
// nFlow Runtime executes workflows created in the nFlow visual designer.
// It provides a REST API for workflow execution with support for
// JavaScript-based actions, security sandboxing, and resource limits.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/arturoeanton/gocommons/utils"
	"github.com/arturoeanton/nflow-runtime/commons"
	"github.com/arturoeanton/nflow-runtime/endpoints"
	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/literals"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/arturoeanton/nflow-runtime/ratelimit"
	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	_ "github.com/arturoeanton/nflow-runtime/engine"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var (
	// verbose enables verbose logging when set via -v flag
	verbose = flag.Bool("v", false, "Enable verbose logging")
	app     = flag.String("a", "app", "Application name / Filename for the playbooks")
)

// CheckError handles error responses in a standardized way.
// It sends a JSON error response if err is not nil and returns true,
// otherwise returns false. This helps maintain consistent error handling
// across all API endpoints.
func CheckError(c echo.Context, err error, code int) bool {
	if err != nil {
		c.JSON(code, echo.Map{
			"message": err.Error(),
			"code":    code,
		})
		return true
	}
	return false
}

// URL parsing cache for better performance
type urlParseResult struct {
	endpoint           string
	positionTagNflowID int
	positionTagNflowTK int
}

var urlCache = struct {
	sync.RWMutex
	cache map[string]*urlParseResult
}{
	cache: make(map[string]*urlParseResult),
}

// urlCacheAdapter implements endpoints.URLCacheInterface
type urlCacheAdapter struct{}

func (u *urlCacheAdapter) GetSize() int {
	urlCache.RLock()
	defer urlCache.RUnlock()
	return len(urlCache.cache)
}

func (u *urlCacheAdapter) GetEntries() []endpoints.URLCacheEntry {
	urlCache.RLock()
	defer urlCache.RUnlock()

	entries := make([]endpoints.URLCacheEntry, 0, len(urlCache.cache))
	for url, result := range urlCache.cache {
		entries = append(entries, endpoints.URLCacheEntry{
			URL:      url,
			Endpoint: result.endpoint,
		})
	}
	return entries
}

func (u *urlCacheAdapter) Clear() {
	urlCache.Lock()
	defer urlCache.Unlock()
	urlCache.cache = make(map[string]*urlParseResult)
}

// parseURL extracts endpoint and position tags with caching
func parseURL(requestURI string) (string, int, int) {
	// Check cache first
	urlCache.RLock()
	if result, ok := urlCache.cache[requestURI]; ok {
		urlCache.RUnlock()
		return result.endpoint, result.positionTagNflowID, result.positionTagNflowTK
	}
	urlCache.RUnlock()

	// Parse URL
	endpoint := strings.Split(requestURI, "?")[0]
	endpointParts := strings.Split(endpoint, "/")
	lenEndpointParts := len(endpointParts)
	positionTagNflowID := -1
	positionTagNflowTK := -1

	// Sequential search is faster than goroutines for small arrays
	for i := 0; i < (lenEndpointParts - 1); i++ {
		if endpointParts[i] == literals.FORMNFLOWID {
			positionTagNflowID = i
		}
		if endpointParts[i] == literals.FORMNFLOWTK {
			positionTagNflowTK = i
		}
	}

	// Cache result (with size limit to prevent unbounded growth)
	urlCache.Lock()
	if len(urlCache.cache) < 10000 {
		urlCache.cache[requestURI] = &urlParseResult{
			endpoint:           endpoint,
			positionTagNflowID: positionTagNflowID,
			positionTagNflowTK: positionTagNflowTK,
		}
	}
	urlCache.Unlock()

	return endpoint, positionTagNflowID, positionTagNflowTK
}

// extractNextNodeRun extracts nflow_next_node_run from various sources
func extractNextNodeRun(c echo.Context, endpointParts []string, positionTagNflowID int, positionTagNflowTK int) string {
	nflowNextNodeRun := ""

	// Get from URL if present
	if positionTagNflowID > -1 && positionTagNflowID < len(endpointParts)-1 {
		nflowNextNodeRun = endpointParts[positionTagNflowID+1]
	}

	// If not in URL, check request parameters
	if nflowNextNodeRun == "" {
		method := c.Request().Method
		if method == "POST" || method == "PUT" {
			if formValue := c.Request().FormValue("nflow_next_node_run"); formValue != "" {
				nflowNextNodeRun = formValue
			}
		} else if method == "GET" {
			if queryValue := c.Request().URL.Query().Get("nflow_next_node_run"); queryValue != "" {
				nflowNextNodeRun = queryValue
			}
		}
	}

	return nflowNextNodeRun
}

// run is the main handler for all workflow execution requests.
// It processes incoming HTTP requests, loads the appropriate workflow,
// and executes it with the provided parameters. This function handles
// all HTTP methods (GET, POST, PUT, etc.) and routes them to the
// corresponding workflow based on the URL path.
func run(c echo.Context, appJson string) error {
	ctx := c.Request().Context()

	// Get database connection
	db, err := engine.GetDB()
	if err != nil {
		logger.Error("Failed to get database connection:", err)
		c.HTML(http.StatusNotFound, literals.NOT_FOUND)
		return nil
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		logger.Error("Failed to get database connection:", err)
		c.HTML(http.StatusNotFound, literals.NOT_FOUND)
		return nil
	}
	defer conn.Close()

	// Set repositories to non-dynamic mode
	repoTemplate := engine.GetRepositoryTemplate()
	repoTemplate.SetDinamic(false)

	modulesRepo := engine.GetRepositoryModules()
	modulesRepo.SetDinamic(false)

	// Get playbook repository
	repo := engine.GetPlaybookRepository()
	if repo == nil {
		logger.Error("PlaybookRepository not initialized")
		c.HTML(http.StatusInternalServerError, "Internal Server Error")
		return nil
	}

	// Load playbooks
	appPlaybooks, err := repo.LoadPlaybook(ctx, appJson)
	if CheckError(c, err, 500) {
		return nil
	}

	// Parse URL with caching
	endpoint, positionTagNflowID, positionTagNflowTK := parseURL(c.Request().RequestURI)
	endpointParts := strings.Split(endpoint, "/")

	// Adjust position tags
	if positionTagNflowTK > positionTagNflowID && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	} else if positionTagNflowTK == -1 && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	}

	// Build final endpoint
	if positionTagNflowTK > -1 {
		endpoint = strings.Join(endpointParts[:positionTagNflowTK], "/")
	}

	// Extract next node run parameter
	nflowNextNodeRun := extractNextNodeRun(c, endpointParts, positionTagNflowID, positionTagNflowTK)

	// Clear session if needed
	if nflowNextNodeRun == "" {
		clearSession(c)
	}

	// Get workflow
	runeable, vars, code, _, err := engine.GetWorkflow(c, appPlaybooks, endpoint, c.Request().Method, appJson)
	if CheckError(c, err, code) {
		return nil
	}

	logger.Verbose("Run endpoint:", endpoint, "nflowNextNodeRun:", runeable)

	// Execute workflow
	uuid1 := uuid.New().String()
	return runeable.Run(c, vars, nflowNextNodeRun, endpoint, uuid1, nil)
}

// clearSession clears the nflow_form session
func clearSession(c echo.Context) {
	syncsession.EchoSessionsMutex.Lock()
	defer syncsession.EchoSessionsMutex.Unlock()
	s, _ := session.Get("nflow_form", c)
	s.Values = make(map[interface{}]interface{})
	s.Save(c.Request(), c.Response())
}

func main() {
	flag.Parse()

	// Initialize logger with verbose flag
	logger.Initialize(*verbose)
	logger.Info("Starting nFlow Runtime")
	if *verbose {
		logger.Verbose("Verbose logging enabled")
	}

	configPath := "config.toml"

	// Initialize ConfigRepository
	configRepo := engine.GetConfigRepository()

	// Load configuration
	var config engine.ConfigWorkspace
	if utils.Exists(configPath) {
		data, _ := utils.FileToString(configPath)
		if _, err := toml.Decode(data, &config); err != nil {
			logger.Error("Failed to decode config.toml:", err)
		}
		configRepo.SetConfig(config)
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisConfig.Host,
		Password: config.RedisConfig.Password, // no password set
		DB:       0,                           // use default DB
	})
	configRepo.SetRedisClient(redisClient)

	engine.UpdateQueries()

	engine.LoadPlugins()

	engine.StartTracker(70) // Start tracker with 70 workers

	// Initialize database and repository
	db, err := engine.GetDB()
	if err != nil {
		logger.Fatal("Failed to initialize database:", err)
	}
	engine.InitializePlaybookRepository(db)
	logger.Info("PlaybookRepository initialized")

	// Initialize ProcessRepository
	process.InitializeRepository()
	logger.Info("ProcessRepository initialized")

	appJson, shouldReturn := checkAndLoadFileApp()
	if shouldReturn {
		return
	}
	logger.Info("Using playbook app:", appJson)

	// Initialize Session Manager
	logger.Info("Starting Session Manager cleanup routine...")
	go syncsession.Manager.StartCleanupRoutine()

	// VM pooling is disabled for now
	// A fresh VM is created for each request to ensure stability
	logger.Info("VM pooling disabled - creating fresh VM per request for stability")

	// Initialize rate limiter
	var rateLimiter ratelimit.RateLimiter
	if config.RateLimitConfig.Enabled {
		rateLimiter = ratelimit.NewRateLimiter(&config.RateLimitConfig, redisClient)
		logger.Info("Rate limiting enabled")
		logger.Infof("IP rate limit: %d requests per %d minute(s)",
			config.RateLimitConfig.IPRateLimit,
			config.RateLimitConfig.IPWindowMinutes)
	}

	// Create Echo server
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Add rate limiting middleware before session middleware
	if config.RateLimitConfig.Enabled && rateLimiter != nil {
		e.Use(ratelimit.Middleware(&config.RateLimitConfig, rateLimiter))
	}

	e.Use(session.Middleware(commons.GetSessionStore(&config.PgSessionConfig)))

	// Register monitoring endpoints (health and metrics)
	endpoints.RegisterMonitoringEndpoints(e, &config)

	// Register debug endpoints if enabled
	endpoints.RegisterDebugEndpoints(e, &config, appJson, &urlCacheAdapter{})

	// Legacy debug endpoints (kept for backward compatibility)
	if config.DebugConfig.Enabled {
		e.GET("/debug/invalidate-cache", func(c echo.Context) error {
			repo := engine.GetPlaybookRepository()
			if repo != nil {
				repo.InvalidateAllCache()
				return c.JSON(200, echo.Map{"message": "Cache invalidated"})
			}
			return c.JSON(500, echo.Map{"error": "Repository not available"})
		})

		e.GET("/debug/clean-json", func(c echo.Context) error {
			return handleDebugCleanJSON(c, appJson)
		})

		e.GET("/debug/starters", func(c echo.Context) error {
			return handleDebugStarters(c, appJson)
		})
	}

	// Main workflow handler - must be last
	e.Any("/*", func(c echo.Context) error {
		return run(c, appJson)
	})

	// Start server
	logger.Info("Starting nFlow Runtime on :8080")
	if config.MonitorConfig.Enabled {
		logger.Infof("Health check available at %s", config.MonitorConfig.HealthCheckPath)
		logger.Infof("Prometheus metrics available at %s", config.MonitorConfig.MetricsPath)
	}
	if config.DebugConfig.Enabled {
		logger.Info("Debug endpoints enabled at /debug/*")
	}

	// Add shutdown handler
	go func() {
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Cleanup rate limiter
	if rateLimiter != nil {
		rateLimiter.Close()
		logger.Info("Rate limiter closed")
	}

	logger.Info("Server shutting down")
}

// Debug handler functions

// handleDebugCleanJSON removes corrupted starter nodes from playbooks
func handleDebugCleanJSON(c echo.Context, appJson string) error {
	ctx := c.Request().Context()
	repo := engine.GetPlaybookRepository()
	if repo == nil {
		return c.JSON(500, echo.Map{"error": "Repository not available"})
	}

	// Force reload to get fresh data from database
	repo.InvalidateAllCache()
	appPlaybooks, err := repo.LoadPlaybook(ctx, appJson)
	if err != nil {
		return c.JSON(500, echo.Map{"error": err.Error()})
	}

	// Clean the playbooks
	cleanedPlaybooks, removedCount := cleanPlaybooks(appPlaybooks)

	// Wrap in drawflow structure for database storage
	result := map[string]interface{}{
		"drawflow": cleanedPlaybooks,
	}

	return c.JSON(200, map[string]interface{}{
		"message":       fmt.Sprintf("Cleaned JSON ready for database storage. Removed %d corrupted starter nodes.", removedCount),
		"removed_nodes": removedCount,
		"clean_json":    result,
	})
}

// cleanPlaybooks removes corrupted starter nodes
func cleanPlaybooks(appPlaybooks map[string]map[string]*model.Playbook) (map[string]map[string]*model.Playbook, int) {
	cleanedPlaybooks := make(map[string]map[string]*model.Playbook)
	removedCount := 0

	for key, flows := range appPlaybooks {
		cleanedFlows := make(map[string]*model.Playbook)
		for flowKey, pb := range flows {
			if pb == nil {
				cleanedFlows[flowKey] = pb
				continue
			}

			cleanedPlaybook := make(model.Playbook)
			for nodeID, node := range *pb {
				if isValidNode(node, nodeID, &removedCount) {
					cleanedPlaybook[nodeID] = node
				}
			}

			cleanedFlows[flowKey] = &cleanedPlaybook
		}
		cleanedPlaybooks[key] = cleanedFlows
	}

	return cleanedPlaybooks, removedCount
}

// isValidNode checks if a node is valid (not a corrupted starter)
func isValidNode(node *model.Node, nodeID string, removedCount *int) bool {
	if node == nil || node.Data == nil {
		return true
	}

	// Check if this is a starter node
	nodeType, ok := node.Data["type"]
	if !ok || nodeType != "starter" {
		return true
	}

	// Check if starter has proper connections
	if node.Outputs == nil {
		logger.Verbosef("DEBUG: Removing starter node %s - no outputs", nodeID)
		*removedCount++
		return false
	}

	output1, exists := node.Outputs["output_1"]
	if !exists || output1 == nil {
		logger.Verbosef("DEBUG: Removing starter node %s - no output_1", nodeID)
		*removedCount++
		return false
	}

	if output1.Connections == nil || len(output1.Connections) == 0 {
		logger.Verbosef("DEBUG: Removing starter node %s - empty connections", nodeID)
		*removedCount++
		return false
	}

	return true
}

// handleDebugStarters shows all starter nodes in the playbooks
func handleDebugStarters(c echo.Context, appJson string) error {
	ctx := c.Request().Context()
	repo := engine.GetPlaybookRepository()
	if repo == nil {
		return c.JSON(500, echo.Map{"error": "Repository not available"})
	}

	// Force reload to see fresh data
	repo.InvalidateAllCache()
	appPlaybooks, err := repo.LoadPlaybook(ctx, appJson)
	if err != nil {
		return c.JSON(500, echo.Map{"error": err.Error()})
	}

	starters := collectStarters(appPlaybooks)

	return c.JSON(200, map[string]interface{}{
		"total_starters": len(starters),
		"starters":       starters,
	})
}

// collectStarters collects all starter nodes from playbooks
func collectStarters(appPlaybooks map[string]map[string]*model.Playbook) []map[string]interface{} {
	starters := []map[string]interface{}{}

	for key, flows := range appPlaybooks {
		for flowKey, pb := range flows {
			if pb == nil {
				continue
			}
			for nodeID, node := range *pb {
				if starter := extractStarterInfo(node, nodeID, key, flowKey); starter != nil {
					starters = append(starters, starter)
				}
			}
		}
	}

	return starters
}

// extractStarterInfo extracts information from a starter node
func extractStarterInfo(node *model.Node, nodeID, flowKey, subKey string) map[string]interface{} {
	if node == nil || node.Data == nil {
		return nil
	}

	nodeType, ok := node.Data["type"]
	if !ok || nodeType != "starter" {
		return nil
	}

	starter := map[string]interface{}{
		"flow_key":    flowKey,
		"sub_key":     subKey,
		"node_id":     nodeID,
		"urlpattern":  node.Data["urlpattern"],
		"method":      node.Data["method"],
		"name":        node.Data["name_box"],
		"has_outputs": node.Outputs != nil,
	}

	// Add connection information
	addConnectionInfo(starter, node)

	return starter
}

// addConnectionInfo adds connection details to starter info
func addConnectionInfo(starter map[string]interface{}, node *model.Node) {
	if node.Outputs == nil {
		starter["connections_count"] = "no_outputs"
		starter["has_connections"] = false
		return
	}

	output1, exists := node.Outputs["output_1"]
	if !exists || output1 == nil {
		starter["connections_count"] = "no_output_1"
		starter["has_connections"] = false
		return
	}

	starter["connections_count"] = len(output1.Connections)
	starter["has_connections"] = len(output1.Connections) > 0

	if len(output1.Connections) > 0 {
		starter["first_connection"] = map[string]string{
			"node":   output1.Connections[0].Node,
			"output": output1.Connections[0].Output,
		}
	}
}

// checkAndLoadFileApp loads a playbook file if specified
func checkAndLoadFileApp() (string, bool) {
	appJson := *app

	if !strings.HasSuffix(appJson, ".json") {
		return appJson, false
	}

	// Validate file exists
	if _, err := os.Stat(appJson); os.IsNotExist(err) {
		logger.Error("Playbook file does not exist:", appJson)
		logger.Error("Please provide a valid playbook file using -a flag")
		return "", true
	}

	logger.Info("Using playbook file:", appJson)

	// Load and parse file
	if err := loadPlaybookFile(appJson); err != nil {
		logger.Error("Failed to load playbook:", err)
		return "", true
	}

	return appJson, false
}

// loadPlaybookFile loads a playbook from a JSON file
func loadPlaybookFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var drawflow map[string]map[string]map[string]*model.Playbook
	if err := json.Unmarshal(data, &drawflow); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	repo := engine.GetPlaybookRepository()
	if repo == nil {
		return fmt.Errorf("PlaybookRepository not initialized")
	}

	// Save to cache
	repo.Set(filename, drawflow["drawflow"])
	repo.SetReloaded(filename)

	return nil
}
