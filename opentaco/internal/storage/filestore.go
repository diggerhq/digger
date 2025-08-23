package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type fileStore struct {
	mu      sync.RWMutex
	baseDir string
}

func NewFileStore(baseDir string) StateStore {
	return &fileStore{
		baseDir: baseDir,
	}
}

func (f *fileStore) statePath(id string) string {
	// Replace slashes to avoid directory traversal
	safeID := strings.ReplaceAll(id, "/", "_")
	return filepath.Join(f.baseDir, "objects", safeID+".tfstate")
}

func (f *fileStore) metadataPath(id string) string {
	safeID := strings.ReplaceAll(id, "/", "_")
	return filepath.Join(f.baseDir, "objects", safeID+".meta.json")
}

func (f *fileStore) lockPath(id string) string {
	safeID := strings.ReplaceAll(id, "/", "_")
	return filepath.Join(f.baseDir, "locks", safeID+".lock.json")
}

func (f *fileStore) Create(ctx context.Context, id string) (*StateMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	metaPath := f.metadataPath(id)
	
	// Check if already exists
	if _, err := os.Stat(metaPath); err == nil {
		return nil, ErrAlreadyExists
	}
	
	metadata := &StateMetadata{
		ID:      id,
		Size:    0,
		Updated: time.Now(),
		Locked:  false,
	}
	
	// Save metadata
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return nil, err
	}
	
	// Create empty state file
	if err := os.WriteFile(f.statePath(id), []byte{}, 0644); err != nil {
		return nil, err
	}
	
	return metadata, nil
}

func (f *fileStore) Get(ctx context.Context, id string) (*StateMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	metaPath := f.metadataPath(id)
	
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	
	var metadata StateMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	
	// Check if locked
	lockPath := f.lockPath(id)
	if lockData, err := os.ReadFile(lockPath); err == nil {
		var lockInfo LockInfo
		if err := json.Unmarshal(lockData, &lockInfo); err == nil {
			metadata.Locked = true
			metadata.LockInfo = &lockInfo
		}
	}
	
	return &metadata, nil
}

func (f *fileStore) List(ctx context.Context, prefix string) ([]*StateMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	var results []*StateMetadata
	
	metaDir := filepath.Join(f.baseDir, "objects")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}
	
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		
		// Extract ID from filename
		id := strings.TrimSuffix(entry.Name(), ".meta.json")
		id = strings.ReplaceAll(id, "_", "/")
		
		if prefix != "" && !strings.HasPrefix(id, prefix) {
			continue
		}
		
		metadata, err := f.Get(ctx, id)
		if err != nil {
			continue
		}
		
		results = append(results, metadata)
	}
	
	return results, nil
}

func (f *fileStore) Delete(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Check if exists
	if _, err := os.Stat(f.metadataPath(id)); os.IsNotExist(err) {
		return ErrNotFound
	}
	
	// Remove all files
	os.Remove(f.statePath(id))
	os.Remove(f.metadataPath(id))
	os.Remove(f.lockPath(id))
	
	return nil
}

func (f *fileStore) Download(ctx context.Context, id string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	data, err := os.ReadFile(f.statePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	
	return data, nil
}

func (f *fileStore) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Check if exists
	metadata, err := f.Get(ctx, id)
	if err != nil {
		return err
	}
	
	// Check lock
	if lockID != "" && metadata.LockInfo != nil && metadata.LockInfo.ID != lockID {
		return ErrLockConflict
	}
	
	if lockID == "" && metadata.Locked {
		return ErrLockConflict
	}
	
	// Write state file
	if err := os.WriteFile(f.statePath(id), data, 0644); err != nil {
		return err
	}
	
	// Update metadata
	metadata.Size = int64(len(data))
	metadata.Updated = time.Now()
	
	metaData, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	
	return os.WriteFile(f.metadataPath(id), metaData, 0644)
}

func (f *fileStore) Lock(ctx context.Context, id string, info *LockInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Check if exists
	if _, err := os.Stat(f.metadataPath(id)); os.IsNotExist(err) {
		return ErrNotFound
	}
	
	lockPath := f.lockPath(id)
	
	// Check if already locked
	if _, err := os.Stat(lockPath); err == nil {
		existingLock, _ := f.GetLock(ctx, id)
		if existingLock != nil {
			return fmt.Errorf("%w: state already locked by %s", ErrLockConflict, existingLock.ID)
		}
	}
	
	// Write lock file
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	
	return os.WriteFile(lockPath, data, 0644)
}

func (f *fileStore) Unlock(ctx context.Context, id string, lockID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	lockPath := f.lockPath(id)
	
	// Read existing lock
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("state is not locked")
		}
		return err
	}
	
	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return err
	}
	
	if lockInfo.ID != lockID {
		return ErrLockConflict
	}
	
	return os.Remove(lockPath)
}

func (f *fileStore) GetLock(ctx context.Context, id string) (*LockInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	lockPath := f.lockPath(id)
	
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	
	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, err
	}
	
	return &lockInfo, nil
}