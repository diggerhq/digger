package rbac

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("not found")
	ErrVersionConflict = errors.New("version conflict - object was modified by another operation")
)

// Action represents a permissioned operation on a unit (tfstate workspace).
type Action string

const (
    ActionUnitRead    Action = "unit.read"
    ActionUnitWrite   Action = "unit.write"
    ActionUnitLock    Action = "unit.lock"
    ActionUnitDelete  Action = "unit.delete"
    ActionRBACManage  Action = "rbac.manage"
)

// Principal captures the caller identity and roles/groups.
type Principal struct {
	Subject string
	Email   string
	Roles   []string
	Groups  []string
}

// Permission defines what actions are allowed on what resources
type Permission struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	AbsoluteName string           `json:"absolute_name"`
	Description  string           `json:"description"`
	Rules        []PermissionRule `json:"rules"`
	CreatedAt    time.Time        `json:"created_at"`
	CreatedBy    string           `json:"created_by"`
	OrgID        string           `json:"org_id"`
}

// PermissionRule defines a single rule within a permission
type PermissionRule struct {
	Actions   []Action `json:"actions"`
	Resources []string `json:"resources"` // Can use wildcards like "myapp/*" or "*"
	Effect    string   `json:"effect"`    // "allow" or "deny"
}

// matches checks if this rule matches the given action and resource
func (r PermissionRule) matches(action Action, resource string) bool {
	// Check if action matches
	actionMatch := false
	for _, ruleAction := range r.Actions {
		if ruleAction == action || ruleAction == "*" {
			actionMatch = true
			break
		}
	}
	if !actionMatch {
		return false
	}

	// Check if resource matches
	resourceMatch := false
	for _, ruleResource := range r.Resources {
		if ruleResource == resource || ruleResource == "*" {
			resourceMatch = true
			break
		}
		// Check for wildcard patterns like "dev/*"
		if strings.Contains(ruleResource, "*") {
			pattern := strings.ReplaceAll(ruleResource, "*", ".*")
			if matched, _ := regexp.MatchString("^"+pattern+"$", resource); matched {
				resourceMatch = true
				break
			}
		}
	}

	return resourceMatch
}

