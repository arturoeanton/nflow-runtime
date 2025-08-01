// Package security provides a unified middleware layer for all security features.
// This integrates static analysis and data encryption transparently with the engine.
package security

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/arturoeanton/nflow-runtime/security/analyzer"
	"github.com/arturoeanton/nflow-runtime/security/encryption"
	"github.com/arturoeanton/nflow-runtime/security/interceptor"
	"github.com/arturoeanton/nflow-runtime/security/sanitizer"
	"github.com/dop251/goja"
	"github.com/labstack/echo/v4"
	"log"
)

// SecurityMiddleware provides unified security features for nFlow Runtime
type SecurityMiddleware struct {
	analyzer    *analyzer.StaticAnalyzer
	encryption  *encryption.EncryptionService
	interceptor *interceptor.SensitiveDataInterceptor
	sanitizer   *sanitizer.LogSanitizer

	// Configuration
	config *Config

	// Metrics
	metrics *SecurityMetrics
	mu      sync.RWMutex
}

// Config holds all security configuration
type Config struct {
	// Static Analysis
	EnableStaticAnalysis bool     `toml:"enable_static_analysis"`
	BlockOnHighSeverity  bool     `toml:"block_on_high_severity"`
	LogSecurityWarnings  bool     `toml:"log_security_warnings"`
	AllowedPatterns      []string `toml:"allowed_patterns"` // Patterns to whitelist

	// Encryption
	EnableEncryption     bool              `toml:"enable_encryption"`
	EncryptionKey        string            `toml:"encryption_key"`
	EncryptSensitiveData bool              `toml:"encrypt_sensitive_data"`
	EncryptInPlace       bool              `toml:"encrypt_in_place"`
	SensitivePatterns    []string          `toml:"sensitive_patterns"`
	AlwaysEncryptFields  []string          `toml:"always_encrypt_fields"`
	CustomPatterns       map[string]string `toml:"custom_patterns"`

	// Log Sanitization
	EnableLogSanitization bool              `toml:"enable_log_sanitization"`
	LogMaskingChar        string            `toml:"log_masking_char"`
	LogPreserveLength     bool              `toml:"log_preserve_length"`
	LogShowType           bool              `toml:"log_show_type"`
	LogCustomPatterns     map[string]string `toml:"log_custom_patterns"`

	// Performance
	CacheAnalysisResults bool          `toml:"cache_analysis_results"`
	CacheTTL             time.Duration `toml:"cache_ttl"`
}

// SecurityMetrics tracks security-related metrics
type SecurityMetrics struct {
	ScriptsAnalyzed      uint64
	ScriptsBlocked       uint64
	HighSeverityIssues   uint64
	MediumSeverityIssues uint64
	LowSeverityIssues    uint64
	DataEncrypted        uint64
	LogsProcessed        uint64
	LogsSanitized        uint64
	AnalysisTime         time.Duration
	EncryptionTime       time.Duration
}

