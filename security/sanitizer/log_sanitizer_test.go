package sanitizer

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestNewLogSanitizer(t *testing.T) {
	// Test with nil config (defaults)
	ls := NewLogSanitizer(nil)
	if ls == nil {
		t.Fatal("Sanitizer should not be nil")
	}
	if !ls.enabled {
		t.Error("Should be enabled by default")
	}
	if ls.maskingChar != "*" {
		t.Error("Default masking char should be *")
	}
	if ls.preserveLength {
		t.Error("Should not preserve length by default")
	}
	if !ls.showType {
		t.Error("Should show type by default")
	}
	
	// Test with custom config
	config := &Config{
		Enabled:        true,
		MaskingChar:    "#",
		PreserveLength: true,
		ShowType:       false,
		CustomPatterns: map[string]string{
			"custom_id": `ID-\d{6}`,
		},
	}
	
	ls2 := NewLogSanitizer(config)
	if ls2.maskingChar != "#" {
		t.Error("Custom masking char not set")
	}
	if !ls2.preserveLength {
		t.Error("Preserve length not set")
	}
	if ls2.showType {
		t.Error("Show type should be false")
	}
	
	// Check custom pattern was added
	if len(ls2.customPatterns) != 1 {
		t.Error("Custom pattern not added")
	}
}

