package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersioningAndManagement tests version operations and RBAC management
func TestVersioningAndManagement(t *testing.T) {
	t.Run("version operations with RBAC", func(t *testing.T) {
		testVersionOperationsWithRBAC(t)
	})

	t.Run("list versions requires read permission", func(t *testing.T) {
		testListVersionsRequiresReadPermission(t)
	})

	t.Run("restore version requires write permission", func(t *testing.T) {
		testRestoreVersionRequiresWritePermission(t)
	})

	t.Run("version operations with pattern permissions", func(t *testing.T) {
		testVersionOperationsWithPatternPermissions(t)
	})

	t.Run("version operations respect locks", func(t *testing.T) {
		testVersionOperationsRespectLocks(t)
	})

	t.Run("rbac.manage permission enforcement", func(t *testing.T) {
		testRBACManagePermissionEnforcement(t)
	})

	t.Run("admin role includes rbac.manage", func(t *testing.T) {
		testAdminRoleIncludesRBACManage(t)
	})

	t.Run("non-admin cannot modify RBAC", func(t *testing.T) {
		testNonAdminCannotModifyRBAC(t)
	})

	t.Run("pattern matching edge cases", func(t *testing.T) {
		testPatternMatchingEdgeCases(t)
	})
}

func testVersionOperationsWithRBAC(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create a unit with multiple versions
	unitID := "versioning-test/app1"
	
	// Create initial version
	_, err := env.blobStore.Create(ctx, unitID)
	require.NoError(t, err)
	
	version1Data := []byte(`{"version": 4, "serial": 1}`)
	err = env.blobStore.Upload(ctx, unitID, version1Data, "")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond) // Ensure different timestamps
	
	// Create second version
	version2Data := []byte(`{"version": 4, "serial": 2}`)
	err = env.blobStore.Upload(ctx, unitID, version2Data, "")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	
	// Create third version
	version3Data := []byte(`{"version": 4, "serial": 3}`)
	err = env.blobStore.Upload(ctx, unitID, version3Data, "")
	require.NoError(t, err)

	// Setup RBAC: User with read permission
	setupUserWithPermission(t, env.queryStore, "reader@example.com", "versioning-test/*", 
		[]rbac.Action{rbac.ActionUnitRead})
	
	// Test: User can list versions
	readerCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "reader@example.com",
		Email:   "reader@example.com",
	})
	
	versions, err := env.authRepo.ListVersions(readerCtx, unitID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 2, "Should have at least 2 versions")
	
	// Verify versions are sorted by timestamp (newest first)
	for i := 1; i < len(versions); i++ {
		assert.True(t, versions[i-1].Timestamp.After(versions[i].Timestamp) || 
			versions[i-1].Timestamp.Equal(versions[i].Timestamp),
			"Versions should be sorted newest first")
	}
}

func testListVersionsRequiresReadPermission(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()
	unitID := "read-test/app1"

	// Create unit with version
	_, err := env.blobStore.Create(ctx, unitID)
	require.NoError(t, err)
	err = env.blobStore.Upload(ctx, unitID, []byte(`{"version": 4}`), "")
	require.NoError(t, err)
	err = env.blobStore.Upload(ctx, unitID, []byte(`{"version": 4, "updated": true}`), "")
	require.NoError(t, err)

	// Setup: User WITHOUT read permission
	setupUserWithPermission(t, env.queryStore, "noread@example.com", "other/*", 
		[]rbac.Action{rbac.ActionUnitWrite})

	noReadCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "noread@example.com",
		Email:   "noread@example.com",
	})

	// Test: Should get forbidden error
	_, err = env.authRepo.ListVersions(noReadCtx, unitID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// Setup: User WITH read permission
	setupUserWithPermission(t, env.queryStore, "reader@example.com", "read-test/*", 
		[]rbac.Action{rbac.ActionUnitRead})

	readerCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "reader@example.com",
		Email:   "reader@example.com",
	})

	// Test: Should succeed
	versions, err := env.authRepo.ListVersions(readerCtx, unitID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 1)
}

