package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/labstack/echo/v4"
	"os"
)

const (
	// APIPrefixV2 is the URL path prefix for TFE API endpoints
	APIPrefixV2 = "/tfe/api/v2/"
	// ModuleV1Prefix is the URL path prefix for module registry endpoints
	ModuleV1Prefix = "/v1/modules/"
)

// Update GetWellKnownJson to use real OAuth endpoints and client ID
func (h *TfeHandler) GetWellKnownJson(c echo.Context) error {
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
	discoveryPayload := domain.WellknownDef{
		ModulesV1: ModuleV1Prefix,
		MotdV1:    "/api/terraform/motd",
		StateV2:   APIPrefixV2,
		TfeV2:     APIPrefixV2,
		TfeV21:    APIPrefixV2,
		TfeV22:    APIPrefixV2,
		LoginV1: domain.WellKnownSpec{
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
	return h.authHandler.OAuthAuthorize(c)
}

func (h *TfeHandler) AuthTokenExchange(c echo.Context) error {
	return h.authHandler.OAuthToken(c)
}

// Helper function to get base URL
func getBaseURL(c echo.Context) string {
	scheme := c.Scheme()
	if fwd := c.Request().Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	host := c.Request().Host
	return scheme + "://" + host
}
