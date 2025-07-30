package engine

import (
	"context"
	"database/sql"
	"sync"
)

var (
	modulesRepo *RepositoryModules
	muModules   sync.Mutex
)

type Module struct {
	Form string
	Mod  string
	Code string
}

type RepositoryModules struct {
	moduless map[string]Module
	mu       sync.Mutex
	dinamic  bool
}

func GetRepositoryModules() *RepositoryModules {
	muModules.Lock()
	defer muModules.Unlock()
	if modulesRepo != nil {
		return modulesRepo
	}

	modulesRepo = &RepositoryModules{
		moduless: make(map[string]Module),
		dinamic:  false,
		mu:       sync.Mutex{},
	}
	return modulesRepo
}

func (r *RepositoryModules) IsDinamic() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dinamic
}

func (r *RepositoryModules) SetDinamic(dinamic bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dinamic = dinamic
}

func (r *RepositoryModules) GetModule(name string) (Module, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	module, ok := r.moduless[name]
	return module, ok
}

func (r *RepositoryModules) GetModuleWithFallback(name string, ctx context.Context, conn *sql.Conn) (Module, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.dinamic {
		module, ok := r.moduless[name]
		if ok {
			return module, true
		}
	}

	config := GetConfig()
	row := conn.QueryRowContext(ctx, config.DatabaseNflow.QueryGetModuleByName, name)
	var form string
	var mod string
	var code string
	err := row.Scan(&form, &mod, &code)
	if err == nil {
		code = babelTransform(code)
	}
	module := Module{
		Form: form,
		Mod:  mod,
		Code: code,
	}
	r.moduless[name] = module
	return module, true
}
