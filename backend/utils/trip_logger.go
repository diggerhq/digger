package utils

import (
	"bytes"
	"io"
	"log/slog"
	net "net/http"
)

type LoggingRoundTripper struct {
	Rt net.RoundTripper
}

func (lrt *LoggingRoundTripper) RoundTrip(req *net.Request) (*net.Response, error) {
	// Log the request
	slog.Debug("GitHub API Request",
		"method", req.Method,
		"url", req.URL.String(),
		"headers", req.Header,
	)

	resp, err := lrt.Rt.RoundTrip(req)
	if err != nil {
		slog.Error("GitHub API Request failed", "error", err)
		return nil, err
	}

	// Read and clone response body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return resp, err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // restore the body for the actual client

	slog.Debug("GitHub API Response",
		"status", resp.Status,
		"headers", resp.Header,
		"body", string(bodyBytes),
	)

	return resp, nil
}