func testRestoreVersionRequiresWritePermission(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()
	unitID := "restore-test/app1"

	// Create unit with versions
	_, err := env.blobStore.Create(ctx, unitID)
	require.NoError(t, err)
	
	version1 := []byte(`{"version": 4, "serial": 1}`)
	err = env.blobStore.Upload(ctx, unitID, version1, "")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	
	version2 := []byte(`{"version": 4, "serial": 2}`)
	err = env.blobStore.Upload(ctx, unitID, version2, "")
	require.NoError(t, err)

	// Get version timestamp to restore to
	versions, err := env.blobStore.ListVersions(ctx, unitID)
	require.NoError(t, err)
	require.Greater(t, len(versions), 0)
	oldVersion := versions[len(versions)-1]

	// Setup: User with read-only permission
	setupUserWithPermission(t, env.queryStore, "readonly@example.com", "restore-test/*", 
		[]rbac.Action{rbac.ActionUnitRead})

	readOnlyCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "readonly@example.com",
		Email:   "readonly@example.com",
	})

	// Test: Should get forbidden error
	err = env.authRepo.RestoreVersion(readOnlyCtx, unitID, oldVersion.Timestamp, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// Setup: User with write permission
	setupUserWithPermission(t, env.queryStore, "writer@example.com", "restore-test/*", 
		[]rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite})

	writerCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "writer@example.com",
		Email:   "writer@example.com",
	})

	// Test: Should succeed
	err = env.authRepo.RestoreVersion(writerCtx, unitID, oldVersion.Timestamp, "")
	require.NoError(t, err)

	// Verify restoration worked
	restored, err := env.blobStore.Download(ctx, unitID)
	require.NoError(t, err)
	assert.Equal(t, version1, restored)
}

func testVersionOperationsWithPatternPermissions(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create units in different environments
	devUnit := "dev/myapp"
	prodUnit := "prod/myapp"

	for _, unitID := range []string{devUnit, prodUnit} {
		_, err := env.blobStore.Create(ctx, unitID)
		require.NoError(t, err)
		err = env.blobStore.Upload(ctx, unitID, []byte(fmt.Sprintf(`{"version": 4, "unit": "%s"}`, unitID)), "")
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
		err = env.blobStore.Upload(ctx, unitID, []byte(fmt.Sprintf(`{"version": 4, "unit": "%s", "updated": true}`, unitID)), "")
		require.NoError(t, err)
	}

	// Setup: User with dev/* permissions only
	setupUserWithPermission(t, env.queryStore, "dev-user@example.com", "dev/*", 
		[]rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite})

	devUserCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "dev-user@example.com",
		Email:   "dev-user@example.com",
	})

	// Test: Can list versions for dev unit
	devVersions, err := env.authRepo.ListVersions(devUserCtx, devUnit)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(devVersions), 1)

	// Test: Cannot list versions for prod unit
	_, err = env.authRepo.ListVersions(devUserCtx, prodUnit)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// Test: Can restore dev unit version
	if len(devVersions) > 0 {
		err = env.authRepo.RestoreVersion(devUserCtx, devUnit, devVersions[0].Timestamp, "")
		require.NoError(t, err)
	}

	// Test: Cannot restore prod unit version
	prodVersions, err := env.blobStore.ListVersions(ctx, prodUnit)
	require.NoError(t, err)
	if len(prodVersions) > 0 {
		err = env.authRepo.RestoreVersion(devUserCtx, prodUnit, prodVersions[0].Timestamp, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	}
}

