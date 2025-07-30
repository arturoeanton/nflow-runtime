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

func AddFeatureSession(vm *goja.Runtime, c echo.Context) {
	fmt.Println("[AddFeatureSession] Starting to add session features")
	
	vm.Set("set_session", func(name, k, v string) {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		s.Values[k] = v
		s.Save(c.Request(), c.Response())
	})

	vm.Set("get_session", func(name, k string) string {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		r := fmt.Sprint(s.Values[k])
		return r
	})

	vm.Set("open_session", func(name string) *map[string]interface{} {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		var r = make(map[string]interface{})
		for k, v := range s.Values {
			r[k.(string)] = v
		}
		return &r
	})

	vm.Set("save_session", func(name string, m map[string]interface{}) {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		for k, v := range m {
			s.Values[k] = v
		}
		s.Save(c.Request(), c.Response())
	})

	vm.Set("delete_session", func(name string) {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(name, c)
		for k := range s.Values {
			delete(s.Values, k)
		}
		s.Save(c.Request(), c.Response())
	})

	vm.Set("delete_session_form", func() {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get("nflow_form", c)
		for k := range s.Values {
			delete(s.Values, k)
		}
		s.Save(c.Request(), c.Response())
	})

	vm.Set("open_session_form", func() *map[string]interface{} {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get("nflow_form", c)
		var r = make(map[string]interface{})
		for k, v := range s.Values {
			r[k.(string)] = v
		}
		return &r
	})

	vm.Set("set_profile", func(v map[string]string) {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(literals.AUTH_SESSION, c)
		value, _ := json.Marshal(v)
		s.Values["profile"] = string(value)
		s.Save(c.Request(), c.Response())
	})

	vm.Set("get_profile", func() map[string]string {
		return GetProfile(c)
	})
	fmt.Println("[AddFeatureSession] get_profile function added")

	vm.Set("exist_profile", func() bool {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(literals.AUTH_SESSION, c)
		var v map[string]string
		if s.Values["profile"] != nil {
			er := json.Unmarshal([]byte(s.Values["profile"].(string)), &v)
			if er == nil {
				return true
			}
		}
		return false
	})

	vm.Set("delete_profile", func() {
		syncsession.EchoSessionsMutex.Lock()
		defer syncsession.EchoSessionsMutex.Unlock()
		s, _ := session.Get(literals.AUTH_SESSION, c)
		delete(s.Values, "profile")
		s.Save(c.Request(), c.Response())
	})

}

func GetProfile(c echo.Context) map[string]string {
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
