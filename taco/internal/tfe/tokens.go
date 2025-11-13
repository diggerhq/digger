package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/labstack/echo/v4"
	"net/http"
)

// Healthz handles liveness probe
func (h *TfeHandler) GetTokens(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Info("Getting TFE tokens",
		"operation", "tfe_get_tokens",
	)
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "opentaco",
		"token":   "abc123",
	})
}
