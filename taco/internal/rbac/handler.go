package rbac

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/labstack/echo/v4"
)

// Handler provides RBAC-related HTTP handlers
type Handler struct {
    manager *RBACManager
    signer  *auth.Signer
}

// NewHandler creates a new RBAC handler
func NewHandler(manager *RBACManager, signer *auth.Signer) *Handler {
    return &Handler{
        manager: manager,
        signer:  signer,
    }
}

// Init handles POST /v1/rbac/init
func (h *Handler) Init(c echo.Context) error {
    var req struct {
        Subject string `json:"subject"`
        Email   string `json:"email"`
    }
    
    if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    }
    
    if req.Subject == "" || req.Email == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "subject and email required"})
    }
    
    // Check if RBAC manager is available (only works with S3 storage)
    if h.manager == nil {
        return c.JSON(http.StatusBadRequest, map[string]string{
            "error": "RBAC requires S3 storage", 
            "message": "RBAC is only available when using S3 storage. Please configure S3 storage to use RBAC features.",
        })
    }
    
    // Check if RBAC is already initialized
    enabled, err := h.manager.IsEnabled(c.Request().Context())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check RBAC status"})
    }
    
    if enabled {
        return c.JSON(http.StatusConflict, map[string]string{"error": "RBAC already initialized"})
    }
    
    // Initialize RBAC
    if err := h.manager.InitializeRBAC(c.Request().Context(), req.Subject, req.Email); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to initialize RBAC"})
    }
    
    return c.JSON(http.StatusOK, map[string]string{"message": "RBAC initialized successfully"})
}

// Me handles GET /v1/rbac/me
func (h *Handler) Me(c echo.Context) error {
    // Get user from token
    principal, err := h.getPrincipalFromToken(c)
    if err != nil {
        // Graceful fallback for token verification failures (like auth handler)
        return c.JSON(http.StatusOK, map[string]interface{}{
            "rbac_enabled": false,
            "subject":      "anonymous",
            "email":        "",
            "roles":        []string{},
            "message":      "Token verification failed - check authentication",
        })
    }
    
    // Check if RBAC is enabled
    enabled, err := h.manager.IsEnabled(c.Request().Context())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to check RBAC status"})
    }
    
    if !enabled {
        return c.JSON(http.StatusOK, map[string]interface{}{
            "rbac_enabled": false,
            "subject":      principal.Subject,
            "email":        principal.Email,
            "roles":        []string{},
            "message":      "RBAC not initialized",
        })
    }
    
    // Get user assignment
    assignment, err := h.manager.GetUserInfo(c.Request().Context(), principal.Subject)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get user info"})
    }
    
    if assignment == nil {
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
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    }
    
    if req.Email == "" || req.RoleID == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and role_id required"})
    }
    
    // Use email-based assignment if no subject provided
    if req.Subject == "" {
        if err := h.manager.AssignRoleByEmail(c.Request().Context(), req.Email, req.RoleID); err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to assign role: " + err.Error()})
        }
    } else {
        if err := h.manager.AssignRole(c.Request().Context(), req.Subject, req.Email, req.RoleID); err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to assign role"})
        }
    }
    
    return c.JSON(http.StatusOK, map[string]string{"message": "role assigned successfully"})
}

// RevokeRole handles POST /v1/rbac/users/revoke
func (h *Handler) RevokeRole(c echo.Context) error {
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
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    }
    
    if req.RoleID == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "role_id required"})
    }
    
    if req.Subject == "" && req.Email == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "either subject or email required"})
    }
    
    // Use email-based revocation if email provided, otherwise use subject
    if req.Email != "" {
        if err := h.manager.RevokeRoleByEmail(c.Request().Context(), req.Email, req.RoleID); err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to revoke role: " + err.Error()})
        }
    } else {
        if err := h.manager.RevokeRole(c.Request().Context(), req.Subject, req.RoleID); err != nil {
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to revoke role"})
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
    
    assignments, err := h.manager.ListUserAssignments(c.Request().Context())
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
        ID          string   `json:"id"`
        Name        string   `json:"name"`
        Description string   `json:"description"`
        Permissions []string `json:"permissions"`
    }
    
    if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
    }
    
    if req.ID == "" || req.Name == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "id and name required"})
    }
    
    // Get current user
    principal, err := h.getPrincipalFromToken(c)
    if err != nil {
        return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
    }
    
    role := &Role{
        ID:          req.ID,
        Name:        req.Name,
        Description: req.Description,
        Permissions: req.Permissions,
        CreatedBy:   principal.Subject,
    }
    
    if err := h.manager.CreateRole(c.Request().Context(), role); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create role"})
    }
    
    return c.JSON(http.StatusOK, map[string]string{"message": "role created successfully"})
}

// ListRoles handles GET /v1/rbac/roles
func (h *Handler) ListRoles(c echo.Context) error {
    // Check if user has RBAC manage permission
    if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
        return err
    }
    
    roles, err := h.manager.ListRoles(c.Request().Context())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list roles"})
    }
    
    return c.JSON(http.StatusOK, roles)
}

