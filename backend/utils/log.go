package utils

import (
	"bufio"
	"bytes"
	"log/slog"
	"strings"
)

// SentrySlogWriter adapts Sentry's log output to a structured logger.
type SentrySlogWriter struct {
	logger *slog.Logger
}

// NewSentrySlogWriter creates a new adapter to redirect Sentry logs to slog.
func NewSentrySlogWriter(logger *slog.Logger) *SentrySlogWriter {
	return &SentrySlogWriter{logger: logger}
}

// Write implements io.Writer to process Sentry's logs and send them to slog.
func (s *SentrySlogWriter) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(p))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[Sentry]") {
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				s.logger.Debug(parts[3]) // Extract message without prefix and timestamp
			} else {
				s.logger.Debug(line)
			}
		} else {
			s.logger.Debug(line)
		}
	}
	return len(p), nil
}
