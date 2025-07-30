package process

import (
	"fmt"
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
	c.JSON(200, processes)
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
	for key := range processes {
		WKill(key)
	}

}

func GetProcesses(c echo.Context) error {
	c.JSON(200, processes)
	return nil
}

func GetProcess(c echo.Context) error {
	wid := c.Param("wid")
	c.JSON(200, processes[wid])
	return nil
}

func GetProcessPayload(c echo.Context) error {
	wid := c.Param("wid")
	c.JSON(200, processes[wid].Payload)
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
	for key, p := range processes {
		fmt.Fprintf(&b, "\n%s - %s - %s", key, p.UUIDBoxCurrent, fmt.Sprint(p.Payload))
	}
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
