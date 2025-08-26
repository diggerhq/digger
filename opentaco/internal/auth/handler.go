package auth

import (
    "net/http"

    "github.com/diggerhq/digger/opentaco/internal/middleware"
    "github.com/labstack/echo/v4"
)

// Handler provides auth-related HTTP handlers.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

// Exchange handles POST /v1/auth/exchange
// Request: {"id_token":"..."}
// Response: {"access_token":"...","refresh_token":"...","expires_in":3600,"token_type":"Bearer"}
func (h *Handler) Exchange(c echo.Context) error {
    return middleware.NotImplemented(c)
}

// Token handles POST /v1/auth/token (refresh -> new access)
func (h *Handler) Token(c echo.Context) error {
    return middleware.NotImplemented(c)
}

// IssueS3Creds handles POST /v1/auth/issue-s3-creds (STS)
func (h *Handler) IssueS3Creds(c echo.Context) error {
    return middleware.NotImplemented(c)
}

// Me handles GET /v1/auth/me (debug)
func (h *Handler) Me(c echo.Context) error {
    // In the stub, return a minimal payload to confirm wiring.
    return c.JSON(http.StatusOK, map[string]any{
        "subject": "anonymous",
        "roles":   []string{},
        "groups":  []string{},
        "scopes":  []string{"api", "s3"},
    })
}

// JWKS handles GET /oidc/jwks.json
func (h *Handler) JWKS(c echo.Context) error {
    // Stub empty JWKS set; will be populated when signing keys are added.
    return c.JSON(http.StatusOK, map[string]any{
        "keys": []any{},
    })
}

