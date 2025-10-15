package middleware

import (
	"net/http"
	"os"
	"strings"
	"log/slog"
	"crypto/subtle"
	"errors"

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
//   - X-Org-ID: organization identifier (REQUIRED for org isolation)
func WebhookAuth(orgRepo domain.OrganizationRepository) echo.MiddlewareFunc {
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

			// Require user ID and org ID for org isolation
			if userID == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "X-User-ID header required",
				})
			}
			
			if orgID == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "X-Org-ID header required for org-based isolation",
				})
			}

			
			// Validate org ID format (prevents injection/traversal attacks)
			if !domain.OrgIDPattern.MatchString(orgID) {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "invalid org ID format",
				})
			}

			// ========================================
			// Verify organization exists in database
			// ========================================
			if orgRepo != nil {
				ctx := c.Request().Context()
				_, err := orgRepo.Get(ctx, orgID)
				if err != nil {
					if errors.Is(err, domain.ErrOrgNotFound) {
						slog.Warn("Webhook request for non-existent organization",
							"orgID", orgID,
							"userID", userID,
						)
						return c.JSON(http.StatusNotFound, map[string]string{
							"error": "organization not found",
							"org_id": orgID,
						})
					}
					
					// Database error
					slog.Error("Failed to verify organization existence",
						"orgID", orgID,
						"error", err,
					)
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "failed to verify organization",
					})
				}
				
				slog.Debug("Organization verified for webhook request",
					"orgID", orgID,
					"userID", userID,
				)
			} else {
				// This should never happen if properly configured
				slog.Warn("Webhook middleware configured without org repository - skipping org existence check")
			}


			// Build Principal from headers (for RBAC layer)
			// Note: RBAC IS applied if enabled - it looks up roles from database using Subject
			principal := rbac.Principal{
				Subject: userID,
				Email:   email,
				Roles:   []string{}, // Roles are looked up from DB, not passed via headers
				Groups:  []string{"org:" + orgID}, // Org group for permission matching
			}

			// Store in echo context for handlers that need it
			c.Set("organization_id", orgID)
			c.Set("user_id", userID)
			c.Set("email", email)

			// Add contexts to request context (domain layer contracts)
			ctx := c.Request().Context()
			
			// Add org context (for orgScopedRepository)
			ctx = domain.ContextWithOrg(ctx, orgID)
			
			// Add principal context (for authorizingRepository/RBAC)
			ctx = rbac.ContextWithPrincipal(ctx, principal)
			
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

