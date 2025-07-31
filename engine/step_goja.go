package engine

import (
	"log"
	"net/http"
	"sync"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	babel "github.com/jvatic/goja-babel"
	"github.com/labstack/echo/v4"
)

var (
	semVM     chan int
	semVMOnce sync.Once

	// Cache for babel transforms
	babelCache      = make(map[string]string)
	babelCacheMutex sync.RWMutex

	// Cache for compiled programs
	programCache      = make(map[string]*goja.Program)
	programCacheMutex sync.RWMutex
)

type StepJS struct {
}

func (s *StepJS) Run(cc *model.Controller, actor *model.Node, c echo.Context, vm *goja.Runtime, connection_next string, vars model.Vars, currentProcess *process.Process, payload goja.Value) (string, goja.Value, error) {
	ctx := c.Request().Context()
	db, err := GetDB()
	if err != nil {
		log.Println(err)
		return "", payload, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", payload, err
	}
	defer conn.Close()

	currentProcess.State = "run"
	currentProcess.Killeable = true
	code := "function main(){}"

	// Ya no necesitamos mutex porque cada step tiene su propia copia del actor
	actor.Data["storage_id"] = uuid.New().String()

	_, hasCompile := actor.Data["compile"]

	if !hasCompile {
		scriptName, hasScript := actor.Data["script"]

		if hasScript {
			//*** if actor has a script, we load it from the database                       *****
			//*** we can load other places too example: filesystem, mongo,  etc.            *****
			mr := GetRepositoryModules()
			module, ok := mr.GetModuleWithFallback(scriptName.(string), ctx, conn)
			if ok {
				// Transform and cache the module code
				transformed := babelTransform(module.Code)
				actor.Data["compile"] = transformed
			}
		}
		codeData, hasCode := actor.Data["code"]

		if hasCode {
			code = codeData.(string)
			code = babelTransform(code)
			actor.Data["compile"] = code
		}
	}

	if compileCode, ok := actor.Data["compile"]; ok {
		code = compileCode.(string)
	}
	code = code + "\nmain()"

	outputs := make(map[string]string)
	for key, o := range actor.Outputs {
		outputs[key] = o.Connections[0].Node
	}

	vm.Set("payload", payload)
	if payload == nil || payload.Equals(goja.NaN()) || payload.Equals(goja.Null()) || goja.IsUndefined(payload) {
		vm.Set("payload", make(map[string]interface{}))
	}

	vm.Set("next", connection_next)
	// Ya no necesitamos crear una copia porque el actor ya es una copia
	vm.Set("dromedary_data", actor.Data)
	vm.Set("nflow_data", actor.Data)
	vm.Set("__outputs", outputs)
	vm.Set("__flow_name", cc.FlowName)
	vm.Set("__flow_app", cc.AppName)

	// Initialize semaphore with pool size on first use
	semVMOnce.Do(func() {
		config := GetConfig()
		maxSize := config.VMPoolConfig.MaxSize
		if maxSize <= 0 {
			maxSize = 200 // Default to 200 for 4x performance
		}
		semVM = make(chan int, maxSize)
	})

	// Try to get compiled program from cache
	programCacheMutex.RLock()
	program, hasProgram := programCache[code]
	programCacheMutex.RUnlock()

	if !hasProgram {
		// Compile the program
		var err error
		program, err = goja.Compile("workflow", code, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, echo.Map{
				"message": "Script compilation error: " + err.Error(),
				"actor":   actor,
			})
			currentProcess.State = "error"
			return "", payload, err
		}

		// Cache the compiled program
		programCacheMutex.Lock()
		if len(programCache) > 500 { // Limit cache size
			programCache = make(map[string]*goja.Program)
		}
		programCache[code] = program
		programCacheMutex.Unlock()
	}

	err = func() error {
		defer func() {
			err := recover()
			if err != nil {
				log.Println("runJs_00010 ****", err)
			}
		}()
		semVM <- 1
		_, err := vm.RunProgram(program)
		<-semVM
		return err
	}()

	if err != nil {
		// Verificar si es un error de límite de recursos
		statusCode := http.StatusInternalServerError
		errorMessage := err.Error()

		if IsResourceLimitError(err) {
			statusCode = http.StatusRequestTimeout
			errorMessage = "Script execution exceeded resource limits: " + errorMessage
			log.Printf("Resource limit exceeded in workflow: %v", err)
		}

		c.JSON(statusCode, echo.Map{
			"message": errorMessage,
			"actor":   actor,
		})
		currentProcess.State = "error"
		return "", payload, err

	}
	payload = vm.Get("payload")
	currentProcess.Payload = payload.Export()
	connection_next = vm.Get("next").String()
	currentProcess.State = "end"
	if actor.Outputs != nil {
		if actor.Outputs[connection_next] != nil {
			connection_next = actor.Outputs[connection_next].Connections[0].Node
		} else {
			connection_next = ""
		}
	}
	return connection_next, payload, nil
}

func babelTransform(code string) string {
	// Check cache first
	babelCacheMutex.RLock()
	if cached, ok := babelCache[code]; ok {
		babelCacheMutex.RUnlock()
		return cached
	}
	babelCacheMutex.RUnlock()

	// Transform code
	babel.Init(4) // Setup 4 transformers (can be any number > 0)
	res, err := babel.TransformString(
		code,
		map[string]interface{}{
			"plugins": []string{
				"transform-block-scoping",
				"transform-block-scoped-functions",
				"transform-arrow-functions",
				"transform-classes",
				"transform-computed-properties",
				"transform-destructuring",
				"transform-for-of",
				"transform-template-literals",
				"transform-parameters",
				"transform-spread",
				"transform-shorthand-properties",
				"transform-duplicate-keys",
				"transform-object-super",
				"transform-literals",
				"transform-function-name",
				"transform-sticky-regex",
				"transform-typeof-symbol",
				"transform-unicode-regex",
			},
		},
	)
	if err != nil {
		log.Println(code)
		log.Println(err)
		log.Println("babelTransform_00010 ****", err)
		return code // Return original code on error
	}

	// Cache the result
	babelCacheMutex.Lock()
	// Limit cache size to prevent memory issues
	if len(babelCache) > 1000 {
		// Clear cache when it gets too large
		babelCache = make(map[string]string)
	}
	babelCache[code] = res
	babelCacheMutex.Unlock()

	return res
}
