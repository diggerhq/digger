package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/diggerhq/digger/opentaco/internal/pagination"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/labstack/echo/v4"
)

// Handler provides RBAC-related HTTP handlers
type Handler struct {
	manager    *RBACManager
	signer     *auth.Signer
	queryStore query.Store
	resolver   interface {
		ResolveRole(ctx context.Context, identifier, orgID string) (string, error)
		ResolvePermission(ctx context.Context, identifier, orgID string) (string, error)
	}
}

// NewHandler creates a new RBAC handler
func NewHandler(manager *RBACManager, signer *auth.Signer, queryStore query.Store) *Handler {
	return &Handler{
		manager:    manager,
		signer:     signer,
		queryStore: queryStore,
	}
}

func (h *Handler) resolveRoleIdentifier(c echo.Context, identifier string) (string, error) {
	if h.resolver == nil {
		return identifier, nil
	}

	orgID := c.Get("organization_id")
	if orgID == nil {
		orgID = "default"
	}

	return h.resolver.ResolveRole(c.Request().Context(), identifier, orgID.(string))
}

func (h *Handler) resolvePermissionIdentifier(c echo.Context, identifier string) (string, error) {
	if h.resolver == nil {
		return identifier, nil
	}

	orgID := c.Get("organization_id")
	if orgID == nil {
		orgID = "default"
	}

	return h.resolver.ResolvePermission(c.Request().Context(), identifier, orgID.(string))
}

// Init handles POST /v1/rbac/init
func (h *Handler) Init(c echo.Context) error {
	logger := logging.FromContext(c)
	var req struct {
		Subject string `json:"subject"`
		Email   string `json:"email"`
	}

	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		logger.Warn("Invalid RBAC init request",
			"operation", "rbac_init",
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}

	if req.Subject == "" || req.Email == "" {
		logger.Warn("Missing subject or email in RBAC init",
			"operation", "rbac_init",
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "subject and email required"})
	}

	logger.Info("Initializing RBAC",
		"operation", "rbac_init",
		"subject", req.Subject,
		"email", req.Email,
	)

	// Check if RBAC is already initialized
	ctx := c.Request().Context()
	enabled, err := h.manager.IsEnabled(ctx)
	if err != nil {
		logger.Error("Failed to check RBAC status",
			"operation", "rbac_init",
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check RBAC status"})
	}

	if enabled {
		logger.Warn("RBAC already initialized",
			"operation", "rbac_init",
		)
		return c.JSON(http.StatusConflict, map[string]string{"error": "RBAC already initialized"})
	}

	// Get org UUID from domain context for InitializeRBAC
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		logger.Error("Organization context missing",
			"operation", "rbac_init",
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	if err := h.manager.InitializeRBAC(ctx, orgCtx.OrgID, req.Subject, req.Email); err != nil {
		logger.Error("Failed to initialize RBAC",
			"operation", "rbac_init",
			"org_id", orgCtx.OrgID,
			"subject", req.Subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to initialize RBAC"})
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		h.syncAllRBACData(c.Request().Context())
	}

	logger.Info("RBAC initialized successfully",
		"operation", "rbac_init",
		"org_id", orgCtx.OrgID,
		"subject", req.Subject,
	)
	return c.JSON(http.StatusOK, map[string]string{"message": "RBAC initialized successfully"})
}

// Me handles GET /v1/rbac/me
func (h *Handler) Me(c echo.Context) error {
	logger := logging.FromContext(c)
	// Get user from token
	principal, err := h.getPrincipalFromToken(c)
	if err != nil {
		logger.Info("Token verification failed for RBAC me",
			"operation", "rbac_me",
			"error", err,
		)
		// Graceful fallback for token verification failures (like auth handler)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"rbac_enabled": false,
			"subject":      "anonymous",
			"email":        "",
			"roles":        []string{},
			"message":      "Token verification failed - check authentication",
		})
	}

	logger.Info("Getting RBAC user info",
		"operation", "rbac_me",
		"subject", principal.Subject,
	)

	// Get org UUID from domain context
	ctx := c.Request().Context()

	// Check if RBAC is enabled
	enabled, err := h.manager.IsEnabled(ctx)
	if err != nil {
		logger.Error("Failed to check RBAC status",
			"operation", "rbac_me",
			"subject", principal.Subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check RBAC status"})
	}

	if !enabled {
		logger.Info("RBAC not initialized",
			"operation", "rbac_me",
			"subject", principal.Subject,
		)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"rbac_enabled": false,
			"subject":      principal.Subject,
			"email":        principal.Email,
			"roles":        []string{},
			"message":      "RBAC not initialized",
		})
	}

	// Get user assignment
	assignment, err := h.manager.GetUserInfo(ctx, principal.Subject)
	if err != nil {
		logger.Error("Failed to get user info",
			"operation", "rbac_me",
			"subject", principal.Subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get user info"})
	}

	if assignment == nil {
		logger.Info("User has no RBAC assignments",
			"operation", "rbac_me",
			"subject", principal.Subject,
		)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"rbac_enabled": true,
			"subject":      principal.Subject,
			"email":        principal.Email,
			"roles":        []string{},
			"message":      "No roles assigned",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"rbac_enabled": true,
		"subject":      assignment.Subject,
		"email":        assignment.Email,
		"roles":        assignment.Roles,
		"created_at":   assignment.CreatedAt,
		"updated_at":   assignment.UpdatedAt,
	})
}

