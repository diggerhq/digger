package middleware

import (
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/labstack/echo/v4"
)

const DefaultOrgID = "default"

// JWTOrgResolverMiddleware resolves org name from JWT claims to UUID and adds to domain context
// This should be used AFTER RequireAuth middleware for JWT-authenticated routes
func JWTOrgResolverMiddleware(resolver domain.IdentifierResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get org name from JWT claim (set by RequireAuth middleware)
			orgName, ok := c.Get("jwt_org").(string)
			if !ok || orgName == "" {
				orgName = DefaultOrgID
			}
			
			// Resolve org name to UUID
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				logger := logging.FromContext(c)
				logger.Error("Failed to resolve organization", "org_name", orgName, "error", err, "source", "JWTOrgResolver")
				return echo.NewHTTPError(500, "Failed to resolve organization")
			}
			
			// Add to domain context
			ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
			c.SetRequest(c.Request().WithContext(ctx))
			
			return next(c)
		}
	}
}

// ResolveOrgContextMiddleware resolves org name to UUID and adds to domain context
// This should be used AFTER WebhookAuth for internal routes
func ResolveOrgContextMiddleware(resolver domain.IdentifierResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			method := c.Request().Method
			
			// Skip org resolution for endpoints that create/list orgs
			// These endpoints don't require an existing org context
			skipOrgResolution := (method == "POST" && path == "/internal/api/orgs") ||
				(method == "POST" && path == "/internal/api/orgs/sync") ||
				(method == "GET" && path == "/internal/api/orgs") ||
				(method == "GET" && path == "/internal/api/orgs/user")
			
			if skipOrgResolution {
				return next(c)
			}
			
			// Get org name from echo context (set by WebhookAuth)
			orgName, ok := c.Get("organization_id").(string)
			if !ok || orgName == "" {
				orgName = DefaultOrgID
			}
			
			// Resolve org name to UUID and get full org info
			logger := logging.FromContext(c)
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				logger.Error("Failed to resolve organization", "org_name", orgName, "error", err, "source", "WebhookOrgResolver")
				return echo.NewHTTPError(500, map[string]interface{}{
					"error": "Failed to resolve organization",
					"detail": err.Error(),
					"org_identifier": orgName,
					"hint": "The organization may not exist in the database or the external_org_id doesn't match",
				})
			}
			
			// Get full org info to populate context (avoids repeated queries)
			orgInfo, err := resolver.GetOrganization(c.Request().Context(), orgUUID)
			if err != nil {
				logger.Warn("Failed to get org details - using basic context", "org_uuid", orgUUID, "error", err, "source", "WebhookOrgResolver")
				// Fall back to basic context if org lookup fails
				ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
				c.SetRequest(c.Request().WithContext(ctx))
			} else {
				// Add full org info to domain context to avoid repeated queries
				ctx := domain.ContextWithOrgFull(c.Request().Context(), orgInfo.ID, orgInfo.Name, orgInfo.DisplayName)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			
			return next(c)
		}
	}
}

