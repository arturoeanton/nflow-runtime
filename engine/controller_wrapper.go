package engine

import (
	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

// RuntimeController es un wrapper que hace que model.Controller implemente Runeable
type RuntimeController struct {
	*model.Controller
}

// Run implementa el método requerido por la interfaz Runeable
func (rc *RuntimeController) Run(c echo.Context, vars model.Vars, next string, endpoint string, uuid1 string, payload goja.Value) error {
	// Llamar a la función Run del paquete runtime
	return Run(rc.Controller, c, vars, next, endpoint, uuid1, payload)
}

// GetMethods implementa el método requerido por la interfaz Runeable
func (rc *RuntimeController) GetMethods() []string {
	return rc.Controller.Methods
}

// CreateRuntimeController crea un wrapper para model.Controller
func CreateRuntimeController(cc *model.Controller) model.Runeable {
	return &RuntimeController{Controller: cc}
}