func TestSanitizeEmail(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"User email is john@example.com",
			"User email is [REDACTED:email]",
		},
		{
			"Contact: admin@company.org for help",
			"Contact: [REDACTED:email] for help",
		},
		{
			"Multiple: user1@test.com and user2@test.com",
			"Multiple: [REDACTED:email] and [REDACTED:email]",
		},
		{
			"No email here",
			"No email here",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizePhone(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"Call me at 555-123-4567",
			"Call me at [REDACTED:phone]",
		},
		{
			"Phone: (555) 123-4567",
			"Phone: [REDACTED:phone]",
		},
		{
			"Contact: +1-555-123-4567", 
			"Contact: [REDACTED:phone]",
		},
		{
			"Number is 5551234567",
			"Number is [REDACTED:phone]",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeSSN(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"SSN: 123-45-6789",
			"SSN: [REDACTED:ssn]",
		},
		{
			"User SSN is 987-65-4321",
			"User SSN is [REDACTED:ssn]",
		},
		{
			"No SSN here: 123456789",
			"No SSN here: 123456789",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeAPIKey(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"api_key=sk_test_1234567890abcdefghij",
			"[REDACTED:api_key]",
		},
		{
			"Authorization: api-key: abcdef1234567890abcdef1234567890",
			"Authorization: [REDACTED:api_key]",
		},
		{
			`apiKey: "my_super_secret_key_12345"`,
			`[REDACTED:api_key]`,
		},
		{
			"access_token = 'token_1234567890abcdefghij'",
			"[REDACTED:api_key]",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeJWT(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			fmt.Sprintf("Token: %s", jwt),
			"Token: [REDACTED:jwt]",
		},
		{
			"No JWT here",
			"No JWT here",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeIPAddress(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"Connection from 192.168.1.1",
			"Connection from [REDACTED:ip_address]",
		},
		{
			"IPs: 10.0.0.1 and 172.16.0.1",
			"IPs: [REDACTED:ip_address] and [REDACTED:ip_address]",
		},
		{
			"Not an IP: 999.999.999.999",
			"Not an IP: [REDACTED:ip_address]",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizePassword(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"password: mysecret123",
			"[REDACTED:password]",
		},
		{
			`password: "super_secret_pass"`,
			`[REDACTED:password]`,
		},
		{
			"pwd = 'mypassword'",
			"[REDACTED:password]",
		},
		{
			"Password: too",
			"Password: too", // Too short (less than 4 chars)
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
}

func TestSanitizeMultipleTypes(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	input := "User john@example.com with SSN 123-45-6789 called from 555-123-4567"
	expected := "User [REDACTED:email] with SSN [REDACTED:ssn] called from [REDACTED:phone]"
	
	result := ls.Sanitize(input)
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
}

func TestPreserveLength(t *testing.T) {
	config := &Config{
		Enabled:        true,
		PreserveLength: true,
		ShowType:       false,
	}
	ls := NewLogSanitizer(config)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{
			"Email: test@example.com",
			"Email: ****************",
		},
		{
			"SSN: 123-45-6789",
			"SSN: ***********",
		},
	}
	
	for _, tc := range testCases {
		result := ls.Sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("Input: %s\nExpected: %s\nGot: %s", tc.input, tc.expected, result)
		}
	}
	
	// Test with showType enabled
	ls.showType = true
	result := ls.Sanitize("test@example.com")
	expected := "[email:****************]"
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
}

func TestCustomMaskingChar(t *testing.T) {
	config := &Config{
		Enabled:        true,
		MaskingChar:    "#",
		PreserveLength: true,
		ShowType:       false,
	}
	ls := NewLogSanitizer(config)
	
	result := ls.Sanitize("test@example.com")
	expected := "################"
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
}

func TestSanitizeDisabled(t *testing.T) {
	config := &Config{
		Enabled: false,
	}
	ls := NewLogSanitizer(config)
	
	input := "Email: test@example.com, SSN: 123-45-6789"
	result := ls.Sanitize(input)
	
	if result != input {
		t.Error("Sanitizer should not modify input when disabled")
	}
	
	// Check metrics
	processed, sanitized := ls.GetMetrics()
	if processed != 0 || sanitized != 0 {
		t.Error("No metrics should be recorded when disabled")
	}
}

func TestSanitizeMap(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	input := map[string]interface{}{
		"email":    "test@example.com",
		"phone":    "555-123-4567",
		"message":  "Contact user",
		"metadata": map[string]interface{}{
			"ssn":      "123-45-6789",
			"api_key":  "api_key=secret123456789012345",
			"safe":     "no sensitive data here",
		},
		"numbers": []interface{}{
			"IP: 192.168.1.1",
			"Port: 8080",
		},
	}
	
	result := ls.SanitizeMap(input)
	
	// Check sanitization
	if result["email"] != "[REDACTED:email]" {
		t.Error("Email not sanitized")
	}
	if result["phone"] != "[REDACTED:phone]" {
		t.Error("Phone not sanitized")
	}
	if result["message"] != "Contact user" {
		t.Error("Safe message was modified")
	}
	
	// Check nested map
	metadata := result["metadata"].(map[string]interface{})
	if metadata["ssn"] != "[REDACTED:ssn]" {
		t.Error("Nested SSN not sanitized")
	}
	if metadata["api_key"] != "[REDACTED:api_key]" {
		t.Error("Nested API key not sanitized")
	}
	if metadata["safe"] != "no sensitive data here" {
		t.Error("Safe nested data was modified")
	}
	
	// Check slice
	numbers := result["numbers"].([]interface{})
	if numbers[0] != "IP: [REDACTED:ip_address]" {
		t.Error("IP in slice not sanitized")
	}
	if numbers[1] != "Port: 8080" {
		t.Error("Safe slice item was modified")
	}
}

func TestAddCustomPattern(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	// Add custom pattern
	err := ls.AddCustomPattern("employee_id", `EMP-\d{6}`)
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}
	
	// Test custom pattern
	input := "Employee ID: EMP-123456"
	expected := "Employee ID: [REDACTED:employee_id]"
	result := ls.Sanitize(input)
	
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
	
	// Test invalid pattern
	err = ls.AddCustomPattern("invalid", "[")
	if err == nil {
		t.Error("Should error on invalid regex")
	}
	
	// Remove pattern
	removed := ls.RemoveCustomPattern("employee_id")
	if !removed {
		t.Error("Failed to remove pattern")
	}
	
	// Pattern should no longer match
	result = ls.Sanitize(input)
	if result != input {
		t.Error("Pattern should not match after removal")
	}
	
	// Try to remove non-existent pattern
	removed = ls.RemoveCustomPattern("non_existent")
	if removed {
		t.Error("Should not remove non-existent pattern")
	}
}

func TestMetrics(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	// Initial metrics
	processed, sanitized := ls.GetMetrics()
	if processed != 0 || sanitized != 0 {
		t.Error("Initial metrics should be zero")
	}
	
	// Process some logs
	ls.Sanitize("Email: test@example.com")
	ls.Sanitize("Phone: 555-123-4567")
	ls.Sanitize("No sensitive data")
	ls.Sanitize("Multiple: user@test.com and 123-45-6789")
	
	processed, sanitized = ls.GetMetrics()
	if processed != 4 {
		t.Errorf("Expected 4 logs processed, got %d", processed)
	}
	if sanitized != 4 { // 1 + 1 + 0 + 2
		t.Errorf("Expected 4 data sanitized, got %d", sanitized)
	}
	
	// Reset metrics
	ls.ResetMetrics()
	processed, sanitized = ls.GetMetrics()
	if processed != 0 || sanitized != 0 {
		t.Error("Metrics should be zero after reset")
	}
}

func TestBatchSanitize(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	// Small batch (sequential processing)
	smallBatch := []string{
		"Email: test@example.com",
		"Phone: 555-123-4567",
		"Safe message",
		"SSN: 123-45-6789",
	}
	
	results := ls.BatchSanitize(smallBatch)
	if len(results) != len(smallBatch) {
		t.Error("Result count mismatch")
	}
	
	expected := []string{
		"Email: [REDACTED:email]",
		"Phone: [REDACTED:phone]",
		"Safe message",
		"SSN: [REDACTED:ssn]",
	}
	
	for i, result := range results {
		if result != expected[i] {
			t.Errorf("Batch[%d]: Expected %s, got %s", i, expected[i], result)
		}
	}
	
	// Large batch (parallel processing)
	largeBatch := make([]string, 200)
	for i := range largeBatch {
		if i%2 == 0 {
			largeBatch[i] = fmt.Sprintf("Email: user%d@example.com", i)
		} else {
			largeBatch[i] = "Safe log message"
		}
	}
	
	results = ls.BatchSanitize(largeBatch)
	if len(results) != len(largeBatch) {
		t.Error("Large batch result count mismatch")
	}
	
	// Verify results
	for i, result := range results {
		if i%2 == 0 {
			if !strings.Contains(result, "[REDACTED:email]") {
				t.Errorf("Batch[%d]: Email not sanitized", i)
			}
		} else {
			if result != "Safe log message" {
				t.Errorf("Batch[%d]: Safe message was modified", i)
			}
		}
	}
}

func TestShouldSanitize(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	testCases := []struct {
		input    string
		expected bool
	}{
		{"Email: test@example.com", true},
		{"Phone: 555-123-4567", true},
		{"Safe log message", false},
		{"SSN: 123-45-6789", true},
		{"", false},
	}
	
	for _, tc := range testCases {
		result := ls.ShouldSanitize(tc.input)
		if result != tc.expected {
			t.Errorf("ShouldSanitize(%s) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
	
	// Test when disabled
	ls.SetEnabled(false)
	if ls.ShouldSanitize("test@example.com") {
		t.Error("Should return false when disabled")
	}
}

func TestSetEnabled(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	// Initially enabled
	if !ls.IsEnabled() {
		t.Error("Should be enabled by default")
	}
	
	// Disable
	ls.SetEnabled(false)
	if ls.IsEnabled() {
		t.Error("Should be disabled")
	}
	
	// Enable again
	ls.SetEnabled(true)
	if !ls.IsEnabled() {
		t.Error("Should be enabled")
	}
}

func TestConcurrency(t *testing.T) {
	ls := NewLogSanitizer(nil)
	
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	
	// Concurrent sanitization
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 100; j++ {
				input := fmt.Sprintf("Worker %d: email%d@test.com", id, j)
				result := ls.Sanitize(input)
				
				if !strings.Contains(result, "[REDACTED:email]") {
					errors <- fmt.Errorf("Worker %d: Email not sanitized", id)
					return
				}
			}
		}(i)
	}
	
	// Concurrent pattern operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			patternName := fmt.Sprintf("pattern_%d", id)
			ls.AddCustomPattern(patternName, fmt.Sprintf(`CUSTOM-%d-\d+`, id))
			
			// Test the pattern
			input := fmt.Sprintf("ID: CUSTOM-%d-12345", id)
			result := ls.Sanitize(input)
			
			if !strings.Contains(result, "REDACTED") {
				errors <- fmt.Errorf("Custom pattern %d not working", id)
			}
			
			ls.RemoveCustomPattern(patternName)
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Error(err)
	}
	
	// Verify metrics are consistent
	processed, _ := ls.GetMetrics()
	if processed < 1000 { // At least 10 * 100 from first loop
		t.Errorf("Expected at least 1000 processed, got %d", processed)
	}
}

// Benchmark tests
func BenchmarkSanitize_NoMatch(b *testing.B) {
	ls := NewLogSanitizer(nil)
	input := "This is a safe log message with no sensitive data"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.Sanitize(input)
	}
}

func BenchmarkSanitize_SingleMatch(b *testing.B) {
	ls := NewLogSanitizer(nil)
	input := "User email: test@example.com"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.Sanitize(input)
	}
}

func BenchmarkSanitize_MultipleMatches(b *testing.B) {
	ls := NewLogSanitizer(nil)
	input := "User test@example.com with SSN 123-45-6789 called from 555-123-4567 using API key=secret123456789012345"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.Sanitize(input)
	}
}

func BenchmarkBatchSanitize_Small(b *testing.B) {
	ls := NewLogSanitizer(nil)
	batch := []string{
		"Email: test@example.com",
		"Safe message",
		"Phone: 555-123-4567",
		"Another safe message",
		"SSN: 123-45-6789",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.BatchSanitize(batch)
	}
}

func BenchmarkBatchSanitize_Large(b *testing.B) {
	ls := NewLogSanitizer(nil)
	batch := make([]string, 200)
	for i := range batch {
		if i%3 == 0 {
			batch[i] = fmt.Sprintf("Email: user%d@example.com", i)
		} else if i%3 == 1 {
			batch[i] = fmt.Sprintf("Phone: 555-%03d-%04d", i%1000, i%10000)
		} else {
			batch[i] = "Safe log message"
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.BatchSanitize(batch)
	}
}

func BenchmarkConcurrentSanitize(b *testing.B) {
	ls := NewLogSanitizer(nil)
	input := "User test@example.com with phone 555-123-4567"
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ls.Sanitize(input)
		}
	})
}