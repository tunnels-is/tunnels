package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Level represents the logging level
type Level int

const (
	// DebugLevel represents debug level logging
	DebugLevel Level = iota
	// InfoLevel represents info level logging
	InfoLevel
	// WarnLevel represents warning level logging
	WarnLevel
	// ErrorLevel represents error level logging
	ErrorLevel
)

// Entry represents a log entry
type Entry struct {
	Time     time.Time      `json:"time"`
	Level    Level          `json:"level"`
	Message  string         `json:"message"`
	Fields   map[string]any `json:"fields,omitempty"`
	File     string         `json:"file,omitempty"`
	Line     int            `json:"line,omitempty"`
	Function string         `json:"function,omitempty"`
}

// Logger represents a logger instance
type Logger struct {
	mu       sync.RWMutex
	level    Level
	output   io.Writer
	fields   map[string]any
	jsonMode bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// New creates a new logger instance
func New(level Level, output io.Writer, jsonMode bool) *Logger {
	return &Logger{
		level:    level,
		output:   output,
		fields:   make(map[string]any),
		jsonMode: jsonMode,
	}
}

// Default returns the default logger instance
func Default() *Logger {
	once.Do(func() {
		defaultLogger = New(InfoLevel, os.Stdout, false)
	})
	return defaultLogger
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// WithFields adds fields to the logger
func (l *Logger) WithFields(fields map[string]any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := *l
	newLogger.fields = make(map[string]any)
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &newLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]any) {
	l.log(DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]any) {
	l.log(InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]any) {
	l.log(WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...map[string]any) {
	l.log(ErrorLevel, msg, fields...)
}

// log logs a message at the specified level
func (l *Logger) log(level Level, msg string, fields ...map[string]any) {
	l.mu.RLock()
	if level < l.level {
		l.mu.RUnlock()
		return
	}

	entry := Entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Fields:  make(map[string]any),
	}

	// Add caller information
	if _, file, line, ok := runtime.Caller(2); ok {
		entry.File = filepath.Base(file)
		entry.Line = line
	}
	if pc, _, _, ok := runtime.Caller(2); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			entry.Function = fn.Name()
		}
	}

	// Add fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	for _, f := range fields {
		for k, v := range f {
			entry.Fields[k] = v
		}
	}

	l.mu.RUnlock()

	// Format and write the log entry
	var output []byte
	var err error

	if l.jsonMode {
		output, err = json.Marshal(entry)
		if err != nil {
			log.Printf("Error marshaling log entry: %v", err)
			return
		}
		output = append(output, '\n')
	} else {
		output = []byte(fmt.Sprintf("%s [%s] %s %v\n",
			entry.Time.Format(time.RFC3339),
			level.String(),
			msg,
			entry.Fields))
	}

	l.mu.Lock()
	_, err = l.output.Write(output)
	l.mu.Unlock()

	if err != nil {
		log.Printf("Error writing log: %v", err)
	}
}

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
