package engine

import (
	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/process"
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

var Steps map[string]Step = make(map[string]Step)

type Step interface {
	Run(cc *model.Controller, actor *model.Node, c echo.Context, vm *goja.Runtime, connectionNext string, vars model.Vars, currentProcess *process.Process, payload goja.Value) (string, goja.Value, error)
}

// InitializeSteps inicializa los steps por defecto
func init() {
	Steps["gorutine"] = &StepGorutine{}
	Steps["js"] = &StepJS{}
	Steps["dromedary"] = &StepPlugin{}
	Steps["dromedary_callback"] = &StepPluginCallback{}
}
