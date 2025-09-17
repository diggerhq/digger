package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type memStore struct {
    mu     sync.RWMutex
    units  map[string]*unitData
}

type unitData struct {
    metadata *UnitMetadata
    content  []byte
    versions []*versionData
}

type versionData struct {
	timestamp time.Time
	hash      string
	content   []byte
}

func NewMemStore() UnitStore {
    return &memStore{
        units: make(map[string]*unitData),
    }
}

func (m *memStore) Create(ctx context.Context, id string) (*UnitMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    if _, exists := m.units[id]; exists {
        return nil, ErrAlreadyExists
    }
	
    metadata := &UnitMetadata{
        ID:      id,
        Size:    0,
        Updated: time.Now(),
        Locked:  false,
    }
	
    m.units[id] = &unitData{
        metadata: metadata,
        content:  []byte{},
    }
	
    return metadata, nil
}

func (m *memStore) Get(ctx context.Context, id string) (*UnitMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
    state, exists := m.units[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	// Return a copy to avoid mutations
    return &UnitMetadata{
        ID:       state.metadata.ID,
        Size:     state.metadata.Size,
        Updated:  state.metadata.Updated,
        Locked:   state.metadata.Locked,
        LockInfo: state.metadata.LockInfo,
    }, nil
}

func (m *memStore) List(ctx context.Context, prefix string) ([]*UnitMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
    var results []*UnitMetadata
    for id, unit := range m.units {
		if prefix == "" || strings.HasPrefix(id, prefix) {
			// Return copies
            results = append(results, &UnitMetadata{
                ID:       unit.metadata.ID,
                Size:     unit.metadata.Size,
                Updated:  unit.metadata.Updated,
                Locked:   unit.metadata.Locked,
                LockInfo: unit.metadata.LockInfo,
            })
        }
    }
	
	return results, nil
}

func (m *memStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    if _, exists := m.units[id]; !exists {
        return ErrNotFound
    }

    delete(m.units, id)
	return nil
}

func (m *memStore) Download(ctx context.Context, id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
    state, exists := m.units[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	// Return a copy of the content
	content := make([]byte, len(state.content))
	copy(content, state.content)
	
	return content, nil
}

func (m *memStore) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    state, exists := m.units[id]
	if !exists {
		return ErrNotFound
	}
	
	// Check lock if provided
	if lockID != "" && state.metadata.LockInfo != nil && state.metadata.LockInfo.ID != lockID {
		return ErrLockConflict
	}
	
	// If locked and no lockID provided, fail
	if lockID == "" && state.metadata.Locked {
		return ErrLockConflict
	}
	
	// Archive current content as a version if it exists and has content
	if state.metadata.Size > 0 && len(state.content) > 0 {
		// Generate hash of current content
		hash := sha256.Sum256(state.content)
		hashStr := hex.EncodeToString(hash[:4]) // First 4 bytes = 8 hex characters
		
		// Archive the current content
		archivedContent := make([]byte, len(state.content))
		copy(archivedContent, state.content)
		
		version := &versionData{
			timestamp: time.Now().UTC(),
			hash:      hashStr,
			content:   archivedContent,
		}
		
		state.versions = append(state.versions, version)
		
		// Clean up old versions after successful archiving
		if err := m.cleanupOldVersions(id); err != nil {
			fmt.Printf("Warning: failed to cleanup old versions for %s: %v\n", id, err)
		}
	}
	
	// Update content
	state.content = make([]byte, len(data))
	copy(state.content, data)
	state.metadata.Size = int64(len(data))
	state.metadata.Updated = time.Now()
	
	return nil
}

func (m *memStore) Lock(ctx context.Context, id string, info *LockInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    state, exists := m.units[id]
	if !exists {
		return ErrNotFound
	}
	
    if state.metadata.Locked {
        return fmt.Errorf("%w: unit already locked by %s", ErrLockConflict, state.metadata.LockInfo.ID)
    }
	
	state.metadata.Locked = true
	state.metadata.LockInfo = info
	
	return nil
}

func (m *memStore) Unlock(ctx context.Context, id string, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    state, exists := m.units[id]
	if !exists {
		return ErrNotFound
	}
	
    if !state.metadata.Locked {
        return fmt.Errorf("unit is not locked")
    }
	
	if state.metadata.LockInfo.ID != lockID {
		return ErrLockConflict
	}
	
	state.metadata.Locked = false
	state.metadata.LockInfo = nil
	
	return nil
}

func (m *memStore) GetLock(ctx context.Context, id string) (*LockInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
    state, exists := m.units[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	if !state.metadata.Locked {
		return nil, nil
	}
	
	return state.metadata.LockInfo, nil
}

// ListVersions returns all versions for a given unit ID
func (m *memStore) ListVersions(ctx context.Context, id string) ([]*VersionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
    state, exists := m.units[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	versions := make([]*VersionInfo, 0, len(state.versions))
	for _, v := range state.versions {
		versions = append(versions, &VersionInfo{
			Timestamp: v.timestamp,
			Hash:      v.hash,
			Size:      int64(len(v.content)),
			S3Key:     "", // Not applicable for memstore
		})
	}
	
	// Sort by timestamp (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})
	
	return versions, nil
}

// RestoreVersion restores a specific version to be the current unit tfstate
func (m *memStore) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
    state, exists := m.units[id]
	if !exists {
		return ErrNotFound
	}
	
	// Check lock if provided
	if lockID != "" && state.metadata.LockInfo != nil && state.metadata.LockInfo.ID != lockID {
		return ErrLockConflict
	}
	
	// If locked and no lockID provided, fail
	if lockID == "" && state.metadata.Locked {
		return ErrLockConflict
	}
	
	// Find the target version
	var targetVersion *versionData
	for _, v := range state.versions {
		if v.timestamp.Equal(versionTimestamp) {
			targetVersion = v
			break
		}
	}
	
	if targetVersion == nil {
		return fmt.Errorf("version not found for timestamp: %s", versionTimestamp.Format("2006-01-02 15:04:05"))
	}
	
	// Archive current content as a version if it exists and has content (same as Upload)
	if state.metadata.Size > 0 && len(state.content) > 0 {
		hash := sha256.Sum256(state.content)
		hashStr := hex.EncodeToString(hash[:4])
		
		archivedContent := make([]byte, len(state.content))
		copy(archivedContent, state.content)
		
		version := &versionData{
			timestamp: time.Now().UTC(),
			hash:      hashStr,
			content:   archivedContent,
		}
		
		state.versions = append(state.versions, version)
		
		// Clean up old versions after successful archiving
		if err := m.cleanupOldVersions(id); err != nil {
			fmt.Printf("Warning: failed to cleanup old versions for %s: %v\n", id, err)
		}
	}
	
	// Restore the target version content
	state.content = make([]byte, len(targetVersion.content))
	copy(state.content, targetVersion.content)
	state.metadata.Size = int64(len(targetVersion.content))
	state.metadata.Updated = time.Now()
	
	return nil
}

// getMaxVersions returns the maximum number of versions to keep per unit
// Defaults to 10 if OPENTACO_MAX_VERSIONS is not set or invalid
func (m *memStore) getMaxVersions() int {
	if maxStr := os.Getenv("OPENTACO_MAX_VERSIONS"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil && max > 0 {
			return max
		}
	}
	return 10 // Default
}

// cleanupOldVersions removes old versions beyond the configured maximum
// Note: This method assumes the caller already holds the necessary locks
func (m *memStore) cleanupOldVersions(id string) error {
    state, exists := m.units[id]
	if !exists {
		return ErrNotFound
	}
	
	maxVersions := m.getMaxVersions()
	
	if len(state.versions) <= maxVersions {
		return nil // Nothing to clean up
	}
	
	// Sort versions by timestamp (newest first)
	sort.Slice(state.versions, func(i, j int) bool {
		return state.versions[i].timestamp.After(state.versions[j].timestamp)
	})
	
	// Keep only the most recent N versions
	state.versions = state.versions[:maxVersions]
	
	return nil
}
