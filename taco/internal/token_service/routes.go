package token_service

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all token service routes
func RegisterRoutes(e *echo.Echo, repo *TokenRepository) {
	handler := NewHandler(repo)

	// Health check
	e.GET("/healthz", handler.HealthCheck)
	e.GET("/health", handler.HealthCheck)

	// Token routes under /api/v1/tokens
	v1 := e.Group("/api/v1")
	
	tokens := v1.Group("/tokens")
	tokens.POST("", handler.CreateToken)                // Create token
	tokens.GET("", handler.ListTokens)                  // List tokens (with query params)
	tokens.GET("/:id", handler.GetToken)                // Get specific token by ID
	tokens.DELETE("/:id", handler.DeleteToken)          // Delete token by ID
	tokens.POST("/verify", handler.VerifyToken)         // Verify token
}

