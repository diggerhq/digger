package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueryStore tests the SQLite query backend without the authorization layer
func TestQueryStore(t *testing.T) {
	t.Run("basic unit operations", func(t *testing.T) {
		testBasicUnitOperations(t)
	})

	t.Run("RBAC query methods", func(t *testing.T) {
		testRBACQueries(t)
	})

	t.Run("permission syncing", func(t *testing.T) {
		testPermissionSync(t)
	})

	t.Run("role syncing", func(t *testing.T) {
		testRoleSync(t)
	})

	t.Run("user syncing", func(t *testing.T) {
		testUserSync(t)
	})

	t.Run("list units for user", func(t *testing.T) {
		testListUnitsForUser(t)
	})

	t.Run("can perform action queries", func(t *testing.T) {
		testCanPerformAction(t)
	})

	t.Run("pattern matching in queries", func(t *testing.T) {
		testPatternMatching(t)
	})

	t.Run("filter unit IDs by user", func(t *testing.T) {
		testFilterUnitIDsByUser(t)
	})

	t.Run("has RBAC roles check", func(t *testing.T) {
		testHasRBACRoles(t)
	})

	t.Run("unit locking operations", func(t *testing.T) {
		testUnitLockingOps(t)
	})

	t.Run("view creation and querying", func(t *testing.T) {
		testViewCreation(t)
	})
}

func setupQueryStore(t *testing.T) (query.Store, string, func()) {
	tempDir, err := os.MkdirTemp("", "query-test-*")
	require.NoError(t, err)

	cfg := query.SQLiteConfig{
		Path:              filepath.Join(tempDir, "test.db"),
		Cache:             "shared",
		BusyTimeout:       5 * time.Second,
		MaxOpenConns:      1,
		MaxIdleConns:      1,
		PragmaJournalMode: "WAL",
		PragmaForeignKeys: "ON",
		PragmaBusyTimeout: "5000",
	}

	store, err := NewSQLiteQueryStore(cfg)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, tempDir, cleanup
}

