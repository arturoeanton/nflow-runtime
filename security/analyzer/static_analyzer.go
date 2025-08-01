// Package analyzer provides static analysis capabilities for JavaScript code
// to detect potentially dangerous patterns before execution.
// This is designed as a non-intrusive layer that can be enabled/disabled
// via configuration without modifying the core engine.
package analyzer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Severity levels for security issues
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// SecurityIssue represents a security concern found in the analyzed code
type SecurityIssue struct {
	Type        string `json:"type"`        // Type of security issue (e.g., "eval_usage")
	Severity    string `json:"severity"`    // Severity level: high, medium, low
	Description string `json:"description"` // Human-readable description
	Line        int    `json:"line"`        // Line number where issue was found
	Column      int    `json:"column"`      // Column number where issue was found
	Snippet     string `json:"snippet"`     // Code snippet showing the issue
}

// SecurityPattern defines a pattern to search for in code
type SecurityPattern struct {
	Name        string         // Unique identifier for the pattern
	Pattern     *regexp.Regexp // Compiled regex pattern
	Severity    string         // Severity level if pattern is found
	Description string         // Description of the security risk
}

// StaticAnalyzer performs static analysis on JavaScript code
type StaticAnalyzer struct {
	patterns []SecurityPattern
	mu       sync.RWMutex // Protects patterns slice for thread-safe operations
	
	// Performance optimization: pre-compiled pattern cache
	patternCache map[string]*regexp.Regexp
	cacheMu      sync.RWMutex
}

// NewStaticAnalyzer creates a new analyzer with default security patterns
func NewStaticAnalyzer() *StaticAnalyzer {
	analyzer := &StaticAnalyzer{
		patternCache: make(map[string]*regexp.Regexp),
	}
	
	// Initialize with default dangerous patterns
	analyzer.patterns = []SecurityPattern{
		{
			Name:        "eval_usage",
			Pattern:     regexp.MustCompile(`\beval\s*\(`),
			Severity:    SeverityHigh,
			Description: "Direct eval() usage can execute arbitrary code",
		},
		{
			Name:        "function_constructor",
			Pattern:     regexp.MustCompile(`new\s+Function\s*\(`),
			Severity:    SeverityHigh,
			Description: "Function constructor can create arbitrary functions",
		},
		{
			Name:        "file_system_access",
			Pattern:     regexp.MustCompile(`require\s*\(\s*['"]fs['"]`),
			Severity:    SeverityHigh,
			Description: "Attempting to access file system",
		},
		{
			Name:        "child_process",
			Pattern:     regexp.MustCompile(`require\s*\(\s*['"]child_process['"]|spawn|exec\s*\(`),
			Severity:    SeverityHigh,
			Description: "Attempting to spawn child processes",
		},
		{
			Name:        "network_access",
			Pattern:     regexp.MustCompile(`require\s*\(\s*['"](?:https?|net|dgram)['"]`),
			Severity:    SeverityMedium,
			Description: "Attempting network access",
		},
		{
			Name:        "infinite_loop",
			Pattern:     regexp.MustCompile(`while\s*\(\s*(?:true|1)\s*\)|for\s*\(\s*;\s*;\s*\)`),
			Severity:    SeverityMedium,
			Description: "Potential infinite loop detected",
		},
		{
			Name:        "global_assignment",
			Pattern:     regexp.MustCompile(`(?:global|window|process)\s*\.\s*\w+\s*=`),
			Severity:    SeverityMedium,
			Description: "Modifying global scope",
		},
		{
			Name:        "dangerous_regex",
			Pattern:     regexp.MustCompile(`new\s+RegExp\s*\([^)]*\+\s*[^)]*\*|\(\?\!|\(\?\<\!`),
			Severity:    SeverityLow,
			Description: "Complex regex that might cause ReDoS",
		},
		{
			Name:        "dynamic_require",
			Pattern:     regexp.MustCompile(`require\s*\(\s*[^'"]`),
			Severity:    SeverityMedium,
			Description: "Dynamic require with non-literal argument",
		},
		{
			Name:        "buffer_constructor",
			Pattern:     regexp.MustCompile(`new\s+Buffer\s*\(`),
			Severity:    SeverityLow,
			Description: "Deprecated Buffer constructor usage",
		},
	}
	
	return analyzer
}

