package composer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogEntry(t *testing.T) {
	err := errors.New("test error")
	entry := NewLogEntry("Test message", SeverityError, err)

	assert.Equal(t, "Test message", entry.Message)
	assert.Equal(t, SeverityError, entry.Severity)
	assert.Equal(t, err, entry.Error)
	assert.NotNil(t, entry.Parameters)
	assert.NotZero(t, entry.Date)
}

func TestLogEntrySetParameters(t *testing.T) {
	entry := NewLogEntry("Test message", SeverityInfo, nil)
	params := LogEntryParameters{"key": "value"}

	result := entry.SetParameters(params)

	assert.Equal(t, params, entry.Parameters)
	assert.Equal(t, entry, result) // Check method chaining
}

func TestLogEntrySetParameter(t *testing.T) {
	entry := NewLogEntry("Test message", SeverityInfo, nil)

	result := entry.SetParameter("key1", "value1")
	assert.Equal(t, "value1", entry.Parameters["key1"])
	assert.Equal(t, entry, result) // Check method chaining

	entry.SetParameter("key2", 42)
	assert.Equal(t, 42, entry.Parameters["key2"])
}

func TestLogEntrySetSeverity(t *testing.T) {
	entry := NewLogEntry("Test message", SeverityInfo, nil)

	result := entry.SetSeverity(SeverityError)

	assert.Equal(t, SeverityError, entry.Severity)
	assert.Equal(t, entry, result) // Check method chaining
}

func TestLogEntrySetError(t *testing.T) {
	entry := NewLogEntry("Test message", SeverityInfo, nil)
	err := errors.New("new error")

	result := entry.SetError(err)

	assert.Equal(t, err, entry.Error)
	assert.Equal(t, entry, result) // Check method chaining
}

func TestLoggerFormatError(t *testing.T) {
	lg := &LoggerType{FilterLevel: SeverityTrace}

	// Test with nil error
	result := lg.FormatError(nil)
	assert.Equal(t, "", result)

	// Test with actual error
	err := errors.New("test error")
	result = lg.FormatError(err)
	assert.Contains(t, result, "test error")
}

func TestLoggerInfo(t *testing.T) {
	lg := &LoggerType{FilterLevel: SeverityTrace}

	// Test without parameters
	entry := lg.Info("Test message")
	assert.Equal(t, "Test message", entry.Message)
	assert.Equal(t, SeverityInfo, entry.Severity)
	assert.Nil(t, entry.Error)

	// Test with parameters
	params := LogEntryParameters{"key": "value"}
	entry = lg.Info("Test message", params)
	assert.Equal(t, params, entry.Parameters)
}

func TestLoggerTrace(t *testing.T) {
	lg := &LoggerType{FilterLevel: SeverityTrace}

	// Test without parameters
	entry := lg.Trace("Test message")
	assert.Equal(t, "Test message", entry.Message)
	assert.Equal(t, SeverityTrace, entry.Severity)
	assert.Nil(t, entry.Error)

	// Test with parameters
	params := LogEntryParameters{"key": "value"}
	entry = lg.Trace("Test message", params)
	assert.Equal(t, params, entry.Parameters)
}

func TestLoggerError(t *testing.T) {
	lg := &LoggerType{FilterLevel: SeverityTrace}
	err := errors.New("test error")

	entry := lg.Error("Error message", err)

	assert.Equal(t, "Error message", entry.Message)
	assert.Equal(t, SeverityError, entry.Severity)
	assert.Equal(t, err, entry.Error)
}

func TestLoggerLog(t *testing.T) {
	lg := &LoggerType{FilterLevel: SeverityInfo}

	// Test that trace messages are filtered out
	traceEntry := NewLogEntry("Trace message", SeverityTrace, nil)
	lg.Log(traceEntry) // Should not log anything

	// Test that info messages are logged
	infoEntry := NewLogEntry("Info message", SeverityInfo, nil)
	lg.Log(infoEntry) // Should log

	// Test with error
	err := errors.New("test error")
	errorEntry := NewLogEntry("Error message", SeverityError, err)
	lg.Log(errorEntry) // Should log with error

	// Test with parameters
	params := LogEntryParameters{"key": "value"}
	paramEntry := NewLogEntry("Param message", SeverityInfo, nil).SetParameters(params)
	lg.Log(paramEntry) // Should log with parameters
}

func TestLogEntryLog(t *testing.T) {
	// Save original filter level
	originalFilterLevel := Logger.FilterLevel
	defer func() { Logger.FilterLevel = originalFilterLevel }()

	Logger.FilterLevel = SeverityInfo

	entry := NewLogEntry("Test message", SeverityInfo, nil)
	entry.Log() // Should not panic
}

func TestLoggerFilterLevel(t *testing.T) {
	// Save original filter level
	originalFilterLevel := Logger.FilterLevel
	defer func() { Logger.FilterLevel = originalFilterLevel }()

	Logger.FilterLevel = SeverityError

	// Create entries at different levels
	traceEntry := NewLogEntry("Trace", SeverityTrace, nil)
	infoEntry := NewLogEntry("Info", SeverityInfo, nil)
	errorEntry := NewLogEntry("Error", SeverityError, nil)

	// Only error should pass the filter
	Logger.Log(traceEntry) // Should be filtered
	Logger.Log(infoEntry)  // Should be filtered
	Logger.Log(errorEntry) // Should be logged

	// Test NONE filter level - nothing should log
	Logger.FilterLevel = SeverityNone
	Logger.Log(errorEntry) // Should be filtered
}
