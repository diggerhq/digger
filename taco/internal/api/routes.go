package api

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/diggerhq/digger/opentaco/internal/analytics"
    "github.com/diggerhq/digger/opentaco/internal/tfe"
    
    "github.com/diggerhq/digger/opentaco/internal/backend"
    authpkg "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/middleware"
    "github.com/diggerhq/digger/opentaco/internal/rbac"
    "github.com/diggerhq/digger/opentaco/internal/s3compat"
    unithandlers "github.com/diggerhq/digger/opentaco/internal/unit"
    "github.com/diggerhq/digger/opentaco/internal/observability"
    "github.com/diggerhq/digger/opentaco/internal/oidc"
    "github.com/diggerhq/digger/opentaco/internal/sts"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/labstack/echo/v4"
)

// RegisterRoutes registers all API routes
func RegisterRoutes(e *echo.Echo, store storage.UnitStore, authEnabled bool) {
	// Health checks
	health := observability.NewHealthHandler()
	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)
	
	// Info endpoint for CLI to detect storage type
	e.GET("/v1/info", func(c echo.Context) error {
		info := map[string]interface{}{
			"storage": map[string]interface{}{
				"type": "memory",
			},
		}
		
		// Check if we're using S3 storage
		if s3Store, ok := store.(storage.S3Store); ok {
			info["storage"] = map[string]interface{}{
				"type":   "s3",
				"bucket": s3Store.GetS3Bucket(),
				"prefix": s3Store.GetS3Prefix(),
			}
		}
		
		return c.JSON(http.StatusOK, info)
	})


	// Prepare auth deps
	signer, err := authpkg.NewSignerFromEnv()
	if err != nil {
		fmt.Printf("Failed to create JWT signer: %v\n", err)
	}
	stsi, _ := sts.NewStatelessIssuerFromEnv()
	ver, _ := oidc.NewFromEnv()

	// Auth routes (no auth required)
	authHandler := authpkg.NewHandler(signer, stsi, ver)
	// Opaque API tokens for TFE surface (S3-backed if available)
	apiTokenMgr := authpkg.NewAPITokenManagerFromStore(store)
	authHandler.SetAPITokenManager(apiTokenMgr)

	e.POST("/v1/auth/exchange", authHandler.Exchange)
	e.POST("/v1/auth/token", authHandler.Token)
	e.POST("/v1/auth/issue-s3-creds", authHandler.IssueS3Creds)
	e.GET("/v1/auth/me", authHandler.Me)
	e.GET("/v1/auth/config", authHandler.Config)
	e.GET("/oidc/jwks.json", authHandler.JWKS)

	// Analytics endpoints (no auth required)
	e.GET("/v1/system-id/user-email", func(c echo.Context) error {
		// Get user email from analytics system
		email := analytics.GetUserEmail()
		// Debug logging
		if email == "" {
			log.Printf("DEBUG: No user email found in analytics system")
		} else {
			log.Printf("DEBUG: User email found: %s", email)
		}
		return c.String(http.StatusOK, email)
	})
	
	e.POST("/v1/system-id/user-email", func(c echo.Context) error {
		var req struct {
			Email string `json:"email"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}
		
		// Set user email in analytics system
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := analytics.SetUserEmail(ctx, req.Email); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to set email"})
		}
		
		return c.JSON(http.StatusOK, map[string]string{"message": "Email set successfully"})
	})

	// Terraform auth integration (no auth required)
	e.GET("/oauth/authorization", authHandler.OAuthAuthorize)
	e.POST("/oauth/token", authHandler.OAuthToken)
	e.GET("/oauth/login-redirect", authHandler.OAuthLoginRedirect)
	e.GET("/oauth/oidc-callback", authHandler.OAuthOIDCCallback)
	e.GET("/oauth/debug", authHandler.DebugConfig)


	// API v1 protected group
	v1 := e.Group("/v1")
	var verifyFn middleware.AccessTokenVerifier
	if authEnabled {
		verifyFn = func(token string) error {
			// JWT only for /v1
			if signer == nil {
				return echo.ErrUnauthorized
			}
			_, err := signer.VerifyAccess(token)
			if err != nil {
				// Debug: log the verification failure
				fmt.Printf("[AUTH DEBUG] Token verification failed: %v\n", err)
				tokenPreview := token
				if len(token) > 50 {
					tokenPreview = token[:50] + "..."
				}
				fmt.Printf("[AUTH DEBUG] Token preview: %s\n", tokenPreview)
			}
			return err
		}
		v1.Use(middleware.RequireAuth(verifyFn))
	}

	// Setup RBAC manager if available
	var rbacManager *rbac.RBACManager
	if store != nil {
		if s3Store, ok := store.(storage.S3Store); ok {
			rbacStore := rbac.NewS3RBACStore(s3Store.GetS3Client(), s3Store.GetS3Bucket(), s3Store.GetS3Prefix())
			rbacManager = rbac.NewRBACManager(rbacStore)
		}
	}

	// Unit handlers (management API) - pass RBAC manager and signer for filtering
	unitHandler := unithandlers.NewHandler(store, rbacManager, signer)

	// Management API (units) with RBAC middleware
	if authEnabled && rbacManager != nil {
		v1.POST("/units", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitWrite, "*")(unitHandler.CreateUnit))
		// ListUnits does its own RBAC filtering internally, no middleware needed
		v1.GET("/units", unitHandler.ListUnits)
		v1.GET("/units/:id", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitRead, "{id}")(unitHandler.GetUnit))
		v1.DELETE("/units/:id", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitDelete, "{id}")(unitHandler.DeleteUnit))
		v1.GET("/units/:id/download", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitRead, "{id}")(unitHandler.DownloadUnit))
		v1.POST("/units/:id/upload", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitWrite, "{id}")(unitHandler.UploadUnit))
		v1.POST("/units/:id/lock", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitLock, "{id}")(unitHandler.LockUnit))
		v1.DELETE("/units/:id/unlock", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitLock, "{id}")(unitHandler.UnlockUnit))
		// Dependency/status
		v1.GET("/units/:id/status", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitRead, "{id}")(unitHandler.GetUnitStatus))
		// Version operations
		v1.GET("/units/:id/versions", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitRead, "{id}")(unitHandler.ListVersions))
		v1.POST("/units/:id/restore", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitWrite, "{id}")(unitHandler.RestoreVersion))
	} else {
		// Fallback without RBAC
		v1.POST("/units", unitHandler.CreateUnit)
		v1.GET("/units", unitHandler.ListUnits)
		v1.GET("/units/:id", unitHandler.GetUnit)
		v1.DELETE("/units/:id", unitHandler.DeleteUnit)
		v1.GET("/units/:id/download", unitHandler.DownloadUnit)
		v1.POST("/units/:id/upload", unitHandler.UploadUnit)
		v1.POST("/units/:id/lock", unitHandler.LockUnit)
		v1.DELETE("/units/:id/unlock", unitHandler.UnlockUnit)
		// Dependency/status
		v1.GET("/units/:id/status", unitHandler.GetUnitStatus)
		// Version operations
		v1.GET("/units/:id/versions", unitHandler.ListVersions)
		v1.POST("/units/:id/restore", unitHandler.RestoreVersion)
	}

	// Terraform HTTP backend proxy with RBAC middleware
	backendHandler := backend.NewHandler(store)
	if authEnabled && rbacManager != nil {
		v1.GET("/backend/*", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitRead, "*")(backendHandler.GetState))
		v1.POST("/backend/*", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitWrite, "*")(backendHandler.UpdateState))
		v1.PUT("/backend/*", middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitWrite, "*")(backendHandler.UpdateState))
		// Explicitly wire non-standard HTTP methods used by Terraform backend
		e.Add("LOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitLock, "*")(backendHandler.HandleLockUnlock)))
		e.Add("UNLOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(middleware.RBACMiddleware(rbacManager, signer, rbac.ActionUnitLock, "*")(backendHandler.HandleLockUnlock)))
	} else if authEnabled {
		v1.GET("/backend/*", backendHandler.GetState)
		v1.POST("/backend/*", backendHandler.UpdateState)
		v1.PUT("/backend/*", backendHandler.UpdateState)
		e.Add("LOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(backendHandler.HandleLockUnlock))
		e.Add("UNLOCK", "/v1/backend/*", middleware.RequireAuth(verifyFn)(backendHandler.HandleLockUnlock))
	} else {
		v1.GET("/backend/*", backendHandler.GetState)
		v1.POST("/backend/*", backendHandler.UpdateState)
		v1.PUT("/backend/*", backendHandler.UpdateState)
		e.Add("LOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
		e.Add("UNLOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
	}

	// S3-compatible endpoint (SigV4, token in X-Amz-Security-Token)
	s3h := s3compat.NewHandler(store, signer, stsi)
	// Explicitly wire supported methods; verification handled inside handler
	e.GET("/s3/*", s3h.Handle)
	e.HEAD("/s3/*", s3h.Handle)
	e.PUT("/s3/*", s3h.Handle)
	e.DELETE("/s3/*", s3h.Handle)

	// RBAC routes (only available with S3 storage)
	if rbacManager != nil {
		rbacHandler := rbac.NewHandler(rbacManager, signer)
		
		// RBAC initialization (no auth required for init)
		v1.POST("/rbac/init", rbacHandler.Init)
		
		// RBAC user info (handle auth gracefully in handler, like /v1/auth/me)
		e.GET("/v1/rbac/me", rbacHandler.Me)
		
		// RBAC management routes (require RBAC manage permission)
		v1.POST("/rbac/users/assign", rbacHandler.AssignRole)
		v1.POST("/rbac/users/revoke", rbacHandler.RevokeRole)
		v1.GET("/rbac/users", rbacHandler.ListUserAssignments)
		v1.POST("/rbac/roles", rbacHandler.CreateRole)
		v1.GET("/rbac/roles", rbacHandler.ListRoles)
		v1.DELETE("/rbac/roles/:id", rbacHandler.DeleteRole)
		v1.POST("/rbac/roles/:id/permissions", rbacHandler.AssignPermissionToRole)
		v1.DELETE("/rbac/roles/:id/permissions/:permissionId", rbacHandler.RevokePermissionFromRole)
		v1.POST("/rbac/permissions", rbacHandler.CreatePermission)
		v1.GET("/rbac/permissions", rbacHandler.ListPermissions)
		v1.DELETE("/rbac/permissions/:id", rbacHandler.DeletePermission)
		v1.POST("/rbac/test", rbacHandler.TestPermissions)
	} else {
		// RBAC not available with memory storage - add catch-all route
		v1.Any("/rbac/*", func(c echo.Context) error {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "RBAC requires S3 storage",
				"message": "RBAC is only available when using S3 storage. Please configure S3 storage to use RBAC features.",
			})
		})
	}

	// TFE api - inject auth handler, storage, and RBAC dependencies
	tfeHandler := tfe.NewTFETokenHandler(authHandler, store, rbacManager)  // Pass rbacManager (may be nil)

	// Create protected TFE group
	tfeGroup := e.Group("/tfe/api/v2")
	if authEnabled {
		// Verifier for TFE: accept JWT or opaque TFE tokens
		tfeVerify := func(token string) error {
			// Try JWT first
			if signer != nil {
				if _, err := signer.VerifyAccess(token); err == nil {
					return nil
				}
			}
			// Fallback to opaque via S3-backed manager
			if apiTokenMgr != nil {
				if _, err := apiTokenMgr.Verify(context.Background(), token); err == nil {
					return nil
				}
			}
			return echo.ErrUnauthorized
		}
		tfeGroup.Use(middleware.RequireAuth(tfeVerify))
	}

	// Move TFE endpoints to protected group
	tfeGroup.GET("/ping", tfeHandler.Ping)
	tfeGroup.GET("/organizations/:org_name/entitlement-set", tfeHandler.GetOrganizationEntitlements)
	tfeGroup.GET("/account/details", tfeHandler.AccountDetails)
	tfeGroup.GET("/organizations/:org_name/workspaces/:workspace_name", tfeHandler.GetWorkspace)
	tfeGroup.POST("/workspaces/:workspace_id/actions/lock", tfeHandler.LockWorkspace)
	tfeGroup.POST("/workspaces/:workspace_id/actions/unlock", tfeHandler.UnlockWorkspace)
	tfeGroup.POST("/workspaces/:workspace_id/actions/force-unlock", tfeHandler.ForceUnlockWorkspace)
	tfeGroup.GET("/workspaces/:workspace_id/current-state-version", tfeHandler.GetCurrentStateVersion)
	tfeGroup.POST("/workspaces/:workspace_id/state-versions", tfeHandler.CreateStateVersion)
	tfeGroup.GET("/state-versions/:id/download", tfeHandler.DownloadStateVersion)
	tfeGroup.GET("/state-versions/:id", tfeHandler.ShowStateVersion)

	// Upload endpoints exempt from auth middleware (Terraform doesn't send auth headers)
	// Security: These validate lock ownership and have RBAC checks in handlers
	// Upload URLs can only be obtained from authenticated CreateStateVersion calls
	e.PUT("/tfe/api/v2/state-versions/:id/upload", tfeHandler.UploadStateVersion)
	e.PUT("/tfe/api/v2/state-versions/:id/json-upload", tfeHandler.UploadJSONStateOutputs)

	// Keep discovery endpoints unprotected (needed for terraform login)
	e.GET("/.well-known/terraform.json", tfeHandler.GetWellKnownJson)
	e.GET("/tfe/app/oauth2/auth", tfeHandler.AuthLogin)
	e.POST("/tfe/oauth2/token", tfeHandler.AuthTokenExchange)

	// Return 404 for unknown TFE endpoints so the CLI doesn't retry & show reconnect banner
	e.Any("/tfe/api/v2/*", func(c echo.Context) error {
		fmt.Printf("TFE endpoint not found (404): %s %s\n", c.Request().Method, c.Request().RequestURI)
		// JSON:API-ish minimal error; 404 avoids go-tfe retry loop
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{
				"status": "404",
				"title":  "not_found",
				"detail": "Endpoint not implemented by OpenTaco",
			}},
		})
	})
}
