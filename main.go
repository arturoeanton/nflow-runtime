// Package main implements the nFlow Runtime server.
// nFlow Runtime executes workflows created in the nFlow visual designer.
// It provides a REST API for workflow execution with support for
// JavaScript-based actions, security sandboxing, and resource limits.
package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/arturoeanton/gocommons/utils"
	"github.com/arturoeanton/nflow-runtime/commons"
	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/literals"
	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
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

// run is the main handler for all workflow execution requests.
// It processes incoming HTTP requests, loads the appropriate workflow,
// and executes it with the provided parameters. This function handles
// all HTTP methods (GET, POST, PUT, etc.) and routes them to the
// corresponding workflow based on the URL path.
func run(c echo.Context, appJson string) error {
	ctx := c.Request().Context()
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

	// Obtener el repository
	repo := engine.GetPlaybookRepository()
	if repo == nil {
		logger.Error("PlaybookRepository not initialized")
		c.HTML(http.StatusInternalServerError, "Internal Server Error")
		return nil
	}

	// Cargar playbooks usando el repository
	appPlaybooks, err := repo.LoadPlaybook(ctx, appJson)
	if CheckError(c, err, 500) {
		return nil
	}

	endpoint := strings.Split(c.Request().RequestURI, "?")[0]
	nflowNextNodeRun := ""
	endpointParts := strings.Split(endpoint, "/")
	lenEndpointParts := len(endpointParts)
	positionTagNflowID := -1
	positionTagNflowTK := -1
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < (lenEndpointParts - 1); i++ {
			if endpointParts[i] == literals.FORMNFLOWID {
				nflowNextNodeRun = endpointParts[i+1]
				positionTagNflowID = i
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < (lenEndpointParts - 1); i++ {
			if endpointParts[i] == literals.FORMNFLOWTK {
				positionTagNflowTK = i
				break
			}
		}
	}()

	wg.Wait()

	if positionTagNflowTK > positionTagNflowID && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	} else if positionTagNflowTK == -1 && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	}

	if positionTagNflowTK > -1 {
		endpoint = strings.Join(endpointParts[:positionTagNflowTK], "/")
		if nflowNextNodeRun == "" {
			if c.Request().Method == "POST" || c.Request().Method == "PUT" {
				if c.Request().FormValue("nflow_next_node_run") != "" {
					if c.Request().Form["nflow_next_node_run"] != nil {
						nflowNextNodeRun = c.Request().Form["nflow_next_node_run"][0]
					}
				}
			} else if c.Request().Method == "GET" {
				if c.Request().URL.Query().Get("nflow_next_node_run") != "" {
					nflowNextNodeRun = c.Request().URL.Query().Get("nflow_next_node_run")
				}
			}
		}
	} else {

		if c.Request().Method == "POST" || c.Request().Method == "PUT" {
			if c.Request().FormValue("nflow_next_node_run") != "" {
				if c.Request().Form["nflow_next_node_run"] != nil {
					nflowNextNodeRun = c.Request().Form["nflow_next_node_run"][0]
				}
			}
		}
	}

	if nflowNextNodeRun == "" {
		func() {
			syncsession.EchoSessionsMutex.Lock()
			defer syncsession.EchoSessionsMutex.Unlock()
			s, _ := session.Get("nflow_form", c)
			s.Values = make(map[interface{}]interface{})
			s.Save(c.Request(), c.Response())
		}()
	}

	runeable, vars, code, _, err := engine.GetWorkflow(c, appPlaybooks, endpoint, c.Request().Method, appJson)
	if CheckError(c, err, code) {
		return nil
	}

	logger.Verbose("Run endpoint:", endpoint, "nflowNextNodeRun:", runeable)

	uuid1 := uuid.New().String()
	e := runeable.Run(c, vars, nflowNextNodeRun, endpoint, uuid1, nil)
	return e
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

	// Create Echo server
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(session.Middleware(commons.GetSessionStore(&config.PgSessionConfig)))

	e.Any("/*", func(c echo.Context) error {
		return run(c, appJson)
	})

	// Start server
	logger.Info("Starting nFlow Runtime Example on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}

func checkAndLoadFileApp() (string, bool) {
	appJson := *app

	if strings.HasSuffix(appJson, ".json") { // If a file is provided, use it directly
		if _, err := os.Stat(appJson); os.IsNotExist(err) {
			logger.Error("Playbook file does not exist:", appJson)
			logger.Error("Please provide a valid playbook file using -a flag")
			return "", true
		}

		logger.Info("Using playbook file:", appJson)

		// Load from file
		data, err := os.ReadFile(appJson)
		if err != nil {
			logger.Error("Failed to read playbook file:", err)
			return "", true
		}
		flowJson := string(data)
		drawflow := make(map[string]map[string]map[string]*model.Playbook)
		err = json.Unmarshal([]byte(flowJson), &drawflow)
		if err != nil {
			logger.Error("Failed to parse playbook JSON:", err)
			return "", true
		}
		repo := engine.GetPlaybookRepository()
		if repo == nil {
			logger.Error("PlaybookRepository not initialized")
			return "", true
		}
		// Guardar en cache
		repo.Set(appJson, drawflow["drawflow"])
		repo.SetReloaded(appJson)
	}
	return appJson, false
}
