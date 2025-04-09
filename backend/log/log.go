package log

import (
	"log/slog"
	"os"

	"github.com/go-substrate/strate/backend/config"
)

// Configure sets up the logger based on log configuration
func Configure(logConfig config.LogConfig) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: logConfig.Level}

	if logConfig.Format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Debug("Logger configured",
		"level", logConfig.Level.String(),
		"format", logConfig.Format)
}