// Role represents a collection of permissions
type Role struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	AbsoluteName string    `json:"absolute_name"`
	Description  string    `json:"description"`
	Permissions  []string  `json:"permissions"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
	Version      int64     `json:"version"`
	OrgID        string    `json:"org_id"`
}

// UserAssignment maps a user to their roles
type UserAssignment struct {
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int64     `json:"version"` // For optimistic locking
}

// RBACStore defines the interface for RBAC data storage.
// RBAC is considered "enabled" when permissions exist in the store.
// No separate config/flag storage is needed.
type RBACStore interface {
	// Permission management (org-scoped)
	CreatePermission(ctx context.Context, permission *Permission) error
	GetPermission(ctx context.Context, orgID, id string) (*Permission, error)
	ListPermissions(ctx context.Context, orgID string) ([]*Permission, error)
	DeletePermission(ctx context.Context, orgID, id string) error

	// Role management (org-scoped)
	CreateRole(ctx context.Context, role *Role) error
	GetRole(ctx context.Context, orgID, id string) (*Role, error)
	ListRoles(ctx context.Context, orgID string) ([]*Role, error)
	DeleteRole(ctx context.Context, orgID, id string) error

	// User assignment management (org-scoped)
	AssignRole(ctx context.Context, orgID, subject, email, roleID string) error
	AssignRoleByEmail(ctx context.Context, orgID, email, roleID string) error
	RevokeRole(ctx context.Context, orgID, subject, roleID string) error
	RevokeRoleByEmail(ctx context.Context, orgID, email, roleID string) error
	GetUserAssignment(ctx context.Context, orgID, subject string) (*UserAssignment, error)
	GetUserAssignmentByEmail(ctx context.Context, orgID, email string) (*UserAssignment, error)
	ListUserAssignments(ctx context.Context, orgID string) ([]*UserAssignment, error)
}

// RBACManager provides high-level RBAC operations
type RBACManager struct {
	store RBACStore
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager(store RBACStore) *RBACManager {
	return &RBACManager{store: store}
}

// NewRBACManagerFromQueryStore creates an RBAC manager from a query store.
// This constructor handles the type assertions needed to extract the database
// connection and create a proper RBAC store.
func NewRBACManagerFromQueryStore(queryStore interface{}) (*RBACManager, error) {
	// Type assert to get underlying DB connection
	type dbProvider interface {
		GetDB() *gorm.DB
	}
	
	sqlStore, ok := queryStore.(dbProvider)
	if !ok {
		return nil, fmt.Errorf("query store does not expose GetDB() method - RBAC requires database access")
	}

	// Create RBAC store that writes directly to database
	rbacStore := NewQueryRBACStore(sqlStore.GetDB())

	// Return the configured manager
	return NewRBACManager(rbacStore), nil
}

// InitializeRBAC sets up RBAC for the first time by creating default permissions and roles for an organization.
// RBAC is considered "initialized" and "enabled" when permissions exist in the system.
// This function is idempotent - it can be called multiple times safely.
func (m *RBACManager) InitializeRBAC(ctx context.Context, orgID, initUser, initEmail string) error {
	// For InitializeRBAC, we need explicit orgID since we're creating the org's RBAC structure
	// Check if already initialized for this org
	enabled, err := m.store.ListPermissions(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to check if RBAC is enabled: %w", err)
	}
	if len(enabled) > 0 {
		// Already initialized - just ensure the init user has admin role
		if err := m.store.AssignRole(ctx, orgID, initUser, initEmail, "admin"); err != nil {
			// Ignore duplicate assignment errors
			return nil
		}
		return nil
	}

	// Create default permissions (org-scoped)
	defaultPermission := &Permission{
		ID:          "default",
		OrgID:       orgID, // ✅ Set org ID
		Name:        "Default Permission",
		Description: "Default permission allowing read access to all states",
		Rules: []PermissionRule{
			{
				Actions:   []Action{ActionUnitRead},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: initUser,
	}

	adminPermission := &Permission{
		OrgID:       orgID, // ✅ Set org ID
		ID:          "admin",
		Name:        "Admin Permission",
		Description: "Admin permission allowing all actions on all resources",
		Rules: []PermissionRule{
			{
				Actions:   []Action{ActionUnitRead, ActionUnitWrite, ActionUnitLock, ActionUnitDelete, ActionRBACManage},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: initUser,
	}

	if err := m.store.CreatePermission(ctx, defaultPermission); err != nil {
		return fmt.Errorf("failed to create default permission: %w", err)
	}

	if err := m.store.CreatePermission(ctx, adminPermission); err != nil {
		return fmt.Errorf("failed to create admin permission: %w", err)
	}

	// Create default roles (org-scoped)
	defaultRole := &Role{
		ID:          "default",
		OrgID:       orgID, // ✅ Set org ID
		Name:        "Default Role",
		Description: "Default role with read access",
		Permissions: []string{"default"},
		CreatedAt:   time.Now(),
		CreatedBy:   initUser,
	}

	adminRole := &Role{
		ID:          "admin",
		OrgID:       orgID, // ✅ Set org ID
		Name:        "Admin Role",
		Description: "Admin role with full access",
		Permissions: []string{"admin"},
		CreatedAt:   time.Now(),
		CreatedBy:   initUser,
	}

	if err := m.store.CreateRole(ctx, defaultRole); err != nil {
		return fmt.Errorf("failed to create default role: %w", err)
	}

	if err := m.store.CreateRole(ctx, adminRole); err != nil {
		return fmt.Errorf("failed to create admin role: %w", err)
	}

	// Assign admin role to init user (org-scoped)
	if err := m.store.AssignRole(ctx, orgID, initUser, initEmail, "admin"); err != nil {
		return fmt.Errorf("failed to assign admin role to init user: %w", err)
	}

	return nil
}

// IsEnabled checks if RBAC is enabled for an organization by checking if permissions exist.
// RBAC is considered enabled if there are any permissions in the organization.
// This derives the state from actual data rather than maintaining a separate flag.
// NOTE: This requires database access. If the database is unavailable, this returns an error,
// causing RBAC checks to fail closed (deny all access) rather than fail open.
// The organization is extracted from the context.
func (m *RBACManager) IsEnabled(ctx context.Context) (bool, error) {
	// Extract org from context
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("organization context required for RBAC")
	}
	
	perms, err := m.store.ListPermissions(ctx, orgCtx.OrgID)
	if err != nil {
		return false, fmt.Errorf("RBAC database unavailable: %w", err)
	}
	return len(perms) > 0, nil
}

// Can determines whether a principal is authorized to perform an action on a given unit key.
// The organization is extracted from the context.
func (m *RBACManager) Can(ctx context.Context, principal Principal, action Action, resource string) (bool, error) {
	// Extract org from context
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return false, fmt.Errorf("organization context required for RBAC")
	}
	
	// Allow system principals (e.g., signed URLs, internal services)
	// System principals are pre-authorized and bypass RBAC checks
	if strings.HasPrefix(principal.Subject, "system:") {
		return true, nil
	}
	
	enabled, err := m.IsEnabled(ctx)
	if err != nil {
		return false, err
	}
	if !enabled {
		return true, nil // RBAC disabled, allow all
	}

	// Get user's roles (org-scoped)
	assignment, err := m.store.GetUserAssignment(ctx, orgCtx.OrgID, principal.Subject)
	if err != nil {
		return false, err
	}
	if assignment == nil {
		return false, nil // No roles assigned
	}

	// Check each role's permissions (org-scoped)
	for _, roleID := range assignment.Roles {
		role, err := m.store.GetRole(ctx, orgCtx.OrgID, roleID)
		if err != nil {
			continue // Skip invalid roles
		}

		for _, permissionID := range role.Permissions {
			permission, err := m.store.GetPermission(ctx, orgCtx.OrgID, permissionID)
			if err != nil {
				continue // Skip invalid permissions
			}

			if m.evaluatePermission(permission, action, resource) {
				return true, nil
			}
		}
	}

	return false, nil
}

// evaluatePermission checks if a permission allows the action on the resource
func (m *RBACManager) evaluatePermission(permission *Permission, action Action, resource string) bool {
	for _, rule := range permission.Rules {
		if m.ruleMatches(rule, action, resource) {
			return rule.Effect == "allow"
		}
	}
	return false
}

// ruleMatches checks if a rule matches the action and resource
func (m *RBACManager) ruleMatches(rule PermissionRule, action Action, resource string) bool {
	return rule.matches(action, resource)
}

// GetUserInfo returns user information including roles for the current organization.
// The organization is extracted from the context.
func (m *RBACManager) GetUserInfo(ctx context.Context, subject string) (*UserAssignment, error) {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("organization context required")
	}
	return m.store.GetUserAssignment(ctx, orgCtx.OrgID, subject)
}

// CreateRole creates a new role
func (m *RBACManager) CreateRole(ctx context.Context, role *Role) error {
	return m.store.CreateRole(ctx, role)
}

// AssignRole assigns a role to a user. The organization is extracted from context.
func (m *RBACManager) AssignRole(ctx context.Context, subject, email, roleID string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.AssignRole(ctx, orgCtx.OrgID, subject, email, roleID)
}

// AssignRoleByEmail assigns a role to a user by email. The organization is extracted from context.
func (m *RBACManager) AssignRoleByEmail(ctx context.Context, email, roleID string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.AssignRoleByEmail(ctx, orgCtx.OrgID, email, roleID)
}

// RevokeRole revokes a role from a user. The organization is extracted from context.
func (m *RBACManager) RevokeRole(ctx context.Context, subject, roleID string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.RevokeRole(ctx, orgCtx.OrgID, subject, roleID)
}

// RevokeRoleByEmail revokes a role from a user by email. The organization is extracted from context.
func (m *RBACManager) RevokeRoleByEmail(ctx context.Context, email, roleID string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.RevokeRoleByEmail(ctx, orgCtx.OrgID, email, roleID)
}

// ListRoles returns all roles for the current organization (from context).
func (m *RBACManager) ListRoles(ctx context.Context) ([]*Role, error) {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("organization context required")
	}
	return m.store.ListRoles(ctx, orgCtx.OrgID)
}

// ListPermissions returns all permissions for the current organization (from context).
func (m *RBACManager) ListPermissions(ctx context.Context) ([]*Permission, error) {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("organization context required")
	}
	return m.store.ListPermissions(ctx, orgCtx.OrgID)
}

// ListUserAssignments returns all user assignments for the current organization (from context).
func (m *RBACManager) ListUserAssignments(ctx context.Context) ([]*UserAssignment, error) {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("organization context required")
	}
	return m.store.ListUserAssignments(ctx, orgCtx.OrgID)
}

// FilterUnitsByReadAccess filters a list of units based on read permissions.
// The organization is extracted from the context.
func (m *RBACManager) FilterUnitsByReadAccess(ctx context.Context, principal Principal, units []string) ([]string, error) {
	enabled, err := m.IsEnabled(ctx)
	if err != nil {
		return nil, err
	}
    if !enabled {
        return units, nil // RBAC disabled, return all units
    }

    var filtered []string
    for _, unit := range units {
        canRead, err := m.Can(ctx, principal, ActionUnitRead, unit)
        if err != nil {
            continue // Skip on error
        }
        if canRead { filtered = append(filtered, unit) }
    }

    return filtered, nil
}

// CreatePermission creates a new permission
func (m *RBACManager) CreatePermission(ctx context.Context, permission *Permission) error {
	return m.store.CreatePermission(ctx, permission)
}



// DeletePermission deletes a permission. The organization is extracted from context.
func (m *RBACManager) DeletePermission(ctx context.Context, id string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.DeletePermission(ctx, orgCtx.OrgID, id)
}

// DeleteRole deletes a role. The organization is extracted from context.
func (m *RBACManager) DeleteRole(ctx context.Context, id string) error {
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return fmt.Errorf("organization context required")
	}
	return m.store.DeleteRole(ctx, orgCtx.OrgID, id)
}
