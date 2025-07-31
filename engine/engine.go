package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"sync"
	"sync/atomic"
	"time"

	"github.com/arturoeanton/gocommons/utils"
	"github.com/arturoeanton/nflow-runtime/logger"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/util"
	"github.com/google/uuid"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

var (
	registry     *require.Registry
	registryOnce sync.Once

	// Cache for auth.js to avoid repeated file reads
	authCodeCache struct {
		sync.RWMutex
		code      string
		loaded    bool
		lastCheck time.Time
	}
)

// Helper function to debug output keys
func getOutputKeys(outputs map[string]*model.Output) []string {
	if outputs == nil {
		return []string{"<nil map>"}
	}
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return []string{"<empty map>"}
	}
	return keys
}

// GetRequireRegistry retorna el registry de require
func GetRequireRegistry() *require.Registry {
	return registry
}

func init() {
	// Initialize registry once with sync.Once for thread safety
	registryOnce.Do(func() {
		registry = new(require.Registry)
		registry.RegisterNativeModule("console", console.Require)
		registry.RegisterNativeModule("util", util.Require)
	})
	logger.Info("Started goja")
}

// Run ejecuta el workflow
func Run(cc *model.Controller, c echo.Context, vars model.Vars, next string, endpoint string, uuid1 string, payload goja.Value) error {
	return run(cc, c, vars, next, endpoint, uuid1, payload, false)
}

// RunWithCallback ejecuta el workflow con callback
func RunWithCallback(cc *model.Controller, c echo.Context, vars model.Vars, next string, endpoint string, uuid1 string, payload goja.Value) error {
	return run(cc, c, vars, next, endpoint, uuid1, payload, true)
}

