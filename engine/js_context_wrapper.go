package engine

import (
	"fmt"
	"io"
	"log"
	"net/http"
	
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
)

// JSContextWrapper wraps an Echo context to expose its methods to JavaScript
type JSContextWrapper struct {
	context echo.Context
}

// NewJSContextWrapper creates a wrapper that exposes Echo context methods to JavaScript
func NewJSContextWrapper(c echo.Context) *JSContextWrapper {
	return &JSContextWrapper{context: c}
}

// JSON sends a JSON response - exposed to JavaScript
func (w *JSContextWrapper) JSON(code int, i interface{}) error {
	return w.context.JSON(code, i)
}

// String sends a string response - exposed to JavaScript  
func (w *JSContextWrapper) String(code int, s string) error {
	return w.context.String(code, s)
}

// Redirect performs an HTTP redirect - exposed to JavaScript
func (w *JSContextWrapper) Redirect(code int, url string) error {
	return w.context.Redirect(code, url)
}

// Get retrieves data from context - exposed to JavaScript
func (w *JSContextWrapper) Get(key string) interface{} {
	return w.context.Get(key)
}

// Set saves data in context - exposed to JavaScript
func (w *JSContextWrapper) Set(key string, val interface{}) {
	w.context.Set(key, val)
}

// Request returns the HTTP request - exposed to JavaScript
func (w *JSContextWrapper) Request() interface{} {
	return w.context.Request()
}

// Response returns the HTTP response - exposed to JavaScript
func (w *JSContextWrapper) Response() interface{} {
	return w.context.Response()
}

// Param returns path parameter by name - exposed to JavaScript
func (w *JSContextWrapper) Param(name string) string {
	return w.context.Param(name)
}

// QueryParam returns query parameter by name - exposed to JavaScript
func (w *JSContextWrapper) QueryParam(name string) string {
	return w.context.QueryParam(name)
}

// FormValue returns form value by name - exposed to JavaScript
func (w *JSContextWrapper) FormValue(name string) string {
	return w.context.FormValue(name)
}

