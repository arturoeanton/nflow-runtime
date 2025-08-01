package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestNewEncryptionService(t *testing.T) {
	// Test with 32-byte key
	key32 := strings.Repeat("a", 32)
	es, err := NewEncryptionService(key32)
	if err != nil {
		t.Fatalf("Failed to create service with 32-byte key: %v", err)
	}
	if es == nil {
		t.Fatal("Service should not be nil")
	}
	
	// Test with base64-encoded key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
	es2, err := NewEncryptionService(keyB64)
	if err != nil {
		t.Fatalf("Failed to create service with base64 key: %v", err)
	}
	if es2 == nil {
		t.Fatal("Service should not be nil")
	}
	
	// Test with short key (should derive using SHA-256)
	shortKey := "short-key"
	es3, err := NewEncryptionService(shortKey)
	if err != nil {
		t.Fatalf("Failed to create service with short key: %v", err)
	}
	if es3 == nil {
		t.Fatal("Service should not be nil")
	}
	
	// Test with empty key
	_, err = NewEncryptionService("")
	if err != ErrInvalidKeySize {
		t.Error("Expected ErrInvalidKeySize for empty key")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := strings.Repeat("x", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	testCases := []string{
		"Hello, World!",
		"",
		"Special chars: !@#$%^&*()",
		"Unicode: 你好世界 🌍",
		strings.Repeat("Long text ", 1000),
		"Multi\nline\ntext",
		`{"user":"test","password":"secret123"}`,
	}
	
	for _, plaintext := range testCases {
		// Encrypt
		ciphertext, err := es.Encrypt(plaintext)
		if err != nil {
			t.Errorf("Failed to encrypt '%s': %v", plaintext, err)
			continue
		}
		
		// Empty plaintext should return empty ciphertext
		if plaintext == "" && ciphertext != "" {
			t.Error("Empty plaintext should return empty ciphertext")
			continue
		}
		
		if plaintext != "" {
			// Verify it's base64
			_, err = base64.StdEncoding.DecodeString(ciphertext)
			if err != nil {
				t.Errorf("Ciphertext is not valid base64: %v", err)
			}
			
			// Ciphertext should be different from plaintext
			if ciphertext == plaintext {
				t.Error("Ciphertext should differ from plaintext")
			}
		}
		
		// Decrypt
		decrypted, err := es.Decrypt(ciphertext)
		if err != nil {
			t.Errorf("Failed to decrypt: %v", err)
			continue
		}
		
		// Verify match
		if decrypted != plaintext {
			t.Errorf("Decrypted text doesn't match. Got '%s', want '%s'", 
				decrypted, plaintext)
		}
	}
}

func TestEncryptRandomness(t *testing.T) {
	key := strings.Repeat("k", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	plaintext := "Same text encrypted multiple times"
	ciphertexts := make(map[string]bool)
	
	// Encrypt the same text multiple times
	for i := 0; i < 100; i++ {
		ciphertext, err := es.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}
		
		// Each encryption should produce unique ciphertext due to random nonce
		if ciphertexts[ciphertext] {
			t.Fatal("Same ciphertext produced twice - nonce reuse detected!")
		}
		ciphertexts[ciphertext] = true
	}
}

func TestDecryptInvalid(t *testing.T) {
	key := strings.Repeat("k", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	testCases := []struct {
		name       string
		ciphertext string
		expectErr  bool
	}{
		{"Empty", "", false},
		{"Invalid base64", "not-base64!", true},
		{"Too short", base64.StdEncoding.EncodeToString([]byte("short")), true},
		{"Random data", base64.StdEncoding.EncodeToString(make([]byte, 50)), true},
		{"Corrupted", func() string {
			// Create valid ciphertext then corrupt it
			ct, _ := es.Encrypt("test")
			bytes, _ := base64.StdEncoding.DecodeString(ct)
			bytes[len(bytes)-1] ^= 0xFF // Flip bits in last byte
			return base64.StdEncoding.EncodeToString(bytes)
		}(), true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := es.Decrypt(tc.ciphertext)
			if tc.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestEncryptDecryptBytes(t *testing.T) {
	key := strings.Repeat("b", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	testData := [][]byte{
		[]byte("Hello"),
		{},
		{0x00, 0xFF, 0x42, 0x13, 0x37},
		make([]byte, 1024), // 1KB of zeros
	}
	
	for _, data := range testData {
		encrypted, err := es.EncryptBytes(data)
		if err != nil {
			t.Errorf("Failed to encrypt bytes: %v", err)
			continue
		}
		
		if len(data) == 0 && len(encrypted) != 0 {
			t.Error("Empty data should return empty result")
			continue
		}
		
		decrypted, err := es.DecryptBytes(encrypted)
		if err != nil {
			t.Errorf("Failed to decrypt bytes: %v", err)
			continue
		}
		
		if !bytesEqual(decrypted, data) {
			t.Error("Decrypted bytes don't match original")
		}
	}
}

func TestGetMetrics(t *testing.T) {
	key := strings.Repeat("m", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	// Initial metrics should be zero
	enc, dec := es.GetMetrics()
	if enc != 0 || dec != 0 {
		t.Error("Initial metrics should be zero")
	}
	
	// Perform some operations
	for i := 0; i < 5; i++ {
		ct, _ := es.Encrypt("test")
		es.Decrypt(ct)
	}
	
	encrypted, _ := es.EncryptBytes([]byte("test"))
	es.DecryptBytes(encrypted)
	
	// Check metrics
	enc, dec = es.GetMetrics()
	if enc != 6 { // 5 string + 1 bytes
		t.Errorf("Expected 6 encryptions, got %d", enc)
	}
	if dec != 6 { // 5 string + 1 bytes
		t.Errorf("Expected 6 decryptions, got %d", dec)
	}
	
	// Reset metrics
	es.ResetMetrics()
	enc, dec = es.GetMetrics()
	if enc != 0 || dec != 0 {
		t.Error("Metrics should be zero after reset")
	}
}

func TestIsEncrypted(t *testing.T) {
	key := strings.Repeat("e", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	// Generate some encrypted data
	encrypted, _ := es.Encrypt("This is encrypted")
	
	testCases := []struct {
		data      string
		encrypted bool
	}{
		{encrypted, true},
		{"plain text", false},
		{"", false},
		{"SGVsbG8=", false}, // Valid base64 but too short
		{base64.StdEncoding.EncodeToString(make([]byte, 30)), true},
		{"not-base64!", false},
	}
	
	for _, tc := range testCases {
		result := IsEncrypted(tc.data)
		if result != tc.encrypted {
			t.Errorf("IsEncrypted(%q) = %v, want %v", tc.data, result, tc.encrypted)
		}
	}
}

func TestGenerateKey(t *testing.T) {
	// Test GenerateKey
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d", len(key1))
	}
	
	// Keys should be random
	key2, _ := GenerateKey()
	if bytesEqual(key1, key2) {
		t.Error("Generated keys should be different")
	}
	
	// Test GenerateKeyString
	keyStr, err := GenerateKeyString()
	if err != nil {
		t.Fatalf("Failed to generate key string: %v", err)
	}
	
	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		t.Errorf("Generated key string is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Error("Decoded key should be 32 bytes")
	}
}

func TestConcurrency(t *testing.T) {
	key := strings.Repeat("c", 32)
	es, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	
	// Run concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < 100; j++ {
				text := fmt.Sprintf("Goroutine %d iteration %d", id, j)
				
				// Encrypt
				ct, err := es.Encrypt(text)
				if err != nil {
					errors <- fmt.Errorf("Encrypt failed: %v", err)
					return
				}
				
				// Decrypt
				pt, err := es.Decrypt(ct)
				if err != nil {
					errors <- fmt.Errorf("Decrypt failed: %v", err)
					return
				}
				
				if pt != text {
					errors <- fmt.Errorf("Mismatch: got %q, want %q", pt, text)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Error(err)
	}
	
	// Verify metrics are consistent
	enc, dec := es.GetMetrics()
	if enc != 1000 || dec != 1000 {
		t.Errorf("Expected 1000 operations each, got enc=%d dec=%d", enc, dec)
	}
}

// Benchmark tests
func BenchmarkEncrypt(b *testing.B) {
	key := strings.Repeat("b", 32)
	es, _ := NewEncryptionService(key)
	plaintext := "This is a benchmark test string"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := es.Encrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key := strings.Repeat("b", 32)
	es, _ := NewEncryptionService(key)
	ciphertext, _ := es.Encrypt("This is a benchmark test string")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := es.Decrypt(ciphertext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptLarge(b *testing.B) {
	key := strings.Repeat("b", 32)
	es, _ := NewEncryptionService(key)
	plaintext := strings.Repeat("Large data ", 1000) // ~11KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := es.Encrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentEncrypt(b *testing.B) {
	key := strings.Repeat("b", 32)
	es, _ := NewEncryptionService(key)
	plaintext := "Concurrent benchmark"
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := es.Encrypt(plaintext)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Helper function
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}