func testBasicUnitOperations(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Ensure unit exists
	err := store.SyncEnsureUnit(ctx, "test-unit-1")
	require.NoError(t, err)

	// Test: Get unit
	unit, err := store.GetUnit(ctx, "test-unit-1")
	require.NoError(t, err)
	assert.Equal(t, "test-unit-1", unit.Name)

	// Test: Update metadata
	now := time.Now()
	err = store.SyncUnitMetadata(ctx, "test-unit-1", 1024, now)
	require.NoError(t, err)

	unit, err = store.GetUnit(ctx, "test-unit-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1024), unit.Size)

	// Test: List units
	units, err := store.ListUnits(ctx, "")
	require.NoError(t, err)
	assert.Len(t, units, 1)
	assert.Equal(t, "test-unit-1", units[0].Name)

	// Test: List with prefix
	err = store.SyncEnsureUnit(ctx, "test-unit-2")
	require.NoError(t, err)
	err = store.SyncEnsureUnit(ctx, "other-unit-1")
	require.NoError(t, err)

	units, err = store.ListUnits(ctx, "test-")
	require.NoError(t, err)
	assert.Len(t, units, 2)

	// Test: Delete unit
	err = store.SyncDeleteUnit(ctx, "test-unit-1")
	require.NoError(t, err)

	_, err = store.GetUnit(ctx, "test-unit-1")
	assert.Error(t, err)
}

func testRBACQueries(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should have no RBAC roles
	hasRoles, err := store.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.False(t, hasRoles)

	// Create a permission
	perm := &rbac.Permission{
		ID:          "test-perm",
		Name:        "Test Permission",
		Description: "Test",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "test",
	}
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	// Create a role
	role := &rbac.Role{
		ID:          "test-role",
		Name:        "Test Role",
		Permissions: []string{"test-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "test",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Now should have RBAC roles
	hasRoles, err = store.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.True(t, hasRoles)
}

func testPermissionSync(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create permission with multiple rules
	perm := &rbac.Permission{
		ID:          "multi-rule-perm",
		Name:        "Multi Rule Permission",
		Description: "Permission with multiple rules",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
			{
				Actions:   []rbac.Action{rbac.ActionUnitWrite},
				Resources: []string{"dev/app1"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}

	err := store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	// Verify it was created - we'll test this through role assignment
	// since we don't have direct permission query methods
	role := &rbac.Role{
		ID:          "test-role",
		Name:        "Test Role",
		Permissions: []string{"multi-rule-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Re-sync same permission (test idempotency)
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)
}

func testRoleSync(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create permissions first
	perm1 := &rbac.Permission{
		ID:          "perm1",
		Name:        "Permission 1",
		Description: "First permission",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := store.SyncPermission(ctx, perm1)
	require.NoError(t, err)

	perm2 := &rbac.Permission{
		ID:          "perm2",
		Name:        "Permission 2",
		Description: "Second permission",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitWrite},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = store.SyncPermission(ctx, perm2)
	require.NoError(t, err)

	// Create role with multiple permissions
	role := &rbac.Role{
		ID:          "multi-perm-role",
		Name:        "Multi Permission Role",
		Permissions: []string{"perm1", "perm2"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Re-sync same role (test idempotency)
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Update role with different permissions
	role.Permissions = []string{"perm1"}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)
}

func testUserSync(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create role first
	perm := &rbac.Permission{
		ID:          "user-perm",
		Name:        "User Permission",
		Description: "Permission for user test",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	role := &rbac.Role{
		ID:          "user-role",
		Name:        "User Role",
		Permissions: []string{"user-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user := &rbac.UserAssignment{
		Subject:   "user1@example.com",
		Email:     "user1@example.com",
		Roles:     []string{"user-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Re-sync same user (test idempotency)
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Update user with multiple roles
	role2 := &rbac.Role{
		ID:          "user-role-2",
		Name:        "User Role 2",
		Permissions: []string{"user-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role2)
	require.NoError(t, err)

	user.Roles = []string{"user-role", "user-role-2"}
	user.Version = 2
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)
}

func testListUnitsForUser(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create units
	err := store.SyncEnsureUnit(ctx, "dev/app1")
	require.NoError(t, err)
	err = store.SyncEnsureUnit(ctx, "dev/app2")
	require.NoError(t, err)
	err = store.SyncEnsureUnit(ctx, "prod/app1")
	require.NoError(t, err)

	// Create permission for dev/* access
	perm := &rbac.Permission{
		ID:          "dev-access",
		Name:        "Dev Access",
		Description: "Access to dev environment",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	// Create role
	role := &rbac.Role{
		ID:          "developer",
		Name:        "Developer",
		Permissions: []string{"dev-access"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user := &rbac.UserAssignment{
		Subject:   "dev@example.com",
		Email:     "dev@example.com",
		Roles:     []string{"developer"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// List units for user - should only see dev/* units
	units, err := store.ListUnitsForUser(ctx, "dev@example.com", "")
	require.NoError(t, err)
	assert.Len(t, units, 2)
	
	// Verify only dev units are returned
	unitNames := make([]string, len(units))
	for i, u := range units {
		unitNames[i] = u.Name
	}
	assert.Contains(t, unitNames, "dev/app1")
	assert.Contains(t, unitNames, "dev/app2")
	assert.NotContains(t, unitNames, "prod/app1")

	// List with prefix filter
	units, err = store.ListUnitsForUser(ctx, "dev@example.com", "dev/app1")
	require.NoError(t, err)
	assert.Len(t, units, 1)
	assert.Equal(t, "dev/app1", units[0].Name)
}

func testCanPerformAction(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create unit
	err := store.SyncEnsureUnit(ctx, "test/unit1")
	require.NoError(t, err)

	// Create permission with specific actions
	perm := &rbac.Permission{
		ID:          "rw-permission",
		Name:        "Read Write Permission",
		Description: "Can read and write",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite},
				Resources: []string{"test/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	// Create role
	role := &rbac.Role{
		ID:          "rw-role",
		Name:        "Read Write Role",
		Permissions: []string{"rw-permission"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Create user
	user := &rbac.UserAssignment{
		Subject:   "testuser@example.com",
		Email:     "testuser@example.com",
		Roles:     []string{"rw-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Test: User can read
	canRead, err := store.CanPerformAction(ctx, "testuser@example.com", "unit.read", "test/unit1")
	require.NoError(t, err)
	assert.True(t, canRead)

	// Test: User can write
	canWrite, err := store.CanPerformAction(ctx, "testuser@example.com", "unit.write", "test/unit1")
	require.NoError(t, err)
	assert.True(t, canWrite)

	// Test: User cannot delete (not granted)
	canDelete, err := store.CanPerformAction(ctx, "testuser@example.com", "unit.delete", "test/unit1")
	require.NoError(t, err)
	assert.False(t, canDelete)

	// Test: User cannot access different resource
	canReadOther, err := store.CanPerformAction(ctx, "testuser@example.com", "unit.read", "other/unit")
	require.NoError(t, err)
	assert.False(t, canReadOther)
}

func testPatternMatching(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create units in different namespaces
	units := []string{
		"dev/app1",
		"dev/app2",
		"dev/service/api",
		"prod/app1",
		"staging/app1",
	}
	for _, unitName := range units {
		err := store.SyncEnsureUnit(ctx, unitName)
		require.NoError(t, err)
	}

	// Create permission with pattern
	perm := &rbac.Permission{
		ID:          "pattern-perm",
		Name:        "Pattern Permission",
		Description: "Uses pattern matching",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	role := &rbac.Role{
		ID:          "pattern-role",
		Name:        "Pattern Role",
		Permissions: []string{"pattern-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	user := &rbac.UserAssignment{
		Subject:   "pattern@example.com",
		Email:     "pattern@example.com",
		Roles:     []string{"pattern-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Test: Pattern matches dev/* units
	testCases := []struct {
		unit     string
		expected bool
	}{
		{"dev/app1", true},
		{"dev/app2", true},
		{"dev/service/api", true},
		{"prod/app1", false},
		{"staging/app1", false},
	}

	for _, tc := range testCases {
		t.Run(tc.unit, func(t *testing.T) {
			canRead, err := store.CanPerformAction(ctx, "pattern@example.com", "unit.read", tc.unit)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, canRead, "Pattern matching failed for %s", tc.unit)
		})
	}
}

func testFilterUnitIDsByUser(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create units
	units := []string{"unit1", "unit2", "unit3", "unit4"}
	for _, unitName := range units {
		err := store.SyncEnsureUnit(ctx, unitName)
		require.NoError(t, err)
	}

	// Create permission for unit1 and unit2
	perm := &rbac.Permission{
		ID:          "limited-perm",
		Name:        "Limited Permission",
		Description: "Access to specific units",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"unit1", "unit2"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	role := &rbac.Role{
		ID:          "limited-role",
		Name:        "Limited Role",
		Permissions: []string{"limited-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	user := &rbac.UserAssignment{
		Subject:   "limited@example.com",
		Email:     "limited@example.com",
		Roles:     []string{"limited-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Filter units - should only return unit1 and unit2
	allowedUnits, err := store.FilterUnitIDsByUser(ctx, "limited@example.com", units)
	require.NoError(t, err)
	assert.Len(t, allowedUnits, 2)
	assert.Contains(t, allowedUnits, "unit1")
	assert.Contains(t, allowedUnits, "unit2")
	assert.NotContains(t, allowedUnits, "unit3")
	assert.NotContains(t, allowedUnits, "unit4")

	// Test with empty input
	allowedUnits, err = store.FilterUnitIDsByUser(ctx, "limited@example.com", []string{})
	require.NoError(t, err)
	assert.Len(t, allowedUnits, 0)

	// Test with non-existent units
	allowedUnits, err = store.FilterUnitIDsByUser(ctx, "limited@example.com", []string{"nonexistent"})
	require.NoError(t, err)
	assert.Len(t, allowedUnits, 0)
}

func testHasRBACRoles(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should have no roles
	hasRoles, err := store.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.False(t, hasRoles)

	// Create a permission
	perm := &rbac.Permission{
		ID:          "test-perm",
		Name:        "Test Permission",
		Description: "Test",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	// Still no roles
	hasRoles, err = store.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.False(t, hasRoles)

	// Create a role
	role := &rbac.Role{
		ID:          "test-role",
		Name:        "Test Role",
		Permissions: []string{"test-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	// Now should have roles
	hasRoles, err = store.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.True(t, hasRoles)
}

func testUnitLockingOps(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create unit
	err := store.SyncEnsureUnit(ctx, "lockable-unit")
	require.NoError(t, err)

	// Test: Lock unit
	lockTime := time.Now()
	err = store.SyncUnitLock(ctx, "lockable-unit", "lock-123", "testuser", lockTime)
	require.NoError(t, err)

	// Verify unit is locked
	unit, err := store.GetUnit(ctx, "lockable-unit")
	require.NoError(t, err)
	assert.True(t, unit.Locked)
	assert.Equal(t, "lock-123", unit.LockID)
	assert.Equal(t, "testuser", unit.LockWho)
	assert.NotNil(t, unit.LockCreated)

	// Test: Unlock unit
	err = store.SyncUnitUnlock(ctx, "lockable-unit")
	require.NoError(t, err)

	// Verify unit is unlocked
	unit, err = store.GetUnit(ctx, "lockable-unit")
	require.NoError(t, err)
	assert.False(t, unit.Locked)
	assert.Equal(t, "", unit.LockID)
	assert.Equal(t, "", unit.LockWho)
}

func testViewCreation(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create test data
	err := store.SyncEnsureUnit(ctx, "view-test-1")
	require.NoError(t, err)
	err = store.SyncEnsureUnit(ctx, "view-test-2")
	require.NoError(t, err)

	// Create permission
	perm := &rbac.Permission{
		ID:          "view-perm",
		Name:        "View Permission",
		Description: "Test view",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"view-test-1"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	role := &rbac.Role{
		ID:          "view-role",
		Name:        "View Role",
		Permissions: []string{"view-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	user := &rbac.UserAssignment{
		Subject:   "view@example.com",
		Email:     "view@example.com",
		Roles:     []string{"view-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Query through the view (ListUnitsForUser uses the view)
	units, err := store.ListUnitsForUser(ctx, "view@example.com", "")
	require.NoError(t, err)
	
	// Should only see view-test-1
	assert.Len(t, units, 1)
	assert.Equal(t, "view-test-1", units[0].Name)
}

// TestQueryStoreConcurrency tests concurrent read access to the query store
// Note: SQLite with MaxOpenConns=1 doesn't handle concurrent writes well,
// but concurrent reads are safe and this is the typical use case for RBAC queries
func TestQueryStoreConcurrency(t *testing.T) {
	store, _, cleanup := setupQueryStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial data
	for i := 0; i < 10; i++ {
		err := store.SyncEnsureUnit(ctx, fmt.Sprintf("concurrent-unit-%d", i))
		require.NoError(t, err)
	}

	// Create RBAC data for testing
	perm := &rbac.Permission{
		ID:          "concurrent-perm",
		Name:        "Concurrent Permission",
		Description: "Test concurrent access",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"concurrent-*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := store.SyncPermission(ctx, perm)
	require.NoError(t, err)

	role := &rbac.Role{
		ID:          "concurrent-role",
		Name:        "Concurrent Role",
		Permissions: []string{"concurrent-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = store.SyncRole(ctx, role)
	require.NoError(t, err)

	user := &rbac.UserAssignment{
		Subject:   "concurrent@example.com",
		Email:     "concurrent@example.com",
		Roles:     []string{"concurrent-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = store.SyncUser(ctx, user)
	require.NoError(t, err)

	// Run concurrent READ queries (safe with SQLite)
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			
			// List units
			_, err := store.ListUnits(ctx, "concurrent-")
			assert.NoError(t, err)
			
			// Get specific unit
			unitIdx := idx % 10
			_, err = store.GetUnit(ctx, fmt.Sprintf("concurrent-unit-%d", unitIdx))
			assert.NoError(t, err)
			
			// Check permissions (read query)
			_, err = store.CanPerformAction(ctx, "concurrent@example.com", "unit.read", fmt.Sprintf("concurrent-unit-%d", unitIdx))
			assert.NoError(t, err)
			
			// List units for user (read query)
			_, err = store.ListUnitsForUser(ctx, "concurrent@example.com", "concurrent-")
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify all units still exist
	// Note: ListUnits returns ALL units including pattern units like "concurrent-*"
	// In production, pattern units are metadata and filtered by the view in ListUnitsForUser
	units, err := store.ListUnits(ctx, "concurrent-")
	require.NoError(t, err)
	
	// Filter out pattern units (those containing '*')
	actualUnits := make([]string, 0)
	for _, u := range units {
		if !strings.Contains(u.Name, "*") {
			actualUnits = append(actualUnits, u.Name)
		}
	}
	assert.Len(t, actualUnits, 10, "Should have 10 actual units (excluding pattern metadata)")
}

