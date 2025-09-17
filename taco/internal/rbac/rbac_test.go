package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRBACStore implements RBACStore for testing
type mockRBACStore struct {
	config             *RBACConfig
	permissions        map[string]*Permission
	roles              map[string]*Role
	userAssignments    map[string]*UserAssignment
	userAssignmentsByEmail map[string]*UserAssignment
}

func newMockRBACStore() *mockRBACStore {
	return &mockRBACStore{
		permissions:           make(map[string]*Permission),
		roles:                 make(map[string]*Role),
		userAssignments:       make(map[string]*UserAssignment),
		userAssignmentsByEmail: make(map[string]*UserAssignment),
	}
}

func (m *mockRBACStore) GetConfig(ctx context.Context) (*RBACConfig, error) {
	return m.config, nil
}

func (m *mockRBACStore) SetConfig(ctx context.Context, config *RBACConfig) error {
	m.config = config
	return nil
}

func (m *mockRBACStore) CreatePermission(ctx context.Context, permission *Permission) error {
	m.permissions[permission.ID] = permission
	return nil
}

func (m *mockRBACStore) GetPermission(ctx context.Context, id string) (*Permission, error) {
	if permission, exists := m.permissions[id]; exists {
		return permission, nil
	}
	return nil, ErrNotFound
}

