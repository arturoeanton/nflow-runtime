package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/arturoeanton/nflow-runtime/model"
	"github.com/arturoeanton/nflow-runtime/syncsession"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

var (
	// playbookRepo es la instancia global del repository
	playbookRepo PlaybookRepository
)

// InitializePlaybookRepository inicializa el repository con la base de datos
func InitializePlaybookRepository(db *sql.DB) {
	playbookRepo = NewPlaybookRepository(db)
}

// GetPlaybookRepository retorna el repository actual
func GetPlaybookRepository() PlaybookRepository {
	return playbookRepo
}

func GetPlaybook(ctx context.Context, conn *sql.Conn, pbName string) (map[string]map[string]*model.Playbook, error) {
	config := GetConfig()
	rows, err := conn.QueryContext(ctx, config.DatabaseNflow.QueryGetApp, pbName)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()
	var flowJson string
	var defaultJs string
	for rows.Next() {
		err := rows.Scan(&flowJson, &defaultJs)
		if err != nil {
			log.Println(err)
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		log.Println(err)
		return nil, err
	}

	data := make(map[string]map[string]map[string]*model.Playbook)
	err = json.Unmarshal([]byte(flowJson), &data)
	if err != nil {
		return nil, err
	}

	return data["drawflow"], nil
}

func comparePath(template string, real string) (bool, model.Vars) {
	termsOfTemplate := strings.Split(template, "/")
	termsOfReal := strings.Split(real, "/")
	vars := make(model.Vars)
	if len(termsOfTemplate) != len(termsOfReal) {
		return false, nil
	}
	for i, tt := range termsOfTemplate {
		if tt == "" {
			continue
		}
		tr := termsOfReal[i]
		if tt[0] == ':' {
			vars[tt[1:]] = tr
			continue
		}
		if tt != tr {
			return false, nil
		}
	}
	return true, vars
}

func GetWorkflow(c echo.Context, playbooks map[string]map[string]*model.Playbook, wfPath string, method string, appName string) (model.Runeable, model.Vars, int, string, error) {

	for key, flows := range playbooks {
		for _, pb := range flows {
			for _, item := range *pb {
				data := item.Data
				typeItem := data["type"].(string)

				if typeItem == "starter" {

					methodItem := data["method"]
					if methodItem != "ANY" {
						if methodItem != method {
							continue
						}
					}
					urlpattern := data["urlpattern"].(string)
					flag, vars := comparePath(urlpattern, wfPath)
					if flag {
						if method == "GET" {
							if reset_order_box, ok := data["reset_order_box"]; ok {
								if reset_order_box == "true" {
									if typeItem == "starter" {
										// Check if is a new session and reset order_box log-session
										func() {
											syncsession.EchoSessionsMutex.Lock()
											defer syncsession.EchoSessionsMutex.Unlock()
											log_session, err := session.Get("log-session", c)
											if err != nil {
												log.Println(err)
											}
											log_session.Values["order_box"] = 0
											log_session.Save(c.Request(), c.Response())
										}()
									}
								}
							}
						}

						c := &model.Controller{
							Methods:  []string{method},
							Start:    item,
							Playbook: pb,
							FlowName: key,
							AppName:  appName,
						}

						// Crear wrapper de runtime para que implemente Runeable
						return CreateRuntimeController(c), vars, http.StatusOK, typeItem, nil
					}
				}
			}
		}
	}

	/*
		for key, c := range pb.Controllers {
			flag, vars := comparePath(key, wfPath)

			if !model.ContainsString(c.GetMethods(), method) {
				continue
			}

			if flag {
				return Runeable(&c), vars, nil, http.StatusOK
			}
		}
	*/
	return nil, nil, http.StatusNotFound, "", errors.New("not found")
}
