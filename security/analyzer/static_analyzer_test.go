package analyzer

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewStaticAnalyzer(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	if analyzer == nil {
		t.Fatal("NewStaticAnalyzer returned nil")
	}
	
	patterns := analyzer.GetPatterns()
	if len(patterns) == 0 {
		t.Error("Expected default patterns to be loaded")
	}
	
	// Verify some key patterns exist
	patternNames := make(map[string]bool)
	for _, p := range patterns {
		patternNames[p.Name] = true
	}
	
	expectedPatterns := []string{
		"eval_usage",
		"function_constructor",
		"file_system_access",
		"child_process",
		"network_access",
		"infinite_loop",
	}
	
	for _, expected := range expectedPatterns {
		if !patternNames[expected] {
			t.Errorf("Expected pattern %s not found", expected)
		}
	}
}

func TestAnalyzeScript_Empty(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	issues, err := analyzer.AnalyzeScript("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if len(issues) != 0 {
		t.Error("Expected no issues for empty script")
	}
}

func TestAnalyzeScript_EvalUsage(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	testCases := []struct {
		name     string
		script   string
		expected int
	}{
		{
			name:     "Simple eval",
			script:   `eval("console.log('test')")`,
			expected: 1,
		},
		{
			name:     "Multiple evals",
			script:   `eval(code1); eval(code2);`,
			expected: 2,
		},
		{
			name:     "Eval in function",
			script:   `function dangerous() { return eval(userInput); }`,
			expected: 1,
		},
		{
			name:     "No eval",
			script:   `console.log("evaluation"); // eval is just in comment`,
			expected: 0,
		},
		{
			name:     "Eval as property",
			script:   `obj.eval = 5; obj.evaluate()`,
			expected: 0,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			issues, err := analyzer.AnalyzeScript(tc.script)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			evalIssues := FilterBySeverity(issues, SeverityHigh)
			evalCount := 0
			for _, issue := range evalIssues {
				if issue.Type == "eval_usage" {
					evalCount++
				}
			}
			
			if evalCount != tc.expected {
				t.Errorf("Expected %d eval issues, got %d", tc.expected, evalCount)
			}
		})
	}
}

func TestAnalyzeScript_FileSystemAccess(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	scripts := []string{
		`const fs = require('fs')`,
		`require("fs").readFile()`,
		`const filesystem = require('fs')`,
	}
	
	for _, script := range scripts {
		issues, err := analyzer.AnalyzeScript(script)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		found := false
		for _, issue := range issues {
			if issue.Type == "file_system_access" {
				found = true
				if issue.Severity != SeverityHigh {
					t.Error("File system access should be high severity")
				}
				break
			}
		}
		
		if !found {
			t.Errorf("Failed to detect file system access in: %s", script)
		}
	}
}

func TestAnalyzeScript_InfiniteLoop(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	testCases := []struct {
		script      string
		shouldDetect bool
	}{
		{`while(true) { }`, true},
		{`while (true) { doSomething(); }`, true},
		{`for(;;) { }`, true},
		{`for ( ; ; ) { break; }`, true},
		{`while(1) { }`, true},
		{`while(condition) { }`, false},
		{`for(i=0; i<10; i++) { }`, false},
	}
	
	for _, tc := range testCases {
		issues, err := analyzer.AnalyzeScript(tc.script)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		found := false
		for _, issue := range issues {
			if issue.Type == "infinite_loop" {
				found = true
				break
			}
		}
		
		if found != tc.shouldDetect {
			t.Errorf("Script: %s, expected detection: %v, got: %v", 
				tc.script, tc.shouldDetect, found)
		}
	}
}

func TestAnalyzeScript_MultipleIssues(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	script := `
		const fs = require('fs');
		eval(userInput);
		while(true) {
			new Function(code)();
		}
		require('https').get(url);
	`
	
	issues, err := analyzer.AnalyzeScript(script)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if len(issues) < 5 {
		t.Errorf("Expected at least 5 issues, got %d", len(issues))
	}
	
	// Check that different types of issues were found
	issueTypes := make(map[string]bool)
	for _, issue := range issues {
		issueTypes[issue.Type] = true
	}
	
	expectedTypes := []string{
		"file_system_access",
		"eval_usage",
		"infinite_loop",
		"function_constructor",
		"network_access",
	}
	
	for _, expectedType := range expectedTypes {
		if !issueTypes[expectedType] {
			t.Errorf("Expected issue type %s not found", expectedType)
		}
	}
}

func TestAnalyzeScript_LineAndColumn(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	script := `function test() {
    var x = 1;
    eval("dangerous");
    return x;
}`
	
	issues, err := analyzer.AnalyzeScript(script)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Find the eval issue
	var evalIssue *SecurityIssue
	for i, issue := range issues {
		if issue.Type == "eval_usage" {
			evalIssue = &issues[i]
			break
		}
	}
	
	if evalIssue == nil {
		t.Fatal("Expected to find eval issue")
	}
	
	if evalIssue.Line != 3 {
		t.Errorf("Expected line 3, got %d", evalIssue.Line)
	}
	
	if evalIssue.Column < 5 || evalIssue.Column > 10 {
		t.Errorf("Expected column around 5-10, got %d", evalIssue.Column)
	}
	
	if !strings.Contains(evalIssue.Snippet, "eval") {
		t.Error("Snippet should contain the eval call")
	}
}

