package middleware

import (
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/labstack/echo/v4"
	"log"
)

// JWTOrgUUIDMiddleware extracts org UUID from JWT and adds to domain context
// For CLI routes (v1/, tfe/) - expects org_uuid in JWT claims
func JWTOrgUUIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgUUID, ok := c.Get("jwt_org_uuid").(string)
			if !ok || orgUUID == "" {
				log.Printf("[JWTOrgUUID] No org_uuid in JWT claims")
				return echo.NewHTTPError(400, "Organization UUID not found in token")
			}
			
			log.Printf("[JWTOrgUUID] Found org UUID: %s", orgUUID)
			
			ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
			c.SetRequest(c.Request().WithContext(ctx))
			
			return next(c)
		}
	}
}

// WebhookOrgUUIDMiddleware extracts org UUID from webhook header and adds to domain context
// For internal routes (/internal/api/*) - expects UUID in X-Org-ID header
func WebhookOrgUUIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgUUID, ok := c.Get("organization_id").(string)
			if !ok || orgUUID == "" {
				log.Printf("[WebhookOrgUUID] organization_id not found in context")
				return echo.NewHTTPError(400, "X-Org-ID header is required")
			}
			
			if !domain.IsUUID(orgUUID) {
				log.Printf("[WebhookOrgUUID] organization_id is not a UUID: %s", orgUUID)
				return echo.NewHTTPError(400, "X-Org-ID must be a UUID")
			}
			
			log.Printf("[WebhookOrgUUID] Found org UUID: %s", orgUUID)
			
			ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
			c.SetRequest(c.Request().WithContext(ctx))
			
			return next(c)
		}
	}
}

// SystemOrgMiddleware injects the system org UUID when auth is disabled
// This provides org context for all operations without requiring authentication
func SystemOrgMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := domain.ContextWithOrg(c.Request().Context(), domain.SystemOrgUUID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
