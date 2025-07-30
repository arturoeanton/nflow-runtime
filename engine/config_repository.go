// Package engine provides the core workflow execution engine for nFlow Runtime.
// This file implements a thread-safe repository pattern for managing global
// configuration and resources, eliminating race conditions.
package engine

import (
	"database/sql"
	"sync"

	"github.com/go-redis/redis"
)

// ConfigRepository manages thread-safe access to configuration and global resources
type ConfigRepository interface {
	GetConfig() *ConfigWorkspace
	SetConfig(config ConfigWorkspace)
	GetRedisClient() *redis.Client
	SetRedisClient(client *redis.Client)
	GetDB() (*sql.DB, error)
	SetDB(database *sql.DB)
}

// configRepository concrete implementation of the repository
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

// GetConfigRepository returns the singleton instance of the repository
func GetConfigRepository() ConfigRepository {
	configRepoOnce.Do(func() {
		configRepo = &configRepository{}
	})
	return configRepo
}

// GetConfig retrieves the configuration in a thread-safe manner
func (r *configRepository) GetConfig() *ConfigWorkspace {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy to avoid external modifications
	configCopy := r.config
	return &configCopy
}

// SetConfig sets the configuration in a thread-safe manner
func (r *configRepository) SetConfig(config ConfigWorkspace) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// GetRedisClient retrieves the Redis client in a thread-safe manner
func (r *configRepository) GetRedisClient() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.redisClient
}

// SetRedisClient sets the Redis client in a thread-safe manner
func (r *configRepository) SetRedisClient(client *redis.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redisClient = client
}

// GetDB retrieves the database connection, creating it if necessary
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

// SetDB sets the database connection
func (r *configRepository) SetDB(database *sql.DB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = database
}

// Helper functions to maintain compatibility

// GetConfig returns the current configuration (compatibility helper)
func GetConfig() *ConfigWorkspace {
	return GetConfigRepository().GetConfig()
}

// GetRedisClient returns the current Redis client (compatibility helper)
func GetRedisClient() *redis.Client {
	return GetConfigRepository().GetRedisClient()
}