// SetupJSContext configures the JavaScript VM with the wrapped context
func SetupJSContext(vm *goja.Runtime, c echo.Context) {
	// Create a JavaScript object with all Echo context methods
	obj := vm.NewObject()
	
	// Response methods
	obj.Set("JSON", func(code int, data interface{}) error {
		return c.JSON(code, data)
	})
	
	obj.Set("JSONPretty", func(code int, data interface{}, indent string) error {
		return c.JSONPretty(code, data, indent)
	})
	
	obj.Set("JSONP", func(code int, callback string, data interface{}) error {
		return c.JSONP(code, callback, data)
	})
	
	obj.Set("JSONBlob", func(code int, b []byte) error {
		return c.JSONBlob(code, b)
	})
	
	obj.Set("String", func(code int, s string) error {
		return c.String(code, s)
	})
	
	obj.Set("HTML", func(code int, html string) error {
		return c.HTML(code, html)
	})
	
	obj.Set("HTMLBlob", func(code int, b []byte) error {
		return c.HTMLBlob(code, b)
	})
	
	obj.Set("XML", func(code int, data interface{}) error {
		return c.XML(code, data)
	})
	
	obj.Set("XMLPretty", func(code int, data interface{}, indent string) error {
		return c.XMLPretty(code, data, indent)
	})
	
	obj.Set("XMLBlob", func(code int, b []byte) error {
		return c.XMLBlob(code, b)
	})
	
	obj.Set("Blob", func(code int, contentType string, b []byte) error {
		return c.Blob(code, contentType, b)
	})
	
	obj.Set("Stream", func(code int, contentType string, r interface{}) error {
		// Note: r should be io.Reader but interface{} for JS compatibility
		if reader, ok := r.(io.Reader); ok {
			return c.Stream(code, contentType, reader)
		}
		return fmt.Errorf("invalid reader type")
	})
	
	obj.Set("File", func(file string) error {
		return c.File(file)
	})
	
	obj.Set("Attachment", func(file string, name string) error {
		return c.Attachment(file, name)
	})
	
	obj.Set("Inline", func(file string, name string) error {
		return c.Inline(file, name)
	})
	
	obj.Set("NoContent", func(code int) error {
		return c.NoContent(code)
	})
	
	obj.Set("Redirect", func(code int, url string) error {
		return c.Redirect(code, url)
	})
	
	obj.Set("Error", func(err error) {
		if err != nil {
			c.Error(err)
		}
	})
	
	// Request methods
	obj.Set("Request", func() interface{} {
		return c.Request()
	})
	
	obj.Set("SetRequest", func(r interface{}) {
		if req, ok := r.(*http.Request); ok {
			c.SetRequest(req)
		}
	})
	
	obj.Set("Response", func() interface{} {
		return c.Response()
	})
	
	obj.Set("IsTLS", func() bool {
		return c.IsTLS()
	})
	
	obj.Set("IsWebSocket", func() bool {
		return c.IsWebSocket()
	})
	
	obj.Set("Scheme", func() string {
		return c.Scheme()
	})
	
	obj.Set("RealIP", func() string {
		return c.RealIP()
	})
	
	obj.Set("Path", func() string {
		return c.Path()
	})
	
	obj.Set("SetPath", func(p string) {
		c.SetPath(p)
	})
	
	obj.Set("Param", func(name string) string {
		return c.Param(name)
	})
	
	obj.Set("ParamNames", func() []string {
		return c.ParamNames()
	})
	
	obj.Set("SetParamNames", func(names ...string) {
		c.SetParamNames(names...)
	})
	
	obj.Set("ParamValues", func() []string {
		return c.ParamValues()
	})
	
	obj.Set("SetParamValues", func(values ...string) {
		c.SetParamValues(values...)
	})
	
	obj.Set("QueryParam", func(name string) string {
		return c.QueryParam(name)
	})
	
	obj.Set("QueryParams", func() interface{} {
		return c.QueryParams()
	})
	
	obj.Set("QueryString", func() string {
		return c.QueryString()
	})
	
	obj.Set("FormValue", func(name string) string {
		return c.FormValue(name)
	})
	
	obj.Set("FormParams", func() (interface{}, error) {
		return c.FormParams()
	})
	
	obj.Set("FormFile", func(name string) (interface{}, error) {
		return c.FormFile(name)
	})
	
	obj.Set("MultipartForm", func() (interface{}, error) {
		return c.MultipartForm()
	})
	
	obj.Set("Cookie", func(name string) (interface{}, error) {
		return c.Cookie(name)
	})
	
	obj.Set("SetCookie", func(cookie interface{}) {
		if ck, ok := cookie.(*http.Cookie); ok {
			c.SetCookie(ck)
		}
	})
	
	obj.Set("Cookies", func() []*http.Cookie {
		return c.Cookies()
	})
	
	// Context storage methods
	obj.Set("Get", func(key string) interface{} {
		return c.Get(key)
	})
	
	obj.Set("Set", func(key string, val interface{}) {
		c.Set(key, val)
	})
	
	obj.Set("Bind", func(i interface{}) error {
		return c.Bind(i)
	})
	
	obj.Set("Validate", func(i interface{}) error {
		return c.Validate(i)
	})
	
	obj.Set("Render", func(code int, name string, data interface{}) error {
		return c.Render(code, name, data)
	})
	
	// Logger method
	obj.Set("Logger", func() interface{} {
		return c.Logger()
	})
	
	// Echo instance
	obj.Set("Echo", func() interface{} {
		return c.Echo()
	})
	
	// Handler method
	obj.Set("Handler", func() interface{} {
		return c.Handler()
	})
	
	obj.Set("SetHandler", func(h interface{}) {
		if handler, ok := h.(echo.HandlerFunc); ok {
			c.SetHandler(handler)
		}
	})
	
	// Additional helper for getting request headers
	obj.Set("GetHeader", func(key string) string {
		return c.Request().Header.Get(key)
	})
	
	// Additional helper for setting response headers
	obj.Set("SetHeader", func(key, value string) {
		c.Response().Header().Set(key, value)
	})
	
	vm.Set("c", obj)
	vm.Set("echo_context", obj)
	
	// Debug: verify the object is set correctly
	val := vm.Get("c")
	if val == nil || val == goja.Undefined() || val == goja.Null() {
		log.Printf("[JS Context] WARNING: 'c' was not set properly in VM")
	} else {
		log.Printf("[JS Context] 'c' set successfully, type: %T", val.Export())
		// Check if JSON method exists
		jsonMethod := vm.Get("c").ToObject(vm).Get("JSON")
		if jsonMethod == nil || jsonMethod == goja.Undefined() {
			log.Printf("[JS Context] WARNING: JSON method not found on c object")
		} else {
			log.Printf("[JS Context] JSON method found on c object")
		}
	}
}