func testVersionOperationsRespectLocks(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()
	unitID := "lock-test/app1"

	// Create unit with versions
	_, err := env.blobStore.Create(ctx, unitID)
	require.NoError(t, err)
	
	version1 := []byte(`{"version": 4, "serial": 1}`)
	err = env.blobStore.Upload(ctx, unitID, version1, "")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	
	version2 := []byte(`{"version": 4, "serial": 2}`)
	err = env.blobStore.Upload(ctx, unitID, version2, "")
	require.NoError(t, err)

	versions, err := env.blobStore.ListVersions(ctx, unitID)
	require.NoError(t, err)
	require.Greater(t, len(versions), 0)
	oldVersion := versions[len(versions)-1]

	// Lock the unit
	lockID := "test-lock-123"
	lockInfo := &storage.LockInfo{
		ID:      lockID,
		Who:     "test-user",
		Version: "1.0.0",
		Created: time.Now(),
	}
	err = env.blobStore.Lock(ctx, unitID, lockInfo)
	require.NoError(t, err)

	// Setup: User with write permission
	setupUserWithPermission(t, env.queryStore, "locker@example.com", "lock-test/*", 
		[]rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite, rbac.ActionUnitLock})

	lockerCtx := rbac.ContextWithPrincipal(ctx, rbac.Principal{
		Subject: "locker@example.com",
		Email:   "locker@example.com",
	})

	// Test: Cannot restore without lock ID
	err = env.authRepo.RestoreVersion(lockerCtx, unitID, oldVersion.Timestamp, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock")

	// Test: Cannot restore with wrong lock ID
	err = env.authRepo.RestoreVersion(lockerCtx, unitID, oldVersion.Timestamp, "wrong-lock-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock")

	// Test: Can restore with correct lock ID
	err = env.authRepo.RestoreVersion(lockerCtx, unitID, oldVersion.Timestamp, lockID)
	require.NoError(t, err)

	// Verify restoration worked
	restored, err := env.blobStore.Download(ctx, unitID)
	require.NoError(t, err)
	assert.Equal(t, version1, restored)

	// Cleanup
	err = env.blobStore.Unlock(ctx, unitID, lockID)
	require.NoError(t, err)
}

