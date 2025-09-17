package observability

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Healthz handles liveness probe
func (h *HealthHandler) Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "opentaco",
		"version": Version,
		"commit":  Commit,
	})
}

// Readyz handles readiness probe
func (h *HealthHandler) Readyz(c echo.Context) error {
	// In a real implementation, check dependencies
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ready",
		"service": "opentaco",
	})
}