// AssignRole handles POST /v1/rbac/users/assign
func (h *Handler) AssignRole(c echo.Context) error {
	logger := logging.FromContext(c)
	// Check if user has RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	var req struct {
		Subject string `json:"subject,omitempty"`
		Email   string `json:"email"`
		RoleID  string `json:"role_id"`
	}

	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		logger.Warn("Invalid assign role request",
			"operation", "rbac_assign_role",
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}

	if req.Email == "" || req.RoleID == "" {
		logger.Warn("Missing email or role_id",
			"operation", "rbac_assign_role",
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and role_id required"})
	}

	logger.Info("Assigning role",
		"operation", "rbac_assign_role",
		"subject", req.Subject,
		"email", req.Email,
		"role_id", req.RoleID,
	)

	ctx := c.Request().Context()

	// Use email-based assignment if no subject provided
	if req.Subject == "" {
		if err := h.manager.AssignRoleByEmail(ctx, req.Email, req.RoleID); err != nil {
			logger.Error("Failed to assign role by email",
				"operation", "rbac_assign_role",
				"email", req.Email,
				"role_id", req.RoleID,
				"error", err,
			)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to assign role: " + err.Error()})
		}
	} else {
		if err := h.manager.AssignRole(ctx, req.Subject, req.Email, req.RoleID); err != nil {
			logger.Error("Failed to assign role",
				"operation", "rbac_assign_role",
				"subject", req.Subject,
				"role_id", req.RoleID,
				"error", err,
			)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to assign role"})
		}
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		// Sync is org-aware through context
		subject := req.Subject
		if subject == "" {
			orgCtx, _ := domain.OrgFromContext(ctx)
			if assignment, _ := h.manager.store.GetUserAssignmentByEmail(ctx, orgCtx.OrgID, req.Email); assignment != nil {
				subject = assignment.Subject
			}
		}
		if subject != "" {
			orgCtx, _ := domain.OrgFromContext(ctx)
			if assignment, err := h.manager.store.GetUserAssignment(ctx, orgCtx.OrgID, subject); err == nil {
				h.queryStore.SyncUser(ctx, assignment)
			}
		}
	}

	logger.Info("Role assigned successfully",
		"operation", "rbac_assign_role",
		"subject", req.Subject,
		"email", req.Email,
		"role_id", req.RoleID,
	)
	return c.JSON(http.StatusOK, map[string]string{"message": "role assigned successfully"})
}

// RevokeRole handles POST /v1/rbac/users/revoke
func (h *Handler) RevokeRole(c echo.Context) error {
	logger := logging.FromContext(c)
	// Check if user has RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	var req struct {
		Subject string `json:"subject,omitempty"`
		Email   string `json:"email,omitempty"`
		RoleID  string `json:"role_id"`
	}

	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		logger.Warn("Invalid revoke role request",
			"operation", "rbac_revoke_role",
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}

	if req.RoleID == "" {
		logger.Warn("Missing role_id",
			"operation", "rbac_revoke_role",
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role_id required"})
	}

	if req.Subject == "" && req.Email == "" {
		logger.Warn("Missing subject and email",
			"operation", "rbac_revoke_role",
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "either subject or email required"})
	}

	logger.Info("Revoking role",
		"operation", "rbac_revoke_role",
		"subject", req.Subject,
		"email", req.Email,
		"role_id", req.RoleID,
	)

	ctx := c.Request().Context()

	// Use email-based revocation if email provided, otherwise use subject
	if req.Email != "" {
		if err := h.manager.RevokeRoleByEmail(ctx, req.Email, req.RoleID); err != nil {
			logger.Error("Failed to revoke role by email",
				"operation", "rbac_revoke_role",
				"email", req.Email,
				"role_id", req.RoleID,
				"error", err,
			)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to revoke role: " + err.Error()})
		}
	} else {
		if err := h.manager.RevokeRole(ctx, req.Subject, req.RoleID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to revoke role"})
		}
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		// Sync is org-aware through context
		subject := req.Subject
		if subject == "" && req.Email != "" {
			orgCtx, _ := domain.OrgFromContext(ctx)
			if assignment, _ := h.manager.store.GetUserAssignmentByEmail(ctx, orgCtx.OrgID, req.Email); assignment != nil {
				subject = assignment.Subject
			}
		}
		if subject != "" {
			orgCtx, _ := domain.OrgFromContext(ctx)
			if assignment, err := h.manager.store.GetUserAssignment(ctx, orgCtx.OrgID, subject); err == nil {
				h.queryStore.SyncUser(ctx, assignment)
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "role revoked successfully"})
}

