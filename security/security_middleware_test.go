package security

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestNewSecurityMiddleware(t *testing.T) {
	// Test with nil config (defaults)
	sm, err := NewSecurityMiddleware(nil)
	if err != nil {
		t.Fatalf("Failed to create middleware with defaults: %v", err)
	}
	if sm == nil {
		t.Fatal("Middleware should not be nil")
	}

	// Test with custom config
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  true,
		EncryptSensitiveData: true,
	}

	sm2, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware with config: %v", err)
	}
	if sm2 == nil {
		t.Fatal("Middleware should not be nil")
	}

	// Test with invalid encryption key
	config.EncryptionKey = ""
	sm3, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Should create middleware even without encryption key: %v", err)
	}
	if sm3.encryption != nil {
		t.Error("Encryption should not be initialized without key")
	}
}

func TestAnalyzeScript(t *testing.T) {
	config := &Config{
		EnableStaticAnalysis: true,
		BlockOnHighSeverity:  true,
		LogSecurityWarnings:  true,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	testCases := []struct {
		name      string
		script    string
		shouldErr bool
	}{
		{
			name:      "Safe script",
			script:    `console.log("Hello world");`,
			shouldErr: false,
		},
		{
			name:      "Script with eval",
			script:    `eval("dangerous code");`,
			shouldErr: true,
		},
		{
			name:      "Script with file system access",
			script:    `const fs = require('fs');`,
			shouldErr: true,
		},
		{
			name:      "Multiple issues",
			script:    `eval(code); require('fs'); while(true) {}`,
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := sm.AnalyzeScript(tc.script, tc.name)
			if tc.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}

	// Check metrics
	metrics := sm.GetMetrics()
	if metrics.ScriptsAnalyzed != 4 {
		t.Errorf("Expected 4 scripts analyzed, got %d", metrics.ScriptsAnalyzed)
	}
	if metrics.ScriptsBlocked != 3 {
		t.Errorf("Expected 3 scripts blocked, got %d", metrics.ScriptsBlocked)
	}
	if metrics.HighSeverityIssues == 0 {
		t.Error("Expected high severity issues to be recorded")
	}
}

func TestAnalyzeScriptDisabled(t *testing.T) {
	config := &Config{
		EnableStaticAnalysis: false,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Should not error even with dangerous script
	err = sm.AnalyzeScript(`eval("dangerous");`, "test")
	if err != nil {
		t.Error("Analysis should be skipped when disabled")
	}

	metrics := sm.GetMetrics()
	if metrics.ScriptsAnalyzed != 0 {
		t.Error("No scripts should be analyzed when disabled")
	}
}

func TestProcessResponse(t *testing.T) {
	config := &Config{
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		EncryptSensitiveData: true,
		EncryptInPlace:       true,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Test data with sensitive information
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"phone": "555-123-4567",
		},
		"apiKey": "sk_test_1234567890abcdefghij",
	}

	result, err := sm.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Verify result is modified
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Check metrics
	metrics := sm.GetMetrics()
	if metrics.DataEncrypted == 0 {
		t.Error("Expected data encryption to be recorded")
	}
}

func TestProcessResponseDisabled(t *testing.T) {
	config := &Config{
		EnableEncryption: false,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	data := map[string]interface{}{
		"email": "test@example.com",
	}

	result, err := sm.ProcessResponse(data)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Data should be unchanged
	resultMap := result.(map[string]interface{})
	if resultMap["email"] != "test@example.com" {
		t.Error("Data should be unchanged when encryption is disabled")
	}
}

func TestEncryptDecryptField(t *testing.T) {
	config := &Config{
		EnableEncryption:    true,
		EncryptionKey:       strings.Repeat("k", 32),
		AlwaysEncryptFields: []string{"password", "secret", "api_key"},
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Test field that should be encrypted
	encrypted, err := sm.EncryptField("password", "mysecretpass")
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	if encrypted == "mysecretpass" {
		t.Error("Password field should be encrypted")
	}

	// Test field that should not be encrypted
	notEncrypted, err := sm.EncryptField("username", "john")
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}
	if notEncrypted != "john" {
		t.Error("Username field should not be encrypted")
	}

	// Test decryption
	decrypted, err := sm.DecryptField(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	if decrypted != "mysecretpass" {
		t.Error("Decrypted value doesn't match original")
	}

	// Test decryption of non-encrypted value
	plain, err := sm.DecryptField("plain text")
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}
	if plain != "plain text" {
		t.Error("Plain text should remain unchanged")
	}
}

func TestMetrics(t *testing.T) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  true,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Reset metrics
	sm.ResetMetrics()

	// Perform some operations
	sm.AnalyzeScript(`console.log("safe");`, "test1")
	sm.AnalyzeScript(`eval("dangerous");`, "test2")
	sm.AnalyzeScript(`require('fs');`, "test3")

	sm.ProcessResponse(map[string]interface{}{
		"email": "test@example.com",
	})

	// Get metrics
	metrics := sm.GetMetrics()

	if metrics.ScriptsAnalyzed != 3 {
		t.Errorf("Expected 3 scripts analyzed, got %d", metrics.ScriptsAnalyzed)
	}
	if metrics.ScriptsBlocked < 2 {
		t.Errorf("Expected at least 2 scripts blocked, got %d", metrics.ScriptsBlocked)
	}
	if metrics.HighSeverityIssues == 0 {
		t.Error("Expected high severity issues")
	}
	if metrics.DataEncrypted == 0 {
		t.Error("Expected data encryption count")
	}

	// Test JSON marshaling
	jsonData, err := sm.MarshalMetricsJSON()
	if err != nil {
		t.Fatalf("Failed to marshal metrics: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}
}

func TestSetEnabled(t *testing.T) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		EncryptSensitiveData: true,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Initially enabled
	if !sm.IsEnabled() {
		t.Error("Should be enabled initially")
	}

	// Disable both
	sm.SetEnabled(false, false)
	if sm.IsEnabled() {
		t.Error("Should be disabled")
	}

	// Enable only analysis
	sm.SetEnabled(true, false)
	if !sm.IsEnabled() {
		t.Error("Should be enabled with just analysis")
	}

	// Test that disabled features don't process
	err = sm.AnalyzeScript(`eval("test");`, "test")
	if err == nil {
		t.Error("Should still analyze when analysis is enabled")
	}
}

func TestConcurrency(t *testing.T) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  false, // Don't block to test all paths
		EncryptSensitiveData: true,
	}

	sm, err := NewSecurityMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Analyze scripts
			scripts := []string{
				`console.log("safe");`,
				`eval("dangerous");`,
				`require('fs');`,
			}

			for _, script := range scripts {
				if err := sm.AnalyzeScript(script, fmt.Sprintf("script%d", id)); err != nil {
					errors <- err
				}
			}

			// Process responses
			data := map[string]interface{}{
				"email": fmt.Sprintf("user%d@example.com", id),
				"phone": "555-123-4567",
			}

			if _, err := sm.ProcessResponse(data); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify metrics are consistent
	metrics := sm.GetMetrics()
	if metrics.ScriptsAnalyzed != 30 { // 10 goroutines * 3 scripts
		t.Errorf("Expected 30 scripts analyzed, got %d", metrics.ScriptsAnalyzed)
	}
}

// Benchmark tests
func BenchmarkAnalyzeScript(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		BlockOnHighSeverity:  false,
	}

	sm, _ := NewSecurityMiddleware(config)
	script := `
		function process(data) {
			var result = data.map(x => x * 2);
			console.log(result);
			return result;
		}
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.AnalyzeScript(script, "bench")
	}
}

func BenchmarkProcessResponse(b *testing.B) {
	config := &Config{
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		EncryptSensitiveData: true,
	}

	sm, _ := NewSecurityMiddleware(config)
	data := map[string]interface{}{
		"email": "test@example.com",
		"phone": "555-123-4567",
		"data":  "Some regular data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.ProcessResponse(data)
	}
}

func BenchmarkConcurrentOperations(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  false,
		EncryptSensitiveData: true,
	}

	sm, _ := NewSecurityMiddleware(config)
	script := `console.log("test");`
	data := map[string]interface{}{"email": "test@example.com"}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sm.AnalyzeScript(script, "bench")
			sm.ProcessResponse(data)
		}
	})
}
