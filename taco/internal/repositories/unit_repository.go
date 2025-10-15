package repositories

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// unitRepository implements UnitRepository by coordinating blob storage with query index.
// This is where the coordination logic lives - hidden from handlers.
//
// DEPENDENCY: The query store (database) is REQUIRED for:
// - Fast listing operations (query index)
// - RBAC enforcement (permissions/roles stored in database)
// If the database is unavailable, RBAC checks will fail closed (deny access).
type unitRepository struct {
	blob  storage.UnitStore  // Source of truth for state data
	query query.Store        // Fast index for metadata/listing (REQUIRED for RBAC)
}

// NewUnitRepository creates a repository that coordinates blob storage with query index.
// Coordination happens internally:
// - Write operations update both blob and query
// - List operations use fast query index (with blob fallback)
// - Read operations go directly to blob (no coordination overhead)
//
// IMPORTANT: The query store (database) is critical for RBAC enforcement.
// If the database is unavailable, all RBAC-protected operations will fail.
func NewUnitRepository(blob storage.UnitStore, query query.Store) domain.UnitRepository {
	return &unitRepository{
		blob:  blob,
		query: query,
	}
}

// ============================================
// UnitReader Implementation
// ============================================

// Get retrieves metadata from blob storage (no coordination needed)
func (r *unitRepository) Get(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	return r.blob.Get(ctx, id)
}

// Download retrieves state data from blob storage (no coordination needed)
func (r *unitRepository) Download(ctx context.Context, id string) ([]byte, error) {
	return r.blob.Download(ctx, id)
}

// List retrieves units from the query index with fallback to blob storage.
// COORDINATION: Prefers query index (fast), falls back to blob if query fails.
// NOTE: RBAC enforcement still requires the database, so if auth is enabled and
// the database is unavailable, the request will fail even though blob storage works.
func (r *unitRepository) List(ctx context.Context, prefix string) ([]*storage.UnitMetadata, error) {
	// Try fast database index first
	units, err := r.query.ListUnits(ctx, prefix)
	if err != nil {
		log.Printf("Warning: Query index failed for List, falling back to blob storage: %v", err)
		// Fallback to blob storage (slower but works if DB is down)
		return r.blob.List(ctx, prefix)
	}

	// Convert query types to storage types
	metadata := make([]*storage.UnitMetadata, len(units))
	for i, u := range units {
		var lockInfo *storage.LockInfo
		if u.Locked && u.LockCreated != nil {
			lockInfo = &storage.LockInfo{
				ID:      u.LockID,
				Who:     u.LockWho,
				Created: *u.LockCreated,
			}
		}
		metadata[i] = &storage.UnitMetadata{
			ID:       u.Name,
			Size:     u.Size,
			Updated:  u.UpdatedAt,
			Locked:   u.Locked,
			LockInfo: lockInfo,
		}
	}

	return metadata, nil
}

// GetLock retrieves lock info from blob storage (no coordination needed)
func (r *unitRepository) GetLock(ctx context.Context, id string) (*storage.LockInfo, error) {
	return r.blob.GetLock(ctx, id)
}

// ListVersions retrieves version history from blob storage (no coordination needed)
func (r *unitRepository) ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error) {
	return r.blob.ListVersions(ctx, id)
}

// ============================================
// UnitWriter Implementation
// ============================================

// Create creates a unit in blob storage and syncs to query index
// COORDINATION: Blob write + query sync
func (r *unitRepository) Create(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	// Create in blob storage (source of truth)
	meta, err := r.blob.Create(ctx, id)
	if err != nil {
		return nil, err
	}

	// Sync to query index with retry logic
	err = RetrySync(ctx, func(ctx context.Context) error {
		if err := r.query.SyncEnsureUnit(ctx, id); err != nil {
			return err
		}
		return r.query.SyncUnitMetadata(ctx, id, meta.Size, meta.Updated)
	}, fmt.Sprintf("create unit '%s'", id))
	
	if err != nil {
		log.Printf("CRITICAL: Created unit '%s' in blob but failed to sync to index after retries: %v", id, err)
		// Continue anyway - blob is source of truth
	}

	return meta, nil
}

// Upload writes state data to blob storage and updates query index
// COORDINATION: Blob write + metadata sync
func (r *unitRepository) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	// Upload to blob storage (source of truth)
	err := r.blob.Upload(ctx, id, data, lockID)
	if err != nil {
		return err
	}

	// Sync metadata to query index with retry logic
	meta, err := r.blob.Get(ctx, id)
	if err == nil {
		SyncWithFallback(ctx, func(ctx context.Context) error {
			return r.query.SyncUnitMetadata(ctx, id, meta.Size, meta.Updated)
		}, fmt.Sprintf("upload unit '%s' metadata", id))
	}

	return nil
}

// Delete removes a unit from blob storage and syncs deletion to query index
// COORDINATION: Blob delete + query sync
func (r *unitRepository) Delete(ctx context.Context, id string) error {
	// Delete from blob storage (source of truth)
	err := r.blob.Delete(ctx, id)
	if err != nil {
		return err
	}

	// Sync deletion to query index with retry logic
	SyncWithFallback(ctx, func(ctx context.Context) error {
		return r.query.SyncDeleteUnit(ctx, id)
	}, fmt.Sprintf("delete unit '%s'", id))

	return nil
}

// RestoreVersion restores a version in blob storage (no coordination needed for now)
func (r *unitRepository) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	return r.blob.RestoreVersion(ctx, id, versionTimestamp, lockID)
}

// ============================================
// UnitLocker Implementation
// ============================================

// Lock locks in blob storage and syncs lock status to query index
// COORDINATION: Blob lock + query sync
func (r *unitRepository) Lock(ctx context.Context, id string, info *storage.LockInfo) error {
	// Lock in blob storage (source of truth)
	err := r.blob.Lock(ctx, id, info)
	if err != nil {
		return err
	}

	// Sync lock status to query index with retry logic
	SyncWithFallback(ctx, func(ctx context.Context) error {
		return r.query.SyncUnitLock(ctx, id, info.ID, info.Who, info.Created)
	}, fmt.Sprintf("lock unit '%s'", id))

	return nil
}

// Unlock unlocks in blob storage and syncs unlock status to query index
// COORDINATION: Blob unlock + query sync
func (r *unitRepository) Unlock(ctx context.Context, id string, lockID string) error {
	// Unlock in blob storage (source of truth)
	err := r.blob.Unlock(ctx, id, lockID)
	if err != nil {
		return err
	}

	// Sync unlock status to query index with retry logic
	SyncWithFallback(ctx, func(ctx context.Context) error {
		return r.query.SyncUnitUnlock(ctx, id)
	}, fmt.Sprintf("unlock unit '%s'", id))

	return nil
}