func (m *mockRBACStore) ListPermissions(ctx context.Context) ([]*Permission, error) {
	var permissions []*Permission
	for _, permission := range m.permissions {
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

func (m *mockRBACStore) DeletePermission(ctx context.Context, id string) error {
	delete(m.permissions, id)
	return nil
}

func (m *mockRBACStore) CreateRole(ctx context.Context, role *Role) error {
	m.roles[role.ID] = role
	return nil
}

func (m *mockRBACStore) GetRole(ctx context.Context, id string) (*Role, error) {
	if role, exists := m.roles[id]; exists {
		return role, nil
	}
	return nil, ErrNotFound
}

func (m *mockRBACStore) ListRoles(ctx context.Context) ([]*Role, error) {
	var roles []*Role
	for _, role := range m.roles {
		roles = append(roles, role)
	}
	return roles, nil
}

func (m *mockRBACStore) DeleteRole(ctx context.Context, id string) error {
	delete(m.roles, id)
	return nil
}

func (m *mockRBACStore) AssignRole(ctx context.Context, subject, email, roleID string) error {
	assignment := &UserAssignment{
		Subject:   subject,
		Email:     email,
		Roles:     []string{roleID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.userAssignments[subject] = assignment
	m.userAssignmentsByEmail[email] = assignment
	return nil
}

func (m *mockRBACStore) RevokeRole(ctx context.Context, subject, roleID string) error {
	if assignment, exists := m.userAssignments[subject]; exists {
		var newRoles []string
		for _, role := range assignment.Roles {
			if role != roleID {
				newRoles = append(newRoles, role)
			}
		}
		assignment.Roles = newRoles
		assignment.UpdatedAt = time.Now()
	}
	return nil
}

func (m *mockRBACStore) GetUserAssignment(ctx context.Context, subject string) (*UserAssignment, error) {
	if assignment, exists := m.userAssignments[subject]; exists {
		return assignment, nil
	}
	return nil, ErrNotFound
}

func (m *mockRBACStore) GetUserAssignmentByEmail(ctx context.Context, email string) (*UserAssignment, error) {
	if assignment, exists := m.userAssignmentsByEmail[email]; exists {
		return assignment, nil
	}
	return nil, ErrNotFound
}

func (m *mockRBACStore) ListUserAssignments(ctx context.Context) ([]*UserAssignment, error) {
	var assignments []*UserAssignment
	for _, assignment := range m.userAssignments {
		assignments = append(assignments, assignment)
	}
	return assignments, nil
}

func (m *mockRBACStore) AssignRoleByEmail(ctx context.Context, email, roleID string) error {
	// For testing, we'll create a mock subject
	subject := "test-subject-" + email
	return m.AssignRole(ctx, subject, email, roleID)
}

func (m *mockRBACStore) RevokeRoleByEmail(ctx context.Context, email, roleID string) error {
	if assignment, exists := m.userAssignmentsByEmail[email]; exists {
		return m.RevokeRole(ctx, assignment.Subject, roleID)
	}
	return ErrNotFound
}

func TestRBACManager_InitializeRBAC(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	subject := "test-subject"
	email := "test@example.com"

	err := manager.InitializeRBAC(context.Background(), subject, email)
	require.NoError(t, err)

	// Check that config was created
	config, err := store.GetConfig(context.Background())
	require.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.Equal(t, subject, config.InitUser)

	// Check that default permissions were created
	adminPermission, err := store.GetPermission(context.Background(), "admin")
	require.NoError(t, err)
	assert.Equal(t, "Admin Permission", adminPermission.Name)
	assert.Contains(t, adminPermission.Rules[0].Actions, ActionRBACManage)

	defaultPermission, err := store.GetPermission(context.Background(), "default")
	require.NoError(t, err)
	assert.Equal(t, "Default Permission", defaultPermission.Name)
	assert.Contains(t, defaultPermission.Rules[0].Actions, ActionUnitRead)

	// Check that default roles were created
	adminRole, err := store.GetRole(context.Background(), "admin")
	require.NoError(t, err)
	assert.Equal(t, "Admin Role", adminRole.Name)
	assert.Contains(t, adminRole.Permissions, "admin")

	defaultRole, err := store.GetRole(context.Background(), "default")
	require.NoError(t, err)
	assert.Equal(t, "Default Role", defaultRole.Name)
	assert.Contains(t, defaultRole.Permissions, "default")

	// Check that user was assigned roles
	assignment, err := store.GetUserAssignment(context.Background(), subject)
	require.NoError(t, err)
	assert.Equal(t, email, assignment.Email)
	assert.Contains(t, assignment.Roles, "admin")
	// Note: InitializeRBAC only assigns admin role, not default
}

func TestRBACManager_IsEnabled(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Initially disabled
	enabled, err := manager.IsEnabled(context.Background())
	require.NoError(t, err)
	assert.False(t, enabled)

	// Enable RBAC
	config := &RBACConfig{
		Enabled:     true,
		Initialized: true,
		InitUser:    "test-user",
		InitTime:    time.Now(),
	}
	err = store.SetConfig(context.Background(), config)
	require.NoError(t, err)

	enabled, err = manager.IsEnabled(context.Background())
	require.NoError(t, err)
	assert.True(t, enabled)
}

func TestRBACManager_Can(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Set up RBAC
	config := &RBACConfig{Enabled: true, Initialized: true}
	store.SetConfig(context.Background(), config)
	// Create a permission that allows state.read on dev/*
	permission := &Permission{
		ID:   "dev-access",
		Name: "Dev Access",
		Rules: []PermissionRule{
			{
                Actions:   []Action{ActionUnitRead, ActionUnitWrite},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
	}
	store.CreatePermission(context.Background(), permission)

	// Create a role with the permission
	role := &Role{
		ID:          "developer",
		Name:        "Developer",
		Permissions: []string{"dev-access"},
	}
	store.CreateRole(context.Background(), role)

	// Create a user with the role
	principal := Principal{
		Subject: "test-user",
		Email:   "test@example.com",
		Roles:   []string{"developer"},
	}

	// Assign the role to the user
	err := manager.AssignRole(context.Background(), principal.Subject, principal.Email, "developer")
	require.NoError(t, err)

	// Test allowed access
can, err := manager.Can(context.Background(), principal, ActionUnitRead, "dev/myapp")
	require.NoError(t, err)
	assert.True(t, can)

can, err = manager.Can(context.Background(), principal, ActionUnitWrite, "dev/myapp")
	require.NoError(t, err)
	assert.True(t, can)

	// Test denied access (different resource)
can, err = manager.Can(context.Background(), principal, ActionUnitRead, "prod/myapp")
	require.NoError(t, err)
	assert.False(t, can)

	// Test denied access (different action)
can, err = manager.Can(context.Background(), principal, ActionUnitDelete, "dev/myapp")
	require.NoError(t, err)
	assert.False(t, can)
}

func TestRBACManager_CanWithDenyRule(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Set up RBAC
	config := &RBACConfig{Enabled: true, Initialized: true}
	store.SetConfig(context.Background(), config)

	// Create a permission with both allow and deny rules
	permission := &Permission{
		ID:   "mixed-access",
		Name: "Mixed Access",
		Rules: []PermissionRule{
			{
                Actions:   []Action{ActionUnitRead, ActionUnitWrite},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
			{
                Actions:   []Action{ActionUnitDelete},
				Resources: []string{"dev/prod"},
				Effect:    "deny",
			},
		},
	}
	store.CreatePermission(context.Background(), permission)

	// Create a role with the permission
	role := &Role{
		ID:          "developer",
		Name:        "Developer",
		Permissions: []string{"mixed-access"},
	}
	store.CreateRole(context.Background(), role)

	// Create a user with the role
	principal := Principal{
		Subject: "test-user",
		Email:   "test@example.com",
		Roles:   []string{"developer"},
	}

	// Assign the role to the user
	err := manager.AssignRole(context.Background(), principal.Subject, principal.Email, "developer")
	require.NoError(t, err)

	// Test allowed access
can, err := manager.Can(context.Background(), principal, ActionUnitRead, "dev/myapp")
	require.NoError(t, err)
	assert.True(t, can)

	// Test denied access (explicit deny rule)
can, err = manager.Can(context.Background(), principal, ActionUnitDelete, "dev/prod")
	require.NoError(t, err)
	assert.False(t, can)
}

func TestRBACManager_CreateRole(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	role := &Role{
		ID:          "test-role",
		Name:        "Test Role",
		Description: "A test role",
		Permissions: []string{"permission1", "permission2"},
		CreatedBy:   "test-user",
	}

	err := manager.CreateRole(context.Background(), role)
	require.NoError(t, err)

	// Verify role was created
	createdRole, err := store.GetRole(context.Background(), "test-role")
	require.NoError(t, err)
	assert.Equal(t, "Test Role", createdRole.Name)
	assert.Equal(t, []string{"permission1", "permission2"}, createdRole.Permissions)
}

func TestRBACManager_AssignRole(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	subject := "test-user"
	email := "test@example.com"
	roleID := "developer"

	err := manager.AssignRole(context.Background(), subject, email, roleID)
	require.NoError(t, err)

	// Verify assignment was created
	assignment, err := store.GetUserAssignment(context.Background(), subject)
	require.NoError(t, err)
	assert.Equal(t, email, assignment.Email)
	assert.Contains(t, assignment.Roles, roleID)
}

func TestRBACManager_AssignRoleByEmail(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	email := "test@example.com"
	roleID := "developer"

	err := manager.AssignRoleByEmail(context.Background(), email, roleID)
	require.NoError(t, err)

	// Verify assignment was created
	assignment, err := store.GetUserAssignmentByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, email, assignment.Email)
	assert.Contains(t, assignment.Roles, roleID)
}

func TestRBACManager_RevokeRole(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	subject := "test-user"
	email := "test@example.com"
	roleID := "developer"

	// First assign the role
	err := manager.AssignRole(context.Background(), subject, email, roleID)
	require.NoError(t, err)

	// Then revoke it
	err = manager.RevokeRole(context.Background(), subject, roleID)
	require.NoError(t, err)

	// Verify role was revoked
	assignment, err := store.GetUserAssignment(context.Background(), subject)
	require.NoError(t, err)
	assert.NotContains(t, assignment.Roles, roleID)
}

func TestRBACManager_ListRoles(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Create some roles
	role1 := &Role{ID: "role1", Name: "Role 1"}
	role2 := &Role{ID: "role2", Name: "Role 2"}
	store.CreateRole(context.Background(), role1)
	store.CreateRole(context.Background(), role2)

	roles, err := manager.ListRoles(context.Background())
	require.NoError(t, err)
	assert.Len(t, roles, 2)

	roleIDs := make(map[string]bool)
	for _, role := range roles {
		roleIDs[role.ID] = true
	}
	assert.True(t, roleIDs["role1"])
	assert.True(t, roleIDs["role2"])
}

func TestRBACManager_ListPermissions(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Create some permissions
	permission1 := &Permission{ID: "permission1", Name: "Permission 1"}
	permission2 := &Permission{ID: "permission2", Name: "Permission 2"}
	store.CreatePermission(context.Background(), permission1)
	store.CreatePermission(context.Background(), permission2)

	permissions, err := manager.ListPermissions(context.Background())
	require.NoError(t, err)
	assert.Len(t, permissions, 2)

	permissionIDs := make(map[string]bool)
	for _, permission := range permissions {
		permissionIDs[permission.ID] = true
	}
	assert.True(t, permissionIDs["permission1"])
	assert.True(t, permissionIDs["permission2"])
}

func TestRBACManager_ListUserAssignments(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Create some assignments
	assignment1 := &UserAssignment{
		Subject: "user1",
		Email:   "user1@example.com",
		Roles:   []string{"role1"},
	}
	assignment2 := &UserAssignment{
		Subject: "user2",
		Email:   "user2@example.com",
		Roles:   []string{"role2"},
	}
	store.userAssignments["user1"] = assignment1
	store.userAssignments["user2"] = assignment2

	assignments, err := manager.ListUserAssignments(context.Background())
	require.NoError(t, err)
	assert.Len(t, assignments, 2)

	subjects := make(map[string]bool)
	for _, assignment := range assignments {
		subjects[assignment.Subject] = true
	}
	assert.True(t, subjects["user1"])
	assert.True(t, subjects["user2"])
}

func TestRBACManager_FilterUnitsByReadAccess(t *testing.T) {
	store := newMockRBACStore()
	manager := NewRBACManager(store)

	// Set up RBAC
	config := &RBACConfig{Enabled: true, Initialized: true}
	store.SetConfig(context.Background(), config)

	// Create a permission that allows read access to dev/*
	permission := &Permission{
		ID:   "dev-read",
		Name: "Dev Read",
		Rules: []PermissionRule{
			{
                Actions:   []Action{ActionUnitRead},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
	}
	store.CreatePermission(context.Background(), permission)

	// Create a role with the permission
	role := &Role{
		ID:          "developer",
		Name:        "Developer",
		Permissions: []string{"dev-read"},
	}
	store.CreateRole(context.Background(), role)

	// Create a user with the role
	principal := Principal{
		Subject: "test-user",
		Email:   "test@example.com",
		Roles:   []string{"developer"},
	}

	// Assign the role to the user
	err := manager.AssignRole(context.Background(), principal.Subject, principal.Email, "developer")
	require.NoError(t, err)

    // Test units
    units := []string{
        "dev/app1",
        "dev/app2",
        "prod/app1",
        "staging/app1",
    }

    filtered, err := manager.FilterUnitsByReadAccess(context.Background(), principal, units)
    require.NoError(t, err)

    // Should only include dev/* units
    assert.Len(t, filtered, 2)
    assert.Contains(t, filtered, "dev/app1")
    assert.Contains(t, filtered, "dev/app2")
    assert.NotContains(t, filtered, "prod/app1")
    assert.NotContains(t, filtered, "staging/app1")
}

func TestPermissionRule_Matches(t *testing.T) {
	rule := PermissionRule{
		Actions:   []Action{ActionUnitRead, ActionUnitWrite},
		Resources: []string{"dev/*", "staging/*"},
		Effect:    "allow",
	}

	// Test action matching
assert.True(t, rule.matches(ActionUnitRead, "dev/app"))
assert.True(t, rule.matches(ActionUnitWrite, "staging/app"))
assert.False(t, rule.matches(ActionUnitDelete, "dev/app"))

	// Test resource matching with wildcards
assert.True(t, rule.matches(ActionUnitRead, "dev/myapp"))
assert.True(t, rule.matches(ActionUnitRead, "staging/myapp"))
assert.False(t, rule.matches(ActionUnitRead, "prod/myapp"))

	// Test exact resource matching
	exactRule := PermissionRule{
		Actions:   []Action{ActionUnitRead},
		Resources: []string{"myapp/prod"},
		Effect:    "allow",
	}
assert.True(t, exactRule.matches(ActionUnitRead, "myapp/prod"))
assert.False(t, exactRule.matches(ActionUnitRead, "myapp/staging"))
}

func TestPermissionRule_MatchesWildcard(t *testing.T) {
	rule := PermissionRule{
		Actions:   []Action{ActionUnitRead},
		Resources: []string{"*"},
		Effect:    "allow",
	}

	// Should match any resource
assert.True(t, rule.matches(ActionUnitRead, "any/resource"))
assert.True(t, rule.matches(ActionUnitRead, "dev/app"))
assert.True(t, rule.matches(ActionUnitRead, "prod/app"))
}

func TestPermissionRule_MatchesActionWildcard(t *testing.T) {
	rule := PermissionRule{
		Actions:   []Action{"*"},
		Resources: []string{"dev/*"},
		Effect:    "allow",
	}

	// Should match any action on dev/* resources
assert.True(t, rule.matches(ActionUnitRead, "dev/app"))
assert.True(t, rule.matches(ActionUnitWrite, "dev/app"))
assert.True(t, rule.matches(ActionUnitDelete, "dev/app"))
assert.False(t, rule.matches(ActionUnitRead, "prod/app"))
}
