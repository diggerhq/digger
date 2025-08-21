package logging

import (
	"fmt"
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

		// Use base logger to avoid infinite loop
		reqLog := GetBaseLogger().With(
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
		gid := GetGoroutineID()
		StoreGoroutineLogger(gid, reqLog)

		// TEMPORARY DEBUG - Remove after testing
		fmt.Printf("DEBUG: Stored request logger for goroutine %d\n", gid)

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
