package logging

import (
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
			c.Writer.Header().Set("X-Request-ID", rid)
		}

		reqLog := slog.Default().With(
			slog.String("request_id", rid),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.String("ip", c.ClientIP()),
		)

		// Store in context
		ctx := Inject(c.Request.Context(), reqLog)
		c.Request = c.Request.WithContext(ctx)

		// ALSO store in goroutine map for automatic detection
		gid := getGoroutineID()
		StoreGoroutineLogger(gid, reqLog)

		start := time.Now()
		c.Next()

		// Cleanup
		DeleteGoroutineLogger(gid)

		reqLog.Info("http_request_done",
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.Int("bytes_out", c.Writer.Size()),
		)
	}
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

// Seamless logging functions that automatically check goroutine map
func InfoAuto(msg string, attrs ...any) {
	if logger := getGoroutineLogger(); logger != nil {
		logger.Info(msg, attrs...)
	} else {
		slog.Info(msg, attrs...)
	}
}

func ErrorAuto(msg string, attrs ...any) {
	if logger := getGoroutineLogger(); logger != nil {
		logger.Error(msg, attrs...)
	} else {
		slog.Error(msg, attrs...)
	}
}

func DebugAuto(msg string, attrs ...any) {
	if logger := getGoroutineLogger(); logger != nil {
		logger.Debug(msg, attrs...)
	} else {
		slog.Debug(msg, attrs...)
	}
}

func WarnAuto(msg string, attrs ...any) {
	if logger := getGoroutineLogger(); logger != nil {
		logger.Warn(msg, attrs...)
	} else {
		slog.Warn(msg, attrs...)
	}
}

// Helper to get logger from goroutine map
func getGoroutineLogger() *slog.Logger {
	if logger, ok := goroutineLoggers.Load(getGoroutineID()); ok {
		return logger.(*slog.Logger)
	}
	return nil
}

// Helper functions for middleware to access the goroutine map
func StoreGoroutineLogger(gid uint64, logger *slog.Logger) {
	goroutineLoggers.Store(gid, logger)
}

func DeleteGoroutineLogger(gid uint64) {
	goroutineLoggers.Delete(gid)
}
