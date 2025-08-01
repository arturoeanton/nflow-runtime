// Package sanitizer provides log sanitization capabilities to prevent
// sensitive data exposure in logs. It works transparently with existing
// logging systems without requiring modifications to the core engine.
package sanitizer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// SensitiveDataType identifies the type of sensitive data
type SensitiveDataType string

const (
	TypeEmail       SensitiveDataType = "email"
	TypePhone       SensitiveDataType = "phone"
	TypeSSN         SensitiveDataType = "ssn"
	TypeCreditCard  SensitiveDataType = "credit_card"
	TypeAPIKey      SensitiveDataType = "api_key"
	TypeJWT         SensitiveDataType = "jwt"
	TypeIPAddress   SensitiveDataType = "ip_address"
	TypePassword    SensitiveDataType = "password"
	TypePrivateKey  SensitiveDataType = "private_key"
	TypeAccessToken SensitiveDataType = "access_token"
)

// Pattern defines a pattern for detecting sensitive data in logs
type Pattern struct {
	Type        SensitiveDataType
	Name        string
	Regex       *regexp.Regexp
	Replacement string // Replacement text for matched data
}

// LogSanitizer sanitizes logs by removing or masking sensitive data
type LogSanitizer struct {
	patterns      []Pattern
	customPatterns map[string]*Pattern
	
	// Configuration
	enabled       bool
	maskingChar   string
	preserveLength bool
	showType      bool
	
	// Performance optimization
	compiledPatterns []*Pattern
	patternCache     sync.Map
	bufferPool       sync.Pool
	
	// Metrics
	logsProcessed    uint64
	dataSanitized    uint64
	mu               sync.RWMutex
}

// Config holds sanitizer configuration
type Config struct {
	Enabled        bool
	MaskingChar    string            // Character used for masking (default: "*")
	PreserveLength bool              // Preserve original length when masking
	ShowType       bool              // Show data type in replacement (e.g., [REDACTED:email])
	CustomPatterns map[string]string // name -> regex pattern
}

// NewLogSanitizer creates a new log sanitizer
func NewLogSanitizer(config *Config) *LogSanitizer {
	if config == nil {
		config = &Config{
			Enabled:        true,
			MaskingChar:    "*",
			PreserveLength: false,
			ShowType:       true,
		}
	}
	
	if config.MaskingChar == "" {
		config.MaskingChar = "*"
	}
	
	sanitizer := &LogSanitizer{
		enabled:        config.Enabled,
		maskingChar:    config.MaskingChar,
		preserveLength: config.PreserveLength,
		showType:       config.ShowType,
		customPatterns: make(map[string]*Pattern),
		bufferPool: sync.Pool{
			New: func() interface{} {
				return &strings.Builder{}
			},
		},
	}
	
	// Initialize default patterns
	sanitizer.initializeDefaultPatterns()
	
	// Add custom patterns
	for name, pattern := range config.CustomPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			sanitizer.customPatterns[name] = &Pattern{
				Type:        SensitiveDataType("custom_" + name),
				Name:        name,
				Regex:       compiled,
				Replacement: name,
			}
		}
	}
	
	// Compile patterns for performance (need to lock here since constructor doesn't hold lock)
	sanitizer.mu.Lock()
	sanitizer.compilePatterns()
	sanitizer.mu.Unlock()
	
	return sanitizer
}

// initializeDefaultPatterns sets up common sensitive data patterns
func (ls *LogSanitizer) initializeDefaultPatterns() {
	ls.patterns = []Pattern{
		// API Key pattern should come before phone to avoid false matches
		{
			Type:        TypeAPIKey,
			Name:        "API Key",
			Regex:       regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|access[_-]?token|auth[_-]?token)[\s:=]+["']?[a-zA-Z0-9_\-]{20,}["']?`),
			Replacement: "api_key",
		},
		{
			Type:        TypeJWT,
			Name:        "JWT Token",
			Regex:       regexp.MustCompile(`\b[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
			Replacement: "jwt",
		},
		{
			Type:        TypePrivateKey,
			Name:        "Private Key",
			Regex:       regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]+?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----`),
			Replacement: "private_key",
		},
		{
			Type:        TypePassword,
			Name:        "Password",
			Regex:       regexp.MustCompile(`(?i)(?:password|passwd|pwd)[\s:=]+["']?[^\s"']{4,}["']?`),
			Replacement: "password",
		},
		{
			Type:        TypeAccessToken,
			Name:        "Access Token",
			Regex:       regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`),
			Replacement: "access_token",
		},
		{
			Type:        TypeEmail,
			Name:        "Email Address",
			Regex:       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			Replacement: "email",
		},
		{
			Type:        TypeSSN,
			Name:        "Social Security Number",
			Regex:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			Replacement: "ssn",
		},
		{
			Type:        TypeCreditCard,
			Name:        "Credit Card",
			Regex:       regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
			Replacement: "credit_card",
		},
		{
			Type:        TypeIPAddress,
			Name:        "IP Address",
			Regex:       regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
			Replacement: "ip_address",
		},
		// Phone pattern last to avoid false matches with API keys
		{
			Type:        TypePhone,
			Name:        "Phone Number",
			Regex:       regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`),
			Replacement: "phone",
		},
	}
}

// compilePatterns optimizes patterns for better performance
// MUST be called with ls.mu already locked
func (ls *LogSanitizer) compilePatterns() {
	// Combine all patterns
	allPatterns := make([]*Pattern, 0, len(ls.patterns)+len(ls.customPatterns))
	
	for i := range ls.patterns {
		allPatterns = append(allPatterns, &ls.patterns[i])
	}
	
	for _, p := range ls.customPatterns {
		allPatterns = append(allPatterns, p)
	}
	
	ls.compiledPatterns = allPatterns
}