// run es la función interna que ejecuta el workflow
func run(cc *model.Controller, c echo.Context, vars model.Vars, next string, endpoint string, uuid1 string, payload goja.Value, fork bool) error {

	var p *process.Process

	// Si es un fork (goroutine), usar contexto aislado
	if fork {
		c = NewIsolatedContext(c)
		p = process.CreateProcessWithCallback(uuid1)
		go func(uuid2 string, currentProcess *process.Process) {
			data := <-currentProcess.Callback
			var p map[string]interface{}
			json.Unmarshal([]byte(data), &p)
			if _, ok := p["error_exit"]; ok {
				currentProcess.SetFlagExit(1)
			}

		}(uuid1, p)
	} else {
		p = process.CreateProcess(uuid1)
	}

	defer func() {
		p.SendCallback(`{"error_exit":"exit"}`)
		p.Close()
	}()

	// Set workflow ID header for tracking
	if _, isIsolated := c.(*IsolatedContext); !isIsolated {
		// Check if header already exists before locking
		if c.Response().Header().Get("Nflow-Wid-1") == "" {
			EchoSessionsMutex.Lock()
			c.Response().Header().Add("Nflow-Wid-1", uuid1)
			EchoSessionsMutex.Unlock()
		}
	}

	// Use VM from pool for better performance
	vmManager := GetVMManager()
	var vm *goja.Runtime
	
	// Acquire VM from pool
	vmInstance, err := vmManager.AcquireVM(c)
	if err != nil {
		logger.Errorf("Error acquiring VM from pool: %v", err)
		c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to acquire execution environment"})
		return nil
	}
	defer vmManager.ReleaseVM(vmInstance)
	
	vm = vmInstance.VM
	
	// TODO: Fix resource limits for pooled VMs
	// Currently disabled because trackers are interfering with pooled VMs
	// limits := GetLimitsFromConfig()
	// tracker := SetupVMWithLimits(vm, limits)
	// defer tracker.Stop()

	// IMPORTANT: Re-set request-specific globals for this VM
	// The VM from pool needs fresh context for each request
	AddGlobals(vm, c)

	// Set endpoint and request data in the VM. These global variables
	// are accessible to all JavaScript code in the workflow.
	vm.Set("nflow_endpoint", endpoint)

	// Parse and expose POST data to the workflow
	postData := make(map[string]interface{})
	c.Bind(&postData)
	vm.Set("post_data", postData)

	// Set path variables extracted from the URL
	vm.Set("vars", vars)
	vm.Set("path_vars", vars)

	// Set workflow instance ID for tracking
	vm.Set("wid", uuid1)

	// Provide workflow kill function to allow workflows to terminate other workflows
	vm.Set("wkill", func(wid string) {
		process.WKill(wid)
	})

	// Get the playbook and determine the starting node
	// Use the Controller passed directly to avoid any concurrency issues
	if cc.Playbook == nil {
		c.JSON(http.StatusInternalServerError, echo.Map{"error": "Playbook not loaded."})
		return nil
	}

	pb := *cc.Playbook
	nodeAuth := pb[next]

	if next == "" {
		if cc.Start == nil {
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Start node not configured."})
			return nil
		}

		logger.Verbosef("DEBUG: Processing workflow %s, endpoint %s", cc.FlowName, endpoint)
		logger.Verbosef("DEBUG: Controller address: %p, Start address: %p", cc, cc.Start)
		logger.Verbosef("Start Data: %+v", cc.Start.Data)

		// Defensive programming - check each step carefully
		if cc.Start.Outputs == nil {
			logger.Error("DEBUG: cc.Start.Outputs is nil")
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Start node has no outputs configured."})
			return nil
		}

		output1, exists := cc.Start.Outputs["output_1"]
		if !exists {
			logger.Errorf("DEBUG: output_1 does not exist. Available outputs: %v", getOutputKeys(cc.Start.Outputs))
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Start node missing 'output_1' connection."})
			return nil
		}

		if output1 == nil {
			logger.Error("DEBUG: output1 is nil")
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Output_1 is null."})
			return nil
		}

		if output1.Connections == nil {
			logger.Error("DEBUG: output1.Connections is nil")
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Output_1 connections is null."})
			return nil
		}

		// Make a defensive copy of connections to avoid concurrent modification
		connections := make([]struct {
			Node   string `json:"node"`
			Output string `json:"output"`
		}, len(output1.Connections))
		copy(connections, output1.Connections)

		if len(connections) == 0 {
			logger.Errorf("DEBUG: connections copy is empty! original len: %d, copy len: %d", len(output1.Connections), len(connections))
			logger.Errorf("DEBUG: output1.Connections is empty! output1 address: %p, Connections address: %p", output1, output1.Connections)
			logger.Errorf("DEBUG: output1 full struct: %+v", output1)
			logger.Errorf("DEBUG: cc.Start full struct: %+v", cc.Start)
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "No output connections found for the start node."})
			return nil
		}

		logger.Errorf("DEBUG: SUCCESS! Found %d connections, first connection: %+v", len(connections), connections[0])
		next = connections[0].Node
		nodeAuth = cc.Start

		logger.Verbosef("DEBUG: Successfully found next node: %s", next)
	}

	// Check if authentication is required for this node. The nflow_auth flag
	// in node data determines if authentication should be enforced.
	// No mutex needed since we're working with immutable data
	flag, hasAuthFlag := nodeAuth.Data["nflow_auth"]

	if hasAuthFlag {
		flagString, ok := flag.(string)
		if !ok {
			flagBool := flag.(bool)
			if flagBool {
				flagString = "true"
			} else {
				flagString = "false"
			}
		}
		if flagString != "false" {
			// Execute authentication from default.js
			profile := getAuthProfile(c)
			vm.Set("profile", profile)
			vm.Set("next", next)
			vm.Set("auth_flag", flagString)
			vm.Set("url_access", c.Request().URL.Path)

			// Get auth code with caching
			code := getCachedAuthCode()
			if code == "" {
				return nil
			}
			_, err = vm.RunString(code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
				return nil
			}

			next = vm.Get("next").String()
			logger.Verbose("Next node:", next)
			if next == "login" {
				return c.Redirect(http.StatusTemporaryRedirect, "/nflow_login")
			}
			if next == "break" {
				return nil
			}
		}
	}

	Execute(cc, c, vm, next, vars, p, payload, fork)

	return nil
}

