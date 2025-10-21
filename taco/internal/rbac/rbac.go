package rbac

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	
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
	// Permission management
	CreatePermission(ctx context.Context, permission *Permission) error
	GetPermission(ctx context.Context, id string) (*Permission, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
	DeletePermission(ctx context.Context, id string) error

	// Role management
	CreateRole(ctx context.Context, role *Role) error
	GetRole(ctx context.Context, id string) (*Role, error)
	ListRoles(ctx context.Context) ([]*Role, error)
	DeleteRole(ctx context.Context, id string) error

	// User assignment management
	AssignRole(ctx context.Context, subject, email, roleID string) error
	AssignRoleByEmail(ctx context.Context, email, roleID string) error
	RevokeRole(ctx context.Context, subject, roleID string) error
	RevokeRoleByEmail(ctx context.Context, email, roleID string) error
	GetUserAssignment(ctx context.Context, subject string) (*UserAssignment, error)
	GetUserAssignmentByEmail(ctx context.Context, email string) (*UserAssignment, error)
	ListUserAssignments(ctx context.Context) ([]*UserAssignment, error)
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

// InitializeRBAC sets up RBAC for the first time by creating default permissions and roles.
// RBAC is considered "initialized" and "enabled" when permissions exist in the system.
// This function is idempotent - it can be called multiple times safely.
func (m *RBACManager) InitializeRBAC(ctx context.Context, initUser, initEmail string) error {
	// Check if already initialized
	enabled, err := m.IsEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if RBAC is enabled: %w", err)
	}
	if enabled {
		// Already initialized - just ensure the init user has admin role
		if err := m.store.AssignRole(ctx, initUser, initEmail, "admin"); err != nil {
			// Ignore duplicate assignment errors
			return nil
		}
		return nil
	}

	// Create default permissions
	defaultPermission := &Permission{
		ID:          "default",
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

	// Create default roles
	defaultRole := &Role{
		ID:          "default",
		Name:        "Default Role",
		Description: "Default role with read access",
		Permissions: []string{"default"},
		CreatedAt:   time.Now(),
		CreatedBy:   initUser,
	}

	adminRole := &Role{
		ID:          "admin",
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

	// Assign admin role to init user
	if err := m.store.AssignRole(ctx, initUser, initEmail, "admin"); err != nil {
		return fmt.Errorf("failed to assign admin role to init user: %w", err)
	}

	return nil
}

// IsEnabled checks if RBAC is enabled by checking if permissions exist.
// RBAC is considered enabled if there are any permissions in the system.
// This derives the state from actual data rather than maintaining a separate flag.
// NOTE: This requires database access. If the database is unavailable, this returns an error,
// causing RBAC checks to fail closed (deny all access) rather than fail open.
func (m *RBACManager) IsEnabled(ctx context.Context) (bool, error) {
	perms, err := m.store.ListPermissions(ctx)
	if err != nil {
		return false, fmt.Errorf("RBAC database unavailable: %w", err)
	}
	return len(perms) > 0, nil
}

// Can determines whether a principal is authorized to perform an action on a given unit key.
func (m *RBACManager) Can(ctx context.Context, principal Principal, action Action, resource string) (bool, error) {
	enabled, err := m.IsEnabled(ctx)
	if err != nil {
		return false, err
	}
	if !enabled {
		return true, nil // RBAC disabled, allow all
	}

	// Get user's roles
	assignment, err := m.store.GetUserAssignment(ctx, principal.Subject)
	if err != nil {
		return false, err
	}
	if assignment == nil {
		return false, nil // No roles assigned
	}

	// Check each role's permissions
	for _, roleID := range assignment.Roles {
		role, err := m.store.GetRole(ctx, roleID)
		if err != nil {
			continue // Skip invalid roles
		}

		for _, permissionID := range role.Permissions {
			permission, err := m.store.GetPermission(ctx, permissionID)
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

// GetUserInfo returns user information including roles
func (m *RBACManager) GetUserInfo(ctx context.Context, subject string) (*UserAssignment, error) {
	return m.store.GetUserAssignment(ctx, subject)
}

// CreateRole creates a new role
func (m *RBACManager) CreateRole(ctx context.Context, role *Role) error {
	return m.store.CreateRole(ctx, role)
}

// AssignRole assigns a role to a user
func (m *RBACManager) AssignRole(ctx context.Context, subject, email, roleID string) error {
	return m.store.AssignRole(ctx, subject, email, roleID)
}

// AssignRoleByEmail assigns a role to a user by email (looks up subject)
func (m *RBACManager) AssignRoleByEmail(ctx context.Context, email, roleID string) error {
	return m.store.AssignRoleByEmail(ctx, email, roleID)
}

// RevokeRole revokes a role from a user
func (m *RBACManager) RevokeRole(ctx context.Context, subject, roleID string) error {
	return m.store.RevokeRole(ctx, subject, roleID)
}

// RevokeRoleByEmail revokes a role from a user by email (looks up subject)
func (m *RBACManager) RevokeRoleByEmail(ctx context.Context, email, roleID string) error {
	return m.store.RevokeRoleByEmail(ctx, email, roleID)
}

// ListRoles returns all roles
func (m *RBACManager) ListRoles(ctx context.Context) ([]*Role, error) {
	return m.store.ListRoles(ctx)
}

// ListPermissions returns all permissions
func (m *RBACManager) ListPermissions(ctx context.Context) ([]*Permission, error) {
	return m.store.ListPermissions(ctx)
}

// ListUserAssignments returns all user assignments
func (m *RBACManager) ListUserAssignments(ctx context.Context) ([]*UserAssignment, error) {
	return m.store.ListUserAssignments(ctx)
}

// FilterUnitsByReadAccess filters a list of units based on read permissions
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



// DeletePermission deletes a permission
func (m *RBACManager) DeletePermission(ctx context.Context, id string) error {
	return m.store.DeletePermission(ctx, id)
}
