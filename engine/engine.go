package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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

// formatDuration formatea milisegundos como string sin usar fmt.Sprintf
func formatDuration(ms int64) string {
	return strconv.FormatInt(ms, 10) + "m"
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
				currentProcess.FlagExit = 1
			}

		}(uuid1, p)
	} else {
		p = process.CreateProcess(uuid1)
	}

	defer func() {
		p.SendCallback(`{"error_exit":"exit"}`)
		p.Close()
	}()

	func(uuid1 string) {
		// Solo usar mutex si no es un contexto aislado
		if _, isIsolated := c.(*IsolatedContext); !isIsolated {
			EchoSessionsMutex.Lock()
			defer EchoSessionsMutex.Unlock()
		}
		if c.Response().Header().Get("Nflow-Wid-1") == "" {
			c.Response().Header().Add("Nflow-Wid-1", uuid1)
		}
	}(uuid1)

	// Create a fresh VM for each request
	// This ensures all features are properly initialized
	vm := goja.New()
	
	// Initialize VM with all required modules
	if registry == nil {
		registry = new(require.Registry)
		registry.RegisterNativeModule("console", console.Require)
		registry.RegisterNativeModule("util", util.Require)
	}
	
	registry.Enable(vm)
	console.Enable(vm)
	
	// Add all features
	if syncsession.Manager != nil {
		AddFeatureSessionOptimized(vm, c)
	} else {
		AddFeatureSession(vm, c)
	}
	
	AddFeatureUsers(vm, c)
	AddFeatureToken(vm, c)
	AddFeatureTemplate(vm, c)
	AddGlobals(vm, c)
	
	// Add plugin features
	for _, p := range Plugins {
		for key, fx := range p.AddFeatureJS() {
			vm.Set(key, fx)
		}
	}

	// Set endpoint
	vm.Set("nflow_endpoint", endpoint)

	postData := make(map[string]interface{})
	func() {
		c.Bind(&postData)
		vm.Set("post_data", postData)
	}()

	vm.Set("vars", vars)
	vm.Set("path_vars", vars)

	vm.Set("wid", uuid1)

	vm.Set("wkill", func(wid string) {
		process.WKill(wid)
	})

	pb := *cc.Playbook
	nodeAuth := pb[next]
	if next == "" {
		ActorDataMutex.RLock()
		log.Printf("Start Data: %+v", cc.Start.Data)
		ActorDataMutex.RUnlock()
		if len(cc.Start.Outputs["output_1"].Connections) == 0 {
			c.JSON(http.StatusInternalServerError, echo.Map{"error": "No output connections found for the start node."})
			return nil
		}
		next = cc.Start.Outputs["output_1"].Connections[0].Node
		nodeAuth = cc.Start
	}

	// Exceute auth of default.js?
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
			//execute auth of default.js
			var profile interface{}
			func() {
				// Si es un contexto aislado, no acceder a sesión real
				if _, isIsolated := c.(*IsolatedContext); isIsolated {
					profile = nil
					return
				}
				
				EchoSessionsMutex.Lock()
				defer EchoSessionsMutex.Unlock()
				
				auth_session, err := session.Get("auth-session", c)
				if err != nil {
					log.Println(err)
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

			ctx := c.Request().Context()
			db, err := GetDB()
			if err != nil {
				log.Println(err)
				return nil
			}
			conn, err := db.Conn(ctx)
			if err != nil {
				c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
				return nil
			}
			defer conn.Close()
			row := conn.QueryRowContext(ctx, Config.DatabaseNflow.QueryGetApp, "app")
			var code string
			var jsonCode string
			err = row.Scan(&jsonCode, &code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
				return nil
			}

			code += "\nauth()"
			_, err = vm.RunString(code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
				return nil
			}

			next = vm.Get("next").String()
			log.Println("Next node:", next)
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

	func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println(err)
			}
		}()

		// Si es un contexto aislado, generar valores por defecto
		if _, isIsolated := c.(*IsolatedContext); isIsolated {
			logId = uuid.New().String()
			orderBox = 1
			return
		}

		// Para contextos normales, usar mutex
		EchoSessionsMutex.Lock()
		defer EchoSessionsMutex.Unlock()

		log_session, err := session.Get("log-session", c)
		if err != nil {
			log.Println(err)
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
					log.Println(err)
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
			if Config.DatabaseNflow.QueryInsertLog == "" {
				return
			}

			db, err := GetDB()
			if err != nil {
				log.Println(err)
				return
			}
			ctx := context.Background()
			conn, err := db.Conn(ctx)
			if err != nil {
				return
			}
			defer conn.Close()

			_, err = conn.ExecContext(ctx, Config.DatabaseNflow.QueryInsertLog,
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
				log.Println(err)
			}

		}(logId, boxId, boxName, boxType, connectionNext, username, ip, realip, url, userAgent, queryParam, hostname, host, diff, orderBox, jsonPayload)

		// Get profile before goroutine to avoid race
		logProfile := GetProfile(c)
		
		go func(logProfile map[string]string, actor *model.Node, boxId string, boxName string, boxType string, connectionNext string, diff time.Duration) {

			defer func() {
				if err := recover(); err != nil {
					log.Println(err)
				}
			}()

			//log.Printf("%s - time: %v", sbLog.String(), diff)
			ctx := context.Background()
			db, err := GetDB()
			if err != nil {
				log.Println(err)
				return
			}
			conn, err := db.Conn(ctx)
			if err != nil {
				return
			}
			defer conn.Close()
			row := conn.QueryRowContext(ctx, Config.DatabaseNflow.QueryGetApp, "app")
			var code string
			var jsonCode string
			err = row.Scan(&jsonCode, &code)
			if err != nil {
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
				log.Println(err)
			}

		}(logProfile, actor, boxId, boxName, boxType, connectionNext, diff)

	}()
	defer func() {
		err := recover()
		if err != nil {
			log.Println(err)
			log.Println("step_00010 ****", err)
		}
	}()

	if currentProcess.FlagExit == 1 {
		currentProcess.Close()
		panic("FlagExit")
	}
	pb := *cc.Playbook
	originalActor := pb[next]
	
	// Crear una copia profunda del actor para evitar race conditions
	actor, err := originalActor.DeepCopy()
	if err != nil {
		log.Printf("Error creating actor copy: %v", err)
		// Si falla la copia, usar el original con mutex
		actor = originalActor
	}
	
	sbLog.WriteString("- IDBox:" + next)
	currentProcess.UUIDBoxCurrent = next
	boxId = next

	// Ya no necesitamos mutex si tenemos una copia
	if nameBox, ok := actor.Data["name_box"]; ok {
		boxName = nameBox.(string)
		sbLog.WriteString("- NameBox:" + boxName)
	}

	currentProcess.Type = ""
	if pType, ok := actor.Data["type"]; ok {
		currentProcess.Type = pType.(string)
	}
	boxType = currentProcess.Type

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

func Execute(cc *model.Controller, c echo.Context, vm *goja.Runtime, next string, vars model.Vars, currentProcess *process.Process, payload goja.Value, fork bool) {
	var err error
	prevBox := ""
	if fork {
		log.Println("fork")
	}
	for next != "" {

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
					log.Println(err)
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
			log.Println("fork")
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
						log.Println(err)
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
				log.Println(err)
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