// ListUserAssignments handles GET /v1/rbac/users
func (h *Handler) ListUserAssignments(c echo.Context) error {
	// Check if user has RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	ctx := c.Request().Context()

	assignments, err := h.manager.ListUserAssignments(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list user assignments"})
	}

	return c.JSON(http.StatusOK, assignments)
}

// CreateRole handles POST /v1/rbac/roles
func (h *Handler) CreateRole(c echo.Context) error {
	// Check if user has RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	var req struct {
		Name        string   `json:"name"`        // Identifier like "admin" (required)
		Description string   `json:"description"` // Friendly name/description (optional)
		Permissions []string `json:"permissions"`
	}

	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}

	// Normalize role name to lowercase for case-insensitivity
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name required"})
	}

	// Get current user
	principal, err := h.getPrincipalFromToken(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
	}

	role := &Role{
		ID:          req.Name, // Use identifier as ID for storage (UUID generated by database)
		Name:        req.Name, // Identifier like "admin"
		Description: req.Description,
		Permissions: req.Permissions,
		CreatedBy:   principal.Subject,
	}

	if err := h.manager.CreateRole(c.Request().Context(), role); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create role"})
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		if err := h.queryStore.SyncRole(c.Request().Context(), role); err != nil {
			log.Printf("Warning: Failed to sync role to query backend: %v", err)
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "role created successfully"})
}

// ListRoles handles GET /v1/rbac/roles
func (h *Handler) ListRoles(c echo.Context) error {
	// Check if user has RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	ctx := c.Request().Context()
	pageParams := pagination.Parse(c, 50, 200)

	roles, total, err := h.manager.ListRoles(ctx, pageParams.Page, pageParams.PageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list roles"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"roles":     roles,
		"count":     len(roles),
		"total":     total,
		"page":      pageParams.Page,
		"page_size": pageParams.PageSize,
	})
}

