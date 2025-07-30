package engine

import (
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/nflow-runtime/literals"
	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// withSessionLock executes a function with session mutex if needed
func withSessionLock(c echo.Context, fn func()) {
	// Si es un contexto aislado, no necesita mutex
	if _, isIsolated := c.(*IsolatedContext); !isIsolated {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
	}
	fn()
}

func AddFeatureSession(vm *goja.Runtime, c echo.Context) {
	fmt.Println("[AddFeatureSession] Starting to add session features")
	
	vm.Set("set_session", func(name, k, v string) {
		withSessionLock(c, func() {
			s, _ := session.Get(name, c)
			s.Values[k] = v
			s.Save(c.Request(), c.Response())
		})
	})

	vm.Set("get_session", func(name, k string) string {
		var r string
		withSessionLock(c, func() {
			s, _ := session.Get(name, c)
			r = fmt.Sprint(s.Values[k])
		})
		return r
	})

	vm.Set("open_session", func(name string) *map[string]interface{} {
		var r map[string]interface{}
		withSessionLock(c, func() {
			s, _ := session.Get(name, c)
			r = make(map[string]interface{})
			for k, v := range s.Values {
				r[k.(string)] = v
			}
		})
		return &r
	})

	vm.Set("save_session", func(name string, m map[string]interface{}) {
		withSessionLock(c, func() {
			s, _ := session.Get(name, c)
			for k, v := range m {
				s.Values[k] = v
			}
			s.Save(c.Request(), c.Response())
		})
	})

	vm.Set("delete_session", func(name string) {
		withSessionLock(c, func() {
			s, _ := session.Get(name, c)
			for k := range s.Values {
				delete(s.Values, k)
			}
			s.Save(c.Request(), c.Response())
		})
	})

	vm.Set("delete_session_form", func() {
		withSessionLock(c, func() {
			s, _ := session.Get("nflow_form", c)
			for k := range s.Values {
				delete(s.Values, k)
			}
			s.Save(c.Request(), c.Response())
		})
	})

	vm.Set("open_session_form", func() *map[string]interface{} {
		var r map[string]interface{}
		withSessionLock(c, func() {
			s, _ := session.Get("nflow_form", c)
			r = make(map[string]interface{})
			for k, v := range s.Values {
				r[k.(string)] = v
			}
		})
		return &r
	})

	vm.Set("set_profile", func(v map[string]string) {
		withSessionLock(c, func() {
			s, _ := session.Get(literals.AUTH_SESSION, c)
			value, _ := json.Marshal(v)
			s.Values["profile"] = string(value)
			s.Save(c.Request(), c.Response())
		})
	})

	vm.Set("get_profile", func() map[string]string {
		return GetProfile(c)
	})
	fmt.Println("[AddFeatureSession] get_profile function added")

	vm.Set("exist_profile", func() bool {
		var exists bool
		withSessionLock(c, func() {
			s, _ := session.Get(literals.AUTH_SESSION, c)
			var v map[string]string
			if s.Values["profile"] != nil {
				er := json.Unmarshal([]byte(s.Values["profile"].(string)), &v)
				if er == nil {
					exists = true
				}
			}
		})
		return exists
	})

	vm.Set("delete_profile", func() {
		withSessionLock(c, func() {
			s, _ := session.Get(literals.AUTH_SESSION, c)
			delete(s.Values, "profile")
			s.Save(c.Request(), c.Response())
		})
	})

}

func GetProfile(c echo.Context) map[string]string {
	// Si es un contexto aislado, retornar perfil vacío
	if _, isIsolated := c.(*IsolatedContext); isIsolated {
		if profile, ok := c.Get("_profile").(map[string]string); ok {
			return profile
		}
		return make(map[string]string, 0)
	}
	
	syncsession.EchoSessionsMutex.Lock()
	defer syncsession.EchoSessionsMutex.Unlock()
	s, _ := session.Get(literals.AUTH_SESSION, c)
	var v map[string]string
	if s.Values["profile"] != nil {
		er := json.Unmarshal([]byte(s.Values["profile"].(string)), &v)
		if er == nil {
			return v
		}
	}
	return make(map[string]string, 0)
}
