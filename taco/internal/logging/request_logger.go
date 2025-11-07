package logging

import (
	"log/slog"

	"github.com/labstack/echo/v4"
)

// RequestLogger provides structured logging with request ID context
type RequestLogger struct {
	logger    *slog.Logger
	requestID string
}

// FromContext creates a RequestLogger from an Echo context
// It extracts the request ID from the X-Request-ID response header
func FromContext(c echo.Context) *RequestLogger {
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = "unknown"
	}
	
	// Get or create slog logger
	logger := slog.Default()
	
	return &RequestLogger{
		logger:    logger,
		requestID: requestID,
	}
}

// WithRequestID creates a RequestLogger with a specific request ID
func WithRequestID(requestID string) *RequestLogger {
	if requestID == "" {
		requestID = "unknown"
	}
	
	return &RequestLogger{
		logger:    slog.Default(),
		requestID: requestID,
	}
}

// Info logs an info message with request ID
func (rl *RequestLogger) Info(msg string, args ...any) {
	rl.logger.Info(msg, append([]any{"request_id", rl.requestID}, args...)...)
}

// Warn logs a warning message with request ID
func (rl *RequestLogger) Warn(msg string, args ...any) {
	rl.logger.Warn(msg, append([]any{"request_id", rl.requestID}, args...)...)
}

// Error logs an error message with request ID
func (rl *RequestLogger) Error(msg string, args ...any) {
	rl.logger.Error(msg, append([]any{"request_id", rl.requestID}, args...)...)
}

// Debug logs a debug message with request ID
func (rl *RequestLogger) Debug(msg string, args ...any) {
	rl.logger.Debug(msg, append([]any{"request_id", rl.requestID}, args...)...)
}

// With returns a new logger with additional attributes
func (rl *RequestLogger) With(args ...any) *RequestLogger {
	return &RequestLogger{
		logger:    rl.logger.With(args...),
		requestID: rl.requestID,
	}
}

// RequestID returns the current request ID
func (rl *RequestLogger) RequestID() string {
	return rl.requestID
}

