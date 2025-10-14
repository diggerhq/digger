package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"gorm.io/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRBACIntegration tests the full RBAC stack with SQLite query backend and mock S3 storage
func TestRBACIntegration(t *testing.T) {
	// Setup test environment
	env := setupTestEnvironment(t)
	defer env.cleanup()

	t.Run("initialization creates default roles and admin user", func(t *testing.T) {
		testInitialization(t, env)
	})

	t.Run("admin user has full access to all units", func(t *testing.T) {
		testAdminFullAccess(t, env)
	})

	t.Run("reader role can only read units", func(t *testing.T) {
		testReaderAccess(t, env)
	})

	t.Run("writer role can read and write but not delete", func(t *testing.T) {
		testWriterAccess(t, env)
	})

	t.Run("wildcard permissions work correctly", func(t *testing.T) {
		testWildcardPermissions(t, env)
	})

	t.Run("prefix-based permissions are enforced", func(t *testing.T) {
		testPrefixBasedPermissions(t, env)
	})

	t.Run("unauthorized access is blocked", func(t *testing.T) {
		testUnauthorizedAccess(t, env)
	})

	t.Run("list operations return only authorized units", func(t *testing.T) {
		testListFiltering(t, env)
	})

	t.Run("multiple roles accumulate permissions", func(t *testing.T) {
		testMultipleRoles(t, env)
	})

	t.Run("lock operations respect permissions", func(t *testing.T) {
		testLockPermissions(t, env)
	})

	t.Run("missing principal returns unauthorized", func(t *testing.T) {
		testMissingPrincipal(t, env)
	})

	t.Run("user with no roles has no access", func(t *testing.T) {
		testNoRoles(t, env)
	})
}

// testEnvironment holds all the components needed for integration testing
type testEnvironment struct {
	queryStore query.Store
	blobStore  storage.UnitStore
	repo       domain.UnitRepository      // Repository (no auth)
	authRepo   domain.UnitRepository      // Repository (with auth)
	rbacStore  rbac.RBACStore
	rbacMgr    *rbac.RBACManager
	tempDir    string
}

func (e *testEnvironment) cleanup() {
	if e.queryStore != nil {
		e.queryStore.Close()
	}
	if e.tempDir != "" {
		os.RemoveAll(e.tempDir)
	}
}

// setupTestEnvironment creates a full test environment with SQLite and mock S3
func setupTestEnvironment(t *testing.T) *testEnvironment {
	tempDir, err := os.MkdirTemp("", "rbac-test-*")
	require.NoError(t, err)

	// Create SQLite query store with in-memory database
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

	queryStore, err := NewSQLiteQueryStore(cfg)
	require.NoError(t, err, "failed to create query store")
	require.NotNil(t, queryStore, "query store should not be nil")

	// Create mock S3-backed blob store
	blobStore := storage.NewMemStore() // Using memstore as a simple blob store for now

	// Create RBAC store using queryStore (RBAC data is in database)
	sqlStore, ok := queryStore.(interface{ GetDB() *gorm.DB })
	require.True(t, ok, "queryStore should expose GetDB()")
	
	// Force re-migration of Rule table to ensure ResourcePatterns column exists
	err = sqlStore.GetDB().AutoMigrate(&types.Rule{})
	require.NoError(t, err, "failed to migrate Rule table")
	
	rbacStore := rbac.NewQueryRBACStore(sqlStore.GetDB())
	rbacMgr := rbac.NewRBACManager(rbacStore)

	// Create repository (coordinates blob + query)
	repo := repositories.NewUnitRepository(blobStore, queryStore)

	// Create authorizing repository (enforces RBAC)
	authRepo := repositories.NewAuthorizingRepository(repo, rbacMgr)

	return &testEnvironment{
		queryStore: queryStore,
		blobStore:  blobStore,
		repo:       repo,
		authRepo:   authRepo,
		rbacStore:  rbacStore,
		rbacMgr:    rbacMgr,
		tempDir:    tempDir,
	}
}

