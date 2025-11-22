package tfe

import (
	"os"

	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/labstack/echo/v4"
)

const (
	// APIPrefixV2 is the URL path prefix for TFE API endpoints
	APIPrefixV2 = "/tfe/api/v2/"
	// ModuleV1Prefix is the URL path prefix for module registry endpoints
	ModuleV1Prefix = "/v1/modules/"
)

func (h *TfeHandler) MessageOfTheDay(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Debug("TFE message of the day",
		"operation", "tfe_motd",
	)

	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	res := tfe.MotdResponse{Msg: tfe.MotdMessage()}
	return c.JSON(200, res)
}

// Update GetWellKnownJson to use real OAuth endpoints and client ID
func (h *TfeHandler) GetWellKnownJson(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Info("TFE well-known discovery",
		"operation", "tfe_well_known",
	)

	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	baseURL := getBaseURL(c)

	// Get the real client ID from environment (same as auth handler)
	clientID := os.Getenv("OPENTACO_AUTH_CLIENT_ID")
	if clientID == "" {
		clientID = "terraform" // fallback for compatibility
	}

	// Use the same OAuth endpoints as the main auth flow
	discoveryPayload := tfe.WellKnownSpec{
		Modules:         ModuleV1Prefix,
		MessageOfTheDay: "/tfe/api/v2/motd",
		State:           APIPrefixV2,
		TfeApiV2:        APIPrefixV2,
		TfeApiV21:       APIPrefixV2,
		TfeApiV22:       APIPrefixV2,
		Login: tfe.LoginSpec{
			Client:     clientID, // Use real client ID
			GrantTypes: []string{"authz_code"},
			Authz:      baseURL + "/oauth/authorization", // Real OAuth endpoint
			Token:      baseURL + "/oauth/token",         // Real OAuth endpoint
			Ports:      []int{10000, 10010},
		},
	}

	return c.JSON(200, discoveryPayload)
}

// Delegate auth endpoints to real handlers
func (h *TfeHandler) AuthLogin(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Info("TFE OAuth authorize delegation",
		"operation", "tfe_auth_login",
	)
	return h.authHandler.OAuthAuthorize(c)
}

func (h *TfeHandler) AuthTokenExchange(c echo.Context) error {
	logger := logging.FromContext(c)
	logger.Info("TFE OAuth token exchange delegation",
		"operation", "tfe_auth_token",
	)
	return h.authHandler.OAuthToken(c)
}

// Helper function to get base URL
func getBaseURL(c echo.Context) string {
	scheme := c.Scheme()
	allowForwardedFor := os.Getenv("OPENTACO_ALLOW_X_FORWARDED_FOR")
	if allowForwardedFor == "true" {
		if fwd := c.Request().Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		}
	}

	host := c.Request().Host
	if allowForwardedFor == "true" {
		if fwdHost := c.Request().Header.Get("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
	}

	return scheme + "://" + host
}
