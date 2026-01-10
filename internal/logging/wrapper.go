package logging

import (
	"fmt"

	"github.com/charmbracelet/log"
)

// Wrapper functions that capture logs and then call the underlying logger
// These can be used as drop-in replacements for log.* calls

// Debug captures and logs a debug message
func Debug(msg string, args ...interface{}) {
	CaptureLog("debug", msg, args...)
	log.Debug(msg, args...)
}

// Info captures and logs an info message
func Info(msg string, args ...interface{}) {
	CaptureLog("info", msg, args...)
	log.Info(msg, args...)
}

// Warn captures and logs a warning message
func Warn(msg string, args ...interface{}) {
	CaptureLog("warn", msg, args...)
	log.Warn(msg, args...)
}

// Error captures and logs an error message
func Error(msg string, args ...interface{}) {
	CaptureLog("error", msg, args...)
	log.Error(msg, args...)
}

// Fatal captures and logs a fatal message
func Fatal(msg string, args ...interface{}) {
	CaptureLog("fatal", msg, args...)
	log.Fatal(msg, args...)
}

// Debugf captures and logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	// Format the message first before capturing
	msg := fmt.Sprintf(format, args...)
	CaptureLog("debug", msg)
	log.Debugf(format, args...)
}

// Infof captures and logs a formatted info message
func Infof(format string, args ...interface{}) {
	// Format the message first before capturing
	msg := fmt.Sprintf(format, args...)
	CaptureLog("info", msg)
	log.Infof(format, args...)
}

// Warnf captures and logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	// Format the message first before capturing
	msg := fmt.Sprintf(format, args...)
	CaptureLog("warn", msg)
	log.Warnf(format, args...)
}

// Errorf captures and logs a formatted error message
func Errorf(format string, args ...interface{}) {
	// Format the message first before capturing
	msg := fmt.Sprintf(format, args...)
	CaptureLog("error", msg)
	log.Errorf(format, args...)
}

// Fatalf captures and logs a formatted fatal message
func Fatalf(format string, args ...interface{}) {
	// Format the message first before capturing
	msg := fmt.Sprintf(format, args...)
	CaptureLog("fatal", msg)
	log.Fatalf(format, args...)
}

// SetLevel sets the log level (passes through to underlying logger)
func SetLevel(level log.Level) {
	log.SetLevel(level)
}
