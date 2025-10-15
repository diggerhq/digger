package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/rbac"
    "github.com/labstack/echo/v4"
)

// AccessTokenVerifier is a function that validates an access token.
// It should return nil if valid, or an error if invalid.
type AccessTokenVerifier func(token string) error

// RequireAuth returns middleware that verifies Bearer access tokens and sets principal in context.
func RequireAuth(verify AccessTokenVerifier, signer *auth.Signer) echo.MiddlewareFunc {
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
            
            // Verify token and get claims in one call
            if signer != nil {
                claims, err := signer.VerifyAccess(token)
                if err != nil {
                    return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_token"})
                }
                
                // Set principal in context
                p := rbac.Principal{ 
                    Subject: claims.Subject,
                    Email:   claims.Email,
                    Roles:   claims.Roles,
                    Groups:  claims.Groups,
                }
                ctx := rbac.ContextWithPrincipal(c.Request().Context(), p)
                c.SetRequest(c.Request().WithContext(ctx))
            } else {
                // Fallback to generic verify function if no signer
                if err := verify(token); err != nil {
                    return c.JSON(http.StatusUnauthorized, map[string]string{"error":"invalid_token"})
                }
            }
            
            return next(c)
        }
    }
}

// RBACMiddleware creates middleware that checks RBAC permissions
func RBACMiddleware(rbacManager *rbac.RBACManager, signer *auth.Signer, apiTokenMgr *auth.APITokenManager, action rbac.Action, resourcePattern string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // If RBAC is not enabled, allow all
            enabled, err := rbacManager.IsEnabled(c.Request().Context())
            if err != nil || !enabled {
                return next(c)
            }
            
            // Get user from token (JWT or opaque)
            principal, err := getPrincipalFromToken(c, signer, apiTokenMgr)
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
                return c.JSON(http.StatusForbidden, map[string]string{
                    "error": "insufficient permissions to access workspace",
                    "hint":  "contact your administrator to grant " + string(action) + " permission",
                })
            }
            
            return next(c)
        }
    }
}

// getPrincipalFromToken extracts principal information from the bearer token (JWT or opaque)
func getPrincipalFromToken(c echo.Context, signer *auth.Signer, apiTokenMgr *auth.APITokenManager) (rbac.Principal, error) {
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
    }
    
    token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    
    // Try JWT token first
    if signer != nil {
        if claims, err := signer.VerifyAccess(token); err == nil {
            return rbac.Principal{
                Subject: claims.Subject,
                Email:   claims.Email,
                Roles:   claims.Roles,
                Groups:  claims.Groups,
            }, nil
        }
    }
    
    // Fallback to opaque token
    if apiTokenMgr != nil {
        if tokenRecord, err := apiTokenMgr.Verify(c.Request().Context(), token); err == nil {
            return rbac.Principal{
                Subject: tokenRecord.Subject,
                Email:   tokenRecord.Email,
                Roles:   []string{}, // Opaque tokens don't have roles directly
                Groups:  tokenRecord.Groups,
            }, nil
        }
    }
    
    return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
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

// JWTOnlyVerifier creates a token verifier that only accepts JWT tokens
func JWTOnlyVerifier(signer *auth.Signer) AccessTokenVerifier {
    return func(token string) error {
        if signer == nil {
            return echo.NewHTTPError(http.StatusInternalServerError, "JWT signer not configured")
        }
        
        if _, err := signer.VerifyAccess(token); err != nil {
            return echo.ErrUnauthorized
        }
        
        return nil
    }
}

// OpaqueOnlyVerifier creates a token verifier that only accepts opaque tokens
func OpaqueOnlyVerifier(apiTokenMgr *auth.APITokenManager) AccessTokenVerifier {
    return func(token string) error {
        if apiTokenMgr == nil {
            return echo.NewHTTPError(http.StatusInternalServerError, "API token manager not configured")
        }
        
        if _, err := apiTokenMgr.Verify(context.Background(), token); err != nil {
            return echo.ErrUnauthorized
        }
        
        return nil
    }
}

