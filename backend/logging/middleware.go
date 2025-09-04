package logging

import (
	"log/slog"
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

		// Protect against nil base logger
		baseLogger := GetBaseLogger()
		if baseLogger == nil {
			// Fallback to default if initialization failed
			baseLogger = slog.Default()
		}

		reqLog := baseLogger.With(
			slog.String("request_id", rid),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.String("ip", c.ClientIP()),
		)

		// Store in context
		ctx := Inject(c.Request.Context(), reqLog)
		c.Request = c.Request.WithContext(ctx)

		// Store in goroutine map with error protection
		gid := GetGoroutineID()
		if gid != 0 {  // Only store if we got a valid ID
			StoreGoroutineLogger(gid, reqLog)
			defer DeleteGoroutineLogger(gid)  // Ensure cleanup
		}

		start := time.Now()
		c.Next()

		reqLog.Info("http_request_done",
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.Int("bytes_out", c.Writer.Size()),
		)
	}
}
