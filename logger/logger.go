// Package logger provides a structured logging system with configurable verbosity levels
// for the nFlow Runtime. It supports both normal and verbose logging modes.
package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents the logging level
type Level int

const (
	// LevelError logs only errors
	LevelError Level = iota
	// LevelInfo logs informational messages and errors
	LevelInfo
	// LevelVerbose logs everything including debug information
	LevelVerbose
)

// Logger represents a structured logger with configurable verbosity
type Logger struct {
	level      Level
	prefix     string
	mu         sync.RWMutex
	timeFormat string
}

var (
	// Default is the default logger instance
	Default *Logger
	once    sync.Once
)

// Initialize sets up the default logger with the specified verbosity
func Initialize(verbose bool) {
	once.Do(func() {
		level := LevelInfo
		if verbose {
			level = LevelVerbose
		}
		Default = New("nflow", level)
	})
}

// New creates a new logger instance
func New(prefix string, level Level) *Logger {
	return &Logger{
		level:      level,
		prefix:     prefix,
		timeFormat: "2006-01-02 15:04:05.000",
	}
}

// SetLevel changes the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current logging level
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Error logs an error message (always shown)
func (l *Logger) Error(args ...interface{}) {
	l.log(LevelError, "ERROR", args...)
}

// Errorf logs a formatted error message (always shown)
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(LevelError, "ERROR", format, args...)
}

// Info logs an informational message
func (l *Logger) Info(args ...interface{}) {
	l.log(LevelInfo, "INFO", args...)
}

// Infof logs a formatted informational message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(LevelInfo, "INFO", format, args...)
}

// Verbose logs a verbose/debug message (only shown with -v flag)
func (l *Logger) Verbose(args ...interface{}) {
	l.log(LevelVerbose, "VERBOSE", args...)
}

// Verbosef logs a formatted verbose/debug message (only shown with -v flag)
func (l *Logger) Verbosef(format string, args ...interface{}) {
	l.logf(LevelVerbose, "VERBOSE", format, args...)
}

// log is the internal logging function
func (l *Logger) log(level Level, levelStr string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	caller := l.getCaller()
	timestamp := time.Now().Format(l.timeFormat)
	prefix := fmt.Sprintf("[%s] [%s] [%s] %s:", timestamp, l.prefix, levelStr, caller)

	message := fmt.Sprint(args...)
	log.Printf("%s %s", prefix, message)
}

// logf is the internal formatted logging function
func (l *Logger) logf(level Level, levelStr string, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	caller := l.getCaller()
	timestamp := time.Now().Format(l.timeFormat)
	prefix := fmt.Sprintf("[%s] [%s] [%s] %s:", timestamp, l.prefix, levelStr, caller)

	message := fmt.Sprintf(format, args...)
	log.Printf("%s %s", prefix, message)
}

// shouldLog checks if a message should be logged based on current level
func (l *Logger) shouldLog(level Level) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level <= l.level
}

// getCaller returns the calling function's file and line number
func (l *Logger) getCaller() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown:0"
	}

	// Get only the filename, not the full path
	parts := strings.Split(file, "/")
	filename := parts[len(parts)-1]

	return fmt.Sprintf("%s:%d", filename, line)
}

// Global convenience functions that use the default logger

// Error logs an error using the default logger
func Error(args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Error(args...)
}

// Errorf logs a formatted error using the default logger
func Errorf(format string, args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Errorf(format, args...)
}

// Info logs an info message using the default logger
func Info(args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Info(args...)
}

// Infof logs a formatted info message using the default logger
func Infof(format string, args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Infof(format, args...)
}

// Verbose logs a verbose message using the default logger
func Verbose(args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Verbose(args...)
}

// Verbosef logs a formatted verbose message using the default logger
func Verbosef(format string, args ...interface{}) {
	if Default == nil {
		Initialize(false)
	}
	Default.Verbosef(format, args...)
}

// Fatal logs an error and exits the program
func Fatal(args ...interface{}) {
	Error(args...)
	os.Exit(1)
}

// Fatalf logs a formatted error and exits the program
func Fatalf(format string, args ...interface{}) {
	Errorf(format, args...)
	os.Exit(1)
}
