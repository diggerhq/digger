package domain

import (
	"context"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// ============================================
// Mock Implementations for Testing
// ============================================

// MockStateOperations provides a minimal mock for testing handlers that use StateOperations.
// Only 6 methods to implement - much easier than mocking full UnitStore (11 methods).
type MockStateOperations struct {
	GetFunc      func(ctx context.Context, id string) (*storage.UnitMetadata, error)
	DownloadFunc func(ctx context.Context, id string) ([]byte, error)
	GetLockFunc  func(ctx context.Context, id string) (*storage.LockInfo, error)
	UploadFunc   func(ctx context.Context, id string, data []byte, lockID string) error
	LockFunc     func(ctx context.Context, id string, info *storage.LockInfo) error
	UnlockFunc   func(ctx context.Context, id string, lockID string) error
}

func (m *MockStateOperations) Get(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return &storage.UnitMetadata{ID: id, Size: 100, Updated: time.Now()}, nil
}

func (m *MockStateOperations) Download(ctx context.Context, id string) ([]byte, error) {
	if m.DownloadFunc != nil {
		return m.DownloadFunc(ctx, id)
	}
	return []byte(`{"version": 4}`), nil
}

func (m *MockStateOperations) GetLock(ctx context.Context, id string) (*storage.LockInfo, error) {
	if m.GetLockFunc != nil {
		return m.GetLockFunc(ctx, id)
	}
	return nil, storage.ErrNotFound
}

func (m *MockStateOperations) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	if m.UploadFunc != nil {
		return m.UploadFunc(ctx, id, data, lockID)
	}
	return nil
}

func (m *MockStateOperations) Lock(ctx context.Context, id string, info *storage.LockInfo) error {
	if m.LockFunc != nil {
		return m.LockFunc(ctx, id, info)
	}
	return nil
}

func (m *MockStateOperations) Unlock(ctx context.Context, id string, lockID string) error {
	if m.UnlockFunc != nil {
		return m.UnlockFunc(ctx, id, lockID)
	}
	return nil
}

// MockTFEOperations provides a mock for testing TFE handler.
// 7 methods - adds Create to StateOperations.
type MockTFEOperations struct {
	MockStateOperations
	CreateFunc func(ctx context.Context, id string) (*storage.UnitMetadata, error)
}

func (m *MockTFEOperations) Create(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, id)
	}
	return &storage.UnitMetadata{ID: id, Size: 0, Updated: time.Now()}, nil
}

// MockUnitManagement provides a mock for testing unit handler.
// 11 methods - full management interface.
type MockUnitManagement struct {
	MockTFEOperations
	ListFunc           func(ctx context.Context, prefix string) ([]*storage.UnitMetadata, error)
	DeleteFunc         func(ctx context.Context, id string) error
	ListVersionsFunc   func(ctx context.Context, id string) ([]*storage.VersionInfo, error)
	RestoreVersionFunc func(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error
}

func (m *MockUnitManagement) List(ctx context.Context, prefix string) ([]*storage.UnitMetadata, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, prefix)
	}
	return []*storage.UnitMetadata{}, nil
}

func (m *MockUnitManagement) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockUnitManagement) ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error) {
	if m.ListVersionsFunc != nil {
		return m.ListVersionsFunc(ctx, id)
	}
	return []*storage.VersionInfo{}, nil
}

func (m *MockUnitManagement) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	if m.RestoreVersionFunc != nil {
		return m.RestoreVersionFunc(ctx, id, versionTimestamp, lockID)
	}
	return nil
}

