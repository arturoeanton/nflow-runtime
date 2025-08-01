package interceptor

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	
	"github.com/arturoeanton/nflow-runtime/security/encryption"
)

func setupInterceptor(t testing.TB, config *Config) *SensitiveDataInterceptor {
	key := strings.Repeat("k", 32)
	encService, err := encryption.NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}
	
	if config == nil {
		config = &Config{
			Enabled:        true,
			EncryptInPlace: true,
			MetadataKey:    "_encrypted",
		}
	}
	
	return NewSensitiveDataInterceptor(encService, config)
}

func TestNewSensitiveDataInterceptor(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	if interceptor == nil {
		t.Fatal("Interceptor should not be nil")
	}
	
	// Check default patterns are loaded
	if len(interceptor.patterns) == 0 {
		t.Error("Default patterns should be loaded")
	}
	
	// Verify key patterns exist
	expectedPatterns := []PatternType{
		PatternEmail,
		PatternPhone,
		PatternSSN,
		PatternCreditCard,
		PatternAPIKey,
		PatternJWT,
	}
	
	for _, pt := range expectedPatterns {
		if _, exists := interceptor.patterns[pt]; !exists {
			t.Errorf("Pattern %s should exist", pt)
		}
	}
}

func TestDetectEmail(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	testCases := []struct {
		input    string
		expected int
	}{
		{"user@example.com", 1},
		{"Contact: john.doe@company.org", 1},
		{"Multiple: alice@test.com and bob@test.com", 2},
		{"Not an email: user@", 0},
		{"Also not: @example.com", 0},
		{"test.email+tag@sub.domain.com", 1},
	}
	
	for _, tc := range testCases {
		detections := interceptor.detectSensitiveData(tc.input)
		emailCount := 0
		for _, d := range detections {
			if d.Type == PatternEmail {
				emailCount++
			}
		}
		
		if emailCount != tc.expected {
			t.Errorf("Input %q: expected %d emails, got %d", tc.input, tc.expected, emailCount)
		}
	}
}

func TestDetectPhone(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	testCases := []struct {
		input    string
		expected int
	}{
		{"Call me at 123-456-7890", 1},
		{"(555) 123-4567", 1},
		{"+1 234 567 8900", 1},
		{"5551234567", 1},
		{"Multiple: 111-222-3333 and (444) 555-6666", 2},
		{"Not a phone: 123-45-6789", 0}, // SSN format
		{"Too short: 123-456", 0},
	}
	
	for _, tc := range testCases {
		detections := interceptor.detectSensitiveData(tc.input)
		phoneCount := 0
		for _, d := range detections {
			if d.Type == PatternPhone {
				phoneCount++
			}
		}
		
		if phoneCount != tc.expected {
			t.Errorf("Input %q: expected %d phones, got %d", tc.input, tc.expected, phoneCount)
		}
	}
}

func TestDetectSSN(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	testCases := []struct {
		input    string
		expected int
	}{
		{"SSN: 123-45-6789", 1},
		{"Invalid: 123-456-789", 0},
		{"Also invalid: 1234-56-789", 0},
		{"Two SSNs: 111-22-3333 and 444-55-6666", 2},
	}
	
	for _, tc := range testCases {
		detections := interceptor.detectSensitiveData(tc.input)
		ssnCount := 0
		for _, d := range detections {
			if d.Type == PatternSSN {
				ssnCount++
			}
		}
		
		if ssnCount != tc.expected {
			t.Errorf("Input %q: expected %d SSNs, got %d", tc.input, tc.expected, ssnCount)
		}
	}
}

func TestDetectAPIKey(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	testCases := []struct {
		input    string
		expected int
	}{
		{`api_key: "sk_test_1234567890abcdefghij"`, 1},
		{`"apiKey": "1234567890123456789012345678901234567890"`, 1},
		{`access_token=abcdefghijklmnopqrstuvwxyz123456`, 1},
		{`auth-token: 'short'`, 0}, // Too short
		{`Multiple: api_key=key1234567890123456 and api_key=key0987654321098765`, 2},
	}
	
	for _, tc := range testCases {
		detections := interceptor.detectSensitiveData(tc.input)
		apiCount := 0
		for _, d := range detections {
			if d.Type == PatternAPIKey {
				apiCount++
			}
		}
		
		if apiCount != tc.expected {
			t.Errorf("Input %q: expected %d API keys, got %d", tc.input, tc.expected, apiCount)
		}
	}
}