func testRBACManagePermissionEnforcement(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	// Initialize RBAC
	err := env.rbacMgr.InitializeRBAC(ctx, "admin@example.com", "admin@example.com")
	require.NoError(t, err)

	// Sync to query store
	adminPerm, err := env.rbacStore.GetPermission(ctx, "admin")
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, adminPerm)
	require.NoError(t, err)
	
	adminRole, err := env.rbacStore.GetRole(ctx, "admin")
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, adminRole)
	require.NoError(t, err)
	
	adminUser, err := env.rbacStore.GetUserAssignment(ctx, "admin@example.com")
	require.NoError(t, err)
	err = env.queryStore.SyncUser(ctx, adminUser)
	require.NoError(t, err)

	// Create a user WITHOUT rbac.manage permission
	noManagePerm := &rbac.Permission{
		ID:          "no-manage",
		Name:        "No Manage Permission",
		Description: "Can read/write units but not manage RBAC",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin@example.com",
	}
	err = env.queryStore.SyncPermission(ctx, noManagePerm)
	require.NoError(t, err)

	noManageRole := &rbac.Role{
		ID:          "no-manage-role",
		Name:        "No Manage Role",
		Permissions: []string{"no-manage"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin@example.com",
	}
	err = env.queryStore.SyncRole(ctx, noManageRole)
	require.NoError(t, err)

	noManageUser := &rbac.UserAssignment{
		Subject:   "nomanage@example.com",
		Email:     "nomanage@example.com",
		Roles:     []string{"no-manage-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = env.queryStore.SyncUser(ctx, noManageUser)
	require.NoError(t, err)

	// Test: User without rbac.manage cannot perform RBAC operations
	// This would be tested if we had RBAC operations exposed through the query store
	// For now, we verify the permission structure is correct
	canManage, err := env.queryStore.CanPerformAction(ctx, "nomanage@example.com", "rbac.manage", "*")
	require.NoError(t, err)
	assert.False(t, canManage, "User without rbac.manage should not be able to manage RBAC")

	// Create a user WITH rbac.manage permission
	managePerm := &rbac.Permission{
		ID:          "with-manage",
		Name:        "With Manage Permission",
		Description: "Can manage RBAC",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionRBACManage},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin@example.com",
	}
	err = env.queryStore.SyncPermission(ctx, managePerm)
	require.NoError(t, err)

	manageRole := &rbac.Role{
		ID:          "manage-role",
		Name:        "Manage Role",
		Permissions: []string{"with-manage"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin@example.com",
	}
	err = env.queryStore.SyncRole(ctx, manageRole)
	require.NoError(t, err)

	manageUser := &rbac.UserAssignment{
		Subject:   "manager@example.com",
		Email:     "manager@example.com",
		Roles:     []string{"manage-role"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = env.queryStore.SyncUser(ctx, manageUser)
	require.NoError(t, err)

	// Test: User with rbac.manage can perform RBAC operations
	canManage, err = env.queryStore.CanPerformAction(ctx, "manager@example.com", "rbac.manage", "*")
	require.NoError(t, err)
	assert.True(t, canManage, "User with rbac.manage should be able to manage RBAC")
}

func testAdminRoleIncludesRBACManage(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	// Initialize RBAC (creates admin role)
	err := env.rbacMgr.InitializeRBAC(ctx, "admin@example.com", "admin@example.com")
	require.NoError(t, err)

	// Sync to query store
	adminPerm, err := env.rbacStore.GetPermission(ctx, "admin")
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, adminPerm)
	require.NoError(t, err)
	
	adminRole, err := env.rbacStore.GetRole(ctx, "admin")
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, adminRole)
	require.NoError(t, err)
	
	adminUser, err := env.rbacStore.GetUserAssignment(ctx, "admin@example.com")
	require.NoError(t, err)
	err = env.queryStore.SyncUser(ctx, adminUser)
	require.NoError(t, err)

	// Test: Admin user has rbac.manage permission
	canManage, err := env.queryStore.CanPerformAction(ctx, "admin@example.com", "rbac.manage", "*")
	require.NoError(t, err)
	assert.True(t, canManage, "Admin should have rbac.manage permission")

	// Test: Admin has all other permissions too
	actions := []string{"unit.read", "unit.write", "unit.lock", "unit.delete"}
	for _, action := range actions {
		can, err := env.queryStore.CanPerformAction(ctx, "admin@example.com", action, "any-unit")
		require.NoError(t, err)
		assert.True(t, can, fmt.Sprintf("Admin should have %s permission", action))
	}
}

func testNonAdminCannotModifyRBAC(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	// Initialize RBAC
	err := env.rbacMgr.InitializeRBAC(ctx, "admin@example.com", "admin@example.com")
	require.NoError(t, err)

	// Sync to query store
	defaultPerm, err := env.rbacStore.GetPermission(ctx, "default")
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, defaultPerm)
	require.NoError(t, err)
	
	defaultRole, err := env.rbacStore.GetRole(ctx, "default")
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, defaultRole)
	require.NoError(t, err)

	// Create regular user with default role (no rbac.manage)
	regularUser := &rbac.UserAssignment{
		Subject:   "regular@example.com",
		Email:     "regular@example.com",
		Roles:     []string{"default"}, // Default role has only read permission
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = env.queryStore.SyncUser(ctx, regularUser)
	require.NoError(t, err)

	// Test: Regular user cannot manage RBAC
	canManage, err := env.queryStore.CanPerformAction(ctx, "regular@example.com", "rbac.manage", "*")
	require.NoError(t, err)
	assert.False(t, canManage, "Regular user should not have rbac.manage permission")

	// Test: Regular user can only read
	canRead, err := env.queryStore.CanPerformAction(ctx, "regular@example.com", "unit.read", "any-unit")
	require.NoError(t, err)
	assert.True(t, canRead, "Regular user should have read permission")

	canWrite, err := env.queryStore.CanPerformAction(ctx, "regular@example.com", "unit.write", "any-unit")
	require.NoError(t, err)
	assert.False(t, canWrite, "Regular user should not have write permission")
}

func testPatternMatchingEdgeCases(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	ctx := context.Background()

	testCases := []struct {
		name       string
		pattern    string
		testUnits  map[string]bool // unit -> should match
		actions    []rbac.Action
	}{
		{
			name:    "deep nesting",
			pattern: "org/team/env/*",
			testUnits: map[string]bool{
				"org/team/env/app1":     true,
				"org/team/env/app2":     true,
				"org/team/other/app1":   false,
				"org/other/env/app1":    false,
			},
			actions: []rbac.Action{rbac.ActionUnitRead},
		},
		{
			name:    "special characters in path",
			pattern: "app-name-v2/*",
			testUnits: map[string]bool{
				"app-name-v2/prod":   true,
				"app-name-v2/dev":    true,
				"app-name-v1/prod":   false,
				"app-name/v2/prod":   false,
			},
			actions: []rbac.Action{rbac.ActionUnitRead},
		},
		{
			name:    "multiple path segments with wildcard",
			pattern: "myapp/*/database",
			testUnits: map[string]bool{
				"myapp/prod/database":    true,
				"myapp/dev/database":     true,
				"myapp/staging/database": true,
				"myapp/database":         false,
				"myapp/prod/api":         false,
			},
			actions: []rbac.Action{rbac.ActionUnitRead},
		},
		{
			name:    "single wildcard matches all",
			pattern: "*",
			testUnits: map[string]bool{
				"anything":      true,
				"any/thing":     true,
				"a/b/c/d":       true,
			},
			actions: []rbac.Action{rbac.ActionUnitRead},
		},
		{
			name:    "root level namespace",
			pattern: "myapp/*",
			testUnits: map[string]bool{
				"myapp/prod":         true,
				"myapp/dev":          true,
				"myapp/staging/db":   true,
				"otherapp/prod":      false,
				"myapp":              false, // Exact match, not pattern match
			},
			actions: []rbac.Action{rbac.ActionUnitRead},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create permission with pattern
			perm := &rbac.Permission{
				ID:          fmt.Sprintf("pattern-perm-%s", tc.name),
				Name:        fmt.Sprintf("Pattern Permission %s", tc.name),
				Description: fmt.Sprintf("Test pattern: %s", tc.pattern),
				Rules: []rbac.PermissionRule{
					{
						Actions:   tc.actions,
						Resources: []string{tc.pattern},
						Effect:    "allow",
					},
				},
				CreatedAt: time.Now(),
				CreatedBy: "test",
			}
			err := env.queryStore.SyncPermission(ctx, perm)
			require.NoError(t, err)

			// Create role
			role := &rbac.Role{
				ID:          fmt.Sprintf("pattern-role-%s", tc.name),
				Name:        fmt.Sprintf("Pattern Role %s", tc.name),
				Permissions: []string{perm.ID},
				CreatedAt:   time.Now(),
				CreatedBy:   "test",
			}
			err = env.queryStore.SyncRole(ctx, role)
			require.NoError(t, err)

			// Create user
			userEmail := fmt.Sprintf("pattern-user-%s@example.com", tc.name)
			user := &rbac.UserAssignment{
				Subject:   userEmail,
				Email:     userEmail,
				Roles:     []string{role.ID},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Version:   1,
			}
			err = env.queryStore.SyncUser(ctx, user)
			require.NoError(t, err)

			// Test each unit
			for unitID, shouldMatch := range tc.testUnits {
				// Ensure unit exists in query store
				err := env.queryStore.SyncEnsureUnit(ctx, unitID)
				require.NoError(t, err)

				// Test permission
				for _, action := range tc.actions {
					canPerform, err := env.queryStore.CanPerformAction(ctx, userEmail, string(action), unitID)
					require.NoError(t, err)
					
					if shouldMatch {
						assert.True(t, canPerform, 
							"Pattern %s should match unit %s for action %s", tc.pattern, unitID, action)
					} else {
						assert.False(t, canPerform, 
							"Pattern %s should NOT match unit %s for action %s", tc.pattern, unitID, action)
					}
				}
			}
		})
	}
}

// Helper functions

func setupUserWithPermission(t *testing.T, queryStore query.Store, email string, resource string, actions []rbac.Action) {
	ctx := context.Background()
	
	permID := fmt.Sprintf("perm-%s", email)
	perm := &rbac.Permission{
		ID:          permID,
		Name:        fmt.Sprintf("Permission for %s", email),
		Description: fmt.Sprintf("Test permission for %s", email),
		Rules: []rbac.PermissionRule{
			{
				Actions:   actions,
				Resources: []string{resource},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "test",
	}
	err := queryStore.SyncPermission(ctx, perm)
	require.NoError(t, err)

	roleID := fmt.Sprintf("role-%s", email)
	role := &rbac.Role{
		ID:          roleID,
		Name:        fmt.Sprintf("Role for %s", email),
		Permissions: []string{permID},
		CreatedAt:   time.Now(),
		CreatedBy:   "test",
	}
	err = queryStore.SyncRole(ctx, role)
	require.NoError(t, err)

	user := &rbac.UserAssignment{
		Subject:   email,
		Email:     email,
		Roles:     []string{roleID},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	err = queryStore.SyncUser(ctx, user)
	require.NoError(t, err)
}

