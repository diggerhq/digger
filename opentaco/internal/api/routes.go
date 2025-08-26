package api

import (
    "github.com/diggerhq/digger/opentaco/internal/backend"
    authpkg "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/middleware"
    statehandlers "github.com/diggerhq/digger/opentaco/internal/state"
    "github.com/diggerhq/digger/opentaco/internal/observability"
    "github.com/diggerhq/digger/opentaco/internal/oidc"
    "github.com/diggerhq/digger/opentaco/internal/sts"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/labstack/echo/v4"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(e *echo.Echo, store storage.StateStore, authEnabled bool) {
	// Health checks
	health := observability.NewHealthHandler()
	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)

	// Prepare auth deps
    signer, _ := authpkg.NewSignerFromEnv()
    stsi, _ := sts.NewStatelessIssuerFromEnv()
    ver, _ := oidc.NewFromEnv()

	// Auth routes (no auth required)
    authHandler := authpkg.NewHandler(signer, stsi, ver)
    e.POST("/v1/auth/exchange", authHandler.Exchange)
    e.POST("/v1/auth/token", authHandler.Token)
    e.POST("/v1/auth/issue-s3-creds", authHandler.IssueS3Creds)
    e.GET("/v1/auth/me", authHandler.Me)
    e.GET("/oidc/jwks.json", authHandler.JWKS)
    e.GET("/v1/auth/config", authHandler.Config)

	// API v1 protected group
	v1 := e.Group("/v1")
    var verifyFn middleware.AccessTokenVerifier
    if authEnabled {
        verifyFn = func(token string) error {
            if signer == nil { return echo.ErrUnauthorized }
            _, err := signer.VerifyAccess(token)
            return err
        }
        v1.Use(middleware.RequireAuth(verifyFn))
    }
	
    // State handlers (management API)
    stateHandler := statehandlers.NewHandler(store)
	
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
    if authEnabled {
        e.Add("LOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(backendHandler.HandleLockUnlock))
        e.Add("UNLOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(backendHandler.HandleLockUnlock))
    } else {
        e.Add("LOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
        e.Add("UNLOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
    }

    // (auth routes defined above; keep v1 protected for states/backend)
}