// JWTOnlyRBACMiddleware creates RBAC middleware that works with JWT tokens only
func JWTOnlyRBACMiddleware(rbacManager *rbac.RBACManager, signer *auth.Signer, action rbac.Action, resourcePattern string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // If RBAC is not enabled, allow all
            enabled, err := rbacManager.IsEnabled(c.Request().Context())
            if err != nil || !enabled {
                return next(c)
            }
            
            // Get user from JWT token only
            principal, err := getPrincipalFromJWT(c, signer)
            if err != nil {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_jwt_token"})
            }
            
            // Determine the resource for this request
            resource := getResourceFromRequest(c, resourcePattern)
            
            // Check permission
            can, err := rbacManager.Can(c.Request().Context(), principal, action, resource)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check permissions"})
            }
            
            if !can {
                return c.JSON(http.StatusForbidden, map[string]string{
                    "error": "insufficient permissions to access resource",
                    "hint":  "contact your administrator to grant " + string(action) + " permission",
                })
            }
            
            return next(c)
        }
    }
}

// OpaqueOnlyRBACMiddleware creates RBAC middleware that works with opaque tokens only
func OpaqueOnlyRBACMiddleware(rbacManager *rbac.RBACManager, apiTokenMgr *auth.APITokenManager, action rbac.Action, resourcePattern string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // If RBAC is not enabled, allow all
            enabled, err := rbacManager.IsEnabled(c.Request().Context())
            if err != nil || !enabled {
                return next(c)
            }
            
            // Get user from opaque token only
            principal, err := getPrincipalFromOpaque(c, apiTokenMgr)
            if err != nil {
                return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_opaque_token"})
            }
            
            // Determine the resource for this request
            resource := getResourceFromRequest(c, resourcePattern)
            
            // Check permission
            can, err := rbacManager.Can(c.Request().Context(), principal, action, resource)
            if err != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check permissions"})
            }
            
            if !can {
                return c.JSON(http.StatusForbidden, map[string]string{
                    "error": "insufficient permissions to access workspace",
                    "hint":  "contact your administrator to grant " + string(action) + " permission",
                })
            }
            
            return next(c)
        }
    }
}

// getPrincipalFromJWT extracts principal information from JWT tokens only
func getPrincipalFromJWT(c echo.Context, signer *auth.Signer) (rbac.Principal, error) {
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
    }
    
    token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    
    if signer == nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "JWT signer not configured")
    }
    
    claims, err := signer.VerifyAccess(token)
    if err != nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid JWT token")
    }
    
    return rbac.Principal{
        Subject: claims.Subject,
        Email:   claims.Email,
        Roles:   claims.Roles,
        Groups:  claims.Groups,
    }, nil
}

// getPrincipalFromOpaque extracts principal information from opaque tokens only
func getPrincipalFromOpaque(c echo.Context, apiTokenMgr *auth.APITokenManager) (rbac.Principal, error) {
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
    }
    
    token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    
    if apiTokenMgr == nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "API token manager not configured")
    }
    
    tokenRecord, err := apiTokenMgr.Verify(c.Request().Context(), token)
    if err != nil {
        return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid opaque token")
    }
    
    return rbac.Principal{
        Subject: tokenRecord.Subject,
        Email:   tokenRecord.Email,
        Roles:   []string{}, // Opaque tokens don't have roles directly
        Groups:  tokenRecord.Groups,
    }, nil
}

// NotImplemented sends a uniform 501 JSON error per stubs convention.
func NotImplemented(c echo.Context) error {
    return c.JSON(http.StatusNotImplemented, map[string]string{
        "error":   "not_implemented",
        "message": "Milestone 1 dummy endpoint",
        "hint":    "This route will be implemented in a later milestone.",
    })
}
