package plugins

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
	"github.com/sazito/mosalat"
)

type RulePlugin string

var (
	fxsRule map[string]interface{} = make(map[string]interface{})
	funcMap map[string]interface{} = make(map[string]interface{})
)

func (d RulePlugin) Run(c echo.Context,
	vars map[string]string, payloadIn interface{}, RulePluginData string,
	callback chan string,
) (payload_out interface{}, next string, err error) {
	return nil, "output_1", nil
}

func init() {
	addFeatureRuleVM()
}

func (d RulePlugin) AddFeatureJS() map[string]interface{} {
	return fxsRule
}

func (d RulePlugin) Name() string {
	return "rules"
}

func addFeatureRuleVM() {
	fxsRule["rule_add_fx"] = func(name string, fx goja.Value, vm *goja.Runtime) {
		funcMap[name] = func(params ...interface{}) interface{} {
			code := "var " + name + "=" + fx.String() + "\n" + name + "(" + fmt.Sprint(params) + ")"
			vv, _ := vm.RunString(code)
			return vv
		}
	}

	fxsRule["rule_run"] = func(inputMap, outputMap map[string]interface{}, rules ...string) map[string]interface{} {
		output, err := mosalat.Run(rules, funcMap, inputMap, outputMap) // --> [plan_name: "free", feature_1: true]
		return map[string]interface{}{"output": output, "err": err}
	}

}