func TestDetectJWT(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	testCases := []struct {
		input    string
		expected int
	}{
		{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", 1},
		{"token: header.payload.signature", 0}, // Too short
		{"Valid JWT: aaaaaaaaaa.bbbbbbbbbb.cccccccccc", 1},
	}
	
	for _, tc := range testCases {
		detections := interceptor.detectSensitiveData(tc.input)
		jwtCount := 0
		for _, d := range detections {
			if d.Type == PatternJWT {
				jwtCount++
			}
		}
		
		if jwtCount != tc.expected {
			t.Errorf("Input %q: expected %d JWTs, got %d", tc.input, tc.expected, jwtCount)
		}
	}
}

func TestProcessResponseInPlace(t *testing.T) {
	interceptor := setupInterceptor(t, &Config{
		Enabled:        true,
		EncryptInPlace: true,
	})
	
	// Test with map data
	data := map[string]interface{}{
		"name":  "John Doe",
		"email": "john.doe@example.com",
		"phone": "555-123-4567",
		"ssn":   "123-45-6789",
		"note":  "Contact via email or phone",
	}
	
	result, err := interceptor.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	
	// Convert result to JSON to check
	jsonResult, _ := json.Marshal(result)
	strResult := string(jsonResult)
	
	// Check that sensitive data was encrypted
	if strings.Contains(strResult, "john.doe@example.com") {
		t.Error("Email should be encrypted")
	}
	if strings.Contains(strResult, "555-123-4567") {
		t.Error("Phone should be encrypted")
	}
	if strings.Contains(strResult, "123-45-6789") {
		t.Error("SSN should be encrypted")
	}
	
	// Check that encrypted markers exist
	if !strings.Contains(strResult, "[ENCRYPTED_email:") {
		t.Error("Email encryption marker not found")
	}
	if !strings.Contains(strResult, "[ENCRYPTED_phone:") {
		t.Error("Phone encryption marker not found")
	}
	if !strings.Contains(strResult, "[ENCRYPTED_ssn:") {
		t.Error("SSN encryption marker not found")
	}
	
	// Check that non-sensitive data is unchanged
	if !strings.Contains(strResult, "John Doe") {
		t.Error("Name should not be encrypted")
	}
}

func TestProcessResponseWithMetadata(t *testing.T) {
	interceptor := setupInterceptor(t, &Config{
		Enabled:        true,
		EncryptInPlace: false,
		MetadataKey:    "_encrypted_fields",
	})
	
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "Jane Smith",
			"email": "jane@example.org",
			"phone": "111-222-3333",
		},
		"message": "Contact jane@example.org for details",
	}
	
	result, err := interceptor.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}
	
	// Check metadata exists
	metadata, exists := resultMap["_encrypted_fields"]
	if !exists {
		t.Fatal("Encryption metadata should exist")
	}
	
	// Check metadata content
	metadataSlice, ok := metadata.([]Detection)
	if !ok {
		t.Fatal("Metadata should be []Detection")
	}
	
	if len(metadataSlice) < 2 {
		t.Errorf("Expected at least 2 detections, got %d", len(metadataSlice))
	}
	
	// Original data should be unchanged
	userMap := resultMap["user"].(map[string]interface{})
	if userMap["email"] != "jane@example.org" {
		t.Error("Original email should be unchanged")
	}
}

func TestProcessResponseDisabled(t *testing.T) {
	interceptor := setupInterceptor(t, &Config{
		Enabled: false,
	})
	
	data := map[string]interface{}{
		"email": "test@example.com",
		"ssn":   "123-45-6789",
	}
	
	result, err := interceptor.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	
	// Data should be unchanged
	resultMap := result.(map[string]interface{})
	if resultMap["email"] != "test@example.com" {
		t.Error("Email should be unchanged when interceptor is disabled")
	}
	if resultMap["ssn"] != "123-45-6789" {
		t.Error("SSN should be unchanged when interceptor is disabled")
	}
}

