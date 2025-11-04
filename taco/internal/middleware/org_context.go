package middleware

import (
	"log"

	"github.com/diggerhq/digger/opentaco/internal/domain"
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
			if !ok {
				log.Printf("[JWTOrgResolver] WARNING: jwt_org claim not found, defaulting to 'default'")
				orgName = DefaultOrgID
			} else if orgName == "" {
				log.Printf("[JWTOrgResolver] WARNING: jwt_org claim is empty, defaulting to 'default'")
				orgName = DefaultOrgID
			} else {
				log.Printf("[JWTOrgResolver] Found jwt_org claim: '%s'", orgName)
			}
			
			log.Printf("[JWTOrgResolver] Resolving org name '%s' to UUID", orgName)
			
			// Resolve org name to UUID
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				log.Printf("[JWTOrgResolver] ERROR: Failed to resolve organization '%s': %v", orgName, err)
				return echo.NewHTTPError(500, "Failed to resolve organization")
			}
			
			log.Printf("[JWTOrgResolver] Successfully resolved '%s' to UUID: %s", orgName, orgUUID)
			
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
				log.Printf("[WebhookOrgResolver] Skipping org resolution for endpoint: %s %s", method, path)
				return next(c)
			}
			
			// Get org name from echo context (set by WebhookAuth)
			orgName, ok := c.Get("organization_id").(string)
			if !ok {
				log.Printf("[WebhookOrgResolver] WARNING: organization_id not found in context, defaulting to 'default'")
				orgName = DefaultOrgID
			} else if orgName == "" {
				log.Printf("[WebhookOrgResolver] WARNING: organization_id is empty, defaulting to 'default'")
				orgName = DefaultOrgID
			} else {
				log.Printf("[WebhookOrgResolver] Found organization_id in context: '%s'", orgName)
			}
			
			log.Printf("[WebhookOrgResolver] Resolving org name '%s' to UUID", orgName)
			
			// Resolve org name to UUID and get full org info
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				log.Printf("[WebhookOrgResolver] ERROR: Failed to resolve organization '%s': %v", orgName, err)
				log.Printf("[WebhookOrgResolver] ERROR: This likely means the organization doesn't exist in the database yet or the external_org_id doesn't match")
				log.Printf("[WebhookOrgResolver] ERROR: Check if the organization was created successfully with external_org_id='%s'", orgName)
				return echo.NewHTTPError(500, map[string]interface{}{
					"error": "Failed to resolve organization",
					"detail": err.Error(),
					"org_identifier": orgName,
					"hint": "The organization may not exist in the database or the external_org_id doesn't match",
				})
			}
			
			log.Printf("[WebhookOrgResolver] Successfully resolved '%s' to UUID: %s", orgName, orgUUID)
			
			// Get full org info to populate context (avoids repeated queries)
			orgInfo, err := resolver.GetOrganization(c.Request().Context(), orgUUID)
			if err != nil {
				log.Printf("[WebhookOrgResolver] WARNING: Failed to get org details for %s: %v - falling back to basic context", orgUUID, err)
				// Fall back to basic context if org lookup fails
				ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
				c.SetRequest(c.Request().WithContext(ctx))
			} else {
				log.Printf("[WebhookOrgResolver] Successfully got org details for %s", orgInfo.Name)
				// Add full org info to domain context to avoid repeated queries
				ctx := domain.ContextWithOrgFull(c.Request().Context(), orgInfo.ID, orgInfo.Name, orgInfo.DisplayName)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			
			return next(c)
		}
	}
}

