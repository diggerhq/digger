package middleware

import (
    "net/http"
    "strings"
    "log"

    "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/rbac"
    "github.com/diggerhq/digger/opentaco/internal/storage" 
    "github.com/labstack/echo/v4"
    "github.com/diggerhq/digger/opentaco/internal/principal"
)

// AccessTokenVerifier is a function that validates an access token.
// It should return nil if valid, or an error if invalid.
type AccessTokenVerifier func(token string) error

// RequireAuth returns middleware that verifies Bearer access tokens using the provided verifier.
func RequireAuth(verify AccessTokenVerifier) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            if verify == nil {
                return next(c)
            }
            authz := c.Request().Header.Get("Authorization")
            if !strings.HasPrefix(authz, "Bearer ") {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error":"missing_bearer"})
            }
            token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
            if err := verify(token); err != nil {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_token"})
            }
            return next(c)
        }
    }
}

// RBACMiddleware creates middleware that checks RBAC permissions
func RBACMiddleware(rbacManager *rbac.RBACManager, signer *auth.Signer, action rbac.Action, resourcePattern string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // If RBAC is not enabled, allow all
            enabled, err := rbacManager.IsEnabled(c.Request().Context())
            if err != nil || !enabled {
                return next(c)
            }
            
            // Get user from token
            principal, err := getPrincipalFromToken(c, signer)
            if err != nil {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
            }
            
            // Determine the resource for this request
            resource := getResourceFromRequest(c, resourcePattern)
            
            // Check permission
            can, err := rbacManager.Can(c.Request().Context(), principal, action, resource)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check permissions"})
            }
            
            if !can {
                return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
            }
            
            return next(c)
        }
    }
}

// JWTAuthMiddleware creates a middleware that verifies a JWT and injects the user principal into the request context.
func JWTAuthMiddleware(signer *auth.Signer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			log.Printf("DEBUG JWTAuthMiddleware: Called for path: %s", c.Request().URL.Path)
			
			authz := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				log.Printf("DEBUG JWTAuthMiddleware: No Bearer token found")
				// No token, continue. The AuthorizingStore will block the request.
				return next(c)
			}

			token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
			claims, err := signer.VerifyAccess(token)
			if err != nil {
				log.Printf("DEBUG JWTAuthMiddleware: Token verification failed: %v", err)
				// Invalid token, continue. The AuthorizingStore will block the request.
				return next(c)
			}

			log.Printf("DEBUG JWTAuthMiddleware: Token verified for subject: %s", claims.Subject)

			p := principal.Principal{ 
				Subject: claims.Subject,
				Email:   claims.Email,
				Roles:   claims.Roles,
				Groups:  claims.Groups,
			}

			// Add the principal to the context for downstream stores and handlers.
			ctx := storage.ContextWithPrincipal(c.Request().Context(), p)
			c.SetRequest(c.Request().WithContext(ctx))
			
			log.Printf("DEBUG JWTAuthMiddleware: Principal set in context for subject: %s", claims.Subject)

			return next(c)
		}
	}
}

// getPrincipalFromToken extracts principal information from the bearer token
func getPrincipalFromToken(c echo.Context, signer *auth.Signer) (rbac.Principal, error) {
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
    }
    
    token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    if signer == nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "auth not configured")
    }
    
    claims, err := signer.VerifyAccess(token)
    if err != nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
    }
    
    return rbac.Principal{
        Subject: claims.Subject,
        Email:   claims.Email,
        Roles:   claims.Roles,
        Groups:  claims.Groups,
    }, nil
}

// getResourceFromRequest determines the resource for the request based on the pattern
func getResourceFromRequest(c echo.Context, pattern string) string {
    if pattern == "" {
        return "*"
    }
    
    // Replace common placeholders
    resource := pattern
    resource = strings.ReplaceAll(resource, "{id}", c.Param("id"))
    resource = strings.ReplaceAll(resource, "{unit_id}", c.Param("unit_id"))
    
    // If it's a wildcard pattern, return as-is
    if strings.Contains(resource, "*") {
        return resource
    }
    
    // For specific resources, try to extract from path
    path := c.Request().URL.Path
    
    // Extract unit ID from common patterns
    if strings.Contains(path, "/v1/units/") {
        parts := strings.Split(path, "/v1/units/")
        if len(parts) > 1 {
            unitID := strings.Split(parts[1], "/")[0]
            if unitID != "" {
                return unitID
            }
        }
    }
    
    if strings.Contains(path, "/v1/backend/") {
        parts := strings.Split(path, "/v1/backend/")
        if len(parts) > 1 {
            stateID := strings.Split(parts[1], "/")[0]
            if stateID != "" {
                return stateID
            }
        }
    }
    
    return resource
}

// NotImplemented sends a uniform 501 JSON error per stubs convention.
func NotImplemented(c echo.Context) error {
    return c.JSON(http.StatusNotImplemented, map[string]string{
        "error":   "not_implemented",
        "message": "Milestone 1 dummy endpoint",
        "hint":    "This route will be implemented in a later milestone.",
    })
}
