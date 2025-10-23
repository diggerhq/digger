package middleware

import (
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/labstack/echo/v4"
	"log"
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
			
			log.Printf("[JWTOrgResolver] Resolving org name '%s' to UUID", orgName)
			
			// Resolve org name to UUID
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				log.Printf("[JWTOrgResolver] Failed to resolve organization '%s': %v", orgName, err)
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
			log.Printf("[WebhookOrgResolver] MIDDLEWARE INVOKED for path: %s", c.Path())
			
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
			
			// Resolve org name to UUID
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			if err != nil {
				log.Printf("[WebhookOrgResolver] ERROR: Failed to resolve organization '%s': %v", orgName, err)
				return echo.NewHTTPError(500, "Failed to resolve organization")
			}
			
			log.Printf("[WebhookOrgResolver] SUCCESS: Resolved '%s' to UUID: %s", orgName, orgUUID)
			
			// Add to domain context
			ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
			c.SetRequest(c.Request().WithContext(ctx))
			
			log.Printf("[WebhookOrgResolver] Domain context updated with org UUID")
			
			return next(c)
		}
	}
}

