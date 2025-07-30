package engine

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/labstack/echo/v4"
)

// EchoContextManager provides thread-safe access to Echo context
type EchoContextManager struct {
	mu         sync.RWMutex
	context    echo.Context
	sessionMu  sync.Mutex
	responseMu sync.Mutex
	requestMu  sync.RWMutex
}

// NewEchoContextManager creates a new thread-safe Echo context wrapper
func NewEchoContextManager(c echo.Context) *EchoContextManager {
	return &EchoContextManager{
		context: c,
	}
}

// SyncContext wraps an Echo context to make it thread-safe
type SyncContext struct {
	manager *EchoContextManager
	echo.Context
}

// Request returns the request with read lock
func (sc *SyncContext) Request() *http.Request {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.Request()
}

// Response returns a thread-safe response wrapper
func (sc *SyncContext) Response() *echo.Response {
	// Return the original response directly
	// The locking is handled by individual methods
	return sc.manager.context.Response()
}

// Get retrieves a value from context with read lock
func (sc *SyncContext) Get(key string) interface{} {
	sc.manager.mu.RLock()
	defer sc.manager.mu.RUnlock()
	return sc.manager.context.Get(key)
}

// Set sets a value in context with write lock
func (sc *SyncContext) Set(key string, val interface{}) {
	sc.manager.mu.Lock()
	defer sc.manager.mu.Unlock()
	sc.manager.context.Set(key, val)
}

// HTML renders HTML with response lock
func (sc *SyncContext) HTML(code int, html string) error {
	sc.manager.responseMu.Lock()
	defer sc.manager.responseMu.Unlock()
	return sc.manager.context.HTML(code, html)
}

// JSON sends JSON response with response lock
func (sc *SyncContext) JSON(code int, i interface{}) error {
	sc.manager.responseMu.Lock()
	defer sc.manager.responseMu.Unlock()
	return sc.manager.context.JSON(code, i)
}

// String sends string response with response lock
func (sc *SyncContext) String(code int, s string) error {
	sc.manager.responseMu.Lock()
	defer sc.manager.responseMu.Unlock()
	return sc.manager.context.String(code, s)
}

// Redirect performs redirect with response lock
func (sc *SyncContext) Redirect(code int, url string) error {
	sc.manager.responseMu.Lock()
	defer sc.manager.responseMu.Unlock()
	return sc.manager.context.Redirect(code, url)
}

// FormValue gets form value with request lock
func (sc *SyncContext) FormValue(name string) string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.FormValue(name)
}

// FormParams gets form params with request lock
func (sc *SyncContext) FormParams() (url.Values, error) {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.FormParams()
}

// QueryParam gets query param with request lock
func (sc *SyncContext) QueryParam(name string) string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.QueryParam(name)
}

// QueryParams gets query params with request lock
func (sc *SyncContext) QueryParams() url.Values {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.QueryParams()
}

// Bind binds request body with request lock
func (sc *SyncContext) Bind(i interface{}) error {
	sc.manager.requestMu.Lock()
	defer sc.manager.requestMu.Unlock()
	return sc.manager.context.Bind(i)
}

// RealIP gets real IP with request lock
func (sc *SyncContext) RealIP() string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.RealIP()
}

// Param gets path param with request lock
func (sc *SyncContext) Param(name string) string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.Param(name)
}

// ParamNames gets param names with request lock
func (sc *SyncContext) ParamNames() []string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.ParamNames()
}

// ParamValues gets param values with request lock
func (sc *SyncContext) ParamValues() []string {
	sc.manager.requestMu.RLock()
	defer sc.manager.requestMu.RUnlock()
	return sc.manager.context.ParamValues()
}

// Path returns the path
func (sc *SyncContext) Path() string {
	return sc.manager.context.Path()
}

// Echo returns the Echo instance
func (sc *SyncContext) Echo() *echo.Echo {
	return sc.manager.context.Echo()
}

// GetContext returns a thread-safe context wrapper
func (m *EchoContextManager) GetContext() echo.Context {
	return &SyncContext{
		manager: m,
		Context: m.context,
	}
}