func step(cc *model.Controller, c echo.Context, vm *goja.Runtime, next string, vars model.Vars, currentProcess *process.Process, payload goja.Value) (string, goja.Value, error) {
	t1 := time.Now()
	sbLog := strings.Builder{}
	connectionNext := "output_1"

	var logId string
	var orderBox int
	var err error

	// Generate log ID and order box values. This anonymous function handles
	// the difference between isolated contexts (used in concurrent execution)
	// and regular contexts that access session data.
	func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Error processing node:", err)
			}
		}()

		// If it's an isolated context, generate default values
		if _, isIsolated := c.(*IsolatedContext); isIsolated {
			logId = uuid.New().String()
			orderBox = 1
			return
		}

		// For normal contexts, use mutex
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()

		log_session, err := session.Get("log-session", c)
		if err != nil {
			logger.Error("Error processing node:", err)
			// En caso de error, usar valores por defecto
			logId = uuid.New().String()
			orderBox = 1
			return
		}
		if log_session.Values["log_id"] == nil {
			log_session.Values["log_id"] = uuid.New().String()
			log_session.Values["order_box"] = 0
		}

		logId = log_session.Values["log_id"].(string)
		orderBox = log_session.Values["order_box"].(int) + 1
		log_session.Values["order_box"] = orderBox
		log_session.Save(c.Request(), c.Response())

	}()

	var actor *model.Node
	var boxId string
	var boxName string
	var boxType string
	defer func() {
		// Quick exit if tracker is disabled
		if !IsTrackerEnabled() || trackerChannel == nil {
			return
		}

		// Calculate execution time
		diff := time.Since(t1)

		// Create tracker entry with basic info
		entry := TrackerEntry{
			LogId:          logId,
			BoxId:          boxId,
			BoxName:        boxName,
			BoxType:        boxType,
			ConnectionNext: connectionNext,
			Diff:           diff,
			OrderBox:       orderBox,
		}

		// Extract username from profile if available
		if profile := GetProfile(c); profile != nil {
			if username, ok := profile["username"]; ok {
				entry.Username = username
			}
		}

		// Marshal payload efficiently
		if payload != nil {
			PayloadSessionMutex.Lock()
			if data, err := json.Marshal(payload.Export()); err == nil {
				entry.JSONPayload = data
			} else {
				entry.JSONPayload = []byte("{}")
			}
			PayloadSessionMutex.Unlock()
		} else {
			entry.JSONPayload = []byte("{}")
		}

		// Extract request information safely
		if req := c.Request(); req != nil {
			entry.IP = req.RemoteAddr
			entry.RealIP = c.RealIP()
			entry.UserAgent = req.UserAgent()
			entry.Host = req.Host

			if reqURL := req.URL; reqURL != nil {
				entry.URL = reqURL.RawPath
				if entry.URL == "" {
					entry.URL = reqURL.Path
				}
				entry.QueryParam = reqURL.Query().Encode()
				entry.Hostname = reqURL.Hostname()
			}
		}

		// Send to tracker channel (non-blocking)
		select {
		case trackerChannel <- entry:
			// Successfully sent
		default:
			// Channel full, increment dropped counter
			atomic.AddInt64(&trackerStats.Dropped, 1)
		}
	}()
	defer func() {
		err := recover()
		if err != nil {
			logger.Error("Error processing node:", err)
			logger.Error("step_00010 error:", err)
		}
	}()

	if currentProcess.GetFlagExit() == 1 {
		currentProcess.Close()
		panic("FlagExit")
	}

	// Get the actor from the controller playbook
	pb := *cc.Playbook
	originalActor := pb[next]

	if originalActor == nil {
		logger.Errorf("Node not found: %s", next)
		return "", nil, fmt.Errorf("node not found: %s", next)
	}

	// Create a copy to avoid shared state issues
	actor, err = originalActor.DeepCopy()
	if err != nil {
		logger.Errorf("Error creating actor copy: %v", err)
		// If copy fails, use original (with potential race condition risk)
		actor = originalActor
	}

	sbLog.WriteString("- IDBox:" + next)
	currentProcess.UUIDBoxCurrent = next
	boxId = next

	// Extract node name if available. Since we have a copy of the actor,
	// we don't need mutex protection for accessing its data.
	if nameBox, ok := actor.Data["name_box"]; ok {
		boxName = nameBox.(string)
		sbLog.WriteString("- NameBox:" + boxName)
	}

	currentProcess.Type = ""
	if pType, ok := actor.Data["type"]; ok {
		currentProcess.Type = pType.(string)
	}
	boxType = currentProcess.Type

	// Execute the node based on its type. Each node type has a specific
	// implementation in the Steps registry that defines how it should be executed.
	sbLog.WriteString(" - Type:" + currentProcess.Type)
	if s, ok := Steps[currentProcess.Type]; ok {
		connectionNext, payload, err = s.Run(cc, actor, c, vm, connectionNext, vars, currentProcess, payload)
		if err != nil {
			sbLog.WriteString(" - Error: " + err.Error())
			return "", nil, nil
		}
	} else {

		if currentProcess.Type == "starter" {
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "Starter can not run with play button"})
			sbLog.WriteString(" - Error: Starter can not run with play button")
			return "", nil, nil
		}

		c.JSON(http.StatusInternalServerError, echo.Map{"error": "Type node not found", "type": currentProcess.Type})
		sbLog.WriteString(" - Error: Not Found type")
		return "", nil, nil
	}

	sbLog.WriteString(" - Next:" + connectionNext)
	return connectionNext, payload, nil
}

