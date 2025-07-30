package syncsession

import "sync"

// Mantener para compatibilidad con código existente
var EchoSessionsMutex sync.Mutex
var PayloadSessionMutex sync.Mutex

// Inicializar el session manager
func init() {
	Manager.StartCleanupRoutine()
}
