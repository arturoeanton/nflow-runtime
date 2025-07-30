package process

import (
	"log"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type Process struct {
	UUID           string
	UUIDBoxCurrent string
	State          string
	Type           string
	Payload        interface{}
	Killeable      bool
	Callback       chan string     `json:"-"`
	FlagExit       int             `json:"-"`
	Ws             *websocket.Conn `json:"-"`
	mu             sync.Mutex      `json:"-"` // Mutex para proteger campos modificables
}

var (
	// repo es la instancia global del repository
	repo ProcessRepository
)

// InitializeRepository inicializa el repository de procesos
func InitializeRepository() {
	if repo == nil {
		repo = NewProcessRepository()
	}
}

// GetRepository retorna el repository actual
func GetRepository() ProcessRepository {
	if repo == nil {
		InitializeRepository()
	}
	return repo
}

func KillWID(c echo.Context) error {
	WKill(c.Param("wid"))

	// Obtener todos los procesos del repository
	processes := GetRepository().GetAll()

	c.JSON(200, processes)
	return nil
}

func WKill(wid string) {
	process, ok := GetRepository().Get(wid)
	if ok {
		// Proteger acceso a campos del proceso
		process.mu.Lock()
		if process.Ws != nil {
			process.Ws.Close()
		}
		process.FlagExit = 1
		process.mu.Unlock()

		process.SendCallback(`{"error_exit":"exit"}`)
	}
}

func WKillAll() {
	// Obtener todas las keys del repository
	keys := GetRepository().GetAllKeys()

	// Matar cada proceso
	for _, key := range keys {
		WKill(key)
	}
}

func GetProcesses(c echo.Context) error {
	processes := GetRepository().GetAll()
	c.JSON(200, processes)
	return nil
}

func GetProcess(c echo.Context) error {
	wid := c.Param("wid")

	process, exists := GetRepository().Get(wid)
	if !exists {
		c.JSON(404, echo.Map{"error": "process not found"})
		return nil
	}

	c.JSON(200, process)
	return nil
}

func GetProcessPayload(c echo.Context) error {
	wid := c.Param("wid")

	process, exists := GetRepository().Get(wid)
	if !exists {
		c.JSON(404, echo.Map{"error": "process not found"})
		return nil
	}

	c.JSON(200, process.Payload)
	return nil
}

func GetProcessID(wid string) (*Process, bool) {
	return GetRepository().Get(wid)
}

func SetProcessID(wid string, p *Process) {
	GetRepository().Set(wid, p)
}

func CreateProcess(wid string) *Process {
	p := &Process{
		UUID:           wid,
		State:          "wait",
		UUIDBoxCurrent: "",
		Type:           "",
		Killeable:      true,
	}
	GetRepository().Set(wid, p)
	return p
}

func CreateProcessWithCallback(wid string) *Process {
	p := &Process{
		UUID:           wid,
		State:          "wait",
		UUIDBoxCurrent: "",
		Type:           "",
		Callback:       make(chan string, 1), // Buffer de 1 para evitar bloqueos
		Killeable:      true,
	}
	GetRepository().Set(wid, p)
	return p
}

func Ps() string {
	var b strings.Builder

	processes := GetRepository().GetAll()
	for key, p := range processes {
		// Evitar fmt para reducir race conditions
		b.WriteString("\n")
		b.WriteString(key)
		b.WriteString(" - ")
		b.WriteString(p.UUIDBoxCurrent)
		b.WriteString(" - ")
		// Para payload, usar representación simple
		if p.Payload != nil {
			b.WriteString("<payload>")
		} else {
			b.WriteString("<nil>")
		}
	}

	return b.String()
}

func (p *Process) SendCallback(data string) {
	if p.Callback != nil {
		select {
		case p.Callback <- data:
			// Enviado exitosamente
		default:
			// No hay receptor, no bloqueamos
			log.Printf("Warning: Callback channel full or no receiver for process %s", p.UUID)
		}
	}
}

func (p *Process) Close() {
	GetRepository().Delete(p.UUID)
}

func (p *Process) Kill() {
	WKill(p.UUID)
}

// GetFlagExit devuelve el valor de FlagExit de forma thread-safe
func (p *Process) GetFlagExit() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.FlagExit
}

// SetFlagExit establece el valor de FlagExit de forma thread-safe
func (p *Process) SetFlagExit(value int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FlagExit = value
}
