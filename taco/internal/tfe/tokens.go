package tfe

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

// Healthz handles liveness probe
func (h *TfeHandler) GetTokens(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "opentaco",
		"token":   "abc123",
	})
}