// Execute runs the main workflow execution loop. It processes nodes sequentially,
// following the connections between them until there are no more nodes to execute.
// The function handles forking for parallel execution and maintains the workflow state.
// Parameters:
//   - cc: The controller containing the workflow definition
//   - c: Echo context for HTTP request/response
//   - vm: JavaScript VM for executing node code
//   - next: ID of the next node to execute
//   - vars: Variables extracted from the URL path
//   - currentProcess: Current process instance tracking execution
//   - payload: Data passed between nodes
//   - fork: Whether this is a forked execution
func Execute(cc *model.Controller, c echo.Context, vm *goja.Runtime, next string, vars model.Vars, currentProcess *process.Process, payload goja.Value, fork bool) {
	var err error
	var wg sync.WaitGroup
	prevBox := ""
	if fork {
		logger.Verbose("Processing fork")
	}
	// Main execution loop - continues until there are no more nodes to process
	for next != "" {
		// Set current and previous node IDs in the VM for use in node scripts
		vm.Set("current_box", next)
		vm.Set("prev_box", prevBox)

		prevBox = next

		wg.Add(1)
		go func() {
			defer wg.Done()

			payloadMap := make(map[string]interface{})

			if payload != nil {
				func() {
					PayloadSessionMutex.Lock()
					defer PayloadSessionMutex.Unlock()
					payloadMap = payload.Export().(map[string]interface{})
				}()
			}

			// Si es un contexto aislado, no acceder a la sesión real
			if _, isIsolated := c.(*IsolatedContext); !isIsolated {
				EchoSessionsMutex.Lock()
				defer EchoSessionsMutex.Unlock()

				var s *sessions.Session
				s, err = session.Get("nflow_form", c)
				if err != nil {
					logger.Error("Error in start data:", err)
				} else if s != nil && s.Values != nil {
					for k, v := range s.Values {
						if k == "break" {
							continue
						}
						payloadMap[k.(string)] = v
					}
				}
			}

			func() {
				PayloadSessionMutex.Lock()
				defer PayloadSessionMutex.Unlock()
				payload = vm.ToValue(payloadMap)
			}()
		}()
		wg.Wait()

		next, payload, err = step(cc, c, vm, next, vars, currentProcess, payload)
		if err != nil {
			break
		}
		if fork {
			logger.Verbose("Processing fork")
		}

		// cut
		if payload != nil {
			if rawPayload, ok := payload.Export().(map[string]interface{}); ok {
				wg.Add(1)
				go func() {
					defer wg.Done()

					// Si es un contexto aislado, no guardar en sesión real
					if _, isIsolated := c.(*IsolatedContext); isIsolated {
						return
					}

					EchoSessionsMutex.Lock()
					defer EchoSessionsMutex.Unlock()

					s, err := session.Get("nflow_form", c)
					if err != nil {
						logger.Error("Error in start data:", err)
						return
					}
					for k, v := range rawPayload {
						s.Values[k] = v
					}

					s.Save(c.Request(), c.Response())
				}()
				wg.Wait()

				if raw, ok := rawPayload["break"]; ok {
					if flag, ok := raw.(bool); ok {
						if flag {
							break
						}
					}
					if flag, ok := raw.(string); ok {
						if flag == "true" {
							break
						}
					}
				}
			}
		}

	}

	if next == "" && !fork {
		func() {
			// Si es un contexto aislado, no limpiar sesión real
			if _, isIsolated := c.(*IsolatedContext); isIsolated {
				return
			}

			EchoSessionsMutex.Lock()
			defer EchoSessionsMutex.Unlock()

			s, err := session.Get("nflow_form", c)
			if err != nil {
				logger.Error("Error processing node:", err)
				return
			}
			s.Values = make(map[interface{}]interface{})
			s.Save(c.Request(), c.Response())
		}()

		currentProcess.State = "end"
		currentProcess.Killeable = false
		currentProcess.Close()

	}

}