func TestAddPattern(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	// Add a custom pattern
	err := analyzer.AddPattern(
		"custom_alert",
		`\balert\s*\(`,
		SeverityLow,
		"Alert usage detected",
	)
	
	if err != nil {
		t.Fatalf("Failed to add pattern: %v", err)
	}
	
	// Test the custom pattern
	script := `alert("Hello world");`
	issues, err := analyzer.AnalyzeScript(script)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	found := false
	for _, issue := range issues {
		if issue.Type == "custom_alert" {
			found = true
			if issue.Severity != SeverityLow {
				t.Error("Custom pattern should have low severity")
			}
			break
		}
	}
	
	if !found {
		t.Error("Custom pattern was not detected")
	}
	
	// Test invalid pattern
	err = analyzer.AddPattern("invalid", "[", SeverityLow, "Invalid regex")
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestRemovePattern(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	
	// Add a pattern first
	analyzer.AddPattern("test_pattern", `test`, SeverityLow, "Test pattern")
	
	// Remove it
	removed := analyzer.RemovePattern("test_pattern")
	if !removed {
		t.Error("Failed to remove pattern")
	}
	
	// Try to remove non-existent pattern
	removed = analyzer.RemovePattern("non_existent")
	if removed {
		t.Error("Should not remove non-existent pattern")
	}
	
	// Verify pattern is gone
	patterns := analyzer.GetPatterns()
	for _, p := range patterns {
		if p.Name == "test_pattern" {
			t.Error("Pattern should have been removed")
		}
	}
}

func TestFilterBySeverity(t *testing.T) {
	issues := []SecurityIssue{
		{Type: "eval", Severity: SeverityHigh},
		{Type: "loop", Severity: SeverityMedium},
		{Type: "deprecated", Severity: SeverityLow},
		{Type: "fs", Severity: SeverityHigh},
	}
	
	// Filter high severity
	high := FilterBySeverity(issues, SeverityHigh)
	if len(high) != 2 {
		t.Errorf("Expected 2 high severity issues, got %d", len(high))
	}
	
	// Filter multiple severities
	mediumLow := FilterBySeverity(issues, SeverityMedium, SeverityLow)
	if len(mediumLow) != 2 {
		t.Errorf("Expected 2 medium/low severity issues, got %d", len(mediumLow))
	}
	
	// No filter (empty severities)
	all := FilterBySeverity(issues)
	if len(all) != len(issues) {
		t.Error("Empty filter should return all issues")
	}
}

func TestHasHighSeverityIssues(t *testing.T) {
	// No issues
	if HasHighSeverityIssues([]SecurityIssue{}) {
		t.Error("Empty issues should not have high severity")
	}
	
	// Only low/medium issues
	lowMedium := []SecurityIssue{
		{Severity: SeverityLow},
		{Severity: SeverityMedium},
	}
	if HasHighSeverityIssues(lowMedium) {
		t.Error("Should not detect high severity")
	}
	
	// Has high severity
	withHigh := []SecurityIssue{
		{Severity: SeverityLow},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
	}
	if !HasHighSeverityIssues(withHigh) {
		t.Error("Should detect high severity issue")
	}
}

func TestThreadSafety(t *testing.T) {
	analyzer := NewStaticAnalyzer()
	script := `eval("test"); require('fs');`
	
	// Run concurrent analyses
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				// Analyze
				issues, err := analyzer.AnalyzeScript(script)
				if err != nil {
					t.Errorf("Goroutine %d: %v", id, err)
				}
				if len(issues) < 2 {
					t.Errorf("Goroutine %d: Expected at least 2 issues", id)
				}
				
				// Add/remove patterns
				patternName := fmt.Sprintf("pattern_%d", id)
				analyzer.AddPattern(patternName, `test`, SeverityLow, "Test")
				analyzer.RemovePattern(patternName)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkAnalyzeScript_Small(b *testing.B) {
	analyzer := NewStaticAnalyzer()
	script := `console.log("Hello world");`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeScript(script)
	}
}

func BenchmarkAnalyzeScript_Medium(b *testing.B) {
	analyzer := NewStaticAnalyzer()
	script := strings.Repeat(`
		function test() {
			var x = Math.random();
			if (x > 0.5) {
				console.log(x);
			}
			return x * 2;
		}
	`, 10)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeScript(script)
	}
}

func BenchmarkAnalyzeScript_WithIssues(b *testing.B) {
	analyzer := NewStaticAnalyzer()
	script := `
		eval("dangerous");
		require('fs').readFile();
		while(true) { }
		new Function("code")();
	`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeScript(script)
	}
}

func BenchmarkConcurrentAnalysis(b *testing.B) {
	analyzer := NewStaticAnalyzer()
	script := `eval("test"); require('fs');`
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			analyzer.AnalyzeScript(script)
		}
	})
}