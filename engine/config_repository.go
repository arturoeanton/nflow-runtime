package engine

import (
	"database/sql"
	"sync"

	"github.com/go-redis/redis"
)

// ConfigRepository maneja el acceso thread-safe a la configuración y recursos globales
type ConfigRepository interface {
	GetConfig() *ConfigWorkspace
	SetConfig(config ConfigWorkspace)
	GetRedisClient() *redis.Client
	SetRedisClient(client *redis.Client)
	GetDB() (*sql.DB, error)
	SetDB(database *sql.DB)
}

// configRepository implementación concreta del repository
type configRepository struct {
	mu          sync.RWMutex
	config      ConfigWorkspace
	redisClient *redis.Client
	db          *sql.DB
}

// singleton instance
var (
	configRepo     ConfigRepository
	configRepoOnce sync.Once
)

// GetConfigRepository retorna la instancia singleton del repository
func GetConfigRepository() ConfigRepository {
	configRepoOnce.Do(func() {
		configRepo = &configRepository{}
	})
	return configRepo
}

// GetConfig obtiene la configuración de forma thread-safe
func (r *configRepository) GetConfig() *ConfigWorkspace {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Retornamos una copia para evitar modificaciones externas
	configCopy := r.config
	return &configCopy
}

// SetConfig establece la configuración de forma thread-safe
func (r *configRepository) SetConfig(config ConfigWorkspace) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// GetRedisClient obtiene el cliente Redis de forma thread-safe
func (r *configRepository) GetRedisClient() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.redisClient
}

// SetRedisClient establece el cliente Redis de forma thread-safe
func (r *configRepository) SetRedisClient(client *redis.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redisClient = client
}

// GetDB obtiene la conexión a la base de datos, creándola si es necesario
func (r *configRepository) GetDB() (*sql.DB, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.db == nil {
		config := r.config
		var err error
		r.db, err = sql.Open(config.DatabaseNflow.Driver, config.DatabaseNflow.DSN)
		if err != nil {
			return nil, err
		}
	}
	return r.db, nil
}

// SetDB establece la conexión a la base de datos
func (r *configRepository) SetDB(database *sql.DB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = database
}

// Helper functions para mantener compatibilidad

// GetConfig retorna la configuración actual (helper para compatibilidad)
func GetConfig() *ConfigWorkspace {
	return GetConfigRepository().GetConfig()
}

// GetRedisClient retorna el cliente Redis actual (helper para compatibilidad)
func GetRedisClient() *redis.Client {
	return GetConfigRepository().GetRedisClient()
}