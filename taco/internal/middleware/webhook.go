package middleware

import (
	"net/http"
	"os"
	"strings"
	"log/slog"
	"crypto/subtle"

	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/labstack/echo/v4"
)

// WebhookAuth returns middleware that verifies webhook secret and extracts user context
// Expects headers:
//   - Authorization: Bearer <webhook_secret>
//   - X-User-ID: user identifier (subject)
//   - X-Email: user email
//   - X-Org-ID: organization UUID (required, must be valid UUID)
func WebhookAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			webhookSecret := os.Getenv("OPENTACO_ENABLE_INTERNAL_ENDPOINTS")
			
			// If no webhook secret is configured, deny access
			if webhookSecret == "" {
				slog.Error("Critical - webhook middleware called but OPENTACO_ENABLE_INTERNAL_ENDPOINTS not configured")
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "webhook authentication not configured",
				})
			}

			// Check Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "no authorization header provided",
				})
			}

			// Extract token from "Bearer <token>" format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid authorization format, expected Bearer token",
				})
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			
			// Verify token matches webhook secret
			if subtle.ConstantTimeCompare([]byte(token), []byte(webhookSecret)) != 1 {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "invalid webhook secret",
				})
			}

	userID := c.Request().Header.Get("X-User-ID")
	email := c.Request().Header.Get("X-Email")
	orgID := c.Request().Header.Get("X-Org-ID")

	// Skip org validation for endpoints that don't require existing org
	path := c.Request().URL.Path
	method := c.Request().Method
	skipOrgHeader := (method == http.MethodPost && path == "/internal/api/orgs") ||
		(method == http.MethodPost && path == "/internal/api/orgs/sync") ||
		(method == http.MethodGet && path == "/internal/api/orgs") ||
		(method == http.MethodGet && path == "/internal/api/orgs/user")

	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "X-User-ID header required",
		})
	}
	
	// Require org ID for all requests except org creation/listing
	if !skipOrgHeader {
		if orgID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "X-Org-ID header required",
			})
		}
	}

		principal := rbac.Principal{
			Subject: userID,
			Email:   email,
			Roles:   []string{},
			Groups:  []string{"org:" + orgID},
		}

		c.Set("organization_id", orgID)
		c.Set("user_id", userID)
		c.Set("email", email)

		ctx := c.Request().Context()
		ctx = rbac.ContextWithPrincipal(ctx, principal)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
		}
	}
}

