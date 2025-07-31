package engine

import (
	"context"
	"log"
)

// ConfigWorkspace represents the complete configuration structure for nFlow Runtime.
// It is loaded from the config.toml file and contains all settings for databases,
// services, plugins, and security configurations.
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
	TrackerConfig        TrackerConfig     `toml:"tracker"`
}

// VMPoolConfig configures the JavaScript VM pool for workflow execution.
// It includes settings for pool size, resource limits, and security sandboxing.
type VMPoolConfig struct {
	MaxSize         int  `toml:"max_size"`         // Maximum number of VMs in pool (default: 50)
	PreloadSize     int  `toml:"preload_size"`     // Number of VMs to preload (default: max_size/2)
	IdleTimeout     int  `toml:"idle_timeout"`     // Minutes before idle VM is removed (default: 10)
	CleanupInterval int  `toml:"cleanup_interval"` // Minutes between cleanup runs (default: 5)
	EnableMetrics   bool `toml:"enable_metrics"`   // Enable VM pool metrics logging

	// Resource limits
	MaxMemoryMB         int   `toml:"max_memory_mb"`         // Max memory per VM in MB (default: 128)
	MaxExecutionSeconds int   `toml:"max_execution_seconds"` // Max execution time in seconds (default: 30)
	MaxOperations       int64 `toml:"max_operations"`        // Max JS operations (default: 10M)

	// Sandbox settings
	EnableFileSystem bool `toml:"enable_filesystem"` // Allow filesystem access (default: false)
	EnableNetwork    bool `toml:"enable_network"`    // Allow network access (default: false)
	EnableProcess    bool `toml:"enable_process"`    // Allow process access (default: false)
}

// TrackerConfig configures the performance tracking system.
// It allows fine-tuning of the tracker's behavior to minimize performance impact.
type TrackerConfig struct {
	Enabled        bool `toml:"enabled"`         // Enable/disable tracker (default: false)
	Workers        int  `toml:"workers"`         // Number of worker goroutines (default: 4)
	BatchSize      int  `toml:"batch_size"`      // Batch size for database inserts (default: 100)
	FlushInterval  int  `toml:"flush_interval"`  // Flush interval in milliseconds (default: 250)
	ChannelBuffer  int  `toml:"channel_buffer"`  // Channel buffer size (default: 100000)
	VerboseLogging bool `toml:"verbose_logging"` // Enable verbose logging (default: false)
	StatsInterval  int  `toml:"stats_interval"`  // Stats reporting interval in seconds (default: 300)
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
	repo := GetConfigRepository()
	config := repo.GetConfig()

	if config.DatabaseNflow.Query == "" {
		config.DatabaseNflow.Query = "SELECT name,query FROM queries"
		repo.SetConfig(*config)
		config = repo.GetConfig()
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
	rows, err := conn.QueryContext(context.Background(), config.DatabaseNflow.Query)
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
	// Actualizar las queries en la configuración
	config.DatabaseNflow.QueryGetUser = queries["QueryGetUser"]
	config.DatabaseNflow.QueryGetApp = queries["QueryGetApp"]
	config.DatabaseNflow.QueryGetModules = queries["QueryGetModules"]
	config.DatabaseNflow.QueryCountModulesByName = queries["QueryCountModulesByName"]
	config.DatabaseNflow.QueryGetModuleByName = queries["QueryGetModuleByName"]
	config.DatabaseNflow.QueryUpdateModModuleByName = queries["QueryUpdateModModuleByName"]
	config.DatabaseNflow.QueryUpdateFormModuleByName = queries["QueryUpdateFormModuleByName"]
	config.DatabaseNflow.QueryUpdateCodeModuleByName = queries["QueryUpdateCodeModuleByName"]
	config.DatabaseNflow.QueryUpdateApp = queries["QueryUpdateApp"]
	config.DatabaseNflow.QueryInsertModule = queries["QueryInsertModule"]
	config.DatabaseNflow.QueryDeleteModule = queries["QueryDeleteModule"]
	config.DatabaseNflow.QueryInsertLog = queries["QueryInsertLog"]
	config.DatabaseNflow.QueryGetToken = queries["QueryGetToken"]
	config.DatabaseNflow.QueryGetTemplateCount = queries["QueryGetTemplateCount"]
	config.DatabaseNflow.QueryGetTemplate = queries["QueryGetTemplate"]
	config.DatabaseNflow.QueryGetTemplates = queries["QueryGetTemplates"]
	config.DatabaseNflow.QueryUpdateTemplate = queries["QueryUpdateTemplate"]
	config.DatabaseNflow.QueryInsertTemplate = queries["QueryInsertTemplate"]
	config.DatabaseNflow.QueryDeleteTemplate = queries["QueryDeleteTemplate"]

	// Guardar la configuración actualizada
	repo.SetConfig(*config)
	log.Println("Queries updated")

}
