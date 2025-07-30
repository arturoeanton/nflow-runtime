package engine

import (
	"context"
	"log"
)

// ConfigWorkspace is ...
type ConfigWorkspace struct {
	ConfigBasedate       ConfigBasedate    `toml:"database"`
	ConfigMail           ConfigMail        `toml:"mail"`
	URLConfig            URLConfig         `toml:"url"`
	MongoConfig          MongoConfig       `toml:"mongo"`
	PluginConfig         PluginConfig      `toml:"plugin"`
	RedisConfig          RedisConfig       `toml:"redis"`
	PgSessionConfig      PgSessionConfig   `toml:"pg_session"`
	TwilioConfig         TwilioConfig      `toml:"twilio"`
	Env                  map[string]string `toml:"env"`
	HttpsEngineConfig    HttpsConfig       `toml:"https_engine"`
	HttpsDesingnerConfig HttpsConfig       `toml:"https_designer"`
	DatabaseNflow        DatabaseNflow     `toml:"database_nflow"`
	VMPoolConfig         VMPoolConfig      `toml:"vm_pool"`
}

// VMPoolConfig configures the VM pool
type VMPoolConfig struct {
	MaxSize         int  `toml:"max_size"`         // Maximum number of VMs in pool (default: 50)
	PreloadSize     int  `toml:"preload_size"`     // Number of VMs to preload (default: max_size/2)
	IdleTimeout     int  `toml:"idle_timeout"`     // Minutes before idle VM is removed (default: 10)
	CleanupInterval int  `toml:"cleanup_interval"` // Minutes between cleanup runs (default: 5)
	EnableMetrics   bool `toml:"enable_metrics"`   // Enable VM pool metrics logging
}

type DatabaseNflow struct {
	Driver                      string `tom:"driver"`
	DSN                         string `tom:"dsn"`
	Query                       string `tom:"query"`
	QueryGetUser                string `tom:"QueryGetUser"`
	QueryGetApp                 string `tom:"QueryGetApp"`
	QueryGetModules             string `tom:"QueryGetModules"`
	QueryCountModulesByName     string `tom:"QueryCountModulesByName"`
	QueryGetModuleByName        string `tom:"QueryGetModuleByName"`
	QueryUpdateModModuleByName  string `tom:"QueryUpdateModModuleByName"`
	QueryUpdateFormModuleByName string `tom:"QueryUpdateFormModuleByName"`
	QueryUpdateCodeModuleByName string `tom:"QueryUpdateCodeModuleByName"`
	QueryUpdateApp              string `tom:"QueryUpdateApp"`
	QueryInsertModule           string `tom:"QueryInsertModule"`
	QueryDeleteModule           string `tom:"QueryDeleteModule"`
	QueryInsertLog              string `tom:"QueryInsertLog"`
	QueryGetToken               string `tom:"QueryGetToken"`
	QueryGetTemplateCount       string `tom:"QueryGetTemplateCount"`
	QueryGetTemplate            string `tom:"QueryGetTemplate"`
	QueryGetTemplates           string `tom:"QueryGetTemplates"`
	QueryUpdateTemplate         string `tom:"QueryUpdateTemplate"`
	QueryInsertTemplate         string `tom:"QueryInsertTemplate"`
	QueryDeleteTemplate         string `tom:"QueryDeleteTemplate"`
}
type HttpsConfig struct {
	Enable      bool   `tom:"enable"`
	Cert        string `tom:"cert"`
	Key         string `tom:"key"`
	Address     string `tom:"address"`
	Description string `tom:"description"`
	HTTPBasic   bool   `tom:"httpbasic"`
}

type PgSessionConfig struct {
	Url string `tom:"url"`
}

type RedisConfig struct {
	Host              string `tom:"host"`
	Password          string `tom:"password"`
	MaxConnectionPool int    `tom:"maxconnectionpool"`
}

type TwilioConfig struct {
	Enable          bool   `toml:"enable"`
	AccountSid      string `toml:"account_sid"`
	AuthToken       string `toml:"auth_token"`
	VerifyServiceID string `toml:"verify_service_id"`
}

type MongoConfig struct {
	URL string `tom:"url"`
}

type PluginConfig struct {
	Plugins []string `toml:"plugins"`
}

type URLConfig struct {
	URLBase string `toml:"url_base"`
}

// ConfigBasedate is ...
type ConfigBasedate struct {
	DatabaseURL    string `toml:"url"`
	DatabaseDriver string `toml:"driver"`
	DatabaseInit   string `toml:"init"`
}

// ConfigMail is ...
type ConfigMail struct {
	MailSMTP     string `toml:"smtp"`
	MailSMTPPort string `toml:"port"`
	MailFrom     string `toml:"from"`
	MailPassword string `toml:"password"`
}

func UpdateQueries() {
	log.Println("Updating queries")
	if Config.DatabaseNflow.Query == "" {
		Config.DatabaseNflow.Query = "SELECT name,query FROM queries"
	}
	db, err := GetDB()
	if err != nil {
		log.Println(err)
		return
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	queries := make(map[string]string)
	rows, err := conn.QueryContext(context.Background(), Config.DatabaseNflow.Query)
	if err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var name, query string
		err = rows.Scan(&name, &query)
		if err != nil {
			log.Println(err)
			return
		}
		queries[name] = query
	}
	Config.DatabaseNflow.QueryGetUser = queries["QueryGetUser"]
	Config.DatabaseNflow.QueryGetApp = queries["QueryGetApp"]
	Config.DatabaseNflow.QueryGetModules = queries["QueryGetModules"]
	Config.DatabaseNflow.QueryCountModulesByName = queries["QueryCountModulesByName"]
	Config.DatabaseNflow.QueryGetModuleByName = queries["QueryGetModuleByName"]
	Config.DatabaseNflow.QueryUpdateModModuleByName = queries["QueryUpdateModModuleByName"]
	Config.DatabaseNflow.QueryUpdateFormModuleByName = queries["QueryUpdateFormModuleByName"]
	Config.DatabaseNflow.QueryUpdateCodeModuleByName = queries["QueryUpdateCodeModuleByName"]
	Config.DatabaseNflow.QueryUpdateApp = queries["QueryUpdateApp"]
	Config.DatabaseNflow.QueryInsertModule = queries["QueryInsertModule"]
	Config.DatabaseNflow.QueryDeleteModule = queries["QueryDeleteModule"]
	Config.DatabaseNflow.QueryInsertLog = queries["QueryInsertLog"]
	Config.DatabaseNflow.QueryGetToken = queries["QueryGetToken"]
	Config.DatabaseNflow.QueryGetTemplateCount = queries["QueryGetTemplateCount"]
	Config.DatabaseNflow.QueryGetTemplate = queries["QueryGetTemplate"]
	Config.DatabaseNflow.QueryGetTemplates = queries["QueryGetTemplates"]
	Config.DatabaseNflow.QueryUpdateTemplate = queries["QueryUpdateTemplate"]
	Config.DatabaseNflow.QueryInsertTemplate = queries["QueryInsertTemplate"]
	Config.DatabaseNflow.QueryDeleteTemplate = queries["QueryDeleteTemplate"]
	log.Println("Queries updated")

}
