package plugins

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type GojaPlugin string

var fxsGoja map[string]interface{} = make(map[string]interface{})

func (d GojaPlugin) Run(c echo.Context,
	vars map[string]string, payloadIn interface{}, dromedaryData string,
	callback chan string,
) (payload_out interface{}, next string, err error) {
	return nil, "output_1", nil
}

func init() {
	addFeatureCommon()
	log.Println("Started goja")
}

func (d GojaPlugin) AddFeatureJS() map[string]interface{} {
	return fxsGoja
}

func (d GojaPlugin) Name() string {
	return "goja"
}

func FileToString(name string) string {
	content, err := ioutil.ReadFile(name)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

func StringToFile(filenanme string, content string) error {
	d1 := []byte(content)
	err := ioutil.WriteFile(filenanme, d1, 0644)
	return err
}

func cleanJSON(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func addFeatureCommon() {

	fxsGoja["file_to_string"] = FileToString
	fxsGoja["string_to_file"] = StringToFile

	fxsGoja["find_element"] = func(path string, payload interface{}) interface{} {
		terms := strings.Split(path, ".")
		elem := payload
		for _, v := range terms {
			if array, ok := elem.([]interface{}); ok {
				var i int
				i, _ = strconv.Atoi(v)
				elem = array[i]
				continue
			}
			elem = elem.(map[string]interface{})[v]
		}
		return elem
	}

	fxsGoja["atob"] = func(value string) string {
		if d, err := base64.StdEncoding.DecodeString(value); err == nil {
			return string(d)
		}
		return ""
	}

	fxsGoja["set_env"] = func(key string, value string) {
		os.Setenv(key, value)
	}

	fxsGoja["get_env"] = func(key string) string {
		return os.Getenv(key)
	}

	fxsGoja["sleep"] = func(s int64) {
		time.Sleep(time.Duration(s) * time.Millisecond)
	}

	fxsGoja["url_values_to_map"] = func(s url.Values) map[string][]string {
		return map[string][]string(s)
	}

	fxsGoja["new_map"] = func() map[string]interface{} {
		return make(map[string]interface{})
	}

	fxsGoja["time_now_unix"] = func() int64 {
		return time.Now().Unix()
	}

	fxsGoja["time_now_unix_nano"] = func() int64 {
		return time.Now().UnixNano()
	}

	fxsGoja["time_now_day"] = func() int {
		return time.Now().Day()
	}

	fxsGoja["time_now_month"] = func() int {
		return int(time.Now().Month())
	}

	fxsGoja["time_now_year"] = func() int {
		return time.Now().Year()
	}

	fxsGoja["time_now_hour"] = func() int {
		return time.Now().Hour()
	}

	fxsGoja["time_now_minute"] = func() int {
		return time.Now().Minute()
	}

	fxsGoja["time_now_second"] = func() int {
		return time.Now().Second()
	}

	fxsGoja["time_now_weekday"] = func() int {
		return int(time.Now().Weekday())
	}

	fxsGoja["uuid"] = uuid.New().String

	fxsGoja["clean_json"] = cleanJSON

}
