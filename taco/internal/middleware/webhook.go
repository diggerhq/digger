package middleware

import (
	"net/http"
	"os"
	"strings"
	"log/slog"
	"crypto/subtle"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/labstack/echo/v4"
)

// WebhookAuth returns middleware that verifies webhook secret and extracts user context
// This extracts data and delegates to domain/repository layers
// Expects headers:
//   - Authorization: Bearer <webhook_secret>
//   - X-User-ID: user identifier (subject)
//   - X-Email: user email
//   - X-Org-ID: organization identifier (defaults to "default" for self-hosted)
// Note: Org validation and UUID resolution happens in ResolveOrgContextMiddleware
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

		// Extract user context from headers
		userID := c.Request().Header.Get("X-User-ID")
		email := c.Request().Header.Get("X-Email")
		orgID := c.Request().Header.Get("X-Org-ID")

		// Skip org validation for create org endpoint
		isCreateOrg := c.Request().Method == http.MethodPost && c.Path() == "/internal/api/orgs"

		// Require user ID
		if userID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "X-User-ID header required",
			})
		}
		
		// Default to "default" org if not provided (except for create org endpoint)
		if !isCreateOrg && orgID == "" {
			orgID = "default"
			slog.Debug("No X-Org-ID header provided, defaulting to 'default' org")
		}
		
		// Validate org ID format (prevents injection/traversal attacks)
		if orgID != "" && !domain.OrgIDPattern.MatchString(orgID) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid org ID format",
			})
		}

		// Note: Org existence validation and resolution to UUID happens in 
		// ResolveOrgContextMiddleware which runs after this middleware.
		// This middleware just extracts the org name from headers.

		// Build Principal from headers (for RBAC layer)
			// Note: RBAC IS applied if enabled - it looks up roles from database using Subject
			principal := rbac.Principal{
				Subject: userID,
				Email:   email,
				Roles:   []string{}, // Roles are looked up from DB, not passed via headers
				Groups:  []string{"org:" + orgID}, // Org group for permission matching
			}

		// Store in echo context for downstream middleware
		c.Set("organization_id", orgID)  // ResolveOrgContextMiddleware will resolve this to UUID
		c.Set("user_id", userID)
		c.Set("email", email)

		// Add principal context (for authorizingRepository/RBAC)
		ctx := c.Request().Context()
		ctx = rbac.ContextWithPrincipal(ctx, principal)
		c.SetRequest(c.Request().WithContext(ctx))

		// NOTE: Org context is NOT set here - ResolveOrgContextMiddleware will:
		// 1. Read orgID from c.Get("organization_id") 
		// 2. Resolve org name "default" to its UUID
		// 3. Add UUID to domain context via domain.ContextWithOrg()

		return next(c)
		}
	}
}

