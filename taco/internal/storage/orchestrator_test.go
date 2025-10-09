package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestOrchestratingStore tests the orchestration layer between blob and query stores
func TestOrchestratingStore(t *testing.T) {
	t.Run("create syncs to query store", func(t *testing.T) {
		testCreateSyncsToQueryStore(t)
	})

	t.Run("create handles query sync failure gracefully", func(t *testing.T) {
		testCreateHandlesSyncFailure(t)
	})

	t.Run("upload syncs metadata to query store", func(t *testing.T) {
		testUploadSyncsMetadata(t)
	})

	t.Run("delete syncs to query store", func(t *testing.T) {
		testDeleteSyncsToQueryStore(t)
	})

	t.Run("delete handles query sync failure gracefully", func(t *testing.T) {
		testDeleteHandlesSyncFailure(t)
	})

	t.Run("list uses query store not blob store", func(t *testing.T) {
		testListUsesQueryStore(t)
	})

	t.Run("lock syncs to query store", func(t *testing.T) {
		testLockSyncsToQueryStore(t)
	})

	t.Run("unlock syncs to query store", func(t *testing.T) {
		testUnlockSyncsToQueryStore(t)
	})

	t.Run("get passes through to blob store", func(t *testing.T) {
		testGetPassesThrough(t)
	})

	t.Run("download passes through to blob store", func(t *testing.T) {
		testDownloadPassesThrough(t)
	})
}

func testCreateSyncsToQueryStore(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/unit1"

	// Setup expectations
	expectedMeta := &UnitMetadata{
		ID:      unitID,
		Size:    0,
		Updated: time.Now(),
		Locked:  false,
	}

	blobStore.On("Create", ctx, unitID).Return(expectedMeta, nil)
	queryStore.On("SyncEnsureUnit", ctx, unitID).Return(nil)
	queryStore.On("SyncUnitMetadata", ctx, unitID, int64(0), mock.AnythingOfType("time.Time")).Return(nil)

	// Execute
	meta, err := orchStore.Create(ctx, unitID)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, expectedMeta, meta)
	
	// Verify blob store was called first
	blobStore.AssertCalled(t, "Create", ctx, unitID)
	
	// Verify query store sync was called
	queryStore.AssertCalled(t, "SyncEnsureUnit", ctx, unitID)
	queryStore.AssertCalled(t, "SyncUnitMetadata", ctx, unitID, int64(0), mock.AnythingOfType("time.Time"))
}

func testCreateHandlesSyncFailure(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/unit-sync-fail"

	expectedMeta := &UnitMetadata{
		ID:      unitID,
		Size:    0,
		Updated: time.Now(),
		Locked:  false,
	}

	// Blob succeeds
	blobStore.On("Create", ctx, unitID).Return(expectedMeta, nil)
	
	// Query sync fails (simulating database error)
	syncError := errors.New("database connection lost")
	queryStore.On("SyncEnsureUnit", ctx, unitID).Return(syncError)

	// Execute
	meta, err := orchStore.Create(ctx, unitID)

	// Verify: Create should succeed even though sync failed
	// This is intentional - blob store is source of truth
	// The CRITICAL log warning will alert ops team
	require.NoError(t, err, "Create should succeed even if query sync fails")
	assert.Equal(t, expectedMeta, meta)
	
	// Verify both were called
	blobStore.AssertCalled(t, "Create", ctx, unitID)
	queryStore.AssertCalled(t, "SyncEnsureUnit", ctx, unitID)
	
	// Note: Metadata sync is NOT called because unit sync failed
	queryStore.AssertNotCalled(t, "SyncUnitMetadata", ctx, unitID, mock.Anything, mock.Anything)
}

func testUploadSyncsMetadata(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/upload-unit"
	data := []byte(`{"version": 4}`)
	lockID := ""

	// Setup
	blobStore.On("Upload", ctx, unitID, data, lockID).Return(nil)
	blobStore.On("Get", ctx, unitID).Return(&UnitMetadata{
		ID:      unitID,
		Size:    int64(len(data)),
		Updated: time.Now(),
	}, nil)
	queryStore.On("IsEnabled").Return(true)
	queryStore.On("SyncUnitMetadata", ctx, unitID, int64(len(data)), mock.AnythingOfType("time.Time")).Return(nil)

	// Execute
	err := orchStore.Upload(ctx, unitID, data, lockID)

	// Verify
	require.NoError(t, err)
	blobStore.AssertCalled(t, "Upload", ctx, unitID, data, lockID)
	blobStore.AssertCalled(t, "Get", ctx, unitID)
	queryStore.AssertCalled(t, "SyncUnitMetadata", ctx, unitID, int64(len(data)), mock.AnythingOfType("time.Time"))
}