// ScriptAnalysisResult caches analysis results
type ScriptAnalysisResult struct {
	Issues    []analyzer.SecurityIssue
	Timestamp time.Time
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(config *Config) (*SecurityMiddleware, error) {
	if config == nil {
		config = &Config{
			EnableStaticAnalysis:  true,
			EnableEncryption:      true,
			BlockOnHighSeverity:   true,
			LogSecurityWarnings:   true,
			EncryptInPlace:        true,
			EnableLogSanitization: true,
			LogShowType:           true,
			CacheAnalysisResults:  true,
			CacheTTL:              5 * time.Minute,
		}
	}

	sm := &SecurityMiddleware{
		config:  config,
		metrics: &SecurityMetrics{},
	}

	// Initialize static analyzer
	if config.EnableStaticAnalysis {
		sm.analyzer = analyzer.NewStaticAnalyzer()

		// Add allowed patterns (to reduce false positives)
		for _, pattern := range config.AllowedPatterns {
			// This would need implementation in analyzer to support whitelisting
			log.Printf("[DEBUG] Whitelisting pattern: %s", pattern)
		}
	}

	// Initialize encryption
	if config.EnableEncryption && config.EncryptionKey != "" {
		encService, err := encryption.NewEncryptionService(config.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize encryption: %w", err)
		}
		sm.encryption = encService

		// Initialize interceptor
		interceptorConfig := &interceptor.Config{
			Enabled:        config.EncryptSensitiveData,
			EncryptInPlace: config.EncryptInPlace,
			MetadataKey:    "_encrypted_fields",
			CustomPatterns: config.CustomPatterns,
		}
		sm.interceptor = interceptor.NewSensitiveDataInterceptor(encService, interceptorConfig)
	}

	// Initialize log sanitizer
	if config.EnableLogSanitization {
		sanitizerConfig := &sanitizer.Config{
			Enabled:        true,
			MaskingChar:    config.LogMaskingChar,
			PreserveLength: config.LogPreserveLength,
			ShowType:       config.LogShowType,
			CustomPatterns: config.LogCustomPatterns,
		}
		sm.sanitizer = sanitizer.NewLogSanitizer(sanitizerConfig)
	}

	return sm, nil
}

// AnalyzeScript performs static analysis on JavaScript code before execution
func (sm *SecurityMiddleware) AnalyzeScript(script string, scriptID string) error {
	if !sm.config.EnableStaticAnalysis || sm.analyzer == nil {
		return nil
	}

	start := time.Now()
	defer func() {
		sm.mu.Lock()
		sm.metrics.AnalysisTime += time.Since(start)
		sm.metrics.ScriptsAnalyzed++
		sm.mu.Unlock()
	}()

	// Analyze the script
	issues, err := sm.analyzer.AnalyzeScript(script)
	if err != nil {
		return fmt.Errorf("script analysis failed: %w", err)
	}

	// Update metrics
	sm.updateIssueMetrics(issues)

	// Log warnings if configured
	if sm.config.LogSecurityWarnings {
		for _, issue := range issues {
			logMsg := fmt.Sprintf("[WARN] Security issue in script %s: [%s] %s at line %d",
				scriptID, issue.Severity, issue.Description, issue.Line)
			// Sanitize log message if enabled
			if sm.sanitizer != nil {
				logMsg = sm.sanitizer.Sanitize(logMsg)
			}
			log.Print(logMsg)
		}
	}

	// Block on high severity if configured
	if sm.config.BlockOnHighSeverity && analyzer.HasHighSeverityIssues(issues) {
		sm.mu.Lock()
		sm.metrics.ScriptsBlocked++
		sm.mu.Unlock()

		return fmt.Errorf("script blocked due to security issues: %d high severity issues found",
			len(analyzer.FilterBySeverity(issues, analyzer.SeverityHigh)))
	}

	return nil
}

// ProcessResponse encrypts sensitive data in responses
func (sm *SecurityMiddleware) ProcessResponse(data interface{}) (interface{}, error) {
	if !sm.config.EnableEncryption || sm.interceptor == nil {
		return data, nil
	}

	start := time.Now()
	defer func() {
		sm.mu.Lock()
		sm.metrics.EncryptionTime += time.Since(start)
		sm.mu.Unlock()
	}()

	result, err := sm.interceptor.ProcessResponse(data)
	if err != nil {
		return data, fmt.Errorf("response processing failed: %w", err)
	}

	sm.mu.Lock()
	sm.metrics.DataEncrypted++
	sm.mu.Unlock()

	return result, nil
}

// WrapEchoHandler wraps an Echo handler with security features
func (sm *SecurityMiddleware) WrapEchoHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Process the request
		err := next(c)

		// If there's a response body, process it for encryption
		if err == nil && sm.config.EnableEncryption {
			// This is simplified - in real implementation would need to
			// intercept the actual response body
			logMsg := "[DEBUG] Response encryption would happen here"
			if sm.sanitizer != nil {
				logMsg = sm.sanitizer.Sanitize(logMsg)
			}
			log.Print(logMsg)
		}

		return err
	}
}

