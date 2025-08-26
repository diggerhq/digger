package api

import (
    "github.com/diggerhq/digger/opentaco/internal/backend"
    authhandlers "github.com/diggerhq/digger/opentaco/internal/auth"
    statehandlers "github.com/diggerhq/digger/opentaco/internal/state"
    "github.com/diggerhq/digger/opentaco/internal/observability"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/labstack/echo/v4"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(e *echo.Echo, store storage.StateStore) {
	// Health checks
	health := observability.NewHealthHandler()
	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)

	// API v1 group
	v1 := e.Group("/v1")
	
    // State handlers (management API)
    stateHandler := statehandlers.NewHandler(store)
    authHandler := authhandlers.NewHandler()
	
    // Management API
    v1.POST("/states", stateHandler.CreateState)
	v1.GET("/states", stateHandler.ListStates)
	v1.GET("/states/:id", stateHandler.GetState)
	v1.DELETE("/states/:id", stateHandler.DeleteState)
	v1.GET("/states/:id/download", stateHandler.DownloadState)
	v1.POST("/states/:id/upload", stateHandler.UploadState)
	v1.POST("/states/:id/lock", stateHandler.LockState)
	v1.DELETE("/states/:id/unlock", stateHandler.UnlockState)
	
    // Terraform HTTP backend proxy
    backendHandler := backend.NewHandler(store)
	v1.GET("/backend/*", backendHandler.GetState)
	v1.POST("/backend/*", backendHandler.UpdateState)
	v1.PUT("/backend/*", backendHandler.UpdateState)
	// Explicitly wire non-standard HTTP methods used by Terraform backend
	e.Add("LOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
    e.Add("UNLOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)

    // Auth & STS routes
    v1.POST("/auth/exchange", authHandler.Exchange)
    v1.POST("/auth/token", authHandler.Token)
    v1.POST("/auth/issue-s3-creds", authHandler.IssueS3Creds)
    v1.GET("/auth/me", authHandler.Me)

    // JWKS (well-known-ish)
    e.GET("/oidc/jwks.json", authHandler.JWKS)
}
