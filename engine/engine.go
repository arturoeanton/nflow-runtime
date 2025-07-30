package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"sync"
	"time"

	"github.com/arturoeanton/gocommons/utils"
	"github.com/arturoeanton/nflow-runtime/logger"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/arturoeanton/nflow-runtime/syncsession"

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
	registry *require.Registry
	wg       sync.WaitGroup = sync.WaitGroup{}
)

// GetRequireRegistry retorna el registry de require
func GetRequireRegistry() *require.Registry {
	return registry
}

func init() {
	// Initialize registry globally
	registry = new(require.Registry)
	registry.RegisterNativeModule("console", console.Require)
	registry.RegisterNativeModule("util", util.Require)
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

	// Set workflow ID header for tracking. This anonymous function handles
	// concurrency differently based on context type to avoid race conditions.
	// IsolatedContext doesn't need mutex protection as it doesn't share state.
	func(uuid1 string) {
		// Only use mutex if not an isolated context
		if _, isIsolated := c.(*IsolatedContext); !isIsolated {
			EchoSessionsMutex.Lock()
			defer EchoSessionsMutex.Unlock()
		}
		if c.Response().Header().Get("Nflow-Wid-1") == "" {
			c.Response().Header().Add("Nflow-Wid-1", uuid1)
		}
	}(uuid1)

	// Create a fresh VM for each request with security limits
	// This ensures all features are properly initialized
	limits := GetLimitsFromConfig()
	sandboxConfig := GetSandboxConfigFromConfig()

	vm, tracker, err := CreateSecureVM(limits, sandboxConfig)
	if err != nil {
		logger.Errorf("Error creating secure VM: %v", err)
		c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to create execution environment"})
		return nil
	}
	defer tracker.Stop()

	// Initialize VM with all required modules. The registry is a singleton
	// that provides Node.js-compatible modules to the JavaScript environment.
	// This includes console and util modules for basic functionality.
	if registry == nil {
		registry = new(require.Registry)
		registry.RegisterNativeModule("console", console.Require)
		registry.RegisterNativeModule("util", util.Require)
	}

	registry.Enable(vm)
	// We don't need console.Enable because the sandbox provides its own secure version

	// Add all features to the VM. These features provide various APIs
	// that workflows can use. The optimized session version uses the
	// syncsession.Manager for better performance and thread safety.
	if syncsession.Manager != nil {
		AddFeatureSessionOptimized(vm, c)
	} else {
		AddFeatureSession(vm, c)
	}

	AddFeatureUsers(vm, c)
	AddFeatureToken(vm, c)
	AddFeatureTemplate(vm, c)
	AddGlobals(vm, c)

	// Add plugin features to the VM. Plugins can extend the JavaScript
	// environment with custom functions and objects. Each plugin returns
	// a map of feature names to their implementations.
	for _, p := range Plugins {
		for key, fx := range p.AddFeatureJS() {
			vm.Set(key, fx)
		}
	}

	// Set endpoint and request data in the VM. These global variables
	// are accessible to all JavaScript code in the workflow.
	vm.Set("nflow_endpoint", endpoint)

	// Parse and expose POST data to the workflow
	postData := make(map[string]interface{})
	func() {
		c.Bind(&postData)
		vm.Set("post_data", postData)
	}()

	// Set path variables extracted from the URL
	vm.Set("vars", vars)
	vm.Set("path_vars", vars)

	// Set workflow instance ID for tracking
	vm.Set("wid", uuid1)

	// Provide workflow kill function to allow workflows to terminate other workflows
	vm.Set("wkill", func(wid string) {
		process.WKill(wid)
	})

	// Get the playbook and determine the starting node. If no specific node
	// is requested (next == ""), start from the beginning of the workflow.
	// The playbook is a map of node IDs to their configurations.
	pb := *cc.Playbook
	nodeAuth := pb[next]
	if next == "" {
		ActorDataMutex.RLock()
		logger.Verbosef("Start Data: %+v", cc.Start.Data)
		ActorDataMutex.RUnlock()
		if len(cc.Start.Outputs["output_1"].Connections) == 0 {
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "No output connections found for the start node."})
			return nil
		}
		next = cc.Start.Outputs["output_1"].Connections[0].Node
		nodeAuth = cc.Start
	}

	// Check if authentication is required for this node. The nflow_auth flag
	// in node data determines if authentication should be enforced.
	ActorDataMutex.RLock()
	flag, hasAuthFlag := nodeAuth.Data["nflow_auth"]
	ActorDataMutex.RUnlock()

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
			// Execute authentication from default.js. This retrieves the user's
			// profile from the session and makes it available to the workflow.
			var profile interface{}
			func() {
				// If it's an isolated context, don't access real session
				if _, isIsolated := c.(*IsolatedContext); isIsolated {
					profile = nil
					return
				}

				EchoSessionsMutex.Lock()
				defer EchoSessionsMutex.Unlock()

				auth_session, err := session.Get("auth-session", c)
				if err != nil {
					logger.Error("Error in start data:", err)
					profile = nil
					return
				}

				auth_session.Values["redirect_url"] = c.Request().URL.Path
				auth_session.Save(c.Request(), c.Response())

				profile = auth_session.Values["profile"]
			}()
			vm.Set("profile", profile)
			vm.Set("next", next)
			vm.Set("auth_flag", flagString)
			vm.Set("url_access", c.Request().URL.Path)

			code := ""
			triggersFold := os.Getenv("NFLOE_TRIGGERS_FOLD")
			if triggersFold == "" {
				triggersFold = "triggers/"
			}
			if utils.Exists(triggersFold + "auth.js") {
				data, err := utils.FileToString(triggersFold + "auth.js")
				if err != nil {
					logger.Error("Error reading auth.js:", err)
					return nil
				}
				if data == "" {
					logger.Error("auth.js is empty")
					return nil
				}
				code += data
			}

			code += "\nauth()"
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
		now := time.Now()
		diff := now.Sub(t1)

		// Extract data from context before goroutine to avoid race conditions
		var username, ip, realip, url, userAgent, queryParam, hostname, host string
		var jsonPayload []byte

		// Get profile with proper locking
		profile := GetProfile(c)
		if profile != nil {
			if _, ok := profile["username"]; ok {
				username = profile["username"]
			}
		}

		// Marshal payload with locking
		var err error
		func() {
			PayloadSessionMutex.Lock()
			defer PayloadSessionMutex.Unlock()
			jsonPayload, err = json.Marshal(payload.Export())
		}()
		if err != nil {
			jsonPayload = []byte("{}")
		}

		// Extract request info
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Error in start data:", err)
				}
			}()
			ip = c.Request().RemoteAddr
			realip = c.RealIP()
			url = c.Request().URL.RawPath
			userAgent = c.Request().UserAgent()
			queryParam = c.Request().URL.Query().Encode()
			hostname = c.Request().URL.Hostname()
			host = c.Request().Host
		}()

		go func(logId, boxId, boxName, boxType, connectionNext, username, ip, realip, url, userAgent, queryParam, hostname, host string, diff time.Duration, orderBox int, jsonPayload []byte) {
			config := GetConfig()
			if config.DatabaseNflow.QueryInsertLog == "" {
				return
			}

			db, err := GetDB()
			if err != nil {
				logger.Error("Error processing node:", err)
				return
			}
			ctx := context.Background()
			conn, err := db.Conn(ctx)
			if err != nil {
				return
			}
			defer conn.Close()

			_, err = conn.ExecContext(ctx, config.DatabaseNflow.QueryInsertLog,
				logId,                                   // $1
				boxId,                                   // $2
				boxName,                                 // $3
				boxType,                                 // $4
				url,                                     // $5
				username,                                // $6
				connectionNext,                          // $7
				fmt.Sprintf("%dm", diff.Milliseconds()), // $8
				orderBox,                                // $9
				string(jsonPayload),                     // $10
				ip,                                      // $11
				realip,                                  // $12
				userAgent,                               // $13
				queryParam,                              // $14
				hostname,                                // $15
				host,                                    // $16

			)
			if err != nil {
				logger.Error("Error processing node:", err)
			}

		}(logId, boxId, boxName, boxType, connectionNext, username, ip, realip, url, userAgent, queryParam, hostname, host, diff, orderBox, jsonPayload)

		// Get profile before goroutine to avoid race
		logProfile := GetProfile(c)

		go func(logProfile map[string]string, actor *model.Node, boxId string, boxName string, boxType string, connectionNext string, diff time.Duration) {

			defer func() {
				if err := recover(); err != nil {
					logger.Error("Error in start data:", err)
				}
			}()

			//log.Printf("%s - time: %v", sbLog.String(), diff)
			code := ""
			triggersFold := os.Getenv("NFLOE_TRIGGERS_FOLD")
			if triggersFold == "" {
				triggersFold = "triggers/"
			}
			if utils.Exists(triggersFold + "log.js") {
				data, err := utils.FileToString(triggersFold + "log.js")
				if err != nil {
					logger.Error("Error reading log.js:", err)
					return
				}
				if data == "" {
					logger.Error("log.js is empty")
					return
				}
				code += data
			} else {
				logger.Info("log.js not found, using default logging")
				return
			}

			// Crear una VM separada para logging para evitar race conditions
			logVM := goja.New()

			// Inicializar la VM de logging con los módulos necesarios
			if registry != nil {
				registry.Enable(logVM)
			}
			console.Enable(logVM)

			// Add minimal required functions for logging with captured profile
			logVM.Set("get_profile", func() interface{} {
				return logProfile
			})

			logVM.Set("box_id", boxId)
			logVM.Set("box_name", boxName)
			logVM.Set("box_type", boxType)
			logVM.Set("connection_next", connectionNext)

			logVM.Set("duration_mc", diff.Microseconds())
			logVM.Set("duration_ms", diff.Milliseconds())
			logVM.Set("duration_s", diff.Seconds())

			code += "\nlog()"
			_, err = logVM.RunString(code)
			if err != nil {
				logger.Error("Error processing node:", err)
			}

		}(logProfile, actor, boxId, boxName, boxType, connectionNext, diff)

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
	pb := *cc.Playbook
	originalActor := pb[next]

	// Crear una copia profunda del actor para evitar race conditions
	actor, err := originalActor.DeepCopy()
	if err != nil {
		logger.Errorf("Error creating actor copy: %v", err)
		// Si falla la copia, usar el original con mutex
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
