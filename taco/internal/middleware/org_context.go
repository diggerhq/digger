package middleware

import (
	"github.com/labstack/echo/v4"
)

const DefaultOrgID = "default"

// JWTOrgContextMiddleware extracts org from JWT claims (for CLI/user auth)
func JWTOrgContextMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get org from JWT claim (set by auth middleware)
			orgID := ""
			if org, ok := c.Get("jwt_org").(string); ok && org != "" {
				orgID = org
			}
			
			// Fall back to default org
			if orgID == "" {
				orgID = DefaultOrgID
			}
			
			c.Set("organization_id", orgID)
			
			return next(c)
		}
	}
}

// WebhookOrgContextMiddleware extracts org from X-Organization-ID header (for internal routes)
func WebhookOrgContextMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID := c.Request().Header.Get("X-Organization-ID")
			
			if orgID == "" {
				orgID = DefaultOrgID
			}
			
			c.Set("organization_id", orgID)
			
			return next(c)
		}
	}
}

