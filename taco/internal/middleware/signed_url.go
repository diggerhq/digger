package middleware

import (
	"log/slog"
	"net/http"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/labstack/echo/v4"
)


// VerifySignedURL middleware
func VerifySignedURL(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := auth.VerifySignedUrl(c.Request().URL.String())
		if err != nil {
			slog.Error("could not verify signature", "error", err.Error())
			return c.NoContent(http.StatusUnauthorized)
		}
		return next(c)
	}
}