package logging

import (
	"context"
	"log/slog"

	"github.com/getsentry/sentry-go"
)

// SentryHandler wraps any slog.Handler and automatically sends ERROR and WARN logs to Sentry
type SentryHandler struct {
	next slog.Handler
}

// NewSentryHandler creates a new SentryHandler that wraps the given handler
func NewSentryHandler(next slog.Handler) *SentryHandler {
	return &SentryHandler{next: next}
}

func (h *SentryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *SentryHandler) Handle(ctx context.Context, record slog.Record) error {
	// Send WARN and ERROR logs to Sentry
	if record.Level >= slog.LevelWarn {
		// Extract all attributes into a map for Sentry context
		attrs := make(map[string]interface{})
		record.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.Any()
			return true
		})

		// Determine Sentry level
		var sentryLevel sentry.Level
		switch {
		case record.Level >= slog.LevelError:
			sentryLevel = sentry.LevelError
		case record.Level >= slog.LevelWarn:
			sentryLevel = sentry.LevelWarning
		default:
			sentryLevel = sentry.LevelInfo
		}

		// Send to Sentry with context
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetContext("log_attributes", attrs)
			scope.SetLevel(sentryLevel)
			
			// Add fingerprint for better grouping
			if msg := record.Message; msg != "" {
				scope.SetFingerprint([]string{msg})
			}
			
			// If there's an error in the attributes, capture it as an exception
			if err, ok := attrs["error"].(error); ok && err != nil {
				scope.SetTag("log_message", record.Message)
				sentry.CaptureException(err)
			} else {
				// Otherwise, capture as a message
				sentry.CaptureMessage(record.Message)
			}
		})
	}

	// Always pass through to the next handler for normal logging
	return h.next.Handle(ctx, record)
}

func (h *SentryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SentryHandler{next: h.next.WithAttrs(attrs)}
}

func (h *SentryHandler) WithGroup(name string) slog.Handler {
	return &SentryHandler{next: h.next.WithGroup(name)}
}
