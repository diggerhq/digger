package tfe

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func (h *TfeHandler) Ping(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}
