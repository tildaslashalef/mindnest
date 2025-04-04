package loggy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	globalLogger *Logger
	once         sync.Once
)

// Config configures the logger
type Config struct {
	Level       slog.Level
	Format      string                                       // "json" or "text"
	Output      string                                       // "stdout", "stderr", or a file path
	AddSource   bool                                         // Include source code position in logs
	TimeFormat  string                                       // Time format for logs (empty uses RFC3339)
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr // Custom attribute replacer
}

// DefaultConfig returns a default configuration for the logger
func DefaultConfig() Config {
	return Config{
		Level:      slog.LevelInfo,
		Format:     "text",
		Output:     "stdout",
		AddSource:  true,
		TimeFormat: time.RFC3339,
	}
}

// Logger wraps slog.Logger with additional context
type Logger struct {
	slogger *slog.Logger
}

// Init initializes the global logger
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		var output io.Writer
		switch cfg.Output {
		case "stdout":
			output = os.Stdout
		case "stderr":
			output = os.Stderr
		default:
			// Treat as file path
			dir := filepath.Dir(cfg.Output)
			if err = os.MkdirAll(dir, 0755); err != nil {
				err = fmt.Errorf("failed to create log directory: %w", err)
				return
			}

			var file *os.File
			file, err = os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				err = fmt.Errorf("failed to open log file: %w", err)
				return
			}
			output = file
		}

		var handler slog.Handler
		handlerOpts := &slog.HandlerOptions{
			Level:       cfg.Level,
			AddSource:   cfg.AddSource,
			ReplaceAttr: cfg.ReplaceAttr,
		}

		if cfg.TimeFormat != "" {
			originalReplaceAttr := handlerOpts.ReplaceAttr
			handlerOpts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					if t, ok := a.Value.Any().(time.Time); ok {
						return slog.String(a.Key, t.Format(cfg.TimeFormat))
					}
				}
				if originalReplaceAttr != nil {
					return originalReplaceAttr(groups, a)
				}
				return a
			}
		}

		if cfg.Format == "json" {
			handler = slog.NewJSONHandler(output, handlerOpts)
		} else {
			handler = slog.NewTextHandler(output, handlerOpts)
		}

		globalLogger = &Logger{
			slogger: slog.New(handler),
		}
	})

	// If there was an error initializing, create a noop logger as fallback
	if err != nil {
		NewNoopLogger()
	}

	return err
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	return globalLogger
}

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// NewNoopLogger creates and sets a logger that discards all output, useful for testing
func NewNoopLogger() *Logger {
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	noopLogger := &Logger{
		slogger: slog.New(handler),
	}

	// Set this as the global logger
	SetGlobalLogger(noopLogger)

	return noopLogger
}

// getCaller returns the source file and line number of the caller,
// skipping a specified number of frames to identify the actual calling code
func getCaller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", 0
	}
	return file, line
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2) // Skip 2 frames: this function and runtime.Caller
		globalLogger.logWithSource(slog.LevelDebug, file, line, msg, args...)
	}
}

// Info logs at info level
func Info(msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2)
		globalLogger.logWithSource(slog.LevelInfo, file, line, msg, args...)
	}
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2)
		globalLogger.logWithSource(slog.LevelWarn, file, line, msg, args...)
	}
}

// Error logs at error level
func Error(msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2)
		globalLogger.logWithSource(slog.LevelError, file, line, msg, args...)
	}
}

// Log logs at the specified level
func Log(level slog.Level, msg string, args ...any) {
	if globalLogger != nil {
		file, line := getCaller(2)
		globalLogger.logWithSource(level, file, line, msg, args...)
	}
}

// logWithSource adds source information and logs the message
func (l *Logger) logWithSource(level slog.Level, file string, line int, msg string, args ...any) {
	if l != nil && l.slogger != nil {
		// Create a new context with the source location
		ctx := context.Background()

		// Create a Record with source information
		r := slog.NewRecord(time.Now(), level, msg, 0)
		r.AddAttrs(slog.String("source", fmt.Sprintf("%s:%d", file, line)))

		// Add the other args
		r.Add(args...)

		// Log with this record
		l.slogger.Handler().Handle(ctx, r)
	}
}

// LogAttrs logs at the specified level with attributes
func LogAttrs(level slog.Level, msg string, attrs ...slog.Attr) {
	if globalLogger != nil {
		file, line := getCaller(2)
		attrs = append(attrs, slog.String("source", fmt.Sprintf("%s:%d", file, line)))
		globalLogger.LogAttrs(context.Background(), level, msg, attrs...)
	}
}

// With returns a new Logger with the given attributes
func With(args ...any) *Logger {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.With(args...)
}

// WithAttrs returns a new Logger with the given attributes as a slice of slog.Attr
// This is a convenience wrapper around With() that accepts slog.Attr directly
func WithAttrs(attrs ...slog.Attr) *Logger {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.WithAttrs(attrs...)
}

// WithGroup returns a new Logger with the given group
func WithGroup(name string) *Logger {
	if globalLogger == nil {
		return nil
	}
	return globalLogger.WithGroup(name)
}

// WithContext returns a new Logger with trace context
func WithContext(ctx context.Context) *Logger {
	if globalLogger == nil {
		return nil
	}

	if logger := FromContext(ctx); logger != nil {
		return logger
	}

	return globalLogger
}

// Logger instance methods
func (l *Logger) Debug(msg string, args ...any) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		l.logWithSource(slog.LevelDebug, file, line, msg, args...)
	}
}

func (l *Logger) Info(msg string, args ...any) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		l.logWithSource(slog.LevelInfo, file, line, msg, args...)
	}
}

func (l *Logger) Warn(msg string, args ...any) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		l.logWithSource(slog.LevelWarn, file, line, msg, args...)
	}
}

func (l *Logger) Error(msg string, args ...any) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		l.logWithSource(slog.LevelError, file, line, msg, args...)
	}
}

func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		l.logWithSource(level, file, line, msg, args...)
	}
}

func (l *Logger) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if l != nil && l.slogger != nil {
		file, line := getCaller(2)
		sourceAttr := slog.String("source", fmt.Sprintf("%s:%d", file, line))
		attrs = append([]slog.Attr{sourceAttr}, attrs...)
		l.slogger.LogAttrs(ctx, level, msg, attrs...)
	}
}

func (l *Logger) With(args ...any) *Logger {
	if l == nil || l.slogger == nil {
		return l
	}
	return &Logger{
		slogger: l.slogger.With(args...),
	}
}

// WithAttrs returns a Logger that includes the given attributes in each output operation
func (l *Logger) WithAttrs(attrs ...slog.Attr) *Logger {
	if l == nil || l.slogger == nil {
		return l
	}
	// Convert attrs to args that slog.With can accept
	args := make([]any, len(attrs)*2)
	for i, attr := range attrs {
		args[i*2] = attr.Key
		args[i*2+1] = attr.Value.Any()
	}
	return &Logger{
		slogger: l.slogger.With(args...),
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	if l == nil || l.slogger == nil {
		return l
	}
	return &Logger{
		slogger: l.slogger.WithGroup(name),
	}
}

// Handler returns the underlying slog.Handler
func (l *Logger) Handler() slog.Handler {
	return l.slogger.Handler()
}
