package engine

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type StepPluginCallback struct {
}

func (s *StepPluginCallback) Run(cc *model.Controller, actor *model.Node, c echo.Context, vm *goja.Runtime, connection_next string, vars model.Vars, currentProcess *process.Process, payload goja.Value) (string, goja.Value, error) {
	currentProcess.State = "run"
	time.Sleep(1 * time.Second)
	
	// Ya no necesitamos mutex porque el actor es una copia
	name := actor.Data["dromedary_name"].(string)
	
	dataJs, _ := json.Marshal(actor.Data)

	output := "output_2"
	if len(actor.Outputs) == 1 {
		output = "output_1"
	}
	if actor.Outputs[output] != nil {
		next2 := actor.Outputs[output].Connections[0].Node
		//processFather := process
		go func() {
			uuid2 := uuid.New().String()
			secondProcess := process.CreateProcessWithCallback(uuid2)
			defer func() {
				secondProcess.Close()
			}()
			dromedary := Plugins[name]
			go dromedary.Run(c, vars, &payload, string(dataJs), secondProcess.Callback)
			for {
				data := <-secondProcess.Callback
				var p map[string]interface{}
				json.Unmarshal([]byte(data), &p)
				payload = vm.ToValue(p)
				if _, ok := p["next"]; ok {
					next2 = actor.Outputs[p["next"].(string)].Connections[0].Node
				}
				if _, ok := p["error_exit"]; ok {
					break
				}
				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()
					Execute(cc, c, vm, next2, vars, secondProcess, payload, false)
				}()
				wg.Wait()
			}
		}()
	}

	if len(actor.Outputs) > 1 {
		connection_next = actor.Outputs[connection_next].Connections[0].Node
	}
	return connection_next, payload, nil
}