func testDeleteSyncsToQueryStore(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/delete-unit"

	// Setup
	blobStore.On("Delete", ctx, unitID).Return(nil)
	queryStore.On("SyncDeleteUnit", ctx, unitID).Return(nil)

	// Execute
	err := orchStore.Delete(ctx, unitID)

	// Verify
	require.NoError(t, err)
	blobStore.AssertCalled(t, "Delete", ctx, unitID)
	queryStore.AssertCalled(t, "SyncDeleteUnit", ctx, unitID)
}

func testDeleteHandlesSyncFailure(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/delete-sync-fail"

	// Blob delete succeeds
	blobStore.On("Delete", ctx, unitID).Return(nil)
	
	// Query sync fails
	syncError := errors.New("database error")
	queryStore.On("SyncDeleteUnit", ctx, unitID).Return(syncError)

	// Execute
	err := orchStore.Delete(ctx, unitID)

	// Verify: Delete should succeed even though sync failed
	require.NoError(t, err, "Delete should succeed even if query sync fails")
	blobStore.AssertCalled(t, "Delete", ctx, unitID)
	queryStore.AssertCalled(t, "SyncDeleteUnit", ctx, unitID)
}

func testListUsesQueryStore(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	prefix := "test/"

	// Setup query store to return units
	expectedUnits := []types.Unit{
		{
			Name:      "test/unit1",
			Size:      1024,
			UpdatedAt: time.Now(),
			Locked:    false,
		},
		{
			Name:      "test/unit2",
			Size:      2048,
			UpdatedAt: time.Now(),
			Locked:    true,
			LockID:    "lock-123",
			LockWho:   "alice",
			LockCreated: &[]time.Time{time.Now()}[0],
		},
	}

	queryStore.On("ListUnits", ctx, prefix).Return(expectedUnits, nil)

	// Execute
	units, err := orchStore.List(ctx, prefix)

	// Verify
	require.NoError(t, err)
	assert.Len(t, units, 2)
	
	// Verify query store was called
	queryStore.AssertCalled(t, "ListUnits", ctx, prefix)
	
	// IMPORTANT: Verify blob store was NOT called
	// This is the optimization - List bypasses blob store
	blobStore.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
	
	// Verify metadata conversion
	assert.Equal(t, "test/unit1", units[0].ID)
	assert.Equal(t, int64(1024), units[0].Size)
	assert.False(t, units[0].Locked)
	
	assert.Equal(t, "test/unit2", units[1].ID)
	assert.Equal(t, int64(2048), units[1].Size)
	assert.True(t, units[1].Locked)
	assert.NotNil(t, units[1].LockInfo)
	assert.Equal(t, "lock-123", units[1].LockInfo.ID)
	assert.Equal(t, "alice", units[1].LockInfo.Who)
}

func testLockSyncsToQueryStore(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/lock-unit"
	lockInfo := &LockInfo{
		ID:      "lock-456",
		Who:     "bob",
		Version: "1.0.0",
		Created: time.Now(),
	}

	// Setup
	blobStore.On("Lock", ctx, unitID, lockInfo).Return(nil)
	queryStore.On("SyncUnitLock", ctx, unitID, lockInfo.ID, lockInfo.Who, lockInfo.Created).Return(nil)

	// Execute
	err := orchStore.Lock(ctx, unitID, lockInfo)

	// Verify
	require.NoError(t, err)
	blobStore.AssertCalled(t, "Lock", ctx, unitID, lockInfo)
	queryStore.AssertCalled(t, "SyncUnitLock", ctx, unitID, lockInfo.ID, lockInfo.Who, lockInfo.Created)
}

func testUnlockSyncsToQueryStore(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/unlock-unit"
	lockID := "lock-789"

	// Setup
	blobStore.On("Unlock", ctx, unitID, lockID).Return(nil)
	queryStore.On("SyncUnitUnlock", ctx, unitID).Return(nil)

	// Execute
	err := orchStore.Unlock(ctx, unitID, lockID)

	// Verify
	require.NoError(t, err)
	blobStore.AssertCalled(t, "Unlock", ctx, unitID, lockID)
	queryStore.AssertCalled(t, "SyncUnitUnlock", ctx, unitID)
}

