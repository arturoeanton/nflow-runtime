package engine

import (
	"bytes"
	"net/http"
	"net/url"
	"sync"

	"github.com/labstack/echo/v4"
)

// IsolatedContext provides a context for goroutines that doesn't share HTTP response
type IsolatedContext struct {
	echo.Context
	request        *http.Request
	responseBuffer *bytes.Buffer
	headers        http.Header
	cookies        []*http.Cookie
	sessionData    map[string]interface{}
	mu             sync.RWMutex
}

// NewIsolatedContext creates a new isolated context from an Echo context
func NewIsolatedContext(c echo.Context) *IsolatedContext {
	// Clone the request
	req := c.Request().Clone(c.Request().Context())
	
	// Create isolated context
	ic := &IsolatedContext{
		Context:        c,
		request:        req,
		responseBuffer: new(bytes.Buffer),
		headers:        make(http.Header),
		cookies:        make([]*http.Cookie, 0),
		sessionData:    make(map[string]interface{}),
	}
	
	// Copy current session data
	if sess, ok := c.Get("_session_data").(map[string]interface{}); ok {
		for k, v := range sess {
			ic.sessionData[k] = v
		}
	}
	
	return ic
}

// Request returns the cloned request
func (ic *IsolatedContext) Request() *http.Request {
	return ic.request
}

// Response returns an isolated response that doesn't affect the original
func (ic *IsolatedContext) Response() *echo.Response {
	// Return a dummy response that doesn't write to the actual HTTP response
	return &echo.Response{
		Writer: &isolatedResponseWriter{
			buffer:  ic.responseBuffer,
			headers: ic.headers,
			cookies: &ic.cookies,
		},
	}
}

// Get retrieves a value from context
func (ic *IsolatedContext) Get(key string) interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	
	// Check session data first
	if key == "_session_data" {
		return ic.sessionData
	}
	
	// Fall back to original context for other values
	return ic.Context.Get(key)
}

// Set sets a value in context
func (ic *IsolatedContext) Set(key string, val interface{}) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	
	// Store session data locally
	if key == "_session_data" {
		if data, ok := val.(map[string]interface{}); ok {
			ic.sessionData = data
		}
		return
	}
	
	// For other values, use original context
	ic.Context.Set(key, val)
}

// FormValue returns form value from the cloned request
func (ic *IsolatedContext) FormValue(name string) string {
	return ic.request.FormValue(name)
}

// FormParams returns form params from the cloned request
func (ic *IsolatedContext) FormParams() (url.Values, error) {
	if ic.request.Form == nil {
		err := ic.request.ParseForm()
		if err != nil {
			return nil, err
		}
	}
	return ic.request.Form, nil
}

// isolatedResponseWriter captures writes without affecting the real response
type isolatedResponseWriter struct {
	buffer  *bytes.Buffer
	headers http.Header
	cookies *[]*http.Cookie
	status  int
}

func (w *isolatedResponseWriter) Header() http.Header {
	return w.headers
}

func (w *isolatedResponseWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *isolatedResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

// GetOutput returns the buffered output
func (ic *IsolatedContext) GetOutput() string {
	return ic.responseBuffer.String()
}

// GetHeaders returns the collected headers
func (ic *IsolatedContext) GetHeaders() http.Header {
	return ic.headers
}

// GetCookies returns the collected cookies
func (ic *IsolatedContext) GetCookies() []*http.Cookie {
	return ic.cookies
}

// HTML writes HTML response to buffer
func (ic *IsolatedContext) HTML(code int, html string) error {
	ic.Response().WriteHeader(code)
	ic.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := ic.responseBuffer.WriteString(html)
	return err
}

// JSON writes JSON response to buffer
func (ic *IsolatedContext) JSON(code int, i interface{}) error {
	// For isolated context, we just log that JSON was called
	// The actual response won't be sent
	return nil
}

// String writes string response to buffer
func (ic *IsolatedContext) String(code int, s string) error {
	ic.Response().WriteHeader(code)
	_, err := ic.responseBuffer.WriteString(s)
	return err
}

// Redirect captures redirect but doesn't execute it
func (ic *IsolatedContext) Redirect(code int, url string) error {
	ic.Response().Header().Set("Location", url)
	ic.Response().WriteHeader(code)
	return nil
}