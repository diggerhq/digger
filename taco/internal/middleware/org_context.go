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

// ResolveOrgContextMiddleware resolves org identifier to UUID and adds to domain context
// For internal routes (/internal/api/*) - resolves X-Org-ID header (UUID or external org ID)
// Skips validation for endpoints that don't require an existing org (like creating/listing orgs)
func ResolveOrgContextMiddleware(resolver domain.IdentifierResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip org resolution for endpoints that create/list orgs
			path := c.Request().URL.Path
			method := c.Request().Method
			
			// These endpoints don't require an existing org context
			skipOrgResolution := (method == "POST" && path == "/internal/api/orgs") ||
				(method == "POST" && path == "/internal/api/orgs/sync") ||
				(method == "GET" && path == "/internal/api/orgs") ||
				(method == "GET" && path == "/internal/api/orgs/user")
			
			if skipOrgResolution {
				log.Printf("[ResolveOrgContext] Skipping org resolution for %s %s", method, path)
				return next(c)
			}
			
			// Get org identifier from webhook auth middleware
			orgIdentifier, ok := c.Get("organization_id").(string)
			if !ok || orgIdentifier == "" {
				log.Printf("[ResolveOrgContext] organization_id not found in context")
				return echo.NewHTTPError(400, "X-Org-ID header is required")
			}
			
			// Resolve identifier to UUID (accepts UUID or external org ID, NOT names)
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgIdentifier)
			if err != nil {
				log.Printf("[ResolveOrgContext] Failed to resolve org identifier %q: %v", orgIdentifier, err)
				return echo.NewHTTPError(400, map[string]string{
					"error": "Invalid organization identifier",
					"details": err.Error(),
				})
			}
			
			log.Printf("[ResolveOrgContext] Resolved %q to UUID: %s", orgIdentifier, orgUUID)
			
			// Add org UUID to domain context
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
