package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ctxKey struct{}

var (
	key              = ctxKey{}
	goroutineLoggers sync.Map     // map[goroutineID]*slog.Logger
	baseLogger       *slog.Logger // Store the base logger to avoid loops
)

// contextAwareHandler wraps any slog.Handler and automatically includes request context
type contextAwareHandler struct {
	handler slog.Handler
}

func newContextAwareHandler(h slog.Handler) *contextAwareHandler {
	return &contextAwareHandler{handler: h}
}

func (h *contextAwareHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextAwareHandler) Handle(ctx context.Context, r slog.Record) error {
	// Try to get request-scoped logger from goroutine map first
	logger := getGoroutineLogger()

	if logger != nil {
		return logger.Handler().Handle(ctx, r)
	}

	// Fall back to the wrapped handler
	return h.handler.Handle(ctx, r)
}

func (h *contextAwareHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextAwareHandler{
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *contextAwareHandler) WithGroup(name string) slog.Handler {
	return &contextAwareHandler{
		handler: h.handler.WithGroup(name),
	}
}

// sets up the process wide base logger with automatic context detection
func Init() *slog.Logger {
	logLevel := os.Getenv("DIGGER_LOG_LEVEL")
	var level slog.Leveler

	if logLevel == "DEBUG" {
		level = slog.LevelDebug
	} else if logLevel == "WARN" {
		level = slog.LevelWarn
	} else if logLevel == "ERROR" {
		level = slog.LevelError
	} else {
		level = slog.LevelInfo
	}

	// Create the base JSON handler
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				a.Key = "severity"
			} else if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
			}
			return a
		},
	})

	// Create base logger WITHOUT context-aware handler (to avoid loops)
	baseLogger = slog.New(baseHandler).With(
		slog.String("app", "digger-backend"),
	)

	// Create context-aware handler that wraps the base handler
	contextHandler := &contextAwareHandler{handler: baseHandler}

	// Create the default logger with context-aware handler
	defaultLogger := slog.New(contextHandler).With(
		slog.String("app", "digger-backend"),
	)

	slog.SetDefault(defaultLogger)
	return defaultLogger
}

// inject stores a logger in ctx
func Inject(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, key, l) // FIXED: Use 'key' consistently
}

// from returns the request scoped logger if present, else the global default
func From(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(key).(*slog.Logger); ok && l != nil { // Uses same 'key'
		return l
	}
	return slog.Default()
}

// With returns a new ctx with additional attrs layered onto the existing logger.
func With(ctx context.Context, attrs ...any) context.Context {
	return Inject(ctx, From(ctx).With(attrs...))
}

// Default returns the default logger for backward compatibility
func Default() *slog.Logger {
	return slog.Default()
}

// GetBaseLogger returns the base logger for use in middleware (avoids loops)
func GetBaseLogger() *slog.Logger {
	if baseLogger == nil {
		// SAFE FALLBACK: Create a simple logger without contextAwareHandler
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})).With(slog.String("app", "digger-backend"))
	}
	return baseLogger
}

// Helper functions for middleware to access the goroutine map
func StoreGoroutineLogger(gid uint64, logger *slog.Logger) {
	if gid == 0 {
		// Don't store fallback ID to prevent conflicts
		return
	}
	if logger == nil {
		return
	}
	goroutineLoggers.Store(gid, logger)
}

func DeleteGoroutineLogger(gid uint64) {
	if gid == 0 {
		// Don't delete fallback ID
		return
	}
	goroutineLoggers.Delete(gid)
}

func getGoroutineLogger() *slog.Logger {
	if logger, ok := goroutineLoggers.Load(GetGoroutineID()); ok {
		if slogLogger, ok := logger.(*slog.Logger); ok {
			return slogLogger
		}
		// Log error in debug mode
		if os.Getenv("DIGGER_LOG_LEVEL") == "DEBUG" {
			slog.Debug("Invalid logger type in goroutine map", "type", fmt.Sprintf("%T", logger))
		}
	}
	return nil
}

// Helper function to get goroutine ID
const (
	goroutineStackBufferSize = 64  // Make constant
)

func GetGoroutineID() uint64 {
	buf := make([]byte, goroutineStackBufferSize)
	buf = buf[:runtime.Stack(buf, false)]

	// Stack trace format: "goroutine 123 [running]:"
	stack := string(buf)
	if strings.HasPrefix(stack, "goroutine ") {
		stack = stack[len("goroutine "):]
		if idx := strings.Index(stack, " "); idx > 0 {
			if id, err := strconv.ParseUint(stack[:idx], 10, 64); err == nil {
				return id
			}
			// Log parsing error in debug mode
			if os.Getenv("DIGGER_LOG_LEVEL") == "DEBUG" {
				slog.Debug("Failed to parse goroutine ID", "stack", stack[:idx])
			}
		}
	}
	return 0 // fallback
}

// Add this helper function:
func InheritRequestLogger(ctx context.Context) (cleanup func()) {
	if ctx == nil {
		return func() {}
	}
	
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		// Context cancelled, don't inherit
		return func() {}
	default:
		// Context still active, proceed
	}
	
	if reqLogger := From(ctx); reqLogger != nil {
		newGID := GetGoroutineID()
		if newGID == 0 {
			// Don't store with fallback ID
			return func() {}
		}
		
		StoreGoroutineLogger(newGID, reqLogger)
		return func() { DeleteGoroutineLogger(newGID) }
	}
	return func() {}
}






// parseLogArgs intelligently parses variadic arguments to extract context and attributes
func parseLogArgs(args ...any) (context.Context, []any) {
	if len(args) == 0 {
		return nil, nil
	}

	var ctx context.Context
	var attrs []any

	// Check if last argument is context
	lastIdx := len(args) - 1
	if c, ok := args[lastIdx].(context.Context); ok {
		ctx = c
		args = args[:lastIdx] // Remove context from args
	}

	// Process remaining arguments
	for _, arg := range args {
		switch v := arg.(type) {
		case map[string]any:
			// Convert map to slog attributes
			for key, value := range v {
				attrs = append(attrs, slog.Any(key, value))
			}
		default:
			// Pass through regular slog attributes (key-value pairs)
			attrs = append(attrs, arg)
		}
	}

	return ctx, attrs
}