func testGetPassesThrough(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/get-unit"
	expectedMeta := &UnitMetadata{
		ID:   unitID,
		Size: 5678,
	}

	// Setup
	blobStore.On("Get", ctx, unitID).Return(expectedMeta, nil)

	// Execute
	meta, err := orchStore.Get(ctx, unitID)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, expectedMeta, meta)
	blobStore.AssertCalled(t, "Get", ctx, unitID)
	
	// Query store should NOT be involved in Get
	queryStore.AssertNotCalled(t, "GetUnit", mock.Anything, mock.Anything)
}

func testDownloadPassesThrough(t *testing.T) {
	blobStore := &mockUnitStore{}
	queryStore := &mockQueryStore{}
	orchStore := NewOrchestratingStore(blobStore, queryStore)

	ctx := context.Background()
	unitID := "test/download-unit"
	expectedData := []byte(`{"version": 4, "resources": []}`)

	// Setup
	blobStore.On("Download", ctx, unitID).Return(expectedData, nil)

	// Execute
	data, err := orchStore.Download(ctx, unitID)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, expectedData, data)
	blobStore.AssertCalled(t, "Download", ctx, unitID)
	
	// Query store should NOT be involved in Download
	queryStore.AssertNotCalled(t, "ListUnits", mock.Anything, mock.Anything)
}

// Mock implementations

type mockUnitStore struct {
	mock.Mock
}

func (m *mockUnitStore) Create(ctx context.Context, id string) (*UnitMetadata, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UnitMetadata), args.Error(1)
}

func (m *mockUnitStore) Get(ctx context.Context, id string) (*UnitMetadata, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UnitMetadata), args.Error(1)
}

func (m *mockUnitStore) List(ctx context.Context, prefix string) ([]*UnitMetadata, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*UnitMetadata), args.Error(1)
}

func (m *mockUnitStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUnitStore) Download(ctx context.Context, id string) ([]byte, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockUnitStore) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	args := m.Called(ctx, id, data, lockID)
	return args.Error(0)
}

func (m *mockUnitStore) Lock(ctx context.Context, id string, info *LockInfo) error {
	args := m.Called(ctx, id, info)
	return args.Error(0)
}

func (m *mockUnitStore) Unlock(ctx context.Context, id string, lockID string) error {
	args := m.Called(ctx, id, lockID)
	return args.Error(0)
}

func (m *mockUnitStore) GetLock(ctx context.Context, id string) (*LockInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LockInfo), args.Error(1)
}

func (m *mockUnitStore) ListVersions(ctx context.Context, id string) ([]*VersionInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*VersionInfo), args.Error(1)
}

func (m *mockUnitStore) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	args := m.Called(ctx, id, versionTimestamp, lockID)
	return args.Error(0)
}

type mockQueryStore struct {
	mock.Mock
}

func (m *mockQueryStore) SyncEnsureUnit(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockQueryStore) SyncUnitMetadata(ctx context.Context, id string, size int64, updated time.Time) error {
	args := m.Called(ctx, id, size, updated)
	return args.Error(0)
}

func (m *mockQueryStore) SyncDeleteUnit(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockQueryStore) SyncUnitLock(ctx context.Context, id, lockID, who string, created time.Time) error {
	args := m.Called(ctx, id, lockID, who, created)
	return args.Error(0)
}

func (m *mockQueryStore) SyncUnitUnlock(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockQueryStore) ListUnits(ctx context.Context, prefix string) ([]types.Unit, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Unit), args.Error(1)
}

func (m *mockQueryStore) IsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockQueryStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Implement remaining query.Store interface methods (not used in these tests)
func (m *mockQueryStore) GetUnit(ctx context.Context, id string) (*types.Unit, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockQueryStore) SyncPermission(ctx context.Context, permission interface{}) error {
	return errors.New("not implemented in mock")
}

func (m *mockQueryStore) SyncRole(ctx context.Context, role interface{}) error {
	return errors.New("not implemented in mock")
}

func (m *mockQueryStore) SyncUser(ctx context.Context, user interface{}) error {
	return errors.New("not implemented in mock")
}

func (m *mockQueryStore) ListUnitsForUser(ctx context.Context, userSubject, prefix string) ([]types.Unit, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockQueryStore) CanPerformAction(ctx context.Context, userSubject, action, resourceID string) (bool, error) {
	return false, errors.New("not implemented in mock")
}

func (m *mockQueryStore) FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockQueryStore) HasRBACRoles(ctx context.Context) (bool, error) {
	return false, errors.New("not implemented in mock")
}