func testInitialization(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Initialize RBAC
	adminSubject := "admin@example.com"
	adminEmail := "admin@example.com"

	err := env.rbacMgr.InitializeRBAC(ctx, adminSubject, adminEmail)
	require.NoError(t, err)

	// Verify RBAC is enabled
	enabled, err := env.rbacMgr.IsEnabled(ctx)
	require.NoError(t, err)
	assert.True(t, enabled)

	// Verify admin permission was created
	adminPerm, err := env.rbacStore.GetPermission(ctx, "admin")
	require.NoError(t, err)
	assert.NotNil(t, adminPerm)
	assert.Contains(t, adminPerm.Rules[0].Actions, rbac.ActionRBACManage)

	// Verify admin role was created
	adminRole, err := env.rbacStore.GetRole(ctx, "admin")
	require.NoError(t, err)
	assert.NotNil(t, adminRole)
	assert.Contains(t, adminRole.Permissions, "admin")

	// Verify admin user was assigned admin role
	assignment, err := env.rbacStore.GetUserAssignment(ctx, adminSubject)
	require.NoError(t, err)
	assert.NotNil(t, assignment)
	assert.Contains(t, assignment.Roles, "admin")

	// Sync RBAC data to query store
	err = env.queryStore.SyncPermission(ctx, adminPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, adminRole)
	require.NoError(t, err)
	err = env.queryStore.SyncUser(ctx, assignment)
	require.NoError(t, err)

	// Verify database has roles
	hasRoles, err := env.queryStore.HasRBACRoles(ctx)
	require.NoError(t, err)
	assert.True(t, hasRoles)
}

func testAdminFullAccess(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup: Initialize RBAC and create test units
	setupRBACWithUnits(t, env)

	adminPrincipal := rbac.Principal{
		Subject: "admin@example.com",
		Email:   "admin@example.com",
	}
	ctx = rbac.ContextWithPrincipal(ctx, adminPrincipal)

	// Admin should be able to list all units
	units, err := env.authRepo.List(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(units), 3, "admin should see all units")

	// Admin should be able to read
	data, err := env.authRepo.Download(ctx, "dev/app1")
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Admin should be able to write
	err = env.authRepo.Upload(ctx, "dev/app1", []byte("updated"), "")
	require.NoError(t, err)

	// Admin should be able to lock
	lockInfo := &storage.LockInfo{
		ID:      "lock-123",
		Who:     "admin",
		Created: time.Now(),
	}
	err = env.authRepo.Lock(ctx, "dev/app1", lockInfo)
	require.NoError(t, err)

	// Admin should be able to unlock
	err = env.authRepo.Unlock(ctx, "dev/app1", "lock-123")
	require.NoError(t, err)

	// Admin should be able to delete
	err = env.authRepo.Delete(ctx, "dev/app1")
	require.NoError(t, err)
}

