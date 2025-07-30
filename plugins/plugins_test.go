package plugins

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test HTTP Client Plugin
func TestHTTPClient_GET(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		
		response := map[string]interface{}{
			"status": "success",
			"data":   "test response",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create VM
	vm := goja.New()

	// Test GET request
	args := map[string]interface{}{
		"url": server.URL + "/test",
		"headers": map[string]string{
			"X-Test-Header": "test-value",
		},
	}

	result, err := CallHttpClient(vm, args)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify response
	response, ok := result.(map[string]interface{})
	require.True(t, ok)
	
	body, ok := response["body"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "test response", body["data"])
}

func TestHTTPClient_POST(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		
		// Read body
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		
		var requestBody map[string]interface{}
		err = json.Unmarshal(bodyBytes, &requestBody)
		assert.NoError(t, err)
		assert.Equal(t, "test data", requestBody["data"])
		
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "123",
			"created": true,
		})
	}))
	defer server.Close()

	// Create VM
	vm := goja.New()

	// Test POST request
	args := map[string]interface{}{
		"url":    server.URL + "/create",
		"method": "POST",
		"body": map[string]interface{}{
			"data": "test data",
		},
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}

	result, err := CallHttpClient(vm, args)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify response
	response, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(201), response["statusCode"])
}

func TestHTTPClient_ErrorHandling(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "invalid URL",
			args: map[string]interface{}{
				"url": "not-a-valid-url",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			args: map[string]interface{}{
				"method": "GET",
			},
			wantErr: true,
		},
		{
			name: "connection refused",
			args: map[string]interface{}{
				"url": "http://localhost:99999/should-fail",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CallHttpClient(vm, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// Test Email Plugin
func TestEmail_Send(t *testing.T) {
	// This is a mock test since we can't actually send emails
	// In a real scenario, you'd mock the SMTP server

	vm := goja.New()

	args := map[string]interface{}{
		"from":    "test@example.com",
		"to":      []string{"recipient@example.com"},
		"subject": "Test Email",
		"body":    "This is a test email body",
		"html":    false,
	}

	// Note: This will fail without proper SMTP configuration
	// In production tests, you'd mock the SMTP connection
	_, err := SendEmail(vm, args)
	// We expect an error since SMTP is not configured
	assert.Error(t, err)
}

// Test Template Plugin
func TestTemplate_Render(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple template",
			template: "Hello, {{name}}!",
			data: map[string]interface{}{
				"name": "World",
			},
			want:    "Hello, World!",
			wantErr: false,
		},
		{
			name:     "complex template",
			template: "User: {{user.name}} ({{user.email}})",
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
			want:    "User: John Doe (john@example.com)",
			wantErr: false,
		},
		{
			name:     "list template",
			template: "Items: {{#items}}{{.}}, {{/items}}",
			data: map[string]interface{}{
				"items": []string{"apple", "banana", "orange"},
			},
			want:    "Items: apple, banana, orange, ",
			wantErr: false,
		},
		{
			name:     "conditional template",
			template: "{{#show}}Visible{{/show}}{{^show}}Hidden{{/show}}",
			data: map[string]interface{}{
				"show": false,
			},
			want:    "Hidden",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"template": tt.template,
				"data":     tt.data,
			}

			result, err := RenderTemplate(vm, args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

// Test Goja Extensions
func TestGojaExtensions(t *testing.T) {
	vm := goja.New()

	// Test atob/btoa
	t.Run("base64 encoding", func(t *testing.T) {
		// Set up atob and btoa functions
		vm.Set("btoa", func(s string) string {
			return base64Encode(s)
		})
		vm.Set("atob", func(s string) string {
			decoded, _ := base64Decode(s)
			return decoded
		})

		// Test encoding
		result, err := vm.RunString(`btoa("Hello, World!")`)
		assert.NoError(t, err)
		assert.Equal(t, "SGVsbG8sIFdvcmxkIQ==", result.String())

		// Test decoding
		result, err = vm.RunString(`atob("SGVsbG8sIFdvcmxkIQ==")`)
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!", result.String())
	})

	// Test console methods
	t.Run("console methods", func(t *testing.T) {
		// Capture console output
		var logOutput bytes.Buffer
		
		console := map[string]interface{}{
			"log": func(args ...interface{}) {
				for _, arg := range args {
					logOutput.WriteString(ConvertToString(arg))
					logOutput.WriteString(" ")
				}
				logOutput.WriteString("\n")
			},
		}
		
		vm.Set("console", console)

		// Test console.log
		_, err := vm.RunString(`console.log("Test", 123, true)`)
		assert.NoError(t, err)
		assert.Contains(t, logOutput.String(), "Test 123 true")
	})
}

// Test Rule Engine Plugin
func TestRuleEngine(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name     string
		rules    []map[string]interface{}
		data     map[string]interface{}
		expected interface{}
	}{
		{
			name: "simple rule",
			rules: []map[string]interface{}{
				{
					"condition": "data.age >= 18",
					"action":    "'adult'",
				},
				{
					"condition": "data.age < 18",
					"action":    "'minor'",
				},
			},
			data: map[string]interface{}{
				"age": 25,
			},
			expected: "adult",
		},
		{
			name: "complex rule",
			rules: []map[string]interface{}{
				{
					"condition": "data.score > 90 && data.attendance > 0.9",
					"action":    "'A'",
				},
				{
					"condition": "data.score > 80",
					"action":    "'B'",
				},
				{
					"condition": "true",
					"action":    "'C'",
				},
			},
			data: map[string]interface{}{
				"score":      85,
				"attendance": 0.95,
			},
			expected: "B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EvaluateRules(vm, tt.rules, tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test Type Conversion Utilities
func TestTypeConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "number",
			input:    42,
			expected: "42",
		},
		{
			name:     "float",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
		{
			name: "object",
			input: map[string]interface{}{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
		{
			name:     "array",
			input:    []interface{}{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests
func BenchmarkHTTPClient(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	vm := goja.New()
	args := map[string]interface{}{
		"url": server.URL,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CallHttpClient(vm, args)
	}
}

func BenchmarkTemplateRender(b *testing.B) {
	vm := goja.New()
	args := map[string]interface{}{
		"template": "Hello {{name}}, you have {{count}} messages",
		"data": map[string]interface{}{
			"name":  "User",
			"count": 5,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RenderTemplate(vm, args)
	}
}