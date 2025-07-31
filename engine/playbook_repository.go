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
	// Check cache first (now that JSON unmarshaling is thread-safe)
	if !r.NeedsReload(appName) {
		if playbooks, err := r.Get(appName); err == nil && playbooks != nil {
			// CRITICAL: Create deep copy to avoid concurrent modification of shared objects
			// This prevents race conditions when multiple requests access the same workflow
			copiedPlaybooks := deepCopyPlaybooks(playbooks)
			logger.Verbosef("DEBUG: Returned cached playbook %s (deep copy)", appName)
			return copiedPlaybooks, nil
		}
	}

	logger.Verbosef("DEBUG: Loading playbook %s from source (cache miss or reload needed)", appName)

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

	// DEBUG: Validate the freshly loaded playbooks
	validatePlaybooksIntegrity(playbooks, appName)

	// CRITICAL: Clean playbooks BEFORE caching to prevent corrupted data from entering cache
	cleanedPlaybooks := cleanPlaybooksForCache(playbooks, appName)

	// Save cleaned playbooks to cache
	r.Set(appName, cleanedPlaybooks)
	r.SetReloaded(appName)

	return cleanedPlaybooks, nil

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

// deepCopyPlaybooks creates a deep copy of playbooks to prevent concurrent modification
func deepCopyPlaybooks(original map[string]map[string]*model.Playbook) map[string]map[string]*model.Playbook {
	if original == nil {
		return nil
	}

	result := make(map[string]map[string]*model.Playbook, len(original))

	for outerKey, outerValue := range original {
		if outerValue == nil {
			result[outerKey] = nil
			continue
		}

		innerMap := make(map[string]*model.Playbook, len(outerValue))
		for innerKey, playbook := range outerValue {
			if playbook == nil {
				innerMap[innerKey] = nil
				continue
			}

			// Deep copy the playbook (map of nodes)
			copiedPlaybook := make(model.Playbook, len(*playbook))
			for nodeID, node := range *playbook {
				if node == nil {
					copiedPlaybook[nodeID] = nil
					continue
				}

				// Deep copy the node
				copiedNode := &model.Node{
					Data:    make(map[string]interface{}, len(node.Data)),
					Outputs: make(map[string]*model.Output, len(node.Outputs)),
				}

				// Copy data map
				for dataKey, dataValue := range node.Data {
					copiedNode.Data[dataKey] = dataValue
				}

				// Copy outputs map
				for outputKey, output := range node.Outputs {
					if output == nil {
						copiedNode.Outputs[outputKey] = nil
						continue
					}

					// Deep copy the output and its connections
					copiedOutput := &model.Output{
						Connections: make([]struct {
							Node   string `json:"node"`
							Output string `json:"output"`
						}, len(output.Connections)),
					}

					// Copy connections slice
					copy(copiedOutput.Connections, output.Connections)

					// DEBUG: Log the copy operation for starter nodes
					if nodeType, ok := node.Data["type"]; ok && nodeType == "starter" && outputKey == "output_1" {
						logger.Verbosef("DEBUG: Deep copying starter node %s: original connections=%d, copied connections=%d",
							nodeID, len(output.Connections), len(copiedOutput.Connections))
						if len(output.Connections) != len(copiedOutput.Connections) {
							logger.Errorf("DEBUG: DEEP COPY CORRUPTION! Original had %d connections, copy has %d",
								len(output.Connections), len(copiedOutput.Connections))
						}
					}

					copiedNode.Outputs[outputKey] = copiedOutput
				}

				copiedPlaybook[nodeID] = copiedNode
			}

			innerMap[innerKey] = &copiedPlaybook
		}

		result[outerKey] = innerMap
	}

	logger.Verbose("Created deep copy of playbooks to prevent race conditions")
	return result
}

