package utils

import (
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
		"url_path", req.URL.Path,
	)

	resp, err := lrt.Rt.RoundTrip(req)
	if err != nil {
		slog.Error("GitHub API Request failed", "error", err)
		return nil, err
	}

	slog.Debug("GitHub API Response",
		"status", resp.Status,
		"X-RateLimit-Limit", resp.Header.Get("X-RateLimit-Limit"),
		"X-RateLimit-Remaining", resp.Header.Get("X-RateLimit-Remaining"),
		"X-RateLimit-Used", resp.Header.Get("X-RateLimit-Used"),
		"X-RateLimit-Resource", resp.Header.Get("X-RateLimit-Resource"),
		"X-RateLimit-Reset", resp.Header.Get("X-RateLimit-Reset"),
	)

	return resp, nil
}
