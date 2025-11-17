package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/tfe"

	authpkg "github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/backend"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/middleware"
	"github.com/diggerhq/digger/opentaco/internal/observability"
	"github.com/diggerhq/digger/opentaco/internal/oidc"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
	"github.com/diggerhq/digger/opentaco/internal/s3compat"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/diggerhq/digger/opentaco/internal/sts"
	unithandlers "github.com/diggerhq/digger/opentaco/internal/unit"
	"github.com/labstack/echo/v4"
)

// Dependencies holds all the interface-based dependencies for routes.
// This uses interface segregation - each handler gets ONLY what it needs.
type Dependencies struct {
	Repository          domain.UnitRepository // RBAC-wrapped repository (used by all routes)
	UnwrappedRepository domain.UnitRepository // Unwrapped repository (for pre-authorized operations like signed URLs)
	BlobStore           storage.UnitStore     // Direct blob access (for legacy components like API tokens)
	QueryStore          query.Store           // Direct query access (analytics, RBAC)
	RBACManager         *rbac.RBACManager     // RBAC management (RBAC routes only)
	Signer              *authpkg.Signer       // JWT signing (auth, middleware)
	AuthEnabled         bool                  // Whether auth is enabled
}

// RegisterRoutes registers all API routes with interface-scoped dependencies.
// - Backend: StateOperations (6 methods) - cannot create/list/delete/version
// - S3-compat: StateOperations (6 methods) - cannot create/list/delete/version
// - TFE: TFEOperations (6 methods) - read/write/lock only
// - Unit: UnitManagement (11 methods) - full access for management API
// - RBAC: RBACManager + QueryStore - NO unit access at all
func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	queryStore := deps.QueryStore

	// Repository already implements all needed interfaces (UnitManagement embeds StateOperations & TFEOperations)
	// We pass the same repository reference but with different interface types for scoping
	stateOps := domain.StateOperations(deps.Repository)
	unitMgmt := domain.UnitManagement(deps.Repository)

	// Health checks
	health := observability.NewHealthHandler()
	e.GET("/healthz", health.Healthz)
	e.GET("/readyz", health.Readyz)

	// Sync health check (monitors blob/query synchronization)
	syncHealth := observability.NewSyncHealthChecker(deps.Repository, deps.QueryStore)
	e.GET("/healthz/sync", func(c echo.Context) error {
		status := syncHealth.CheckSyncHealth(c.Request().Context())
		if status.Healthy {
			return c.JSON(200, status)
		}
		return c.JSON(503, status)
	})

	// Capabilities endpoint - exposes what features the server supports
	e.GET("/v1/capabilities", func(c echo.Context) error {
		caps := map[string]interface{}{
			"features": map[string]bool{
				"versioning": true, // All stores support versioning
				"rbac":       true, // RBAC available with all storage types
				"query":      true, // Query backend always enabled
			},
		}
		return c.JSON(http.StatusOK, caps)
	})

	// Prepare auth deps
	stsi, _ := sts.NewStatelessIssuerFromEnv()
	ver, _ := oidc.NewFromEnv()

	// Auth routes (no auth required)
	authHandler := authpkg.NewHandler(deps.Signer, stsi, ver)
	// Opaque API tokens for TFE surface (uses blob store for storage)
	apiTokenMgr := authpkg.NewAPITokenManagerFromStore(deps.BlobStore)
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

	// API v1 protected group - JWT tokens only
	v1 := e.Group("/v1")
	// Create identifier resolver for unit name→UUID resolution
	var identifierResolver domain.IdentifierResolver
	if deps.QueryStore != nil {
		if db := repositories.GetDBFromQueryStore(deps.QueryStore); db != nil {
			identifierResolver = repositories.NewIdentifierResolver(db)
		}
	}

	if deps.AuthEnabled {
		jwtVerifyFn := middleware.JWTOnlyVerifier(deps.Signer)
		v1.Use(middleware.RequireAuth(jwtVerifyFn, deps.Signer))

		// Add JWT org resolution middleware (converts org name from JWT to UUID in domain context)
		if identifierResolver != nil {
			v1.Use(middleware.JWTOrgResolverMiddleware(identifierResolver))
			log.Println("JWT org resolver middleware enabled for /v1 routes")
		} else {
			log.Println("WARNING: QueryStore does not implement GetDB() *gorm.DB - JWT org resolution disabled")
		}
	}

	// Unit handlers (management API) - uses UnitManagement interface (11 methods)
	unitHandler := unithandlers.NewHandler(unitMgmt, deps.BlobStore, deps.RBACManager, deps.Signer, queryStore, identifierResolver)

	// Management API (units) with JWT-only RBAC middleware
	if deps.AuthEnabled {
		v1.POST("/units", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitWrite, "*")(unitHandler.CreateUnit))
		// ListUnits does its own RBAC filtering internally, no middleware needed
		v1.GET("/units", unitHandler.ListUnits)
		v1.GET("/units/:id", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitRead, "{id}")(unitHandler.GetUnit))
		v1.DELETE("/units/:id", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitDelete, "{id}")(unitHandler.DeleteUnit))
		v1.GET("/units/:id/download", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitRead, "{id}")(unitHandler.DownloadUnit))
		v1.POST("/units/:id/upload", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitWrite, "{id}")(unitHandler.UploadUnit))
		v1.POST("/units/:id/lock", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitLock, "{id}")(unitHandler.LockUnit))
		v1.DELETE("/units/:id/unlock", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitLock, "{id}")(unitHandler.UnlockUnit))
		// Dependency/status
		v1.GET("/units/:id/status", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitRead, "{id}")(unitHandler.GetUnitStatus))
		// Version operations
		v1.GET("/units/:id/versions", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitRead, "{id}")(unitHandler.ListVersions))
		v1.POST("/units/:id/restore", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitWrite, "{id}")(unitHandler.RestoreVersion))
	} else {
		// Fallback without auth
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

	// Terraform HTTP backend proxy
	// Uses StateOperations interface (6 methods)
	backendHandler := backend.NewHandler(stateOps)
	if deps.AuthEnabled {
		v1.GET("/backend/*", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitRead, "*")(backendHandler.GetState))
		v1.POST("/backend/*", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitWrite, "*")(backendHandler.UpdateState))
		v1.PUT("/backend/*", middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitWrite, "*")(backendHandler.UpdateState))
		// Explicitly wire non-standard HTTP methods used by Terraform backend
		jwtVerifyFn := middleware.JWTOnlyVerifier(deps.Signer)
		e.Add("LOCK", "/v1/backend/*", middleware.RequireAuth(jwtVerifyFn, deps.Signer)(middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitLock, "*")(backendHandler.HandleLockUnlock)))
		e.Add("UNLOCK", "/v1/backend/*", middleware.RequireAuth(jwtVerifyFn, deps.Signer)(middleware.JWTOnlyRBACMiddleware(deps.RBACManager, deps.Signer, rbac.ActionUnitLock, "*")(backendHandler.HandleLockUnlock)))
	} else {
		v1.GET("/backend/*", backendHandler.GetState)
		v1.POST("/backend/*", backendHandler.UpdateState)
		v1.PUT("/backend/*", backendHandler.UpdateState)
		e.Add("LOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
		e.Add("UNLOCK", "/v1/backend/*", backendHandler.HandleLockUnlock)
	}

	// S3-compatible endpoint (SigV4, token in X-Amz-Security-Token)
	// Uses StateOperations interface (6 methods)
	s3h := s3compat.NewHandler(stateOps, deps.Signer, stsi)
	// Explicitly wire supported methods; verification handled inside handler
	e.GET("/s3/*", s3h.Handle)
	e.HEAD("/s3/*", s3h.Handle)
	e.PUT("/s3/*", s3h.Handle)
	e.DELETE("/s3/*", s3h.Handle)

	// RBAC routes: uses RBAC manager only no unit repository access
	rbacHandler := rbac.NewHandler(deps.RBACManager, deps.Signer, queryStore)

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

	// TFE api - inject auth handler, wrapped & unwrapped repositories, blob store for tokens, and RBAC dependencies
	// TFE handler scopes to TFEOperations internally but needs blob store for API token storage
	// Unwrapped repository is used for signed URL operations (pre-authorized, no RBAC checks needed)
	// Create identifier resolver for org resolution
	var tfeIdentifierResolver domain.IdentifierResolver
	var runRepo domain.TFERunRepository
	var planRepo domain.TFEPlanRepository
	var configVerRepo domain.TFEConfigurationVersionRepository

	if deps.QueryStore != nil {
		if db := repositories.GetDBFromQueryStore(deps.QueryStore); db != nil {
			tfeIdentifierResolver = repositories.NewIdentifierResolver(db)
			// Create TFE repositories for runs, plans, and configuration versions
			runRepo = repositories.NewTFERunRepository(db)
			planRepo = repositories.NewTFEPlanRepository(db)
			configVerRepo = repositories.NewTFEConfigurationVersionRepository(db)
			log.Println("TFE repositories initialized successfully")
		}
	}

	tfeHandler := tfe.NewTFETokenHandler(
		authHandler,
		deps.Repository,
		deps.UnwrappedRepository,
		deps.BlobStore,
		deps.RBACManager,
		tfeIdentifierResolver,
		runRepo,
		planRepo,
		configVerRepo,
	)

	// Create protected TFE group - opaque tokens only
	tfeGroup := e.Group("/tfe/api/v2")
	if deps.AuthEnabled {
		opaqueVerifyFn := middleware.OpaqueOnlyVerifier(apiTokenMgr)
		tfeGroup.Use(middleware.RequireAuth(opaqueVerifyFn, deps.Signer))
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
	tfeGroup.GET("/state-versions/:id", tfeHandler.ShowStateVersion)

	// Configuration version routes
	tfeGroup.POST("/workspaces/:workspace_name/configuration-versions", tfeHandler.CreateConfigurationVersions)
	tfeGroup.GET("/configuration-versions/:id", tfeHandler.GetConfigurationVersion)

	// Run routes
	tfeGroup.POST("/runs", tfeHandler.CreateRun)
	tfeGroup.GET("/runs/:id", tfeHandler.GetRun)
	tfeGroup.POST("/runs/:id/actions/apply", tfeHandler.ApplyRun)
	tfeGroup.GET("/runs/:id/policy-checks", tfeHandler.GetPolicyChecks)
	tfeGroup.GET("/runs/:id/task-stages", tfeHandler.GetTaskStages)
	tfeGroup.GET("/runs/:id/cost-estimates", tfeHandler.GetCostEstimates)
	tfeGroup.GET("/runs/:id/run-events", tfeHandler.GetRunEvents)

	// Plan routes
	tfeGroup.GET("/plans/:id", tfeHandler.GetPlan)
	tfeGroup.GET("/plans/:id/json-output", tfeHandler.GetPlanJSONOutput)
	tfeGroup.GET("/plans/:id/json-output-redacted", tfeHandler.GetPlanJSONOutput) // Alias for json-output

	// Apply routes
	tfeGroup.GET("/applies/:id", tfeHandler.GetApply)

	// Upload endpoints exempt from auth middleware (Terraform doesn't send auth headers)
	// Security: These validate lock ownership and have RBAC checks in handlers
	// Upload URLs can only be obtained from authenticated CreateStateVersion calls
	tfeSignedUrlsGroup := e.Group("/tfe/api/v2")
	tfeSignedUrlsGroup.Use(middleware.VerifySignedURL)
	tfeSignedUrlsGroup.GET("/state-versions/:id/download", tfeHandler.DownloadStateVersion)
	tfeSignedUrlsGroup.PUT("/state-versions/:id/upload", tfeHandler.UploadStateVersion)
	tfeSignedUrlsGroup.PUT("/state-versions/:id/json-upload", tfeHandler.UploadJSONStateOutputs)
	tfeSignedUrlsGroup.PUT("/configuration-versions/:id/upload", tfeHandler.UploadConfigurationArchive)

	// Plan log streaming - token-based auth (token embedded in path, not query string)
	// Security: Time-limited HMAC-signed tokens, Terraform CLI preserves path
	e.GET("/tfe/api/v2/plans/:planID/logs/:token", tfeHandler.GetPlanLogs)

	// Apply log streaming - same tokenized approach
	e.GET("/tfe/api/v2/applies/:applyID/logs/:token", tfeHandler.GetApplyLogs)

	// Keep discovery endpoints unprotected (needed for terraform login)
	e.GET("/.well-known/terraform.json", tfeHandler.GetWellKnownJson)
	e.GET("/tfe/api/v2/motd", tfeHandler.MessageOfTheDay)

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

	// Register webhook-authenticated internal routes (if OPENTACO_ENABLE_INTERNAL_ENDPOINTS is set)
	RegisterInternalRoutes(e, deps)
}
