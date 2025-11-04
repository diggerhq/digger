package middleware

import (
	"fmt"
	"log"
	"time"

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
			startMiddleware := time.Now()
			path := c.Request().URL.Path
			method := c.Request().Method
			
			// Extract request ID for correlation with handler logs
			requestID := c.Request().Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = fmt.Sprintf("mw-%d", time.Now().UnixNano())
			}
			
			log.Printf("[%s] 🔷 MIDDLEWARE: Starting org resolution for %s %s", requestID, method, path)
			
			// Skip org resolution for endpoints that create/list orgs
			// These endpoints don't require an existing org context
			skipOrgResolution := (method == "POST" && path == "/internal/api/orgs") ||
				(method == "POST" && path == "/internal/api/orgs/sync") ||
				(method == "GET" && path == "/internal/api/orgs") ||
				(method == "GET" && path == "/internal/api/orgs/user")
			
			if skipOrgResolution {
				log.Printf("[%s] ⏭️  MIDDLEWARE: Skipping org resolution for endpoint: %s %s", requestID, method, path)
				return next(c)
			}
			
			// Get org name from echo context (set by WebhookAuth)
			orgName, ok := c.Get("organization_id").(string)
			if !ok {
				log.Printf("[%s] ⚠️  MIDDLEWARE: organization_id not found in context, defaulting to 'default'", requestID)
				orgName = DefaultOrgID
			} else if orgName == "" {
				log.Printf("[%s] ⚠️  MIDDLEWARE: organization_id is empty, defaulting to 'default'", requestID)
				orgName = DefaultOrgID
			} else {
				log.Printf("[%s] 📋 MIDDLEWARE: Found organization_id: '%s'", requestID, orgName)
			}
			
			// Resolve org name to UUID and get full org info
			resolveStart := time.Now()
			orgUUID, err := resolver.ResolveOrganization(c.Request().Context(), orgName)
			resolveTime := time.Since(resolveStart)
			if err != nil {
				log.Printf("[%s] ❌ MIDDLEWARE: Failed to resolve org '%s' after %dms: %v", requestID, orgName, resolveTime.Milliseconds(), err)
				return echo.NewHTTPError(500, map[string]interface{}{
					"error": "Failed to resolve organization",
					"detail": err.Error(),
					"org_identifier": orgName,
					"hint": "The organization may not exist in the database or the external_org_id doesn't match",
				})
			}
			
			log.Printf("[%s] ✅ MIDDLEWARE: Resolved '%s' → UUID %s (%dms)", requestID, orgName, orgUUID, resolveTime.Milliseconds())
			
			// Get full org info to populate context (avoids repeated queries)
			getOrgStart := time.Now()
			orgInfo, err := resolver.GetOrganization(c.Request().Context(), orgUUID)
			getOrgTime := time.Since(getOrgStart)
			if err != nil {
				log.Printf("[%s] ⚠️  MIDDLEWARE: Failed to get org details for %s after %dms: %v - falling back to basic context", requestID, orgUUID, getOrgTime.Milliseconds(), err)
				// Fall back to basic context if org lookup fails
				ctx := domain.ContextWithOrg(c.Request().Context(), orgUUID)
				c.SetRequest(c.Request().WithContext(ctx))
			} else {
				// Add full org info to domain context to avoid repeated queries
				ctx := domain.ContextWithOrgFull(c.Request().Context(), orgInfo.ID, orgInfo.Name, orgInfo.DisplayName)
				c.SetRequest(c.Request().WithContext(ctx))
				log.Printf("[%s] ✅ MIDDLEWARE: Got org details for %s (%dms)", requestID, orgInfo.Name, getOrgTime.Milliseconds())
			}
			
			totalMiddlewareTime := time.Since(startMiddleware)
			
			// Log timing breakdown if middleware is slow
			if totalMiddlewareTime.Milliseconds() > 500 {
				log.Printf("[%s] ⚠️  MIDDLEWARE: SLOW - total: %dms (resolve: %dms, getOrg: %dms)", requestID, totalMiddlewareTime.Milliseconds(), resolveTime.Milliseconds(), getOrgTime.Milliseconds())
			} else {
				log.Printf("[%s] ✅ MIDDLEWARE: Complete - total: %dms (resolve: %dms, getOrg: %dms)", requestID, totalMiddlewareTime.Milliseconds(), resolveTime.Milliseconds(), getOrgTime.Milliseconds())
			}
			
			return next(c)
		}
	}
}

