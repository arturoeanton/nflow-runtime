// Package security provides comprehensive benchmarks for all security components
package security

import (
	"fmt"
	"strings"
	"testing"

	"github.com/arturoeanton/nflow-runtime/security/analyzer"
	"github.com/arturoeanton/nflow-runtime/security/encryption"
	"github.com/arturoeanton/nflow-runtime/security/interceptor"
)

// Sample scripts for benchmarking
var (
	safeScript = `
		function processData(input) {
			const result = input.map(item => {
				return {
					id: item.id,
					value: item.value * 2,
					timestamp: Date.now()
				};
			});
			console.log('Processed', result.length, 'items');
			return result;
		}
		
		const data = Array.from({length: 100}, (_, i) => ({id: i, value: i}));
		processData(data);
	`

	dangerousScript = `
		const fs = require('fs');
		const exec = require('child_process').exec;
		
		eval("console.log('eval executed')");
		new Function("return process.exit()")();
		
		while(true) {
			// Infinite loop
		}
		
		fs.readFile('/etc/passwd', (err, data) => {
			console.log(data);
		});
		
		exec('rm -rf /', (err) => {
			console.log('Command executed');
		});
		
		const apiKey = "sk_test_1234567890abcdefghijklmnopqrstuvwxyz";
		const jwt = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c";
	`

	mixedScript = `
		// Some safe code
		function calculate(a, b) {
			return a + b;
		}
		
		// Some questionable code
		const result = eval('calculate(1, 2)');
		
		// Network access
		const https = require('https');
		
		// Sensitive data
		const user = {
			name: "John Doe",
			email: "john.doe@example.com",
			phone: "555-123-4567",
			ssn: "123-45-6789",
			apiKey: "api_key=1234567890abcdefghijklmnop"
		};
		
		// More safe code
		const items = [1, 2, 3, 4, 5];
		const doubled = items.map(x => x * 2);
		console.log(doubled);
	`

	// Sample data for encryption benchmarks
	smallData = map[string]interface{}{
		"id":   123,
		"name": "Test User",
		"role": "admin",
	}

	mediumData = map[string]interface{}{
		"user": map[string]interface{}{
			"id":    123,
			"name":  "John Doe",
			"email": "john@example.com",
			"phone": "555-123-4567",
		},
		"settings": map[string]interface{}{
			"theme":         "dark",
			"notifications": true,
			"language":      "en",
		},
		"apiKey": "sk_test_1234567890abcdefghij",
	}

	largeData = generateLargeData()
)

// generateLargeData creates a large nested data structure for benchmarking
func generateLargeData() map[string]interface{} {
	users := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		users[i] = map[string]interface{}{
			"id":       i,
			"name":     fmt.Sprintf("User %d", i),
			"email":    fmt.Sprintf("user%d@example.com", i),
			"phone":    fmt.Sprintf("555-%03d-%04d", i%1000, i),
			"ssn":      fmt.Sprintf("%03d-%02d-%04d", i%1000, i%100, i),
			"apiToken": fmt.Sprintf("token_%d_%s", i, strings.Repeat("x", 20)),
			"profile": map[string]interface{}{
				"bio":      strings.Repeat("Lorem ipsum ", 10),
				"location": fmt.Sprintf("City %d", i),
				"website":  fmt.Sprintf("https://user%d.example.com", i),
			},
		}
	}

	return map[string]interface{}{
		"users":      users,
		"totalCount": len(users),
		"metadata": map[string]interface{}{
			"version":   "1.0",
			"timestamp": "2024-01-01T00:00:00Z",
		},
	}
}

// Benchmarks for Static Analyzer

func BenchmarkAnalyzer_SafeScript(b *testing.B) {
	analyzer := analyzer.NewStaticAnalyzer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.AnalyzeScript(safeScript)
	}
}

func BenchmarkAnalyzer_DangerousScript(b *testing.B) {
	analyzer := analyzer.NewStaticAnalyzer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.AnalyzeScript(dangerousScript)
	}
}

func BenchmarkAnalyzer_MixedScript(b *testing.B) {
	analyzer := analyzer.NewStaticAnalyzer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.AnalyzeScript(mixedScript)
	}
}

func BenchmarkAnalyzer_Parallel(b *testing.B) {
	analyzer := analyzer.NewStaticAnalyzer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = analyzer.AnalyzeScript(mixedScript)
		}
	})
}

// Benchmarks for Encryption Service

func BenchmarkEncryption_Small(b *testing.B) {
	key := strings.Repeat("k", 32)
	es, _ := encryption.NewEncryptionService(key)
	data := "Small piece of sensitive data"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, _ := es.Encrypt(data)
		_, _ = es.Decrypt(encrypted)
	}
}

func BenchmarkEncryption_Medium(b *testing.B) {
	key := strings.Repeat("k", 32)
	es, _ := encryption.NewEncryptionService(key)
	data := strings.Repeat("Medium sized data content ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, _ := es.Encrypt(data)
		_, _ = es.Decrypt(encrypted)
	}
}

