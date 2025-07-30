package engine

import (
	"context"
	"database/sql"
	"sync"

	"github.com/arturoeanton/nflow-runtime/logger"
	"github.com/arturoeanton/nflow-runtime/model"
)

// PlaybookRepository maneja el acceso thread-safe a los playbooks
type PlaybookRepository interface {
	Get(appName string) (map[string]map[string]*model.Playbook, error)
	Set(appName string, playbooks map[string]map[string]*model.Playbook)
	NeedsReload(appName string) bool
	SetReloaded(appName string)
	LoadPlaybook(ctx context.Context, appName string) (map[string]map[string]*model.Playbook, error)
	InvalidateCache(appName string)
	InvalidateAllCache()
}

// playbookRepository implementación concreta del repository
type playbookRepository struct {
	mu          sync.RWMutex
	playbooks   map[string]map[string]map[string]*model.Playbook
	needsReload map[string]bool
	db          *sql.DB
}

// NewPlaybookRepository crea una nueva instancia del repository
func NewPlaybookRepository(db *sql.DB) PlaybookRepository {
	return &playbookRepository{
		playbooks:   make(map[string]map[string]map[string]*model.Playbook),
		needsReload: make(map[string]bool),
		db:          db,
	}
}

// Get obtiene los playbooks para una aplicación
func (r *playbookRepository) Get(appName string) (map[string]map[string]*model.Playbook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if playbook, exists := r.playbooks[appName]; exists {
		return playbook, nil
	}

	return nil, nil
}

// Set establece los playbooks para una aplicación
func (r *playbookRepository) Set(appName string, playbooks map[string]map[string]*model.Playbook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.playbooks[appName] = playbooks
}

// NeedsReload verifica si una aplicación necesita recargar sus playbooks
func (r *playbookRepository) NeedsReload(appName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	needsReload, exists := r.needsReload[appName]
	return !exists || needsReload
}

// SetReloaded marca una aplicación como recargada
func (r *playbookRepository) SetReloaded(appName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.needsReload[appName] = false
}

// LoadPlaybook carga un playbook desde la base de datos si es necesario
func (r *playbookRepository) LoadPlaybook(ctx context.Context, appName string) (map[string]map[string]*model.Playbook, error) {
	// Verify if the playbook needs to be reloaded
	if !r.NeedsReload(appName) {
		if playbooks, err := r.Get(appName); err == nil && playbooks != nil {
			return playbooks, nil
		}
	}

	conn, err := r.db.Conn(ctx)
	if err != nil {
		logger.Error("Failed to get database connection:", err)
		return nil, err
	}
	defer conn.Close()

	playbooks, err := GetPlaybook(ctx, conn, appName)
	if err != nil {
		logger.Error("Failed to load playbook from database:", err)
		return nil, err
	}
	// Save to cache
	r.Set(appName, playbooks)
	r.SetReloaded(appName)

	return playbooks, nil

}

// InvalidateCache invalida el cache para forzar recarga
func (r *playbookRepository) InvalidateCache(appName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.needsReload[appName] = true
}

// InvalidateAllCache invalida todo el cache
func (r *playbookRepository) InvalidateAllCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for appName := range r.needsReload {
		r.needsReload[appName] = true
	}
}
