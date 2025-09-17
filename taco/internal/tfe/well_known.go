package tfe

import (
	"os"
	"github.com/labstack/echo/v4"
)

const (
	// APIPrefixV2 is the URL path prefix for TFE API endpoints
	APIPrefixV2 = "/tfe/api/v2/"
	// ModuleV1Prefix is the URL path prefix for module registry endpoints
	ModuleV1Prefix = "/v1/modules/"
)

const (
	// OAuth2 client ID - purely advisory according to:
	// https://developer.hashicorp.com/terraform/internals/v1.3.x/login-protocol#client
	ClientID = "terraform"

	AuthRoute  = "/tfe/app/oauth2/auth"
	TokenRoute = "/tfe/oauth2/token"
)

// login stuff, TODO: move to own package etc
type DiscoverySpec struct {
	Client     string   `json:"client"`
	GrantTypes []string `json:"grant_types"`
	Authz      string   `json:"authz"`
	Token      string   `json:"token"`
	Ports      []int    `json:"ports"`
}

var Discovery = DiscoverySpec{
	Client:     ClientID,
	GrantTypes: []string{"authz_code"},
	Authz:      AuthRoute,
	Token:      TokenRoute,
	Ports:      []int{10000, 10010},
}

type DiscoveryDef struct {
	ModulesV1 string        `json:"modules.v1"`
	MotdV1    string        `json:"motd.v1"`
	StateV2   string        `json:"state.v2"`
	TfeV2     string        `json:"tfe.v2"`
	TfeV21    string        `json:"tfe.v2.1"`
	TfeV22    string        `json:"tfe.v2.2"`
	LoginV1   DiscoverySpec `json:"login.v1"`
}

var discoveryPayload = DiscoveryDef{
	ModulesV1: ModuleV1Prefix,
	MotdV1:    "/api/terraform/motd",
	StateV2:   APIPrefixV2,
	TfeV2:     APIPrefixV2,
	TfeV21:    APIPrefixV2,
	TfeV22:    APIPrefixV2,
	LoginV1:   Discovery,
}

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
	discoveryPayload := DiscoveryDef{
		ModulesV1: ModuleV1Prefix,
		MotdV1:    "/api/terraform/motd",
		StateV2:   APIPrefixV2,
		TfeV2:     APIPrefixV2,
		TfeV21:    APIPrefixV2,
		TfeV22:    APIPrefixV2,
		LoginV1: DiscoverySpec{
			Client:     clientID,                          // Use real client ID
			GrantTypes: []string{"authz_code"},
			Authz:      baseURL + "/oauth/authorization",  // Real OAuth endpoint
			Token:      baseURL + "/oauth/token",          // Real OAuth endpoint  
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
