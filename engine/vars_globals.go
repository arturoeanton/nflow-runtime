package engine

import (
	"context"
	"database/sql"
	"net/url"
	"time"

	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// Las variables globales se han movido al ConfigRepository
// Usar GetConfigRepository() para acceder a ellas

func GetDB() (*sql.DB, error) {
	return GetConfigRepository().GetDB()
}

func saveInSession(form url.Values, c echo.Context) {
	// Si el contexto es aislado, no guardar en sesión real
	if _, isIsolated := c.(*IsolatedContext); isIsolated {
		// En contexto aislado, guardar en memoria local
		c.Set("_session_form_data", form)
		return
	}

	syncsession.EchoSessionsMutex.Lock()
	defer syncsession.EchoSessionsMutex.Unlock()
	s, _ := session.Get("nflow_form", c)
	for k, v := range form {
		if k == "nflow_next_node_run" {
			continue
		}
		if len(v) == 1 {
			s.Values[k] = v[0]
			continue
		}
		s.Values[k] = v
	}
	s.Save(c.Request(), c.Response())
}

func AddGlobals(vm *goja.Runtime, c echo.Context) {
	// Set Echo context for HTTP operations using wrapper for proper JS access
	SetupJSContext(vm, c)

	header := make(map[string][]string)
	if c.Request().Header != nil {
		header = (map[string][]string)(c.Request().Header)
	}
	vm.Set("header", header)
	form, err1 := c.FormParams()
	if err1 != nil {
		vm.Set("form", make(map[string][]string))
	} else {
		vm.Set("form", (map[string][]string)(form))
	}

	saveInSession(form, c)

	redisClient := GetRedisClient()
	vm.Set("redis_hset", redisClient.HSet)
	vm.Set("redis_hget", redisClient.HGet)
	vm.Set("redis_hdel", redisClient.HDel)
	vm.Set("redis_expire", func(key string, s int32) {
		redisClient.Expire(key, time.Duration(s)*time.Second)
	})

	//fmt.Println("REDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDIS")
	config := GetConfig()
	vm.Set("config", config)
	vm.Set("env", config.Env)

	vm.Set("url_base", config.URLConfig.URLBase)
	vm.Set("__vm", *vm)
	vm.Set("ctx", context.Background())

}
