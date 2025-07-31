package engine

import (
	"sync"
	"sync/atomic"

	"github.com/arturoeanton/nflow-runtime/model"
)

// ImmutableWorkflowSnapshot representa un snapshot inmutable del workflow
type ImmutableWorkflowSnapshot struct {
	playbook  map[string]*model.Node
	startNode *model.Node
	version   int64 // Para invalidación de cache
}

// ImmutableWorkflowManager maneja snapshots inmutables
type ImmutableWorkflowManager struct {
	current  atomic.Value // Stores *ImmutableWorkflowSnapshot
	updateMu sync.Mutex   // Solo para updates, no para reads
	version  int64
}

var (
	workflowManager = &ImmutableWorkflowManager{}
)

// GetWorkflowManager retorna el manager global
func GetWorkflowManager() *ImmutableWorkflowManager {
	return workflowManager
}

// UpdateWorkflow actualiza el snapshot inmutable (thread-safe)
func (iwm *ImmutableWorkflowManager) UpdateWorkflow(cc *model.Controller) {
	iwm.updateMu.Lock()
	defer iwm.updateMu.Unlock()

	// Crear snapshot inmutable
	snapshot := &ImmutableWorkflowSnapshot{
		version: atomic.AddInt64(&iwm.version, 1),
	}

	// Deep copy del playbook - esto garantiza inmutabilidad
	if cc.Playbook != nil {
		snapshot.playbook = make(map[string]*model.Node, len(*cc.Playbook))
		for k, v := range *cc.Playbook {
			if v != nil {
				// Deep copy de cada nodo
				nodeCopy := &model.Node{
					Data:    make(map[string]interface{}),
					Outputs: make(map[string]*model.Output),
				}

				// Copy data map
				for dk, dv := range v.Data {
					nodeCopy.Data[dk] = dv
				}

				// Copy outputs map
				for ok, ov := range v.Outputs {
					if ov != nil {
						outputCopy := &model.Output{
							Connections: make([]struct {
								Node   string `json:"node"`
								Output string `json:"output"`
							}, len(ov.Connections)),
						}
						copy(outputCopy.Connections, ov.Connections)
						nodeCopy.Outputs[ok] = outputCopy
					}
				}

				snapshot.playbook[k] = nodeCopy
			}
		}
	}

	// Deep copy del start node
	if cc.Start != nil {
		snapshot.startNode = &model.Node{
			Data:    make(map[string]interface{}),
			Outputs: make(map[string]*model.Output),
		}

		// Copy data
		for k, v := range cc.Start.Data {
			snapshot.startNode.Data[k] = v
		}

		// Copy outputs
		for k, v := range cc.Start.Outputs {
			if v != nil {
				outputCopy := &model.Output{
					Connections: make([]struct {
						Node   string `json:"node"`
						Output string `json:"output"`
					}, len(v.Connections)),
				}
				copy(outputCopy.Connections, v.Connections)
				snapshot.startNode.Outputs[k] = outputCopy
			}
		}
	}

	// Atomic swap del snapshot
	iwm.current.Store(snapshot)
}

// GetSnapshot retorna el snapshot actual (lock-free read)
func (iwm *ImmutableWorkflowManager) GetSnapshot() *ImmutableWorkflowSnapshot {
	if snapshot := iwm.current.Load(); snapshot != nil {
		return snapshot.(*ImmutableWorkflowSnapshot)
	}
	return nil
}

// Métodos del snapshot (todos inmutables)

// GetPlaybook retorna el playbook inmutable
func (iws *ImmutableWorkflowSnapshot) GetPlaybook() map[string]*model.Node {
	return iws.playbook // Ya es una copia, es seguro retornarlo
}

// GetNode retorna un nodo específico (thread-safe)
func (iws *ImmutableWorkflowSnapshot) GetNode(nodeID string) *model.Node {
	if iws.playbook == nil {
		return nil
	}
	return iws.playbook[nodeID]
}

// GetStartNode retorna el nodo de inicio (thread-safe)
func (iws *ImmutableWorkflowSnapshot) GetStartNode() *model.Node {
	return iws.startNode
}

// GetStartNextNode retorna el siguiente nodo desde start (thread-safe)
func (iws *ImmutableWorkflowSnapshot) GetStartNextNode() (string, error) {
	if iws.startNode == nil {
		return "", NewWorkflowError("Start node not configured")
	}

	if iws.startNode.Outputs == nil {
		return "", NewWorkflowError("Start node has no outputs configured")
	}

	output1, exists := iws.startNode.Outputs["output_1"]
	if !exists {
		return "", NewWorkflowError("Start node missing 'output_1' connection")
	}

	if output1.Connections == nil || len(output1.Connections) == 0 {
		return "", NewWorkflowError("No output connections found for the start node")
	}

	return output1.Connections[0].Node, nil
}

// GetVersion retorna la versión del snapshot
func (iws *ImmutableWorkflowSnapshot) GetVersion() int64 {
	return iws.version
}

// HasPlaybook verifica si hay un playbook válido
func (iws *ImmutableWorkflowSnapshot) HasPlaybook() bool {
	return iws != nil && iws.playbook != nil
}

// WorkflowError para errores específicos de workflow
type WorkflowError struct {
	Message string
}

func (we *WorkflowError) Error() string {
	return we.Message
}

func NewWorkflowError(message string) *WorkflowError {
	return &WorkflowError{Message: message}
}

// Funciones de utilidad para migración gradual

// InitializeImmutableWorkflow inicializa el manager con un controller
func InitializeImmutableWorkflow(cc *model.Controller) {
	workflowManager.UpdateWorkflow(cc)
}

// GetImmutableWorkflow retorna el snapshot actual
func GetImmutableWorkflow() *ImmutableWorkflowSnapshot {
	return workflowManager.GetSnapshot()
}
