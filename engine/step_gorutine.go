package engine

import (
	"encoding/json"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type StepGorutine struct {
}

func CloneValue(value goja.Value, vm *goja.Runtime) goja.Value {
	runtime := goja.New()
	// Serializar el valor a JSON

	if value == nil {
		return runtime.ToValue(nil)
	}

	jsonBytes, err := json.Marshal(value.Export())
	if err != nil {
		panic(err)
	}

	// Deserializar el JSON para clonar el valor
	var clonedValue interface{}
	err = json.Unmarshal(jsonBytes, &clonedValue)
	if err != nil {
		panic(err)
	}

	// Convertir el valor clonado a goja.Value
	return runtime.ToValue(clonedValue)
}

func (s *StepGorutine) Run(cc *model.Controller, actor *model.Node, c echo.Context, vm *goja.Runtime, connectionNext string, vars model.Vars, currentProcess *process.Process, payload goja.Value) (string, goja.Value, error) {
	currentProcess.State = "run"
	payloadClone1 := CloneValue(payload, vm)
	payloadClone2 := CloneValue(payload, vm)
	if actor.Outputs["output_2"] != nil {
		next2 := actor.Outputs["output_2"].Connections[0].Node
		uuid2 := uuid.New().String()
		c.Response().Header().Add("Dromedary-Wid-2", uuid2)
		// fmt.Println("gorutine")
		// fmt.Printf("%+v\n", payloadClone1.Export())
		go RunWithCallback(cc, c, vars, next2, "go_rutine_"+uuid2, uuid2, payloadClone1)
	}
	connectionNext = actor.Outputs[connectionNext].Connections[0].Node
	currentProcess.State = "end"

	// fmt.Println("gorutine2")
	// fmt.Printf("%+v\n", payloadClone2.Export())
	return connectionNext, payloadClone2, nil
}