// Helper functions for better performance and code organization

// getCachedAuthCode returns the auth.js code with caching to avoid repeated file reads
func getCachedAuthCode() string {
	// Check cache with read lock
	authCodeCache.RLock()
	if authCodeCache.loaded && time.Since(authCodeCache.lastCheck) < 5*time.Minute {
		code := authCodeCache.code
		authCodeCache.RUnlock()
		return code
	}
	authCodeCache.RUnlock()

	// Load with write lock
	authCodeCache.Lock()
	defer authCodeCache.Unlock()

	// Double-check after acquiring write lock
	if authCodeCache.loaded && time.Since(authCodeCache.lastCheck) < 5*time.Minute {
		return authCodeCache.code
	}

	// Load from file
	triggersFold := os.Getenv("NFLOE_TRIGGERS_FOLD")
	if triggersFold == "" {
		triggersFold = "triggers/"
	}

	authFile := triggersFold + "auth.js"
	if !utils.Exists(authFile) {
		authCodeCache.loaded = true
		authCodeCache.code = ""
		authCodeCache.lastCheck = time.Now()
		return ""
	}

	data, err := utils.FileToString(authFile)
	if err != nil || data == "" {
		logger.Error("Error reading auth.js:", err)
		return ""
	}

	// Update cache
	authCodeCache.code = data + "\nauth()"
	authCodeCache.loaded = true
	authCodeCache.lastCheck = time.Now()

	return authCodeCache.code
}

// getAuthProfile extracts the user profile from session with proper locking
func getAuthProfile(c echo.Context) interface{} {
	// If it's an isolated context, don't access real session
	if _, isIsolated := c.(*IsolatedContext); isIsolated {
		return nil
	}

	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()

	authSession, err := session.Get("auth-session", c)
	if err != nil {
		logger.Error("Error in start data:", err)
		return nil
	}

	authSession.Values["redirect_url"] = c.Request().URL.Path
	authSession.Save(c.Request(), c.Response())

	return authSession.Values["profile"]
}

// mergeSessionPayload merges session data with the current payload
func mergeSessionPayload(c echo.Context, vm *goja.Runtime, payload goja.Value) goja.Value {
	payloadMap := make(map[string]interface{})

	// Extract existing payload
	if payload != nil {
		PayloadSessionMutex.Lock()
		payloadMap = payload.Export().(map[string]interface{})
		PayloadSessionMutex.Unlock()
	}

	// Skip session merge for isolated contexts
	if _, isIsolated := c.(*IsolatedContext); isIsolated {
		return payload
	}

	// Merge session data
	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()

	s, err := session.Get("nflow_form", c)
	if err == nil && s != nil && s.Values != nil {
		for k, v := range s.Values {
			if k != "break" {
				if key, ok := k.(string); ok {
					payloadMap[key] = v
				}
			}
		}
	}

	// Convert back to goja value
	PayloadSessionMutex.Lock()
	defer PayloadSessionMutex.Unlock()
	return vm.ToValue(payloadMap)
}

// savePayloadToSession saves the payload to session and returns true if break is requested
func savePayloadToSession(c echo.Context, payload goja.Value) bool {
	if payload == nil {
		return false
	}

	rawPayload, ok := payload.Export().(map[string]interface{})
	if !ok {
		return false
	}

	// Skip for isolated contexts
	if _, isIsolated := c.(*IsolatedContext); !isIsolated {
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()

		s, err := session.Get("nflow_form", c)
		if err == nil {
			for k, v := range rawPayload {
				s.Values[k] = v
			}
			s.Save(c.Request(), c.Response())
		}
	}

	// Check for break flag
	if breakVal, exists := rawPayload["break"]; exists {
		switch v := breakVal.(type) {
		case bool:
			return v
		case string:
			return v == "true"
		}
	}

	return false
}

// cleanupSession clears the session values
func cleanupSession(c echo.Context) {
	// Skip for isolated contexts
	if _, isIsolated := c.(*IsolatedContext); isIsolated {
		return
	}

	EchoSessionsMutex.Lock()
	defer EchoSessionsMutex.Unlock()

	s, err := session.Get("nflow_form", c)
	if err == nil {
		s.Values = make(map[interface{}]interface{})
		s.Save(c.Request(), c.Response())
	}
}