// Sanitize processes a log message and removes/masks sensitive data
func (ls *LogSanitizer) Sanitize(logMessage string) string {
	if !ls.enabled || logMessage == "" {
		return logMessage
	}
	
	atomic.AddUint64(&ls.logsProcessed, 1)
	
	// Get a string builder from pool
	builder := ls.bufferPool.Get().(*strings.Builder)
	defer func() {
		builder.Reset()
		ls.bufferPool.Put(builder)
	}()
	
	ls.mu.RLock()
	patterns := ls.compiledPatterns
	ls.mu.RUnlock()
	
	result := logMessage
	sanitizedCount := uint64(0)
	
	// Apply each pattern
	for _, pattern := range patterns {
		if pattern.Regex.MatchString(result) {
			result = pattern.Regex.ReplaceAllStringFunc(result, func(match string) string {
				sanitizedCount++
				return ls.maskMatch(match, pattern)
			})
		}
	}
	
	if sanitizedCount > 0 {
		atomic.AddUint64(&ls.dataSanitized, sanitizedCount)
	}
	
	return result
}

// maskMatch masks a matched sensitive data string
func (ls *LogSanitizer) maskMatch(match string, pattern *Pattern) string {
	if ls.preserveLength {
		// Preserve original length
		masked := strings.Repeat(ls.maskingChar, len(match))
		if ls.showType {
			return fmt.Sprintf("[%s:%s]", pattern.Replacement, masked)
		}
		return masked
	}
	
	// Fixed length masking
	if ls.showType {
		return fmt.Sprintf("[REDACTED:%s]", pattern.Replacement)
	}
	return "[REDACTED]"
}

// SanitizeMap sanitizes all string values in a map
func (ls *LogSanitizer) SanitizeMap(data map[string]interface{}) map[string]interface{} {
	if !ls.enabled || data == nil {
		return data
	}
	
	result := make(map[string]interface{}, len(data))
	
	for key, value := range data {
		switch v := value.(type) {
		case string:
			result[key] = ls.Sanitize(v)
		case map[string]interface{}:
			result[key] = ls.SanitizeMap(v)
		case []interface{}:
			result[key] = ls.sanitizeSlice(v)
		default:
			result[key] = value
		}
	}
	
	return result
}

// sanitizeSlice sanitizes all string values in a slice
func (ls *LogSanitizer) sanitizeSlice(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	
	for i, value := range data {
		switch v := value.(type) {
		case string:
			result[i] = ls.Sanitize(v)
		case map[string]interface{}:
			result[i] = ls.SanitizeMap(v)
		case []interface{}:
			result[i] = ls.sanitizeSlice(v)
		default:
			result[i] = value
		}
	}
	
	return result
}

// AddCustomPattern adds a custom pattern for detection
func (ls *LogSanitizer) AddCustomPattern(name, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}
	
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	ls.customPatterns[name] = &Pattern{
		Type:        SensitiveDataType("custom_" + name),
		Name:        name,
		Regex:       compiled,
		Replacement: name,
	}
	
	// Recompile patterns
	ls.compilePatterns()
	
	return nil
}

// RemoveCustomPattern removes a custom pattern
func (ls *LogSanitizer) RemoveCustomPattern(name string) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	
	if _, exists := ls.customPatterns[name]; exists {
		delete(ls.customPatterns, name)
		ls.compilePatterns()
		return true
	}
	
	return false
}

// SetEnabled enables or disables the sanitizer
func (ls *LogSanitizer) SetEnabled(enabled bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.enabled = enabled
}

// IsEnabled returns whether the sanitizer is enabled
func (ls *LogSanitizer) IsEnabled() bool {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.enabled
}

// GetMetrics returns sanitization metrics
func (ls *LogSanitizer) GetMetrics() (logsProcessed, dataSanitized uint64) {
	return atomic.LoadUint64(&ls.logsProcessed), atomic.LoadUint64(&ls.dataSanitized)
}

// ResetMetrics resets the metrics counters
func (ls *LogSanitizer) ResetMetrics() {
	atomic.StoreUint64(&ls.logsProcessed, 0)
	atomic.StoreUint64(&ls.dataSanitized, 0)
}

// BatchSanitize processes multiple log messages efficiently
func (ls *LogSanitizer) BatchSanitize(messages []string) []string {
	if !ls.enabled || len(messages) == 0 {
		return messages
	}
	
	result := make([]string, len(messages))
	
	// Process in parallel for large batches
	if len(messages) > 100 {
		var wg sync.WaitGroup
		workers := 4 // Optimal for most cases
		batchSize := len(messages) / workers
		
		for i := 0; i < workers; i++ {
			start := i * batchSize
			end := start + batchSize
			if i == workers-1 {
				end = len(messages)
			}
			
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for j := start; j < end; j++ {
					result[j] = ls.Sanitize(messages[j])
				}
			}(start, end)
		}
		
		wg.Wait()
	} else {
		// Process sequentially for small batches
		for i, msg := range messages {
			result[i] = ls.Sanitize(msg)
		}
	}
	
	return result
}

// ShouldSanitize checks if a log message contains sensitive data
func (ls *LogSanitizer) ShouldSanitize(logMessage string) bool {
	if !ls.enabled || logMessage == "" {
		return false
	}
	
	ls.mu.RLock()
	patterns := ls.compiledPatterns
	ls.mu.RUnlock()
	
	for _, pattern := range patterns {
		if pattern.Regex.MatchString(logMessage) {
			return true
		}
	}
	
	return false
}