func BenchmarkEncryption_Large(b *testing.B) {
	key := strings.Repeat("k", 32)
	es, _ := encryption.NewEncryptionService(key)
	data := strings.Repeat("Large data content for benchmarking ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, _ := es.Encrypt(data)
		_, _ = es.Decrypt(encrypted)
	}
}

func BenchmarkEncryption_Parallel(b *testing.B) {
	key := strings.Repeat("k", 32)
	es, _ := encryption.NewEncryptionService(key)
	data := "Concurrent encryption test data"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			encrypted, _ := es.Encrypt(data)
			_, _ = es.Decrypt(encrypted)
		}
	})
}

// Benchmarks for Sensitive Data Interceptor

func BenchmarkInterceptor_SmallData(b *testing.B) {
	key := strings.Repeat("k", 32)
	encService, _ := encryption.NewEncryptionService(key)
	config := &interceptor.Config{
		Enabled:        true,
		EncryptInPlace: true,
	}
	sdi := interceptor.NewSensitiveDataInterceptor(encService, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sdi.ProcessResponse(smallData)
	}
}

func BenchmarkInterceptor_MediumData(b *testing.B) {
	key := strings.Repeat("k", 32)
	encService, _ := encryption.NewEncryptionService(key)
	config := &interceptor.Config{
		Enabled:        true,
		EncryptInPlace: true,
	}
	sdi := interceptor.NewSensitiveDataInterceptor(encService, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sdi.ProcessResponse(mediumData)
	}
}

func BenchmarkInterceptor_LargeData(b *testing.B) {
	key := strings.Repeat("k", 32)
	encService, _ := encryption.NewEncryptionService(key)
	config := &interceptor.Config{
		Enabled:        true,
		EncryptInPlace: true,
	}
	sdi := interceptor.NewSensitiveDataInterceptor(encService, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sdi.ProcessResponse(largeData)
	}
}

func BenchmarkInterceptor_WithMetadata(b *testing.B) {
	key := strings.Repeat("k", 32)
	encService, _ := encryption.NewEncryptionService(key)
	config := &interceptor.Config{
		Enabled:        true,
		EncryptInPlace: false, // Use metadata mode
		MetadataKey:    "_encrypted",
	}
	sdi := interceptor.NewSensitiveDataInterceptor(encService, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sdi.ProcessResponse(mediumData)
	}
}

// Benchmarks for Complete Security Middleware

func BenchmarkMiddleware_Complete_Small(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  false,
		EncryptSensitiveData: true,
	}
	sm, _ := NewSecurityMiddleware(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.AnalyzeScript(safeScript, "bench")
		_, _ = sm.ProcessResponse(smallData)
	}
}

func BenchmarkMiddleware_Complete_Large(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  false,
		EncryptSensitiveData: true,
	}
	sm, _ := NewSecurityMiddleware(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.AnalyzeScript(mixedScript, "bench")
		_, _ = sm.ProcessResponse(largeData)
	}
}

func BenchmarkMiddleware_AnalysisOnly(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     false,
		BlockOnHighSeverity:  false,
	}
	sm, _ := NewSecurityMiddleware(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.AnalyzeScript(dangerousScript, "bench")
	}
}

func BenchmarkMiddleware_EncryptionOnly(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: false,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		EncryptSensitiveData: true,
	}
	sm, _ := NewSecurityMiddleware(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sm.ProcessResponse(largeData)
	}
}

func BenchmarkMiddleware_Parallel(b *testing.B) {
	config := &Config{
		EnableStaticAnalysis: true,
		EnableEncryption:     true,
		EncryptionKey:        strings.Repeat("k", 32),
		BlockOnHighSeverity:  false,
		EncryptSensitiveData: true,
	}
	sm, _ := NewSecurityMiddleware(config)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = sm.AnalyzeScript(safeScript, "bench")
			_, _ = sm.ProcessResponse(mediumData)
		}
	})
}

// Memory allocation benchmarks

func BenchmarkAlloc_Analyzer(b *testing.B) {
	analyzer := analyzer.NewStaticAnalyzer()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.AnalyzeScript(mixedScript)
	}
}

func BenchmarkAlloc_Encryption(b *testing.B) {
	key := strings.Repeat("k", 32)
	es, _ := encryption.NewEncryptionService(key)
	data := strings.Repeat("Test data ", 100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, _ := es.Encrypt(data)
		_, _ = es.Decrypt(encrypted)
	}
}

func BenchmarkAlloc_Interceptor(b *testing.B) {
	key := strings.Repeat("k", 32)
	encService, _ := encryption.NewEncryptionService(key)
	config := &interceptor.Config{
		Enabled:        true,
		EncryptInPlace: true,
	}
	sdi := interceptor.NewSensitiveDataInterceptor(encService, config)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sdi.ProcessResponse(mediumData)
	}
}
