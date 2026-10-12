package composer

import (
	"fmt"
	"log"
	"time"
)

// ComposerLogLevel represents the logging level
type ComposerLogLevel string

const (
	ComposerLogLevelTrace ComposerLogLevel = "TRACE"
	ComposerLogLevelInfo  ComposerLogLevel = "INFO"
	ComposerLogLevelError ComposerLogLevel = "ERROR"
	ComposerLogLevelNone  ComposerLogLevel = "NONE"
)

// DodgeballLogLevel is an alias for backward compatibility
type DodgeballLogLevel = ComposerLogLevel

// Severity represents the numeric severity level
type Severity int

const (
	SeverityTrace Severity = 0
	SeverityInfo  Severity = 1
	SeverityError Severity = 2
	SeverityNone  Severity = 3
)

// LogEntryParameters is a map of parameters for a log entry
type LogEntryParameters map[string]interface{}

// LogEntry represents a single log entry
type LogEntry struct {
	Severity   Severity
	Message    string
	Error      error
	Parameters LogEntryParameters
	Date       time.Time
}

// NewLogEntry creates a new log entry with the given message and severity
func NewLogEntry(message string, severity Severity, err error) *LogEntry {
	return &LogEntry{
		Message:    message,
		Severity:   severity,
		Error:      err,
		Date:       time.Now(),
		Parameters: make(LogEntryParameters),
	}
}

// SetParameters sets the parameters for the log entry
func (l *LogEntry) SetParameters(parameters LogEntryParameters) *LogEntry {
	l.Parameters = parameters
	return l
}

// SetParameter sets a single parameter for the log entry
func (l *LogEntry) SetParameter(key string, value interface{}) *LogEntry {
	if l.Parameters == nil {
		l.Parameters = make(LogEntryParameters)
	}
	l.Parameters[key] = value
	return l
}

// SetSeverity sets the severity of the log entry
func (l *LogEntry) SetSeverity(severity Severity) *LogEntry {
	l.Severity = severity
	return l
}

// SetError sets the error for the log entry
func (l *LogEntry) SetError(err error) *LogEntry {
	l.Error = err
	return l
}

// Log logs the entry using the Logger
func (l *LogEntry) Log() {
	Logger.Log(l)
}

// LoggerType manages logging for the SDK
type LoggerType struct {
	FilterLevel Severity
}

// Logger is the global logger instance
var Logger = &LoggerType{
	FilterLevel: SeverityInfo,
}

// IsTraceEnabled reports whether a trace entry would be written. Callers check this
// before assembling an expensive trace message, because the message is built before
// Log gets a chance to discard it.
func (lg *LoggerType) IsTraceEnabled() bool {
	return SeverityTrace >= lg.FilterLevel
}

// Info creates an info-level log entry
func (lg *LoggerType) Info(message string, parameters ...LogEntryParameters) *LogEntry {
	entry := NewLogEntry(message, SeverityInfo, nil)
	if len(parameters) > 0 {
		entry.SetParameters(parameters[0])
	}
	return entry
}

// Trace creates a trace-level log entry
func (lg *LoggerType) Trace(message string, parameters ...LogEntryParameters) *LogEntry {
	entry := NewLogEntry(message, SeverityTrace, nil)
	if len(parameters) > 0 {
		entry.SetParameters(parameters[0])
	}
	return entry
}

// Error creates an error-level log entry
func (lg *LoggerType) Error(message string, err error) *LogEntry {
	return NewLogEntry(message, SeverityError, err)
}

// FormatError formats an error for logging
func (lg *LoggerType) FormatError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("error: %v", err)
}

// Log logs the given log entry if it meets the filter level
func (lg *LoggerType) Log(logEvent *LogEntry) {
	if logEvent.Severity >= lg.FilterLevel {
		logResults := fmt.Sprintf("\nSeverity: %s\nDate: %s\nMessage: %s\n",
			severityToString(logEvent.Severity),
			logEvent.Date.Format(time.RFC3339),
			logEvent.Message)

		if logEvent.Error != nil {
			logResults += lg.FormatError(logEvent.Error) + "\n"
		}

		if len(logEvent.Parameters) > 0 {
			logResults += fmt.Sprintf("parameters: %v\n", logEvent.Parameters)
		}

		log.Print(logResults)
	}
}

// severityToString converts a Severity to its string representation
// Internal helper; not directly tested (covered indirectly via Logger methods).
func severityToString(s Severity) string {
	switch s {
	case SeverityTrace:
		return "TRACE"
	case SeverityInfo:
		return "INFO"
	case SeverityError:
		return "ERROR"
	case SeverityNone:
		return "NONE"
	default:
		return "UNKNOWN"
	}
}

// logLevelToSeverity converts a ComposerLogLevel to a Severity
// Internal helper; not directly tested (covered indirectly via Logger methods).
func logLevelToSeverity(level ComposerLogLevel) Severity {
	switch level {
	case ComposerLogLevelTrace:
		return SeverityTrace
	case ComposerLogLevelInfo:
		return SeverityInfo
	case ComposerLogLevelError:
		return SeverityError
	case ComposerLogLevelNone:
		return SeverityNone
	default:
		return SeverityInfo
	}
}
