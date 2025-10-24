package api

import (
	"log"
	"net/http"
	"os"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/middleware"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
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
	
	// Validate org UUID from webhook header and add to domain context
	internal.Use(middleware.WebhookOrgUUIDMiddleware())
	log.Println("Org UUID validation middleware enabled for internal routes")

	// Organization and User management endpoints
	if orgRepo != nil && userRepo != nil {
		// Create handler with repository interfaces (domain layer)
		orgHandler := NewOrgHandler(orgRepo, userRepo, deps.RBACManager)
		
		// Organization endpoints
		internal.POST("/orgs", orgHandler.CreateOrganization)
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

