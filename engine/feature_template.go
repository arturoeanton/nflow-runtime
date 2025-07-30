package engine

import (
	"context"
	"log"

	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

func GetTemplateFromDB(paramName string) string {
	db, err := GetDB()
	if err != nil {
		log.Println(err)
		return ""
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		log.Println(err)
		return ""
	}
	defer conn.Close()
	row := conn.QueryRowContext(context.Background(), Config.DatabaseNflow.QueryGetTemplate, paramName)

	var id int
	var name string
	var content string

	err = row.Scan(&id, &name, &content)
	if err != nil {
		log.Println(err)
		return ""
	}

	return content
}

func AddFeatureTemplate(vm *goja.Runtime, c echo.Context) {

	vm.Set("get_template", func(paramName string) string {
		return GetTemplateFromDB(paramName)
	})

}
