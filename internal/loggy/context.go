package loggy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tildaslashalef/mindnest/internal/ulid"
)

type contextKey string

const (
	loggerKey    contextKey = "logger"
	requestIDKey contextKey = "request_id"
)

// FromContext retrieves the logger from the context
func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return globalLogger
	}

	if logger, ok := ctx.Value(loggerKey).(Logger); ok {
		return &logger
	}

	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}

	return globalLogger
}

// WithLogger returns a new context with the logger attached
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}

	return ""
}

// WithRequestID returns a new context with the request ID attached
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

// NewRequestID generates a new request ID using ULID
func NewRequestID() string {
	return ulid.RequestID()
}

// Fields represents a collection of log fields
type Fields map[string]interface{}

// AddToContext adds fields to a logger in the context and returns the new context
func AddToContext(ctx context.Context, fields Fields) context.Context {
	logger := FromContext(ctx)
	if logger == nil {
		logger = globalLogger
	}

	if logger == nil {
		return ctx
	}

	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	newLogger := logger.With(args...)
	return WithLogger(ctx, newLogger)
}

// WithError adds error details to a logger
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}

	return l.With(
		"error", err.Error(),
		"error_type", fmt.Sprintf("%T", err),
	)
}

// Printf provides compatibility with the standard library logger interface
func Printf(format string, v ...interface{}) {
	if globalLogger != nil {
		file, line := getCaller(2)
		msg := fmt.Sprintf(format, v...)
		globalLogger.logWithSource(slog.LevelInfo, file, line, msg)
	}
}

// Println provides compatibility with the standard library logger interface
func Println(v ...interface{}) {
	if globalLogger != nil {
		file, line := getCaller(2)
		msg := fmt.Sprint(v...)
		globalLogger.logWithSource(slog.LevelInfo, file, line, msg)
	}
}

// Fatal logs at error level and then exits
func Fatal(msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2)
		globalLogger.logWithSource(slog.LevelError, file, line, msg, args...)
	}
	panic(msg) // Will be caught by the recovery handler, if any
}
