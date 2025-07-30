package process

import (
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
}

var (
	processes         map[string]*Process = make(map[string]*Process)
	processesMapMutex                     = sync.Mutex{}
)

func KillWID(c echo.Context) error {
	WKill(c.Param("wid"))
	
	// Crear copia segura de processes
	processesMapMutex.Lock()
	processesCopy := make(map[string]*Process)
	for k, v := range processes {
		processesCopy[k] = v
	}
	processesMapMutex.Unlock()
	
	c.JSON(200, processesCopy)
	return nil
}

func WKill(wid string) {
	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	process, ok := processes[wid]
	if ok {
		if process.Ws != nil {
			process.Ws.Close()
		}
		process.FlagExit = 1
		process.SendCallback(`{"error_exit":"exit"}`)
	}
}

func WKillAll() {
	// Primero obtener todas las keys para evitar locks anidados
	processesMapMutex.Lock()
	keys := make([]string, 0, len(processes))
	for key := range processes {
		keys = append(keys, key)
	}
	processesMapMutex.Unlock()
	
	// Ahora matar cada proceso
	for _, key := range keys {
		WKill(key)
	}
}

func GetProcesses(c echo.Context) error {
	processesMapMutex.Lock()
	processesCopy := make(map[string]*Process)
	for k, v := range processes {
		processesCopy[k] = v
	}
	processesMapMutex.Unlock()
	
	c.JSON(200, processesCopy)
	return nil
}

func GetProcess(c echo.Context) error {
	wid := c.Param("wid")
	
	processesMapMutex.Lock()
	process := processes[wid]
	processesMapMutex.Unlock()
	
	c.JSON(200, process)
	return nil
}

func GetProcessPayload(c echo.Context) error {
	wid := c.Param("wid")
	
	processesMapMutex.Lock()
	process, exists := processes[wid]
	processesMapMutex.Unlock()
	
	if !exists {
		c.JSON(404, echo.Map{"error": "process not found"})
		return nil
	}
	
	c.JSON(200, process.Payload)
	return nil
}

func GetProcessID(wid string) (*Process, bool) {
	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	p, ok := processes[wid]
	return p, ok
}

func SetProcessID(wid string, p *Process) {

	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	processes[wid] = p
}

func CreateProcess(wid string) *Process {
	p := &Process{
		UUID:           wid,
		State:          "wait",
		UUIDBoxCurrent: "",
		Type:           "",
		Killeable:      true,
	}
	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	processes[wid] = p
	return p
}

func CreateProcessWithCallback(wid string) *Process {
	p := &Process{
		UUID:           wid,
		State:          "wait",
		UUIDBoxCurrent: "",
		Type:           "",
		Callback:       make(chan string),
		Killeable:      true,
	}
	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	processes[wid] = p
	return p
}

func Ps() string {
	var b strings.Builder
	
	processesMapMutex.Lock()
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
	processesMapMutex.Unlock()
	
	return b.String()
}

func (p *Process) SendCallback(data string) {
	if p.Callback != nil {
		p.Callback <- data
	}
}

func (p *Process) Close() {
	processesMapMutex.Lock()
	defer processesMapMutex.Unlock()
	delete(processes, p.UUID)
}

func (p *Process) Kill() {
	WKill(p.UUID)
}
