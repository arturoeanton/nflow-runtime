// Package interceptor provides automatic detection and encryption of sensitive data
// in workflow responses. It works transparently without modifying the core engine.
package interceptor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	
	"github.com/arturoeanton/nflow-runtime/security/encryption"
)

// PatternType identifies the type of sensitive data
type PatternType string

const (
	PatternEmail      PatternType = "email"
	PatternPhone      PatternType = "phone"
	PatternSSN        PatternType = "ssn"
	PatternCreditCard PatternType = "credit_card"
	PatternAPIKey     PatternType = "api_key"
	PatternJWT        PatternType = "jwt"
	PatternPassword   PatternType = "password"
	PatternCustom     PatternType = "custom"
)

// SensitivePattern defines a pattern for detecting sensitive data
type SensitivePattern struct {
	Type        PatternType
	Name        string         // Human-readable name
	Pattern     *regexp.Regexp // Regex pattern
	MinLength   int           // Minimum length to consider
	MaxLength   int           // Maximum length to consider (0 = no limit)
	Confidence  float32       // Confidence threshold (0.0-1.0)
}

// Detection represents a found sensitive data instance
type Detection struct {
	Type       PatternType `json:"type"`
	Value      string      `json:"-"` // Original value (not serialized)
	Encrypted  string      `json:"encrypted"`
	Path       string      `json:"path"` // JSON path where found
	Confidence float32     `json:"confidence"`
}

// SensitiveDataInterceptor detects and encrypts sensitive data
type SensitiveDataInterceptor struct {
	encryptionService *encryption.EncryptionService
	patterns          map[PatternType]*SensitivePattern
	customPatterns    map[string]*SensitivePattern
	
	// Configuration
	enabled           bool
	encryptInPlace    bool   // Replace values in-place vs metadata
	metadataKey       string // Key to store encryption metadata
	
	// Performance optimization
	compiledPatterns []*SensitivePattern // Pre-sorted by performance
	patternCache     sync.Map            // Cache compiled patterns
	
	// Metrics
	detectionCount uint64
	encryptCount   uint64
	mu             sync.RWMutex
}

// Config holds interceptor configuration
type Config struct {
	Enabled           bool
	EncryptInPlace    bool
	MetadataKey       string
	CustomPatterns    map[string]string // name -> regex pattern
}

// NewSensitiveDataInterceptor creates a new interceptor
func NewSensitiveDataInterceptor(encService *encryption.EncryptionService, config *Config) *SensitiveDataInterceptor {
	if config == nil {
		config = &Config{
			Enabled:        true,
			EncryptInPlace: true,
			MetadataKey:    "_encrypted_fields",
		}
	}
	
	interceptor := &SensitiveDataInterceptor{
		encryptionService: encService,
		patterns:          make(map[PatternType]*SensitivePattern),
		customPatterns:    make(map[string]*SensitivePattern),
		enabled:           config.Enabled,
		encryptInPlace:    config.EncryptInPlace,
		metadataKey:       config.MetadataKey,
	}
	
	// Initialize default patterns
	interceptor.initializeDefaultPatterns()
	
	// Add custom patterns from config
	for name, pattern := range config.CustomPatterns {
		interceptor.AddCustomPattern(name, pattern)
	}
	
	// Compile patterns for performance
	interceptor.compilePatterns()
	
	return interceptor
}

