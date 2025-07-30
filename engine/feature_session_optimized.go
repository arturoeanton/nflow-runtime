package engine

import (
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/nflow-runtime/literals"
	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

// addFeatureSessionOptimized usa el SessionManager para mejor performance
func AddFeatureSessionOptimized(vm *goja.Runtime, c echo.Context) {
	sm := syncsession.Manager

	vm.Set("set_session", func(name, k, v string) {
		sm.SetValue(name, k, v, c)
	})

	vm.Set("get_session", func(name, k string) string {
		val, _ := sm.GetValue(name, k, c)
		return fmt.Sprint(val)
	})

	vm.Set("open_session", func(name string) *map[string]interface{} {
		s, _ := sm.GetSession(name, c)
		var r = make(map[string]interface{})
		for k, v := range s.Values {
			r[k.(string)] = v
		}
		return &r
	})

	vm.Set("save_session", func(name string, m map[string]interface{}) {
		// Convertir a map[string]interface{} para SetMultipleValues
		values := make(map[string]interface{})
		for k, v := range m {
			values[k] = v
		}
		sm.SetMultipleValues(name, values, c)
	})

	vm.Set("delete_session", func(name string) {
		sm.DeleteSession(name, c)
	})

	vm.Set("delete_session_form", func() {
		sm.DeleteSession("nflow_form", c)
	})

	vm.Set("open_session_form", func() *map[string]interface{} {
		s, _ := sm.GetSession("nflow_form", c)
		var r = make(map[string]interface{})
		for k, v := range s.Values {
			r[k.(string)] = v
		}
		return &r
	})

	vm.Set("set_profile", func(v map[string]string) {
		value, _ := json.Marshal(v)
		sm.SetValue(literals.AUTH_SESSION, "profile", string(value), c)
	})

	vm.Set("get_profile", func() map[string]string {
		return GetProfileOptimized(c)
	})

	vm.Set("exist_profile", func() bool {
		profile, _ := sm.GetValue(literals.AUTH_SESSION, "profile", c)
		if profile != nil {
			var v map[string]string
			if err := json.Unmarshal([]byte(profile.(string)), &v); err == nil {
				return true
			}
		}
		return false
	})

	vm.Set("delete_profile", func() {
		s, _ := sm.GetSession(literals.AUTH_SESSION, c)
		delete(s.Values, "profile")
		sm.SaveSession(literals.AUTH_SESSION, c, s)
	})
}

func GetProfileOptimized(c echo.Context) map[string]string {
	profile, _ := syncsession.Manager.GetValue(literals.AUTH_SESSION, "profile", c)
	var v map[string]string
	if profile != nil {
		if err := json.Unmarshal([]byte(profile.(string)), &v); err == nil {
			return v
		}
	}
	return make(map[string]string, 0)
}
