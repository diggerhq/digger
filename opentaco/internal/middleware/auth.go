package middleware

import (
    "net/http"

    "github.com/labstack/echo/v4"
)

// AuthMiddleware is a placeholder that currently allows all requests through.
// It will later verify OpenTaco access tokens and enrich context with principal info.
func AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // TODO: verify bearer token and attach principal to context
        return next(c)
    }
}

// RBACMiddleware is a placeholder that currently allows all requests through.
// Future implementation will extract the principal and enforce action-level permissions.
func RBACMiddleware(_ string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // TODO: enforce RBAC based on route/action
            return next(c)
        }
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

