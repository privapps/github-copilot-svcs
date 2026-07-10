package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// DenseTextHandler outputs only values, space-separated, in a fixed order.
type DenseTextHandler struct {
	level slog.Level
}

// Enabled reports whether the handler is enabled for the given level.
func (h *DenseTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats the log record as dense values and writes to stdout.
func (h *DenseTextHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Time.Format(time.RFC3339))
	b.WriteString(" ")
	b.WriteString(r.Level.String())
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("%q\t", r.Message))
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" ")
		switch v := a.Value.Any().(type) {
		case string:
			b.WriteString(v)
		default:
			b.WriteString(fmt.Sprintf("%v", v))
		}
		return true
	})
	b.WriteString("\n")
	_, err := os.Stdout.WriteString(b.String())
	return err
}

// WithAttrs returns the handler unchanged (attrs unused).
func (h *DenseTextHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup returns the handler unchanged (name unused).
func (h *DenseTextHandler) WithGroup(_ string) slog.Handler { return h }

const (
	defaultLogLevel = "info"
)

// Logger wraps slog.Logger for structured logging
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new logger with the specified level
func NewLogger(level string) *Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case defaultLogLevel:
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := &DenseTextHandler{level: logLevel}
	return &Logger{slog.New(handler)}
}

var logger *Logger

// Init initializes the global logger from environment variable
func Init() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = defaultLogLevel
	}
	logger = NewLogger(logLevel)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

// Info logs an info message
func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}

// Error logs an error message
func Error(msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return logger
}
