package middleware

import (
    "net/http"
    "strings"

    "github.com/labstack/echo/v4"
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

// RBACMiddleware placeholder (currently pass-through)
func RBACMiddleware(_ string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error { return next(c) }
    }
}

// NotImplemented sends a uniform 501 JSON error per stubs convention.
func NotImplemented(c echo.Context) error {
    return c.JSON(http.StatusNotImplemented, map[string]string{
        "error":   "not_implemented",
        "message": "Milestone 1 dummy endpoint",
        "hint":    "This route will be implemented in a later milestone.",
    })
}
