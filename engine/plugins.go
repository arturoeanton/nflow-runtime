package engine

import (
	"log"

	"github.com/arturoeanton/nflow-runtime/plugins"
	"github.com/labstack/echo/v4"
)

type NflowPlugin interface {
	Run(c echo.Context, vars map[string]string, payloadIn interface{}, dromedary_data string, callback chan string) (payload_out interface{}, next string, err error)
	Name() string
	AddFeatureJS() map[string]interface{}
}

var Plugins map[string]NflowPlugin

func LoadPlugins() {
	Plugins = make(map[string]NflowPlugin)

	pluing1 := plugins.ClientHTTP("client_http")
	Plugins[pluing1.Name()] = pluing1

	pluing2 := plugins.GojaPlugin("goja")
	Plugins[pluing2.Name()] = pluing2

	pluing3 := plugins.TemplatePluings("template")
	Plugins[pluing3.Name()] = pluing3

	pluing4 := plugins.MailPlugin("mail")
	Plugins[pluing4.Name()] = pluing4

	pluing5 := plugins.RulePlugin("rule")
	Plugins[pluing5.Name()] = pluing5

	pluing6 := plugins.TwilioPlugin("twilio")
	pluing6.Initialize(Config.TwilioConfig.Enable, Config.TwilioConfig.AccountSid, Config.TwilioConfig.AuthToken, Config.TwilioConfig.VerifyServiceID)
	Plugins[pluing6.Name()] = pluing6

	pluing7 := plugins.IAnFlow("ia")
	Plugins[pluing7.Name()] = pluing7

	log.Println("Plugins loaded: ", len(Plugins))

}

//