// validatePlaybooksIntegrity checks if the loaded playbooks have all required connections
func validatePlaybooksIntegrity(playbooks map[string]map[string]*model.Playbook, appName string) {
	if playbooks == nil {
		logger.Error("DEBUG: Loaded playbooks is nil for app:", appName)
		return
	}

	for outerKey, outerValue := range playbooks {
		if outerValue == nil {
			continue
		}

		for innerKey, playbook := range outerValue {
			if playbook == nil {
				continue
			}

			logger.Verbosef("DEBUG: Validating playbook %s/%s", outerKey, innerKey)

			for nodeID, node := range *playbook {
				if node == nil {
					continue
				}

				// Check for starter nodes
				if nodeType, ok := node.Data["type"]; ok && nodeType == "starter" {
					// Extract additional info about the starter
					urlpattern := "unknown"
					method := "unknown"
					if pattern, ok := node.Data["urlpattern"]; ok {
						urlpattern = pattern.(string)
					}
					if m, ok := node.Data["method"]; ok {
						method = m.(string)
					}

					logger.Verbosef("DEBUG: Found starter node %s in %s/%s - URL: %s, Method: %s",
						nodeID, outerKey, innerKey, urlpattern, method)

					if node.Outputs == nil {
						logger.Errorf("DEBUG: CORRUPTION! Starter node %s (%s %s) has nil Outputs", nodeID, method, urlpattern)
						continue
					}

					if output1, exists := node.Outputs["output_1"]; exists {
						if output1 == nil {
							logger.Errorf("DEBUG: CORRUPTION! Starter node %s (%s %s) has nil output_1", nodeID, method, urlpattern)
						} else if output1.Connections == nil {
							logger.Errorf("DEBUG: CORRUPTION! Starter node %s (%s %s) has nil Connections", nodeID, method, urlpattern)
						} else if len(output1.Connections) == 0 {
							logger.Errorf("DEBUG: CORRUPTION! Starter node %s (%s %s) has empty Connections array", nodeID, method, urlpattern)
						} else {
							logger.Verbosef("DEBUG: VALID! Starter node %s (%s %s) has %d connections", nodeID, method, urlpattern, len(output1.Connections))
							for i, conn := range output1.Connections {
								logger.Verbosef("DEBUG: Connection %d: Node=%s, Output=%s", i, conn.Node, conn.Output)
							}
						}
					} else {
						logger.Errorf("DEBUG: CORRUPTION! Starter node %s (%s %s) missing output_1", nodeID, method, urlpattern)
					}
				}
			}
		}
	}
}

// cleanPlaybooksForCache removes corrupted starter nodes before caching to optimize performance
func cleanPlaybooksForCache(playbooks map[string]map[string]*model.Playbook, appName string) map[string]map[string]*model.Playbook {
	if playbooks == nil {
		return nil
	}

	cleanedPlaybooks := make(map[string]map[string]*model.Playbook)
	totalRemoved := 0

	for outerKey, outerValue := range playbooks {
		if outerValue == nil {
			cleanedPlaybooks[outerKey] = nil
			continue
		}

		cleanedFlows := make(map[string]*model.Playbook)
		for innerKey, playbook := range outerValue {
			if playbook == nil {
				cleanedFlows[innerKey] = nil
				continue
			}

			cleanedPlaybook := make(model.Playbook)
			removedFromFlow := 0

			for nodeID, node := range *playbook {
				if node == nil || node.Data == nil {
					cleanedPlaybook[nodeID] = node
					continue
				}

				// Check if this is a corrupted starter node
				if nodeType, ok := node.Data["type"]; ok && nodeType == "starter" {
					// Extract debugging info
					urlpattern := "unknown"
					method := "unknown"
					if pattern, ok := node.Data["urlpattern"]; ok {
						if patternStr, ok := pattern.(string); ok {
							urlpattern = patternStr
						}
					}
					if m, ok := node.Data["method"]; ok {
						if methodStr, ok := m.(string); ok {
							method = methodStr
						}
					}

					// Check if it has proper connections
					if node.Outputs == nil {
						logger.Verbosef("DEBUG: Pre-cache cleanup removing starter node %s (%s %s) - no outputs", nodeID, method, urlpattern)
						removedFromFlow++
						continue
					}

					output1, exists := node.Outputs["output_1"]
					if !exists || output1 == nil {
						logger.Verbosef("DEBUG: Pre-cache cleanup removing starter node %s (%s %s) - no output_1", nodeID, method, urlpattern)
						removedFromFlow++
						continue
					}

					if output1.Connections == nil || len(output1.Connections) == 0 {
						logger.Verbosef("DEBUG: Pre-cache cleanup removing starter node %s (%s %s) - empty connections", nodeID, method, urlpattern)
						removedFromFlow++
						continue
					}

					// Node is valid, keep it
					logger.Verbosef("DEBUG: Pre-cache cleanup keeping valid starter node %s (%s %s) with %d connections", nodeID, method, urlpattern, len(output1.Connections))
				}

				// Node is valid (not a corrupted starter), keep it
				cleanedPlaybook[nodeID] = node
			}

			if removedFromFlow > 0 {
				logger.Verbosef("DEBUG: Pre-cache cleanup removed %d corrupted starter nodes from flow %s/%s", removedFromFlow, outerKey, innerKey)
				totalRemoved += removedFromFlow
			}

			cleanedFlows[innerKey] = &cleanedPlaybook
		}

		cleanedPlaybooks[outerKey] = cleanedFlows
	}

	if totalRemoved > 0 {
		logger.Verbosef("DEBUG: Pre-cache cleanup completed for app %s - removed %d corrupted starter nodes total", appName, totalRemoved)
	}

	return cleanedPlaybooks
}
