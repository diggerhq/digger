package storage

import (
	"context"
	"log"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
)

// OrchestratingStore implements the UnitStore interface to coordinate a blob store
// (like S3) and a database index (like SQLite).
type OrchestratingStore struct {
	blobStore  UnitStore   // The primary source of truth for file content (e.g., S3Store)
	queryStore query.Store // The source of truth for metadata and listings
}

// NewOrchestratingStore creates a new store that synchronizes blob and database storage.
func NewOrchestratingStore(blobStore UnitStore, queryStore query.Store) UnitStore {
	return &OrchestratingStore{
		blobStore:  blobStore,
		queryStore: queryStore,
	}
}

// Create writes to the blob store first, then syncs the metadata to the database.
func (s *OrchestratingStore) Create(ctx context.Context, id string) (*UnitMetadata, error) {
	meta, err := s.blobStore.Create(ctx, id)
	if err != nil {
		return nil, err // If blob storage fails, the whole operation fails.
	}

	if err := s.queryStore.SyncEnsureUnit(ctx, id); err != nil {
		log.Printf("CRITICAL: Unit '%s' created in blob storage but failed to sync to database: %v", id, err)
	} else {
		// Sync metadata too
		if err := s.queryStore.SyncUnitMetadata(ctx, id, meta.Size, meta.Updated); err != nil {
			log.Printf("Warning: Failed to sync metadata for unit '%s': %v", id, err)
		}
	}
	return meta, nil
}

// Upload writes to the blob store first, then syncs the metadata to the database.
func (s *OrchestratingStore) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	err := s.blobStore.Upload(ctx, id, data, lockID)
	if err != nil {
		return err
	}

	// Get metadata to sync size
	meta, err := s.blobStore.Get(ctx, id)
	if err == nil && s.queryStore.IsEnabled() {
		// Sync with full metadata
		if syncer, ok := s.queryStore.(interface {
			SyncUnitMetadata(context.Context, string, int64, time.Time) error
		}); ok {
			syncer.SyncUnitMetadata(ctx, id, meta.Size, meta.Updated)
		}
	}
	
	return nil
}

// Delete removes from the blob store first, then syncs the deletion to the database.
func (s *OrchestratingStore) Delete(ctx context.Context, id string) error {
	err := s.blobStore.Delete(ctx, id)
	if err != nil {
		return err
	}
	if err := s.queryStore.SyncDeleteUnit(ctx, id); err != nil {
		log.Printf("CRITICAL: Unit '%s' deleted from blob storage but failed to sync to database: %v", id, err)
	}
	return nil
}

// List bypasses blob storage and uses the fast database index.
func (s *OrchestratingStore) List(ctx context.Context, prefix string) ([]*UnitMetadata, error) {
	var units []types.Unit
	units, err := s.queryStore.ListUnits(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// Adapt the result from the query store's type to the storage's type.
	metadata := make([]*UnitMetadata, len(units))
	for i, u := range units {
		var lockInfo *LockInfo
		if u.Locked && u.LockCreated != nil {
			lockInfo = &LockInfo{
				ID:      u.LockID,
				Who:     u.LockWho,
				Created: *u.LockCreated,
			}
		}
		metadata[i] = &UnitMetadata{
			ID:       u.Name,
			Size:     u.Size,
			Updated:  u.UpdatedAt,
			Locked:   u.Locked,
			LockInfo: lockInfo,
		}
	}
	return metadata, nil
}

// --- Pass-through methods ---
// For operations that only concern the blob data itself, we pass them directly
// to the underlying blob store.

func (s *OrchestratingStore) Get(ctx context.Context, id string) (*UnitMetadata, error) {
	return s.blobStore.Get(ctx, id)
}

func (s *OrchestratingStore) Download(ctx context.Context, id string) ([]byte, error) {
	return s.blobStore.Download(ctx, id)
}

func (s *OrchestratingStore) Lock(ctx context.Context, id string, info *LockInfo) error {
	err := s.blobStore.Lock(ctx, id, info)
	if err != nil {
		return err
	}
	
	// Sync lock status to database
	if err := s.queryStore.SyncUnitLock(ctx, id, info.ID, info.Who, info.Created); err != nil {
		log.Printf("Warning: Failed to sync lock status for unit '%s': %v", id, err)
	}
	return nil
}

func (s *OrchestratingStore) Unlock(ctx context.Context, id string, lockID string) error {
	err := s.blobStore.Unlock(ctx, id, lockID)
	if err != nil {
		return err
	}
	
	// Sync unlock status to database
	if err := s.queryStore.SyncUnitUnlock(ctx, id); err != nil {
		log.Printf("Warning: Failed to sync unlock status for unit '%s': %v", id, err)
	}
	return nil
}

func (s *OrchestratingStore) GetLock(ctx context.Context, id string) (*LockInfo, error) {
	return s.blobStore.GetLock(ctx, id)
}

func (s *OrchestratingStore) ListVersions(ctx context.Context, id string) ([]*VersionInfo, error) {
	return s.blobStore.ListVersions(ctx, id)
}

func (s *OrchestratingStore) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	return s.blobStore.RestoreVersion(ctx, id, versionTimestamp, lockID)
}