func testReaderAccess(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create reader permission and role
	readerPerm := &rbac.Permission{
		ID:          "reader",
		Name:        "Reader Permission",
		Description: "Read-only access",
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
	err := env.rbacStore.CreatePermission(ctx, readerPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, readerPerm)
	require.NoError(t, err)

	readerRole := &rbac.Role{
		ID:          "reader",
		Name:        "Reader Role",
		Permissions: []string{"reader"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, readerRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, readerRole)
	require.NoError(t, err)

	// Assign reader role to user
	readerSubject := "reader@example.com"
	err = env.rbacStore.AssignRole(ctx, readerSubject, readerSubject, "reader")
	require.NoError(t, err)
	readerAssignment, _ := env.rbacStore.GetUserAssignment(ctx, readerSubject)
	err = env.queryStore.SyncUser(ctx, readerAssignment)
	require.NoError(t, err)

	readerPrincipal := rbac.Principal{
		Subject: readerSubject,
		Email:   readerSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, readerPrincipal)

	// Reader should be able to read
	data, err := env.authRepo.Download(ctx, "dev/app2")
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Reader should NOT be able to write
	err = env.authRepo.Upload(ctx, "dev/app2", []byte("updated"), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// Reader should NOT be able to lock
	lockInfo := &storage.LockInfo{
		ID:      "lock-456",
		Who:     "reader",
		Created: time.Now(),
	}
	err = env.authRepo.Lock(ctx, "dev/app2", lockInfo)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// Reader should NOT be able to delete
	err = env.authRepo.Delete(ctx, "dev/app2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testWriterAccess(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create writer permission and role
	writerPerm := &rbac.Permission{
		ID:          "writer",
		Name:        "Writer Permission",
		Description: "Read and write access",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite, rbac.ActionUnitLock},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := env.rbacStore.CreatePermission(ctx, writerPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, writerPerm)
	require.NoError(t, err)

	writerRole := &rbac.Role{
		ID:          "writer",
		Name:        "Writer Role",
		Permissions: []string{"writer"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, writerRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, writerRole)
	require.NoError(t, err)

	// Assign writer role to user
	writerSubject := "writer@example.com"
	err = env.rbacStore.AssignRole(ctx, writerSubject, writerSubject, "writer")
	require.NoError(t, err)
	writerAssignment, _ := env.rbacStore.GetUserAssignment(ctx, writerSubject)
	err = env.queryStore.SyncUser(ctx, writerAssignment)
	require.NoError(t, err)

	writerPrincipal := rbac.Principal{
		Subject: writerSubject,
		Email:   writerSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, writerPrincipal)

	// Writer should be able to read
	data, err := env.authRepo.Download(ctx, "prod/app1")
	require.NoError(t, err)
	assert.NotNil(t, data)

	// Writer should be able to write
	err = env.authRepo.Upload(ctx, "prod/app1", []byte("updated"), "")
	require.NoError(t, err)

	// Writer should be able to lock
	lockInfo := &storage.LockInfo{
		ID:      "lock-789",
		Who:     "writer",
		Created: time.Now(),
	}
	err = env.authRepo.Lock(ctx, "prod/app1", lockInfo)
	require.NoError(t, err)

	// Unlock for cleanup
	err = env.authRepo.Unlock(ctx, "prod/app1", "lock-789")
	require.NoError(t, err)

	// Writer should NOT be able to delete
	err = env.authRepo.Delete(ctx, "prod/app1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testWildcardPermissions(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create permission with wildcard actions
	wildcardPerm := &rbac.Permission{
		ID:          "wildcard",
		Name:        "Wildcard Permission",
		Description: "All actions on specific resources",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{"*"},
				Resources: []string{"staging/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := env.rbacStore.CreatePermission(ctx, wildcardPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, wildcardPerm)
	require.NoError(t, err)

	wildcardRole := &rbac.Role{
		ID:          "staging-admin",
		Name:        "Staging Admin",
		Permissions: []string{"wildcard"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, wildcardRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, wildcardRole)
	require.NoError(t, err)

	// Create staging unit
	_, err = env.repo.Create(ctx, "staging/app1")
	require.NoError(t, err)
	err = env.repo.Upload(ctx, "staging/app1", []byte("staging data"), "")
	require.NoError(t, err)

	// Assign role to user
	userSubject := "staging-admin@example.com"
	err = env.rbacStore.AssignRole(ctx, userSubject, userSubject, "staging-admin")
	require.NoError(t, err)
	userAssignment, _ := env.rbacStore.GetUserAssignment(ctx, userSubject)
	err = env.queryStore.SyncUser(ctx, userAssignment)
	require.NoError(t, err)

	userPrincipal := rbac.Principal{
		Subject: userSubject,
		Email:   userSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, userPrincipal)

	// User should have all permissions on staging/*
	data, err := env.authRepo.Download(ctx, "staging/app1")
	require.NoError(t, err)
	assert.NotNil(t, data)

	err = env.authRepo.Upload(ctx, "staging/app1", []byte("updated"), "")
	require.NoError(t, err)

	// User should NOT have access to dev/*
	_, err = env.authRepo.Download(ctx, "dev/app2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testPrefixBasedPermissions(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create dev-only permission
	devPerm := &rbac.Permission{
		ID:          "dev-access",
		Name:        "Dev Access",
		Description: "Access to dev environment only",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := env.rbacStore.CreatePermission(ctx, devPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, devPerm)
	require.NoError(t, err)

	devRole := &rbac.Role{
		ID:          "developer",
		Name:        "Developer",
		Permissions: []string{"dev-access"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, devRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, devRole)
	require.NoError(t, err)

	// Assign role to user
	devSubject := "developer@example.com"
	err = env.rbacStore.AssignRole(ctx, devSubject, devSubject, "developer")
	require.NoError(t, err)
	devAssignment, _ := env.rbacStore.GetUserAssignment(ctx, devSubject)
	err = env.queryStore.SyncUser(ctx, devAssignment)
	require.NoError(t, err)

	devPrincipal := rbac.Principal{
		Subject: devSubject,
		Email:   devSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, devPrincipal)

	// User should have access to dev/*
	data, err := env.authRepo.Download(ctx, "dev/app2")
	require.NoError(t, err)
	assert.NotNil(t, data)

	// User should NOT have access to prod/*
	_, err = env.authRepo.Download(ctx, "prod/app1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testUnauthorizedAccess(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create a user with no permissions
	noAccessSubject := "noaccess@example.com"
	noAccessPrincipal := rbac.Principal{
		Subject: noAccessSubject,
		Email:   noAccessSubject,
	}

	// Don't assign any roles - user doesn't exist in RBAC store
	// This should return an error (user not found) which propagates as unauthorized

	ctx = rbac.ContextWithPrincipal(ctx, noAccessPrincipal)

	// All operations should fail with "not found" error (user not in RBAC store)
	_, err := env.authRepo.Download(ctx, "dev/app2")
	assert.Error(t, err)
	// User doesn't exist in RBAC store, so GetUserAssignment returns "not found"
	assert.Contains(t, err.Error(), "not found")

	err = env.authRepo.Upload(ctx, "dev/app2", []byte("data"), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	err = env.authRepo.Delete(ctx, "dev/app2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func testListFiltering(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data with multiple units
	setupRBACWithUnits(t, env)

	// Create permission for dev/* only
	devPerm := &rbac.Permission{
		ID:          "dev-read",
		Name:        "Dev Read",
		Description: "Read dev units",
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
	err := env.rbacStore.CreatePermission(ctx, devPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, devPerm)
	require.NoError(t, err)

	devRole := &rbac.Role{
		ID:          "dev-reader",
		Name:        "Dev Reader",
		Permissions: []string{"dev-read"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, devRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, devRole)
	require.NoError(t, err)

	// Assign role to user
	userSubject := "dev-reader@example.com"
	err = env.rbacStore.AssignRole(ctx, userSubject, userSubject, "dev-reader")
	require.NoError(t, err)
	userAssignment, _ := env.rbacStore.GetUserAssignment(ctx, userSubject)
	err = env.queryStore.SyncUser(ctx, userAssignment)
	require.NoError(t, err)

	userPrincipal := rbac.Principal{
		Subject: userSubject,
		Email:   userSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, userPrincipal)

	// List all units - should only see dev/*
	units, err := env.authRepo.List(ctx, "")
	require.NoError(t, err)
	
	// Verify only dev units are returned
	for _, unit := range units {
		assert.Contains(t, unit.ID, "dev/", "User should only see dev/* units")
	}

	// List with dev prefix
	devUnits, err := env.authRepo.List(ctx, "dev/")
	require.NoError(t, err)
	assert.Greater(t, len(devUnits), 0, "Should see dev units")

	// List with prod prefix - should see nothing
	prodUnits, err := env.authRepo.List(ctx, "prod/")
	require.NoError(t, err)
	assert.Equal(t, 0, len(prodUnits), "Should not see prod units")
}

func testMultipleRoles(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create two different permissions
	devPerm := &rbac.Permission{
		ID:          "dev-perm",
		Name:        "Dev Permission",
		Description: "Access to dev",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitWrite},
				Resources: []string{"dev/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := env.rbacStore.CreatePermission(ctx, devPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, devPerm)
	require.NoError(t, err)

	prodPerm := &rbac.Permission{
		ID:          "prod-perm",
		Name:        "Prod Permission",
		Description: "Read access to prod",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead},
				Resources: []string{"prod/*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err = env.rbacStore.CreatePermission(ctx, prodPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, prodPerm)
	require.NoError(t, err)

	// Create two roles
	devRole := &rbac.Role{
		ID:          "dev-role",
		Name:        "Dev Role",
		Permissions: []string{"dev-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, devRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, devRole)
	require.NoError(t, err)

	prodRole := &rbac.Role{
		ID:          "prod-role",
		Name:        "Prod Role",
		Permissions: []string{"prod-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, prodRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, prodRole)
	require.NoError(t, err)

	// Assign both roles to user
	userSubject := "multi-role@example.com"
	err = env.rbacStore.AssignRole(ctx, userSubject, userSubject, "dev-role")
	require.NoError(t, err)
	err = env.rbacStore.AssignRole(ctx, userSubject, userSubject, "prod-role")
	require.NoError(t, err)
	userAssignment, _ := env.rbacStore.GetUserAssignment(ctx, userSubject)
	err = env.queryStore.SyncUser(ctx, userAssignment)
	require.NoError(t, err)

	userPrincipal := rbac.Principal{
		Subject: userSubject,
		Email:   userSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, userPrincipal)

	// User should have write access to dev
	err = env.authRepo.Upload(ctx, "dev/app2", []byte("updated"), "")
	require.NoError(t, err)

	// User should have read access to prod
	data, err := env.authRepo.Download(ctx, "prod/app1")
	require.NoError(t, err)
	assert.NotNil(t, data)

	// User should NOT have write access to prod
	err = env.authRepo.Upload(ctx, "prod/app1", []byte("updated"), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testLockPermissions(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create permission with lock access
	lockPerm := &rbac.Permission{
		ID:          "lock-perm",
		Name:        "Lock Permission",
		Description: "Can lock units",
		Rules: []rbac.PermissionRule{
			{
				Actions:   []rbac.Action{rbac.ActionUnitRead, rbac.ActionUnitLock},
				Resources: []string{"*"},
				Effect:    "allow",
			},
		},
		CreatedAt: time.Now(),
		CreatedBy: "admin",
	}
	err := env.rbacStore.CreatePermission(ctx, lockPerm)
	require.NoError(t, err)
	err = env.queryStore.SyncPermission(ctx, lockPerm)
	require.NoError(t, err)

	lockRole := &rbac.Role{
		ID:          "locker",
		Name:        "Locker",
		Permissions: []string{"lock-perm"},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err = env.rbacStore.CreateRole(ctx, lockRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, lockRole)
	require.NoError(t, err)

	// Assign role to user
	userSubject := "locker@example.com"
	err = env.rbacStore.AssignRole(ctx, userSubject, userSubject, "locker")
	require.NoError(t, err)
	userAssignment, _ := env.rbacStore.GetUserAssignment(ctx, userSubject)
	err = env.queryStore.SyncUser(ctx, userAssignment)
	require.NoError(t, err)

	userPrincipal := rbac.Principal{
		Subject: userSubject,
		Email:   userSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, userPrincipal)

	// User should be able to lock
	lockInfo := &storage.LockInfo{
		ID:      "lock-abc",
		Who:     "locker",
		Created: time.Now(),
	}
	err = env.authRepo.Lock(ctx, "dev/app2", lockInfo)
	require.NoError(t, err)

	// User should be able to unlock
	err = env.authRepo.Unlock(ctx, "dev/app2", "lock-abc")
	require.NoError(t, err)

	// User should NOT be able to write (no write permission)
	err = env.authRepo.Upload(ctx, "dev/app2", []byte("data"), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func testMissingPrincipal(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Don't add principal to context

	// All operations should return unauthorized
	_, err := env.authRepo.List(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")

	_, err = env.authRepo.Download(ctx, "dev/app2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")

	err = env.authRepo.Upload(ctx, "dev/app2", []byte("data"), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func testNoRoles(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Setup test data
	setupRBACWithUnits(t, env)

	// Create a dummy role for testing
	dummyRole := &rbac.Role{
		ID:          "dummy",
		Name:        "Dummy Role",
		Permissions: []string{},
		CreatedAt:   time.Now(),
		CreatedBy:   "admin",
	}
	err := env.rbacStore.CreateRole(ctx, dummyRole)
	require.NoError(t, err)
	err = env.queryStore.SyncRole(ctx, dummyRole)
	require.NoError(t, err)
	
	// Create user with no roles
	noRoleSubject := "norole@example.com"
	
	// Assign and then revoke dummy role to create user in database
	err = env.rbacStore.AssignRole(ctx, noRoleSubject, noRoleSubject, "dummy")
	require.NoError(t, err)
	err = env.rbacStore.RevokeRole(ctx, noRoleSubject, "dummy")
	require.NoError(t, err)
	
	noRoleAssignment, _ := env.rbacStore.GetUserAssignment(ctx, noRoleSubject)
	err = env.queryStore.SyncUser(ctx, noRoleAssignment)
	require.NoError(t, err)

	noRolePrincipal := rbac.Principal{
		Subject: noRoleSubject,
		Email:   noRoleSubject,
	}
	ctx = rbac.ContextWithPrincipal(ctx, noRolePrincipal)

	// User should not have access to anything
	_, err = env.authRepo.Download(ctx, "dev/app2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")

	// List should return empty
	units, err := env.authRepo.List(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, 0, len(units), "User with no roles should see no units")
}

// setupRBACWithUnits initializes RBAC and creates test units
func setupRBACWithUnits(t *testing.T, env *testEnvironment) {
	ctx := context.Background()

	// Initialize RBAC if not already done
	enabled, _ := env.rbacMgr.IsEnabled(ctx)
	if !enabled {
		err := env.rbacMgr.InitializeRBAC(ctx, "admin@example.com", "admin@example.com")
		require.NoError(t, err)

		// Sync admin data
		adminPerm, _ := env.rbacStore.GetPermission(ctx, "admin")
		env.queryStore.SyncPermission(ctx, adminPerm)
		adminRole, _ := env.rbacStore.GetRole(ctx, "admin")
		env.queryStore.SyncRole(ctx, adminRole)
		adminAssignment, _ := env.rbacStore.GetUserAssignment(ctx, "admin@example.com")
		env.queryStore.SyncUser(ctx, adminAssignment)
	}

	// Create test units
	testUnits := []string{
		"dev/app1",
		"dev/app2",
		"prod/app1",
	}

	for _, unitName := range testUnits {
		_, err := env.repo.Create(ctx, unitName)
		if err != nil && err != storage.ErrAlreadyExists {
			require.NoError(t, err)
		}
		
		// Upload some data
		data := []byte(`{"terraform_version": "1.0.0", "unit": "` + unitName + `"}`)
		err = env.repo.Upload(ctx, unitName, data, "")
		require.NoError(t, err)
	}
}