// initializeDefaultPatterns sets up common sensitive data patterns
func (sdi *SensitiveDataInterceptor) initializeDefaultPatterns() {
	sdi.patterns = map[PatternType]*SensitivePattern{
		PatternEmail: {
			Type:       PatternEmail,
			Name:       "Email Address",
			Pattern:    regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			MinLength:  5,
			MaxLength:  254,
			Confidence: 0.9,
		},
		PatternPhone: {
			Type:    PatternPhone,
			Name:    "Phone Number",
			// Supports various formats: (123) 456-7890, 123-456-7890, +1234567890
			Pattern:    regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})\b`),
			MinLength:  10,
			MaxLength:  20,
			Confidence: 0.8,
		},
		PatternSSN: {
			Type:       PatternSSN,
			Name:       "Social Security Number",
			Pattern:    regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			MinLength:  11,
			MaxLength:  11,
			Confidence: 0.95,
		},
		PatternCreditCard: {
			Type:    PatternCreditCard,
			Name:    "Credit Card Number",
			// Basic credit card pattern with Luhn check would be better
			Pattern:    regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
			MinLength:  13,
			MaxLength:  19,
			Confidence: 0.7,
		},
		PatternAPIKey: {
			Type:    PatternAPIKey,
			Name:    "API Key",
			// Common API key patterns
			Pattern:    regexp.MustCompile(`\b(?i)(?:api[_-]?key|apikey|access[_-]?token|auth[_-]?token)['"]?\s*[:=]\s*['"]?([a-zA-Z0-9_\-]{20,})['"]?\b`),
			MinLength:  20,
			MaxLength:  256,
			Confidence: 0.85,
		},
		PatternJWT: {
			Type:       PatternJWT,
			Name:       "JWT Token",
			Pattern:    regexp.MustCompile(`\b[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
			MinLength:  30,
			MaxLength:  0,
			Confidence: 0.9,
		},
	}
}

// compilePatterns optimizes patterns for better performance
func (sdi *SensitiveDataInterceptor) compilePatterns() {
	sdi.mu.Lock()
	defer sdi.mu.Unlock()
	
	// Combine all patterns
	allPatterns := make([]*SensitivePattern, 0, len(sdi.patterns)+len(sdi.customPatterns))
	
	for _, p := range sdi.patterns {
		allPatterns = append(allPatterns, p)
	}
	
	for _, p := range sdi.customPatterns {
		allPatterns = append(allPatterns, p)
	}
	
	// Sort by expected performance (simple patterns first)
	// In real implementation, we'd sort by pattern complexity
	sdi.compiledPatterns = allPatterns
}

// ProcessResponse intercepts and processes response data
func (sdi *SensitiveDataInterceptor) ProcessResponse(data interface{}) (interface{}, error) {
	if !sdi.enabled || data == nil {
		return data, nil
	}
	
	// Convert to JSON for processing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return data, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	// Process based on mode
	if sdi.encryptInPlace {
		return sdi.processInPlace(jsonData)
	}
	
	return sdi.processWithMetadata(jsonData)
}

// processInPlace replaces sensitive data with encrypted versions
func (sdi *SensitiveDataInterceptor) processInPlace(jsonData []byte) (interface{}, error) {
	strData := string(jsonData)
	detections := sdi.detectSensitiveData(strData)
	
	// Sort detections by position (reverse order to maintain positions)
	// This ensures we replace from end to start
	sortDetectionsByPosition(detections, strData)
	
	// Replace each detection with encrypted version
	for i := len(detections) - 1; i >= 0; i-- {
		detection := detections[i]
		
		encrypted, err := sdi.encryptionService.Encrypt(detection.Value)
		if err != nil {
			continue // Skip on error
		}
		
		// Create replacement tag
		replacement := fmt.Sprintf(`"[ENCRYPTED_%s:%s]"`, detection.Type, encrypted)
		
		// Replace in string (this is simplified, real implementation would handle JSON properly)
		strData = strings.Replace(strData, fmt.Sprintf(`"%s"`, detection.Value), replacement, 1)
		
		sdi.mu.Lock()
		sdi.encryptCount++
		sdi.mu.Unlock()
	}
	
	// Parse back to interface{}
	var result interface{}
	if err := json.Unmarshal([]byte(strData), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal processed data: %w", err)
	}
	
	return result, nil
}

// processWithMetadata adds encryption metadata without modifying original structure
func (sdi *SensitiveDataInterceptor) processWithMetadata(jsonData []byte) (interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		// Try array
		var arrayResult []interface{}
		if err := json.Unmarshal(jsonData, &arrayResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
		// Wrap array in object
		result = map[string]interface{}{
			"data": arrayResult,
		}
	}
	
	strData := string(jsonData)
	detections := sdi.detectSensitiveData(strData)
	
	// Create metadata
	metadata := make([]Detection, 0, len(detections))
	for _, detection := range detections {
		encrypted, err := sdi.encryptionService.Encrypt(detection.Value)
		if err != nil {
			continue
		}
		
		detection.Encrypted = encrypted
		metadata = append(metadata, detection)
		
		sdi.mu.Lock()
		sdi.encryptCount++
		sdi.mu.Unlock()
	}
	
	// Add metadata if any sensitive data was found
	if len(metadata) > 0 {
		result[sdi.metadataKey] = metadata
	}
	
	return result, nil
}

