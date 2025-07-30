package engine

import (
	"context"
	"database/sql"
	"net/url"
	"time"

	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/dop251/goja"
	"github.com/go-redis/redis"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

var (
	RedisClient *redis.Client
	db          *sql.DB
)

func GetDB() (*sql.DB, error) {
	if db == nil {
		var err error
		db, err = sql.Open(Config.DatabaseNflow.Driver, Config.DatabaseNflow.DSN)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func saveInSession(form url.Values, c echo.Context) {
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

	vm.Set("redis_hset", RedisClient.HSet)
	vm.Set("redis_hget", RedisClient.HGet)
	vm.Set("redis_hdel", RedisClient.HDel)
	vm.Set("redis_expire", func(key string, s int32) {
		RedisClient.Expire(key, time.Duration(s)*time.Second)
	})

	//fmt.Println("REDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDISREDIS")
	vm.Set("config", Config)
	vm.Set("env", Config.Env)

	vm.Set("url_base", Config.URLConfig.URLBase)
	vm.Set("__vm", *vm)
	vm.Set("ctx", context.Background())

}