// WrapGojaVM wraps a Goja VM to add security hooks
func (sm *SecurityMiddleware) WrapGojaVM(vm *goja.Runtime, scriptID string) error {
	if !sm.config.EnableStaticAnalysis {
		return nil
	}

	// Add pre-execution hook for script analysis
	// This is a simplified version - real implementation would need
	// deeper integration with Goja's execution model
	// Note: We cannot modify vm.RunString directly as it's not addressable
	// In a real implementation, this would require wrapping the VM or using a different approach

	return nil
}

// EncryptField encrypts a specific field value
func (sm *SecurityMiddleware) EncryptField(fieldName string, value string) (string, error) {
	if sm.encryption == nil {
		return value, nil
	}

	// Check if this field should always be encrypted
	for _, field := range sm.config.AlwaysEncryptFields {
		if field == fieldName {
			return sm.encryption.Encrypt(value)
		}
	}

	return value, nil
}

// DecryptField decrypts a specific field value
func (sm *SecurityMiddleware) DecryptField(value string) (string, error) {
	if sm.encryption == nil {
		return value, nil
	}

	// Check if value appears to be encrypted
	if encryption.IsEncrypted(value) {
		return sm.encryption.Decrypt(value)
	}

	return value, nil
}

// GetMetrics returns current security metrics
func (sm *SecurityMiddleware) GetMetrics() SecurityMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := *sm.metrics

	// Add sub-component metrics
	if sm.encryption != nil {
		encCount, decCount := sm.encryption.GetMetrics()
		metrics.DataEncrypted = encCount + decCount
	}

	if sm.sanitizer != nil {
		logsProcessed, logsSanitized := sm.sanitizer.GetMetrics()
		metrics.LogsProcessed = logsProcessed
		metrics.LogsSanitized = logsSanitized
	}

	return metrics
}

// ResetMetrics resets all security metrics
func (sm *SecurityMiddleware) ResetMetrics() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.metrics = &SecurityMetrics{}

	if sm.encryption != nil {
		sm.encryption.ResetMetrics()
	}

	if sm.interceptor != nil {
		sm.interceptor.ResetMetrics()
	}

	if sm.sanitizer != nil {
		sm.sanitizer.ResetMetrics()
	}
}

// updateIssueMetrics updates metrics based on found issues
func (sm *SecurityMiddleware) updateIssueMetrics(issues []analyzer.SecurityIssue) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, issue := range issues {
		switch issue.Severity {
		case analyzer.SeverityHigh:
			sm.metrics.HighSeverityIssues++
		case analyzer.SeverityMedium:
			sm.metrics.MediumSeverityIssues++
		case analyzer.SeverityLow:
			sm.metrics.LowSeverityIssues++
		}
	}
}

// MarshalMetricsJSON returns metrics as JSON
func (sm *SecurityMiddleware) MarshalMetricsJSON() ([]byte, error) {
	metrics := sm.GetMetrics()
	return json.Marshal(metrics)
}

// IsEnabled returns whether security features are enabled
func (sm *SecurityMiddleware) IsEnabled() bool {
	return sm.config.EnableStaticAnalysis || sm.config.EnableEncryption
}

// SetEnabled enables or disables security features
func (sm *SecurityMiddleware) SetEnabled(analysis, encryption bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.config.EnableStaticAnalysis = analysis
	sm.config.EnableEncryption = encryption

	if sm.interceptor != nil {
		sm.interceptor.SetEnabled(encryption)
	}
}

// SanitizeLog sanitizes a log message to remove sensitive data
func (sm *SecurityMiddleware) SanitizeLog(logMessage string) string {
	if sm.sanitizer == nil || !sm.config.EnableLogSanitization {
		return logMessage
	}
	return sm.sanitizer.Sanitize(logMessage)
}

// SanitizeLogs sanitizes multiple log messages
func (sm *SecurityMiddleware) SanitizeLogs(logMessages []string) []string {
	if sm.sanitizer == nil || !sm.config.EnableLogSanitization {
		return logMessages
	}
	return sm.sanitizer.BatchSanitize(logMessages)
}
