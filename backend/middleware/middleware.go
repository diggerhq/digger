package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
)

func GetWebMiddleware() gin.HandlerFunc {
	if _, ok := os.LookupEnv("JWT_AUTH"); ok {
		slog.Info("Using JWT middleware for web routes")
		auth := services.Auth{
			HttpClient: http.Client{},
			Host:       os.Getenv("AUTH_HOST"),
			Secret:     os.Getenv("AUTH_SECRET"),
			ClientId:   os.Getenv("FRONTEGG_CLIENT_ID"),
		}
		return JWTWebAuth(auth)
	} else if _, ok := os.LookupEnv("HTTP_BASIC_AUTH"); ok {
		slog.Info("Using http basic auth middleware for web routes")
		return HttpBasicWebAuth()
	} else if _, ok := os.LookupEnv("NOOP_AUTH"); ok {
		slog.Info("Using noop auth for web routes")
		return NoopWebAuth()
	} else {
		slog.Error("No authentication method specified. Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
		panic("No authentication method specified. Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
	}
}

func GetApiMiddleware() gin.HandlerFunc {
	if _, ok := os.LookupEnv("JWT_AUTH"); ok {
		slog.Info("Using JWT middleware for API routes")
		auth := services.Auth{
			HttpClient: http.Client{},
			Host:       os.Getenv("AUTH_HOST"),
			Secret:     os.Getenv("AUTH_SECRET"),
			ClientId:   os.Getenv("FRONTEGG_CLIENT_ID"),
		}
		return JWTBearerTokenAuth(auth)
	} else if _, ok := os.LookupEnv("HTTP_BASIC_AUTH"); ok {
		slog.Info("Using http basic auth middleware for API routes")
		return HttpBasicApiAuth()
	} else if _, ok := os.LookupEnv("NOOP_AUTH"); ok {
		slog.Info("Using noop auth for API routes")
		return NoopApiAuth()
	} else {
		slog.Error("No authentication method specified. Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
		panic("No authentication method specified. Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
	}
}

func CheckJobToken(c *gin.Context, token string) (*models.JobToken, error) {
	jobToken, err := models.DB.GetJobToken(token)
	if jobToken == nil {
		slog.Warn("Invalid bearer token")
		c.String(http.StatusForbidden, "Invalid bearer token")
		c.Abort()
		return nil, fmt.Errorf("invalid bearer token")
	}

	if time.Now().After(jobToken.Expiry) {
		slog.Warn("Token has already expired", "tokenValue", jobToken.Value, "expiry", jobToken.Expiry)
		c.String(http.StatusForbidden, "Token has expired")
		c.Abort()
		return nil, fmt.Errorf("token has expired")
	}

	if err != nil {
		slog.Error("Error while fetching token from database", "error", err)
		c.String(http.StatusInternalServerError, "Error occurred while fetching database")
		c.Abort()
		return nil, fmt.Errorf("could not fetch cli token")
	}

	slog.Debug("Token verified", "tokenValue", jobToken.Value, "accessLevel", jobToken.Type, "expiry", jobToken.Expiry)
	return jobToken, nil
}
