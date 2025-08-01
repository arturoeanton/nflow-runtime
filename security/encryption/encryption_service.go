// Package encryption provides AES-256-GCM encryption for sensitive data.
// It's designed to be transparent to the existing engine, working as a
// middleware layer that can be enabled/disabled via configuration.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Common errors
var (
	ErrInvalidKeySize      = errors.New("encryption key must be 32 bytes")
	ErrDecryptionFailed    = errors.New("decryption failed")
	ErrInvalidCiphertext   = errors.New("invalid ciphertext")
	ErrKeyDerivationFailed = errors.New("key derivation failed")
)

// EncryptionService provides AES-256-GCM encryption/decryption
type EncryptionService struct {
	key []byte
	gcm cipher.AEAD
	
	// Performance optimization: reuse buffers
	bufferPool sync.Pool
	
	// Metrics for monitoring
	encryptCount uint64
	decryptCount uint64
	mu           sync.RWMutex
}

// Config holds encryption service configuration
type Config struct {
	Key string // Base64-encoded 32-byte key
	// Future: support for key rotation, KMS integration
}

// NewEncryptionService creates a new encryption service with the provided key
func NewEncryptionService(key string) (*EncryptionService, error) {
	// Decode base64 key if provided in that format
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		// If not base64, use the raw string
		keyBytes = []byte(key)
	}
	
	// Ensure key is exactly 32 bytes for AES-256
	if len(keyBytes) != 32 {
		// If key is not 32 bytes, derive a 32-byte key using SHA-256
		if len(keyBytes) == 0 {
			return nil, ErrInvalidKeySize
		}
		hash := sha256.Sum256(keyBytes)
		keyBytes = hash[:]
	}
	
	return NewEncryptionServiceWithBytes(keyBytes)
}

// NewEncryptionServiceWithBytes creates a service with raw key bytes
func NewEncryptionServiceWithBytes(key []byte) (*EncryptionService, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	service := &EncryptionService{
		key: key,
		gcm: gcm,
		bufferPool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate buffers for better performance
				return make([]byte, 0, 1024)
			},
		},
	}
	
	return service, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext
func (es *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	
	// Generate a new nonce for each encryption
	nonce := make([]byte, es.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Get buffer from pool for better performance
	buf := es.bufferPool.Get().([]byte)
	defer func() {
		buf = buf[:0] // Reset buffer
		es.bufferPool.Put(buf)
	}()
	
	// Seal appends the ciphertext to the nonce
	ciphertext := es.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Update metrics
	es.mu.Lock()
	es.encryptCount++
	es.mu.Unlock()
	
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext
func (es *EncryptionService) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base64", ErrInvalidCiphertext)
	}
	
	// Extract nonce
	nonceSize := es.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("%w: too short", ErrInvalidCiphertext)
	}
	
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	
	// Decrypt
	plaintext, err := es.gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	
	// Update metrics
	es.mu.Lock()
	es.decryptCount++
	es.mu.Unlock()
	
	return string(plaintext), nil
}

// EncryptBytes encrypts binary data
func (es *EncryptionService) EncryptBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	
	nonce := make([]byte, es.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	ciphertext := es.gcm.Seal(nonce, nonce, data, nil)
	
	es.mu.Lock()
	es.encryptCount++
	es.mu.Unlock()
	
	return ciphertext, nil
}

// DecryptBytes decrypts binary data
func (es *EncryptionService) DecryptBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	
	nonceSize := es.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("%w: too short", ErrInvalidCiphertext)
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	
	plaintext, err := es.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	
	es.mu.Lock()
	es.decryptCount++
	es.mu.Unlock()
	
	return plaintext, nil
}

// GetMetrics returns encryption/decryption counts for monitoring
func (es *EncryptionService) GetMetrics() (encryptCount, decryptCount uint64) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.encryptCount, es.decryptCount
}

// ResetMetrics resets the metrics counters
func (es *EncryptionService) ResetMetrics() {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.encryptCount = 0
	es.decryptCount = 0
}

// RotateKey creates a new encryption service with a new key
// This is useful for key rotation scenarios
func (es *EncryptionService) RotateKey(newKey []byte) (*EncryptionService, error) {
	return NewEncryptionServiceWithBytes(newKey)
}

// IsEncrypted checks if a string appears to be encrypted (base64 with proper length)
func IsEncrypted(data string) bool {
	// Try to decode as base64
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return false
	}
	
	// Check if length is reasonable for encrypted data (at least nonce + some data)
	// GCM nonce is typically 12 bytes, plus at least 16 bytes for the tag
	return len(decoded) >= 28
}

// GenerateKey generates a secure 32-byte key for AES-256
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateKeyString generates a base64-encoded key
func GenerateKeyString() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}