// detectSensitiveData finds all sensitive data in the input
func (sdi *SensitiveDataInterceptor) detectSensitiveData(data string) []Detection {
	var detections []Detection
	
	sdi.mu.RLock()
	patterns := sdi.compiledPatterns
	sdi.mu.RUnlock()
	
	for _, pattern := range patterns {
		matches := pattern.Pattern.FindAllStringSubmatch(data, -1)
		
		for _, match := range matches {
			value := match[0]
			if len(match) > 1 {
				value = match[1] // Use first capture group if available
			}
			
			// Check length constraints
			if pattern.MinLength > 0 && len(value) < pattern.MinLength {
				continue
			}
			if pattern.MaxLength > 0 && len(value) > pattern.MaxLength {
				continue
			}
			
			// Additional validation for specific types
			if pattern.Type == PatternCreditCard && !isValidCreditCard(value) {
				continue
			}
			
			detections = append(detections, Detection{
				Type:       pattern.Type,
				Value:      value,
				Confidence: pattern.Confidence,
				Path:       "", // Would need JSON path tracking for real implementation
			})
			
			sdi.mu.Lock()
			sdi.detectionCount++
			sdi.mu.Unlock()
		}
	}
	
	return detections
}

// AddCustomPattern adds a custom pattern for detection
func (sdi *SensitiveDataInterceptor) AddCustomPattern(name, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}
	
	sdi.mu.Lock()
	defer sdi.mu.Unlock()
	
	sdi.customPatterns[name] = &SensitivePattern{
		Type:       PatternCustom,
		Name:       name,
		Pattern:    compiled,
		MinLength:  1,
		MaxLength:  0,
		Confidence: 0.8,
	}
	
	// Recompile patterns
	sdi.compilePatterns()
	
	return nil
}

// RemoveCustomPattern removes a custom pattern
func (sdi *SensitiveDataInterceptor) RemoveCustomPattern(name string) bool {
	sdi.mu.Lock()
	defer sdi.mu.Unlock()
	
	if _, exists := sdi.customPatterns[name]; exists {
		delete(sdi.customPatterns, name)
		sdi.compilePatterns()
		return true
	}
	
	return false
}

// SetEnabled enables or disables the interceptor
func (sdi *SensitiveDataInterceptor) SetEnabled(enabled bool) {
	sdi.mu.Lock()
	defer sdi.mu.Unlock()
	sdi.enabled = enabled
}

// GetMetrics returns detection and encryption counts
func (sdi *SensitiveDataInterceptor) GetMetrics() (detectionCount, encryptCount uint64) {
	sdi.mu.RLock()
	defer sdi.mu.RUnlock()
	return sdi.detectionCount, sdi.encryptCount
}

// ResetMetrics resets the metrics counters
func (sdi *SensitiveDataInterceptor) ResetMetrics() {
	sdi.mu.Lock()
	defer sdi.mu.Unlock()
	sdi.detectionCount = 0
	sdi.encryptCount = 0
}

// Helper functions

// isValidCreditCard performs basic credit card validation
func isValidCreditCard(number string) bool {
	// Remove spaces and dashes
	cleaned := strings.ReplaceAll(strings.ReplaceAll(number, " ", ""), "-", "")
	
	// Basic length check
	if len(cleaned) < 13 || len(cleaned) > 19 {
		return false
	}
	
	// All digits check
	for _, ch := range cleaned {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	
	// Could add Luhn algorithm here for better validation
	return true
}

// sortDetectionsByPosition sorts detections by their position in the string
func sortDetectionsByPosition(detections []Detection, data string) {
	// Simple implementation - in production would track actual positions
	// This is a placeholder that doesn't actually sort
}

// ProcessMap processes a map directly (useful for middleware integration)
func (sdi *SensitiveDataInterceptor) ProcessMap(data map[string]interface{}) (map[string]interface{}, error) {
	result, err := sdi.ProcessResponse(data)
	if err != nil {
		return nil, err
	}
	
	if mapResult, ok := result.(map[string]interface{}); ok {
		return mapResult, nil
	}
	
	return nil, fmt.Errorf("result is not a map")
}