// DeleteRole handles DELETE /v1/rbac/roles/:id
func (h *Handler) DeleteRole(c echo.Context) error {
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	roleIDParam := c.Param("id")
	if roleIDParam == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role id required"})
	}

	roleID, err := h.resolveRoleIdentifier(c, roleIDParam)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "role not found"})
	}

	// Get org UUID from domain context
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	if roleID == "admin" || roleID == "default" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot delete default roles"})
	}

	if err := h.manager.store.DeleteRole(ctx, orgCtx.OrgID, roleID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete role"})
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		if err := h.queryStore.SyncDeleteRole(c.Request().Context(), roleID); err != nil {
			log.Printf("Warning: Failed to sync role deletion to query backend: %v", err)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// Helper functions

func (h *Handler) getPrincipalFromToken(c echo.Context) (Principal, error) {
	// First check if principal is already in context (webhook auth sets this)
	if principal, ok := PrincipalFromContext(c.Request().Context()); ok {
		return principal, nil
	}

	// Fall back to JWT token verification for public API routes
	authz := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if h.signer == nil {
		return Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "auth not configured")
	}

	claims, err := h.signer.VerifyAccess(token)
	if err != nil {
		return Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
	}

	return Principal{
		Subject: claims.Subject,
		Email:   claims.Email,
		Roles:   claims.Roles,
		Groups:  claims.Groups,
	}, nil
}

func (h *Handler) requireRBACPermission(c echo.Context, action Action, resource string) error {
	principal, err := h.getPrincipalFromToken(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	can, err := h.manager.Can(ctx, principal, action, resource)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check permissions"})
	}

	if !can {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
	}

	return nil
}

// CreatePermission handles POST /v1/rbac/permissions
func (h *Handler) CreatePermission(c echo.Context) error {
	// Check RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	var req struct {
		Name        string           `json:"name"`        // Identifier like "unit-read" (required)
		Description string           `json:"description"` // Friendly name/description (optional)
		Rules       []PermissionRule `json:"rules"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Normalize permission name to lowercase for case-insensitivity
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name required"})
	}

	principal, err := h.getPrincipalFromToken(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
	}

	permission := Permission{
		ID:          req.Name, // Use identifier as ID for storage (UUID generated by database)
		Name:        req.Name, // Identifier like "unit-read"
		Description: req.Description,
		Rules:       req.Rules,
		CreatedBy:   principal.Subject,
	}

	if err := h.manager.CreatePermission(c.Request().Context(), &permission); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create permission"})
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		if err := h.queryStore.SyncPermission(c.Request().Context(), &permission); err != nil {
			log.Printf("Warning: Failed to sync permission to query backend: %v", err)
		}
	}

	return c.JSON(http.StatusCreated, permission)
}

// ListPermissions handles GET /v1/rbac/permissions
func (h *Handler) ListPermissions(c echo.Context) error {
	// Check RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	ctx := c.Request().Context()
	pageParams := pagination.Parse(c, 50, 200)

	permissions, total, err := h.manager.ListPermissions(ctx, pageParams.Page, pageParams.PageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"permissions": permissions,
		"count":       len(permissions),
		"total":       total,
		"page":        pageParams.Page,
		"page_size":   pageParams.PageSize,
	})
}

// DeletePermission handles DELETE /v1/rbac/permissions/:id
func (h *Handler) DeletePermission(c echo.Context) error {
	// Check RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	idParam := c.Param("id")
	if idParam == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "permission id required"})
	}

	ctx := c.Request().Context()

	id, err := h.resolvePermissionIdentifier(c, idParam)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "permission not found"})
	}

	if err := h.manager.DeletePermission(ctx, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete permission"})
	}

	if h.queryStore != nil && h.queryStore.IsEnabled() {
		if err := h.queryStore.SyncDeletePermission(c.Request().Context(), id); err != nil {
			log.Printf("Warning: Failed to sync permission deletion to query backend: %v", err)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// TestPermissions handles POST /v1/rbac/test
func (h *Handler) TestPermissions(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Action   string `json:"action"`
		Resource string `json:"resource"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	var userAssignment *UserAssignment
	var err error

	// Get org UUID from domain context
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	if req.Email != "" {
		// Admin mode: test permissions for specified user (requires admin permission)
		if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
			return err
		}
		userAssignment, err = h.manager.store.GetUserAssignmentByEmail(ctx, orgCtx.OrgID, req.Email)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}
	} else {
		// Self-check mode: test permissions for current authenticated user
		principal, err := h.getPrincipalFromToken(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
		}

		userAssignment, err = h.manager.store.GetUserAssignment(ctx, orgCtx.OrgID, principal.Subject)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}
	}

	// Get user's roles
	roles := userAssignment.Roles
	if len(roles) == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":                 "denied",
			"reason":                 "user has no roles assigned",
			"user_roles":             []string{},
			"applicable_permissions": []string{},
		})
	}

	// Test permission for each role
	var applicablePermissions []string
	allowed := false

	for _, roleID := range roles {
		role, err := h.manager.store.GetRole(ctx, orgCtx.OrgID, roleID)
		if err != nil {
			continue // Skip invalid roles
		}

		// Check each permission in the role
		for _, permissionID := range role.Permissions {
			permission, err := h.manager.store.GetPermission(ctx, orgCtx.OrgID, permissionID)
			if err != nil {
				continue // Skip invalid permissions
			}

			// Check if this permission applies to the requested action and resource
			for _, rule := range permission.Rules {
				if h.ruleMatches(rule, req.Action, req.Resource) {
					applicablePermissions = append(applicablePermissions, permissionID)
					if rule.Effect == "allow" {
						allowed = true
					} else if rule.Effect == "deny" {
						return c.JSON(http.StatusOK, map[string]interface{}{
							"status":                 "denied",
							"reason":                 fmt.Sprintf("explicitly denied by permission %s", permissionID),
							"user_roles":             roles,
							"applicable_permissions": applicablePermissions,
						})
					}
				}
			}
		}
	}

	status := "denied"
	reason := "no matching allow rules found"
	if allowed {
		status = "allowed"
		reason = "permission granted by applicable permissions"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":                 status,
		"reason":                 reason,
		"user_roles":             roles,
		"applicable_permissions": applicablePermissions,
	})
}