func TestCustomPatterns(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	// Add custom pattern for employee IDs
	err := interceptor.AddCustomPattern("employee_id", `EMP\d{6}`)
	if err != nil {
		t.Fatalf("Failed to add custom pattern: %v", err)
	}
	
	// Test detection
	data := "Employee ID: EMP123456, Email: test@example.com"
	detections := interceptor.detectSensitiveData(data)
	
	foundEmployeeID := false
	foundEmail := false
	
	for _, d := range detections {
		if d.Type == PatternCustom && strings.Contains(d.Value, "EMP") {
			foundEmployeeID = true
		}
		if d.Type == PatternEmail {
			foundEmail = true
		}
	}
	
	if !foundEmployeeID {
		t.Error("Custom employee ID pattern should be detected")
	}
	if !foundEmail {
		t.Error("Email should still be detected with custom patterns")
	}
	
	// Remove custom pattern
	removed := interceptor.RemoveCustomPattern("employee_id")
	if !removed {
		t.Error("Should successfully remove custom pattern")
	}
	
	// Test it's no longer detected
	detections = interceptor.detectSensitiveData(data)
	for _, d := range detections {
		if d.Type == PatternCustom && strings.Contains(d.Value, "EMP") {
			t.Error("Removed pattern should not be detected")
		}
	}
}

func TestComplexJSON(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	// Complex nested structure
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":    1,
				"email": "user1@example.com",
				"profile": map[string]interface{}{
					"phone": "111-111-1111",
					"ssn":   "111-11-1111",
				},
			},
			map[string]interface{}{
				"id":    2,
				"email": "user2@example.com",
				"profile": map[string]interface{}{
					"phone": "222-222-2222",
					"ssn":   "222-22-2222",
				},
			},
		},
		"api_config": map[string]interface{}{
			"api_key": "sk_test_1234567890abcdefghijklmnop",
			"endpoint": "https://api.example.com",
		},
	}
	
	result, err := interceptor.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	
	// Convert to JSON to verify encryption
	jsonResult, _ := json.Marshal(result)
	strResult := string(jsonResult)
	
	// Check all sensitive data was encrypted
	sensitiveValues := []string{
		"user1@example.com",
		"user2@example.com",
		"111-111-1111",
		"222-222-2222",
		"111-11-1111",
		"222-22-2222",
		"sk_test_1234567890abcdefghijklmnop",
	}
	
	for _, value := range sensitiveValues {
		if strings.Contains(strResult, value) {
			t.Errorf("Sensitive value %q should be encrypted", value)
		}
	}
	
	// Non-sensitive data should remain
	if !strings.Contains(strResult, "https://api.example.com") {
		t.Error("Non-sensitive URL should remain unchanged")
	}
}

func TestMetrics(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	// Reset metrics
	interceptor.ResetMetrics()
	
	// Process some data
	data := map[string]interface{}{
		"email": "test@example.com",
		"phone": "123-456-7890",
		"text":  "Normal text without sensitive data",
	}
	
	_, err := interceptor.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	
	// Check metrics
	detections, encryptions := interceptor.GetMetrics()
	if detections < 2 {
		t.Errorf("Expected at least 2 detections, got %d", detections)
	}
	if encryptions < 2 {
		t.Errorf("Expected at least 2 encryptions, got %d", encryptions)
	}
}

func TestConcurrency(t *testing.T) {
	interceptor := setupInterceptor(t, nil)
	
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	
	// Run concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			data := map[string]interface{}{
				"email": fmt.Sprintf("user%d@example.com", id),
				"phone": fmt.Sprintf("%03d-456-7890", id),
			}
			
			for j := 0; j < 10; j++ {
				_, err := interceptor.ProcessResponse(data)
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

// Benchmark tests
func BenchmarkDetectSensitiveData(b *testing.B) {
	interceptor := setupInterceptor(b, nil)
	data := `{
		"user": {
			"name": "John Doe",
			"email": "john.doe@example.com",
			"phone": "555-123-4567",
			"ssn": "123-45-6789"
		}
	}`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor.detectSensitiveData(data)
	}
}

func BenchmarkProcessResponse(b *testing.B) {
	interceptor := setupInterceptor(b, nil)
	data := map[string]interface{}{
		"email": "test@example.com",
		"phone": "555-123-4567",
		"ssn":   "123-45-6789",
		"text":  "Some regular text content",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := interceptor.ProcessResponse(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentProcess(b *testing.B) {
	interceptor := setupInterceptor(b, nil)
	data := map[string]interface{}{
		"email": "test@example.com",
		"phone": "555-123-4567",
	}
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := interceptor.ProcessResponse(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}