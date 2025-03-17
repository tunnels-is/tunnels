package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(DebugLevel, buf, false)
	if logger == nil {
		t.Error("New returned nil")
	}
	if logger.level != DebugLevel {
		t.Errorf("Expected level %v, got %v", DebugLevel, logger.level)
	}
}

func TestDefaultLogger(t *testing.T) {
	logger1 := Default()
	logger2 := Default()
	if logger1 != logger2 {
		t.Error("Default returned different instances")
	}
}

func TestLogger_WithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(DebugLevel, buf, false)

	fields := map[string]any{
		"key": "value",
	}

	newLogger := logger.WithFields(fields)
	if newLogger == logger {
		t.Error("WithFields returned same instance")
	}

	// Test that fields are properly copied
	newLogger.Info("test message")
	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Error("Fields not properly included in log output")
	}
}

func TestLogger_Levels(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(InfoLevel, buf, false)

	// Debug should not be logged
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("Debug message was logged when it shouldn't be")
	}

	// Info should be logged
	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("Info message was not logged")
	}

	// Warning should be logged
	logger.Warn("warning message")
	if !strings.Contains(buf.String(), "warning message") {
		t.Error("Warning message was not logged")
	}

	// Error should be logged
	logger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("Error message was not logged")
	}
}

func TestLogger_JSONMode(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(DebugLevel, buf, true)

	logger.Info("test message", map[string]any{"key": "value"})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Errorf("Failed to unmarshal JSON log: %v", err)
	}

	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", entry.Message)
	}
	if entry.Level != InfoLevel {
		t.Errorf("Expected level %v, got %v", InfoLevel, entry.Level)
	}
	if entry.Fields["key"] != "value" {
		t.Error("Fields not properly included in JSON output")
	}
}

func TestLogger_CallerInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(DebugLevel, buf, true)

	logger.Info("test message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Errorf("Failed to unmarshal JSON log: %v", err)
	}

	if entry.File == "" {
		t.Error("File information not included")
	}
	if entry.Line == 0 {
		t.Error("Line information not included")
	}
	if entry.Function == "" {
		t.Error("Function information not included")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New(DebugLevel, buf, false)

	logger.SetLevel(InfoLevel)
	if logger.level != InfoLevel {
		t.Errorf("Expected level %v, got %v", InfoLevel, logger.level)
	}

	// Debug should not be logged after level change
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("Debug message was logged after level change")
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{Level(999), "UNKNOWN"},
	}

	for _, test := range tests {
		if test.level.String() != test.expected {
			t.Errorf("Expected %s for level %v, got %s", test.expected, test.level, test.level.String())
		}
	}
}