// ruleMatches checks if a permission rule matches the given action and resource
func (h *Handler) ruleMatches(rule PermissionRule, action, resource string) bool {
	return rule.matches(Action(action), resource)
}

// AssignPermissionToRole handles POST /v1/rbac/roles/:id/permissions
func (h *Handler) AssignPermissionToRole(c echo.Context) error {
	// Check RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	roleID := c.Param("id")

	var req struct {
		PermissionID string `json:"permission_id"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Get org UUID from domain context
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get the role
		role, err := h.manager.store.GetRole(ctx, orgCtx.OrgID, roleID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "role not found"})
		}

		// Check if permission exists
		_, err = h.manager.store.GetPermission(ctx, orgCtx.OrgID, req.PermissionID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "permission not found"})
		}

		// Check if permission is already assigned
		for _, existingPermissionID := range role.Permissions {
			if existingPermissionID == req.PermissionID {
				return c.JSON(http.StatusConflict, map[string]string{"error": "permission already assigned to role"})
			}
		}

		// Add permission to role
		role.Permissions = append(role.Permissions, req.PermissionID)

		// Update the role with optimistic locking
		err = h.manager.store.CreateRole(c.Request().Context(), role)
		if err == nil {
			if h.queryStore != nil && h.queryStore.IsEnabled() {
				if err := h.queryStore.SyncRole(c.Request().Context(), role); err != nil {
					log.Printf("Warning: Failed to sync role to query backend: %v", err)
				}
			}
			return c.JSON(http.StatusOK, map[string]string{"message": "permission assigned to role successfully"})
		}

		// If it's a version conflict and we have retries left, try again
		if strings.Contains(err.Error(), "version conflict") && attempt < maxRetries-1 {
			continue
		}

		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update role"})
	}

	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to assign permission after multiple attempts"})
}

// RevokePermissionFromRole handles DELETE /v1/rbac/roles/:id/permissions/:permissionId
func (h *Handler) RevokePermissionFromRole(c echo.Context) error {
	// Check RBAC manage permission
	if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
		return err
	}

	roleID := c.Param("id")
	permissionID := c.Param("permissionId")

	// Get org UUID from domain context
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	const maxRetries = 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Get the role
		role, err := h.manager.store.GetRole(ctx, orgCtx.OrgID, roleID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "role not found"})
		}

		// Find and remove the permission
		var newPermissions []string
		found := false
		for _, existingPermissionID := range role.Permissions {
			if existingPermissionID != permissionID {
				newPermissions = append(newPermissions, existingPermissionID)
			} else {
				found = true
			}
		}

		if !found {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "permission not assigned to role"})
		}

		// Update the role
		role.Permissions = newPermissions

		// Update the role with optimistic locking
		err = h.manager.store.CreateRole(c.Request().Context(), role)
		if err == nil {
			if h.queryStore != nil && h.queryStore.IsEnabled() {
				if err := h.queryStore.SyncRole(c.Request().Context(), role); err != nil {
					log.Printf("Warning: Failed to sync role to query backend: %v", err)
				}
			}
			return c.NoContent(http.StatusNoContent)
		}

		// If it's a version conflict and we have retries left, try again
		if strings.Contains(err.Error(), "version conflict") && attempt < maxRetries-1 {
			continue
		}

		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update role"})
	}

	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to revoke permission after multiple attempts"})
}

func (h *Handler) syncAllRBACData(ctx context.Context) {
	if h.queryStore == nil || !h.queryStore.IsEnabled() {
		return
	}

	// List methods are org-aware via context
	const syncPageSize = 200
	for page := 1; ; page++ {
		perms, total, err := h.manager.ListPermissions(ctx, page, syncPageSize)
		if err != nil || len(perms) == 0 {
			break
		}
		for _, perm := range perms {
			h.queryStore.SyncPermission(ctx, perm)
		}
		if int64(page*syncPageSize) >= total {
			break
		}
	}

	for page := 1; ; page++ {
		roles, total, err := h.manager.ListRoles(ctx, page, syncPageSize)
		if err != nil || len(roles) == 0 {
			break
		}
		for _, role := range roles {
			h.queryStore.SyncRole(ctx, role)
		}
		if int64(page*syncPageSize) >= total {
			break
		}
	}

	if users, err := h.manager.ListUserAssignments(ctx); err == nil {
		for _, user := range users {
			h.queryStore.SyncUser(ctx, user)
		}
	}
}