// DeleteRole handles DELETE /v1/rbac/roles/:id
func (h *Handler) DeleteRole(c echo.Context) error {
    // Check if user has RBAC manage permission
    if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
        return err
    }
    
    roleID := c.Param("id")
    if roleID == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "role id required"})
    }
    
    // Prevent deletion of default roles
    if roleID == "admin" || roleID == "default" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot delete default roles"})
    }
    
    // TODO: Implement role deletion in RBACManager
    return c.JSON(http.StatusNotImplemented, map[string]string{"error": "role deletion not yet implemented"})
}

// Helper functions

func (h *Handler) getPrincipalFromToken(c echo.Context) (Principal, error) {
    authz := c.Request().Header.Get("Authorization")
    if !strings.HasPrefix(authz, "Bearer ") {
        return Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
    }
    
    token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
    if h.signer == nil {
        return Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "auth not configured")
    }
    
    // Debug: check signer state  
    fmt.Printf("[RBAC DEBUG] Signer nil? %t\n", h.signer == nil)
    fmt.Printf("[RBAC DEBUG] Signer addr: %p\n", h.signer)
    
    claims, err := h.signer.VerifyAccess(token)
    if err != nil {
        // Debug: log the verification failure  
        fmt.Printf("[RBAC DEBUG] Token verification failed: %v\n", err)
        tokenPreview := token
        if len(token) > 50 {
            tokenPreview = token[:50] + "..."
        }
        fmt.Printf("[RBAC DEBUG] Token preview: %s\n", tokenPreview)
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
    
    can, err := h.manager.Can(c.Request().Context(), principal, action, resource)
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
        ID          string           `json:"id"`
        Name        string           `json:"name"`
        Description string           `json:"description"`
        Rules       []PermissionRule `json:"rules"`
    }
    
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
    }
    
    principal, err := h.getPrincipalFromToken(c)
    if err != nil {
        return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
    }
    
    permission := Permission{
        ID:          req.ID,
        Name:        req.Name,
        Description: req.Description,
        Rules:       req.Rules,
        CreatedBy:   principal.Subject,
    }
    
    if err := h.manager.CreatePermission(c.Request().Context(), &permission); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create permission"})
    }
    
    return c.JSON(http.StatusCreated, permission)
}

// ListPermissions handles GET /v1/rbac/permissions
func (h *Handler) ListPermissions(c echo.Context) error {
    // Check RBAC manage permission
    if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
        return err
    }
    
    permissions, err := h.manager.ListPermissions(c.Request().Context())
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list permissions"})
    }
    
    return c.JSON(http.StatusOK, permissions)
}

// DeletePermission handles DELETE /v1/rbac/permissions/:id
func (h *Handler) DeletePermission(c echo.Context) error {
    // Check RBAC manage permission
    if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
        return err
    }
    
    id := c.Param("id")
    
    if err := h.manager.DeletePermission(c.Request().Context(), id); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete permission"})
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
    
    if req.Email != "" {
        // Admin mode: test permissions for specified user (requires admin permission)
        if err := h.requireRBACPermission(c, ActionRBACManage, "*"); err != nil {
            return err
        }
        userAssignment, err = h.manager.store.GetUserAssignmentByEmail(c.Request().Context(), req.Email)
        if err != nil {
            return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
        }
    } else {
        // Self-check mode: test permissions for current authenticated user
        principal, err := h.getPrincipalFromToken(c)
        if err != nil {
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
        }
        
        userAssignment, err = h.manager.store.GetUserAssignment(c.Request().Context(), principal.Subject)
        if err != nil {
            return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
        }
    }
    
    // Get user's roles
    roles := userAssignment.Roles
    if len(roles) == 0 {
        return c.JSON(http.StatusOK, map[string]interface{}{
            "status":              "denied",
            "reason":              "user has no roles assigned",
            "user_roles":          []string{},
            "applicable_permissions": []string{},
        })
    }
    
    // Test permission for each role
    var applicablePermissions []string
    allowed := false
    
    for _, roleID := range roles {
        role, err := h.manager.store.GetRole(c.Request().Context(), roleID)
        if err != nil {
            continue // Skip invalid roles
        }
        
        // Check each permission in the role
        for _, permissionID := range role.Permissions {
            permission, err := h.manager.store.GetPermission(c.Request().Context(), permissionID)
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
                            "status":                "denied",
                            "reason":                fmt.Sprintf("explicitly denied by permission %s", permissionID),
                            "user_roles":            roles,
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
        "status":                status,
        "reason":                reason,
        "user_roles":            roles,
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
    
    const maxRetries = 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Get the role
        role, err := h.manager.store.GetRole(c.Request().Context(), roleID)
        if err != nil {
            return c.JSON(http.StatusNotFound, map[string]string{"error": "role not found"})
        }
        
        // Check if permission exists
        _, err = h.manager.store.GetPermission(c.Request().Context(), req.PermissionID)
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
    
    const maxRetries = 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Get the role
        role, err := h.manager.store.GetRole(c.Request().Context(), roleID)
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
