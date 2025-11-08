package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (h *TfeHandler) Ping(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Debug("TFE ping",
		"operation", "tfe_ping",
	)
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}
