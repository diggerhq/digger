package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (h *TfeHandler) AccountDetails(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Info("Getting TFE account details",
		"operation", "tfe_account_details",
	)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   "user-NFJxcpGThdJGBSjT",
			"type": "users",
			"attributes": map[string]interface{}{
				"avatar-url":         "https://example.com/avatar.png",
				"email":              "user@example.com",
				"is-service-account": false,
				"two-factor": map[string]interface{}{
					"enabled":           true,
					"last-challenge-at": "2025-09-03T12:34:56Z",
				},
				"unconfirmed-email": "pending@example.com",
				"username":          "opentaco",
				"v2-only":           true,
			},
		},
	})
}
