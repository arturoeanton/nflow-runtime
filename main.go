package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/arturoeanton/gocommons/utils"
	"github.com/arturoeanton/nflow-runtime/commons"
	"github.com/arturoeanton/nflow-runtime/engine"
	"github.com/arturoeanton/nflow-runtime/literals"
	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	_ "github.com/arturoeanton/nflow-runtime/engine"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var playbooks map[string]map[string]map[string]*model.Playbook = make(map[string]map[string]map[string]*model.Playbook)

func CheckError(c echo.Context, err error, code int) bool {
	if err != nil {
		c.JSON(code, echo.Map{
			"message": err.Error(),
			"code":    code,
		})
		return true
	}
	return false
}

func run(c echo.Context) error {
	ctx := c.Request().Context()
	db, err := engine.GetDB()
	if err != nil {
		log.Println(err)
		c.HTML(http.StatusNotFound, literals.NOT_FOUND)
		return nil
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		log.Println(err)
		c.HTML(http.StatusNotFound, literals.NOT_FOUND)
		return nil
	}
	defer conn.Close()

	appJson := "app"

	v, ok := engine.FindNewApp[appJson]
	if v || !ok {
		var err error
		playbooks[appJson], err = engine.GetPlaybook(ctx, conn, appJson)
		if CheckError(c, err, 500) {
			return nil
		}
		engine.FindNewApp[appJson] = false
	}

	endpoint := strings.Split(c.Request().RequestURI, "?")[0]
	nflowNextNodeRun := ""
	endpointParts := strings.Split(endpoint, "/")
	lenEndpointParts := len(endpointParts)
	positionTagNflowID := -1
	positionTagNflowTK := -1
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < (lenEndpointParts - 1); i++ {
			if endpointParts[i] == literals.FORMNFLOWID {
				nflowNextNodeRun = endpointParts[i+1]
				positionTagNflowID = i
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < (lenEndpointParts - 1); i++ {
			if endpointParts[i] == literals.FORMNFLOWTK {
				positionTagNflowTK = i
				break
			}
		}
	}()

	wg.Wait()

	if positionTagNflowTK > positionTagNflowID && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	} else if positionTagNflowTK == -1 && positionTagNflowID > -1 {
		positionTagNflowTK = positionTagNflowID
	}

	if positionTagNflowTK > -1 {
		endpoint = strings.Join(endpointParts[:positionTagNflowTK], "/")
		if nflowNextNodeRun == "" {
			if c.Request().Method == "POST" || c.Request().Method == "PUT" {
				if c.Request().FormValue("nflow_next_node_run") != "" {
					if c.Request().Form["nflow_next_node_run"] != nil {
						nflowNextNodeRun = c.Request().Form["nflow_next_node_run"][0]
					}
				}
			} else if c.Request().Method == "GET" {
				if c.Request().URL.Query().Get("nflow_next_node_run") != "" {
					nflowNextNodeRun = c.Request().URL.Query().Get("nflow_next_node_run")
				}
			}
		}
	} else {

		if c.Request().Method == "POST" || c.Request().Method == "PUT" {
			if c.Request().FormValue("nflow_next_node_run") != "" {
				if c.Request().Form["nflow_next_node_run"] != nil {
					nflowNextNodeRun = c.Request().Form["nflow_next_node_run"][0]
				}
			}
		}
	}

	if nflowNextNodeRun == "" {
		func() {
			syncsession.EchoSessionsMutex.Lock()
			defer syncsession.EchoSessionsMutex.Unlock()
			s, _ := session.Get("nflow_form", c)
			s.Values = make(map[interface{}]interface{})
			s.Save(c.Request(), c.Response())
		}()
	}

	runeable, vars, code, _, err := engine.GetWorkflow(c, playbooks[appJson], endpoint, c.Request().Method, appJson)
	if CheckError(c, err, code) {
		return nil
	}

	log.Println("Run endpoint:", endpoint, "nflowNextNodeRun:", runeable)

	uuid1 := uuid.New().String()
	e := runeable.Run(c, vars, nflowNextNodeRun, endpoint, uuid1, nil)
	return e
}

func main() {
	flag.Parse()
	configPath := "config.toml"
	if utils.Exists(configPath) {
		data, _ := utils.FileToString(configPath)
		if _, err := toml.Decode(data, &engine.Config); err != nil {
			log.Println(err)
		}
	}

	engine.RedisClient = redis.NewClient(&redis.Options{
		Addr:     engine.Config.RedisConfig.Host,
		Password: engine.Config.RedisConfig.Password, // no password set
		DB:       0,                                  // use default DB
	})
	engine.UpdateQueries()

	engine.LoadPlugins()

	// Crear servidor Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(session.Middleware(commons.GetSessionStore(&engine.Config.PgSessionConfig)))

	e.Any("/*", run)

	// Iniciar servidor
	log.Println("Starting nFlow Runtime Example on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
