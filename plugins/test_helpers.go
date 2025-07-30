package plugins

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cbroglie/mustache"
	"github.com/dop251/goja"
)

// CallHttpClient simulates HTTP client functionality for testing
func CallHttpClient(vm *goja.Runtime, args map[string]interface{}) (interface{}, error) {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required")
	}

	method, ok := args["method"].(string)
	if !ok {
		method = "GET"
	}

	var body io.Reader
	if bodyData, ok := args["body"]; ok {
		bodyBytes, err := json.Marshal(bodyData)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Add headers
	if headers, ok := args["headers"].(map[string]string); ok {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON
	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err != nil {
		// If not JSON, return as string
		jsonBody = string(respBody)
	}

	return map[string]interface{}{
		"statusCode": float64(resp.StatusCode),
		"headers":    resp.Header,
		"body":       jsonBody,
	}, nil
}

// SendEmail simulates email sending for testing
func SendEmail(vm *goja.Runtime, args map[string]interface{}) (interface{}, error) {
	from, _ := args["from"].(string)
	to, _ := args["to"].([]string)
	subject, _ := args["subject"].(string)
	_, _ = args["body"].(string) // body is intentionally not used in test

	if from == "" || len(to) == 0 || subject == "" {
		return nil, fmt.Errorf("from, to, and subject are required")
	}

	// In a real implementation, this would send an email
	// For testing, we just return an error since SMTP is not configured
	return nil, fmt.Errorf("SMTP not configured")
}

// RenderTemplate renders a mustache template for testing
func RenderTemplate(vm *goja.Runtime, args map[string]interface{}) (interface{}, error) {
	template, ok := args["template"].(string)
	if !ok {
		return nil, fmt.Errorf("template is required")
	}

	data := args["data"]
	if data == nil {
		data = make(map[string]interface{})
	}

	result, err := mustache.Render(template, data)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EvaluateRules evaluates a set of rules for testing
func EvaluateRules(vm *goja.Runtime, rules []map[string]interface{}, data map[string]interface{}) interface{} {
	vm.Set("data", data)

	for _, rule := range rules {
		condition, ok := rule["condition"].(string)
		if !ok {
			continue
		}

		result, err := vm.RunString(condition)
		if err != nil {
			continue
		}

		if result.ToBoolean() {
			action, ok := rule["action"].(string)
			if !ok {
				continue
			}

			actionResult, err := vm.RunString(action)
			if err != nil {
				continue
			}

			return actionResult.Export()
		}
	}

	return nil
}

// ConvertToString converts various types to string for testing
func ConvertToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case nil:
		return ""
	default:
		// Try to marshal as JSON
		bytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(bytes)
	}
}

// base64Encode encodes a string to base64
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// base64Decode decodes a base64 string
func base64Decode(s string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
