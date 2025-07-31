package engine

import (
	"github.com/arturoeanton/nflow-runtime/logger"
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
	logger.Verbosef("DEBUG: Creating controller for %s, address: %p, start address: %p", cc.FlowName, cc, cc.Start)
	if cc.Start != nil && cc.Start.Outputs != nil {
		logger.Verbosef("DEBUG: Start has %d outputs", len(cc.Start.Outputs))

		// DEBUG: Check if the start node has proper connections
		if output1, exists := cc.Start.Outputs["output_1"]; exists && output1 != nil {
			logger.Verbosef("DEBUG: Controller creation - output_1 has %d connections", len(output1.Connections))
			if len(output1.Connections) == 0 {
				logger.Errorf("DEBUG: CONTROLLER CREATED WITH EMPTY CONNECTIONS! Flow: %s", cc.FlowName)
			}
		}
	}

	// Inicializar/actualizar el sistema inmutable con el nuevo controller
	// Esto garantiza que siempre tengamos un snapshot actualizado
	InitializeImmutableWorkflow(cc)

	return &RuntimeController{Controller: cc}
}
