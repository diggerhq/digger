package api

import (
	"log"
	"net/http"
	"os"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/middleware"
	"github.com/diggerhq/digger/opentaco/internal/oidc"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
	"github.com/diggerhq/digger/opentaco/internal/sts"
	"github.com/diggerhq/digger/opentaco/internal/tfe"
	unithandlers "github.com/diggerhq/digger/opentaco/internal/unit"
	"github.com/labstack/echo/v4"
)


func RegisterInternalRoutes(e *echo.Echo, deps Dependencies) {
	webhookSecret := os.Getenv("OPENTACO_ENABLE_INTERNAL_ENDPOINTS")
	if webhookSecret == "" {
		log.Println("OPENTACO_ENABLE_INTERNAL_ENDPOINTS not configured, skipping internal routes")
		return
	}

	log.Println("Registering internal routes with webhook authentication")

	// Create repositories first (needed for webhook middleware)
	var orgRepo domain.OrganizationRepository
	var userRepo domain.UserRepository
	
	if deps.QueryStore != nil {
		orgRepo = repositories.NewOrgRepositoryFromQueryStore(deps.QueryStore)
		userRepo = repositories.NewUserRepositoryFromQueryStore(deps.QueryStore)
	}

	// Create internal group with webhook auth
	internal := e.Group("/internal/api")
	internal.Use(middleware.WebhookAuth())
	
	// Add org resolution middleware - resolves org name to UUID and adds to domain context
	if deps.QueryStore != nil {
		if db := repositories.GetDBFromQueryStore(deps.QueryStore); db != nil {
			// Create identifier resolver (infrastructure layer)
			identifierResolver := repositories.NewIdentifierResolver(db)
			// Pass interface to middleware (clean architecture!)
			internal.Use(middleware.ResolveOrgContextMiddleware(identifierResolver))
			log.Println("Org context resolution middleware enabled for internal routes")
		} else {
			log.Println("WARNING: QueryStore does not implement GetDB() *gorm.DB - org resolution disabled")
		}
	}

	// Organization and User management endpoints
	if orgRepo != nil && userRepo != nil {
		// Create handler with repository interfaces (domain layer)
		orgHandler := NewOrgHandler(orgRepo, userRepo, deps.RBACManager)
		
		// Organization endpoints
		internal.POST("/orgs", orgHandler.CreateOrganization)
		internal.POST("/orgs/sync", orgHandler.SyncExternalOrg)
		internal.GET("/orgs/:orgId", orgHandler.GetOrganization)
		internal.GET("/orgs", orgHandler.ListOrganizations)
		
		// User endpoints
		internal.POST("/users", orgHandler.CreateUser)
		internal.GET("/users/:subject", orgHandler.GetUser)
		internal.GET("/users", orgHandler.ListUsers)
		
		log.Println("Organization management endpoints registered at /internal/api/orgs")
		log.Println("User management endpoints registered at /internal/api/users")
	} else {
		log.Println("Warning: Could not create org/user repositories, endpoints disabled")
	}
	
	// Reuse existing RBAC handler with webhook auth (no duplication)
	if deps.RBACManager != nil {
		rbacHandler := rbac.NewHandler(deps.RBACManager, deps.Signer, deps.QueryStore)
		rbacGroup := internal.Group("/rbac")
		rbacGroup.POST("/roles", rbacHandler.CreateRole)
		rbacGroup.GET("/roles", rbacHandler.ListRoles)
		rbacGroup.POST("/permissions", rbacHandler.CreatePermission)
		rbacGroup.GET("/permissions", rbacHandler.ListPermissions)
		rbacGroup.POST("/assign", rbacHandler.AssignRole)
		rbacGroup.POST("/revoke", rbacHandler.RevokeRole)
		log.Println("RBAC management endpoints registered at /internal/api/rbac")
	}

	// For internal routes, use RBAC-wrapped repository
	// Architecture:
	// - Webhook secret authenticates the SYSTEM (backend orchestrator) 
	// - X-User-ID header identifies the END USER making the request
	// - RBAC enforces what that USER can do (via repository layer)
	// - Org scoping handled by middleware (ResolveOrgContextMiddleware) + database foreign keys
	
	// Create identifier resolver for unit name→UUID resolution
	var identifierResolver domain.IdentifierResolver
	if deps.QueryStore != nil {
		if db := repositories.GetDBFromQueryStore(deps.QueryStore); db != nil {
			identifierResolver = repositories.NewIdentifierResolver(db)
		}
	}
	
	// Create handler with org-scoped + RBAC-wrapped repository
	unitHandler := unithandlers.NewHandler(
		domain.UnitManagement(deps.Repository), // Use RBAC-wrapped repository directly
		deps.BlobStore,
		deps.RBACManager,
		deps.Signer,
		deps.QueryStore,
		identifierResolver,
	)

	// Internal routes with RBAC enforcement
	// Note: Users must have permissions assigned via /internal/api/rbac endpoints
	internal.POST("/units", unitHandler.CreateUnit)
	internal.GET("/units", unitHandler.ListUnits)
	internal.GET("/units/:id", unitHandler.GetUnit)
	internal.DELETE("/units/:id", unitHandler.DeleteUnit)
	internal.GET("/units/:id/download", unitHandler.DownloadUnit)
	internal.POST("/units/:id/upload", unitHandler.UploadUnit)
	internal.POST("/units/:id/lock", unitHandler.LockUnit)
	internal.DELETE("/units/:id/unlock", unitHandler.UnlockUnit)
	internal.GET("/units/:id/status", unitHandler.GetUnitStatus)
	internal.GET("/units/:id/versions", unitHandler.ListVersions)
	internal.POST("/units/:id/restore", unitHandler.RestoreVersion)

	// ====================================================================================
	// TFE API Routes with Webhook Auth (for UI forwarding)
	// ====================================================================================
	// These mirror the public TFE routes but use webhook auth instead of opaque tokens
	// This allows the UI to forward Terraform Cloud API requests on behalf of users
	
	// Prepare auth deps for TFE handler
	stsi, _ := sts.NewStatelessIssuerFromEnv()
	ver, _ := oidc.NewFromEnv()
	authHandler := auth.NewHandler(deps.Signer, stsi, ver)
	apiTokenMgr := auth.NewAPITokenManagerFromStore(deps.BlobStore)
	authHandler.SetAPITokenManager(apiTokenMgr)
	
	// Create identifier resolver for TFE org resolution
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
			log.Println("TFE repositories initialized successfully (internal routes)")
		}
	}
	
	// Create TFE handler with webhook auth context
	// Pass both wrapped (for authenticated calls) and unwrapped (for signed URLs) repositories
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
	
	// TFE group with webhook auth (for UI pass-through)
	tfeInternal := e.Group("/internal/tfe/api/v2")
	tfeInternal.Use(middleware.WebhookAuth())
	
	// Add org resolution middleware for TFE routes
	if tfeIdentifierResolver != nil {
		tfeInternal.Use(middleware.ResolveOrgContextMiddleware(tfeIdentifierResolver))
		log.Println("Org context resolution middleware enabled for internal TFE routes")
	}
	
	// Register TFE endpoints (same handlers as public TFE routes)
	tfeInternal.GET("/ping", tfeHandler.Ping)
	tfeInternal.GET("/organizations/:org_name/entitlement-set", tfeHandler.GetOrganizationEntitlements)
	tfeInternal.GET("/account/details", tfeHandler.AccountDetails)
	tfeInternal.GET("/organizations/:org_name/workspaces/:workspace_name", tfeHandler.GetWorkspace)
	tfeInternal.POST("/workspaces/:workspace_id/actions/lock", tfeHandler.LockWorkspace)
	tfeInternal.POST("/workspaces/:workspace_id/actions/unlock", tfeHandler.UnlockWorkspace)
	tfeInternal.POST("/workspaces/:workspace_id/actions/force-unlock", tfeHandler.ForceUnlockWorkspace)
	tfeInternal.GET("/workspaces/:workspace_id/current-state-version", tfeHandler.GetCurrentStateVersion)
	tfeInternal.POST("/workspaces/:workspace_id/state-versions", tfeHandler.CreateStateVersion)
	tfeInternal.GET("/state-versions/:id/download", tfeHandler.DownloadStateVersion)
	tfeInternal.GET("/state-versions/:id", tfeHandler.ShowStateVersion)

	tfeInternal.POST("/workspaces/:workspace_name/configuration-versions", tfeHandler.CreateConfigurationVersions)
	tfeInternal.GET("/configuration-versions/:id", tfeHandler.GetConfigurationVersion)
	tfeInternal.POST("/runs", tfeHandler.CreateRun)
	tfeInternal.GET("/runs/:id", tfeHandler.GetRun)
	tfeInternal.POST("/runs/:id/actions/apply", tfeHandler.ApplyRun)
	tfeInternal.GET("/runs/:id/policy-checks", tfeHandler.GetPolicyChecks)
	tfeInternal.GET("/runs/:id/task-stages", tfeHandler.GetTaskStages)
	tfeInternal.GET("/runs/:id/cost-estimates", tfeHandler.GetCostEstimates)
	tfeInternal.GET("/runs/:id/run-events", tfeHandler.GetRunEvents)
	tfeInternal.GET("/plans/:id", tfeHandler.GetPlan)
	tfeInternal.GET("/applies/:id", tfeHandler.GetApply)


	log.Println("TFE API endpoints registered at /internal/tfe/api/v2 with webhook auth")
	
	// ====================================================================================
	// Health and Info Endpoints
	// ====================================================================================
	
	// Health check for internal routes
	internal.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":    "ok",
			"type":      "internal",
			"auth_type": "webhook",
		})
	})

	// Info endpoint that shows current user context
	internal.GET("/me", func(c echo.Context) error {
		userID := c.Get("user_id")
		email := c.Get("email")
		orgID := c.Get("organization_id")

		// Get principal from context
		principal, hasPrincipal := rbac.PrincipalFromContext(c.Request().Context())

		info := map[string]interface{}{
			"user_id": userID,
			"email":   email,
			"org_id":  orgID,
		}

		if hasPrincipal {
			info["principal"] = map[string]interface{}{
				"subject": principal.Subject,
				"email":   principal.Email,
				"roles":   principal.Roles,
				"groups":  principal.Groups,
			}
		}

		return c.JSON(http.StatusOK, info)
	})

	log.Printf("Internal routes registered at /internal/api/* with webhook authentication")
}
// wrapWithWebhookRBAC wraps a handler with RBAC permission checking
func wrapWithWebhookRBAC(manager *rbac.RBACManager, action rbac.Action, resource string) func(echo.HandlerFunc) echo.HandlerFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get principal from context (injected by webhook middleware)
			principal, ok := rbac.PrincipalFromContext(c.Request().Context())
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "no principal in context",
				})
			}

			// Check if RBAC is enabled
			enabled, err := manager.IsEnabled(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "failed to check RBAC status",
				})
			}

			// If RBAC is not enabled, allow access
			if !enabled {
				return next(c)
			}

			// Resolve resource pattern (e.g., "{id}" -> actual ID from path param)
			resolvedResource := resource
			if resource == "{id}" {
				resolvedResource = c.Param("id")
			} else if resource == "*" {
				// For wildcard resources, use the path or a default
				resolvedResource = c.Request().URL.Path
			}

			// Check permission
			allowed, err := manager.Can(c.Request().Context(), principal, action, resolvedResource)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "failed to check permission",
				})
			}

			if !allowed {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error":    "permission denied",
					"action":   string(action),
					"resource": resolvedResource,
				})
			}

			return next(c)
		}
	}
}

