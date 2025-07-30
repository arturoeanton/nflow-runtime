package process

import (
	"sync"
)

// ProcessRepository maneja el acceso thread-safe a los procesos
type ProcessRepository interface {
	Get(wid string) (*Process, bool)
	GetAll() map[string]*Process
	Set(wid string, process *Process)
	Delete(wid string)
	Exists(wid string) bool
	GetAllKeys() []string
	Clear()
}

// processRepository implementación concreta del repository
type processRepository struct {
	mu        sync.RWMutex
	processes map[string]*Process
}

// NewProcessRepository crea una nueva instancia del repository
func NewProcessRepository() ProcessRepository {
	return &processRepository{
		processes: make(map[string]*Process),
	}
}

// Get obtiene un proceso por su ID
func (r *processRepository) Get(wid string) (*Process, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	process, exists := r.processes[wid]
	return process, exists
}

// GetAll devuelve una copia de todos los procesos
func (r *processRepository) GetAll() map[string]*Process {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Crear copia para evitar modificaciones externas
	copy := make(map[string]*Process, len(r.processes))
	for k, v := range r.processes {
		copy[k] = v
	}
	return copy
}

// Set establece o actualiza un proceso
func (r *processRepository) Set(wid string, process *Process) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.processes[wid] = process
}

// Delete elimina un proceso
func (r *processRepository) Delete(wid string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.processes, wid)
}

// Exists verifica si un proceso existe
func (r *processRepository) Exists(wid string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.processes[wid]
	return exists
}

// GetAllKeys devuelve todas las keys de procesos
func (r *processRepository) GetAllKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.processes))
	for k := range r.processes {
		keys = append(keys, k)
	}
	return keys
}

// Clear elimina todos los procesos
func (r *processRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.processes = make(map[string]*Process)
}
