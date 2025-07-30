package engine

import (
	"context"
	"crypto/subtle"
	"log"
	"time"

	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

func GetTokenFromDB(paramName string) []map[string]interface{} {
	db, err := GetDB()
	if err != nil {
		log.Println(err)
		return nil
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		log.Println(err)
		return nil
	}
	defer conn.Close()
	config := GetConfig()
	rows, err := conn.QueryContext(context.Background(), config.DatabaseNflow.QueryGetToken, paramName)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()
	var id int
	var name string
	var token string
	var start interface{}
	var expired interface{}
	var active bool
	var header string
	var tokenType string
	ret := make([]map[string]interface{}, 0)
	found := false
	for rows.Next() {
		found = true
		err := rows.Scan(&id, &name, &token, &start, &expired, &active, &header, &tokenType)
		if err != nil {
			log.Println(err)
			return nil
		}
		mapUser := make(map[string]interface{})
		mapUser["id"] = id
		mapUser["name"] = name
		mapUser["token"] = token
		mapUser["start"] = start
		mapUser["expired"] = expired
		mapUser["active"] = active
		mapUser["header"] = header
		mapUser["tokenType"] = tokenType
		ret = append(ret, mapUser)
	}
	if err := rows.Err(); err != nil {
		log.Println(err)
		return nil
	}
	if !found {
		return nil
	}

	return ret
}

func ValidateTokenDB(c echo.Context, name string) bool {
	arrayToken := GetTokenFromDB(name)
	if arrayToken == nil {
		return false
	}
	for _, tokenMap := range arrayToken {
		if !tokenMap["active"].(bool) {
			continue
		}
		token := ""
		id := tokenMap["id"].(int)
		if tokenMap["header"] != nil {
			keyHeader := tokenMap["header"].(string)
			if c.Request().Header.Get(keyHeader) == "" {
				log.Printf("header not found in register [%d]-[%s] database", id, name)
				continue
			}
			token = c.Request().Header.Get(keyHeader)
		}

		if tokenMap["expired"] != nil {
			expiredTime := int64(tokenMap["expired"].(int64))
			if time.Now().Unix() > expiredTime {
				log.Printf("El token [%d]-[%s] ha expirado.", id, name)
				continue
			}
		}

		if subtle.ConstantTimeCompare([]byte(tokenMap["tokenType"].(string)+" "+tokenMap["token"].(string)), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

func AddFeatureToken(vm *goja.Runtime, c echo.Context) {

	vm.Set("validate_token", func(name string) bool {
		return ValidateTokenDB(c, name)
	})

	vm.Set("get_token", func(name string) []map[string]interface{} {
		ret := GetTokenFromDB(name)
		if ret == nil {
			return nil
		}
		return ret
	})

}
