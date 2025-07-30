package model

import (
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

type Playbook map[string]*Node

type Node struct {
	Data    map[string]interface{} `json:"data"`
	Outputs map[string]*Output     `json:"outputs"`
}

type Output struct {
	Connections []struct {
		Node   string `json:"node"`
		Output string `json:"output"`
	} `json:"connections"`
}

type Controller struct {
	Methods  []string
	Start    *Node
	Playbook *Playbook
	FlowName string
	AppName  string
}

type Vars map[string]string

type Runeable interface {
	GetMethods() []string
	Run(c echo.Context, vars Vars, next string, endpoint string, uuid1 string, payload goja.Value) error
}

func (cc *Controller) GetMethods() []string {
	return cc.Methods
}