// AnalyzeScript performs static analysis on the provided JavaScript code
// Returns a slice of SecurityIssue found in the code
func (sa *StaticAnalyzer) AnalyzeScript(script string) ([]SecurityIssue, error) {
	if script == "" {
		return nil, nil
	}
	
	sa.mu.RLock()
	patterns := sa.patterns
	sa.mu.RUnlock()
	
	var issues []SecurityIssue
	lines := strings.Split(script, "\n")
	
	// Analyze each pattern
	for _, pattern := range patterns {
		matches := pattern.Pattern.FindAllStringIndex(script, -1)
		
		for _, match := range matches {
			line, column := getLineAndColumn(script, match[0])
			snippet := extractSnippet(lines, line-1, column)
			
			issues = append(issues, SecurityIssue{
				Type:        pattern.Name,
				Severity:    pattern.Severity,
				Description: pattern.Description,
				Line:        line,
				Column:      column,
				Snippet:     snippet,
			})
		}
	}
	
	return issues, nil
}

// AddPattern adds a custom security pattern to the analyzer
// This allows extending the analyzer without modifying the code
func (sa *StaticAnalyzer) AddPattern(name, pattern, severity, description string) error {
	compiledPattern, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	sa.mu.Lock()
	defer sa.mu.Unlock()
	
	sa.patterns = append(sa.patterns, SecurityPattern{
		Name:        name,
		Pattern:     compiledPattern,
		Severity:    severity,
		Description: description,
	})
	
	return nil
}

// RemovePattern removes a pattern by name
func (sa *StaticAnalyzer) RemovePattern(name string) bool {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	
	for i, p := range sa.patterns {
		if p.Name == name {
			sa.patterns = append(sa.patterns[:i], sa.patterns[i+1:]...)
			return true
		}
	}
	
	return false
}

// GetPatterns returns a copy of current patterns (thread-safe)
func (sa *StaticAnalyzer) GetPatterns() []SecurityPattern {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	
	patterns := make([]SecurityPattern, len(sa.patterns))
	copy(patterns, sa.patterns)
	return patterns
}

// FilterBySeverity returns only issues matching the specified severity levels
func FilterBySeverity(issues []SecurityIssue, severities ...string) []SecurityIssue {
	if len(severities) == 0 {
		return issues
	}
	
	severityMap := make(map[string]bool)
	for _, s := range severities {
		severityMap[s] = true
	}
	
	var filtered []SecurityIssue
	for _, issue := range issues {
		if severityMap[issue.Severity] {
			filtered = append(filtered, issue)
		}
	}
	
	return filtered
}

// HasHighSeverityIssues checks if any high severity issues exist
func HasHighSeverityIssues(issues []SecurityIssue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityHigh {
			return true
		}
	}
	return false
}

// getLineAndColumn calculates line and column number from byte position
func getLineAndColumn(text string, position int) (line, column int) {
	line = 1
	column = 1
	
	for i := 0; i < position && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	
	return line, column
}

// extractSnippet extracts a code snippet around the issue
func extractSnippet(lines []string, lineIndex, column int) string {
	if lineIndex < 0 || lineIndex >= len(lines) {
		return ""
	}
	
	line := lines[lineIndex]
	
	// Extract a window around the issue (40 chars before and after)
	start := column - 40
	if start < 0 {
		start = 0
	}
	
	end := column + 40
	if end > len(line) {
		end = len(line)
	}
	
	snippet := line[start:end]
	
	// Add ellipsis if truncated
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(line) {
		snippet = snippet + "..."
	}
	
	return strings.TrimSpace(snippet)
}