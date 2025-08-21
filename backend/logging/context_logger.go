package logging

import (
	"context"
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
	goroutineLoggers sync.Map // map[goroutineID]*slog.Logger - shared with middleware
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
	if logger := getGoroutineLogger(); logger != nil {
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

	// Wrap it with our context-aware handler
	contextHandler := newContextAwareHandler(baseHandler)
	
	base := slog.New(contextHandler).With(
		slog.String("app", "digger-backend"),
	)
	
	// This makes ALL slog calls automatically context-aware!
	slog.SetDefault(base)
	return base
}

// inject stores a logger in ctx
func Inject(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// from returns the request scoped logger if present, else the global default
func From(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(key).(*slog.Logger); ok && l != nil {
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

// Helper functions for middleware to access the goroutine map
func StoreGoroutineLogger(gid uint64, logger *slog.Logger) {
	goroutineLoggers.Store(gid, logger)
}

func DeleteGoroutineLogger(gid uint64) {
	goroutineLoggers.Delete(gid)
}

func getGoroutineLogger() *slog.Logger {
	if logger, ok := goroutineLoggers.Load(getGoroutineID()); ok {
		return logger.(*slog.Logger)
	}
	return nil
}

// Helper function to get goroutine ID
func getGoroutineID() uint64 {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	
	// Stack trace format: "goroutine 123 [running]:"
	stack := string(buf)
	if strings.HasPrefix(stack, "goroutine ") {
		stack = stack[len("goroutine "):]
		if idx := strings.Index(stack, " "); idx > 0 {
			if id, err := strconv.ParseUint(stack[:idx], 10, 64); err == nil {
				return id
			}
		}
	}
	return 0 // fallback
}

// Smart logging functions that can handle multiple signatures
// These are the ONLY logging functions we need - they handle all cases
func Info(msg string, args ...any) {
	ctx, attrs := parseLogArgs(args...)
	if ctx != nil {
		From(ctx).Info(msg, attrs...)
	} else if logger := getGoroutineLogger(); logger != nil {
		logger.Info(msg, attrs...)
	} else {
		slog.Info(msg, attrs...)
	}
}

func Error(msg string, args ...any) {
	ctx, attrs := parseLogArgs(args...)
	if ctx != nil {
		From(ctx).Error(msg, attrs...)
	} else if logger := getGoroutineLogger(); logger != nil {
		logger.Error(msg, attrs...)
	} else {
		slog.Error(msg, attrs...)
	}
}

func Debug(msg string, args ...any) {
	ctx, attrs := parseLogArgs(args...)
	if ctx != nil {
		From(ctx).Debug(msg, attrs...)
	} else if logger := getGoroutineLogger(); logger != nil {
		logger.Debug(msg, attrs...)
	} else {
		slog.Debug(msg, attrs...)
	}
}

func Warn(msg string, args ...any) {
	ctx, attrs := parseLogArgs(args...)
	if ctx != nil {
		From(ctx).Warn(msg, attrs...)
	} else if logger := getGoroutineLogger(); logger != nil {
		logger.Warn(msg, attrs...)
	} else {
		slog.Warn(msg, attrs...)
	}
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
		case map[string]interface{}:
			// Handle interface{} maps too
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

// Backward compatibility functions (optional - can be removed if not needed)
func InfoMsg(msg string, attrs ...any)  { slog.Default().Info(msg, attrs...) }
func ErrorMsg(msg string, attrs ...any) { slog.Default().Error(msg, attrs...) }
func DebugMsg(msg string, attrs ...any) { slog.Default().Debug(msg, attrs...) }
