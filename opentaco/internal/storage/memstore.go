package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type memStore struct {
	mu     sync.RWMutex
	states map[string]*stateData
}

type stateData struct {
	metadata *StateMetadata
	content  []byte
}

func NewMemStore() StateStore {
	return &memStore{
		states: make(map[string]*stateData),
	}
}

func (m *memStore) Create(ctx context.Context, id string) (*StateMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.states[id]; exists {
		return nil, ErrAlreadyExists
	}
	
	metadata := &StateMetadata{
		ID:      id,
		Size:    0,
		Updated: time.Now(),
		Locked:  false,
	}
	
	m.states[id] = &stateData{
		metadata: metadata,
		content:  []byte{},
	}
	
	return metadata, nil
}

func (m *memStore) Get(ctx context.Context, id string) (*StateMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	state, exists := m.states[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	// Return a copy to avoid mutations
	return &StateMetadata{
		ID:       state.metadata.ID,
		Size:     state.metadata.Size,
		Updated:  state.metadata.Updated,
		Locked:   state.metadata.Locked,
		LockInfo: state.metadata.LockInfo,
	}, nil
}

func (m *memStore) List(ctx context.Context, prefix string) ([]*StateMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var results []*StateMetadata
	for id, state := range m.states {
		if prefix == "" || strings.HasPrefix(id, prefix) {
			// Return copies
			results = append(results, &StateMetadata{
				ID:       state.metadata.ID,
				Size:     state.metadata.Size,
				Updated:  state.metadata.Updated,
				Locked:   state.metadata.Locked,
				LockInfo: state.metadata.LockInfo,
			})
		}
	}
	
	return results, nil
}

func (m *memStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.states[id]; !exists {
		return ErrNotFound
	}
	
	delete(m.states, id)
	return nil
}

func (m *memStore) Download(ctx context.Context, id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	state, exists := m.states[id]
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
	
	state, exists := m.states[id]
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
	
	state, exists := m.states[id]
	if !exists {
		return ErrNotFound
	}
	
	if state.metadata.Locked {
		return fmt.Errorf("%w: state already locked by %s", ErrLockConflict, state.metadata.LockInfo.ID)
	}
	
	state.metadata.Locked = true
	state.metadata.LockInfo = info
	
	return nil
}

func (m *memStore) Unlock(ctx context.Context, id string, lockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state, exists := m.states[id]
	if !exists {
		return ErrNotFound
	}
	
	if !state.metadata.Locked {
		return fmt.Errorf("state is not locked")
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
	
	state, exists := m.states[id]
	if !exists {
		return nil, ErrNotFound
	}
	
	if !state.metadata.Locked {
		return nil, nil
	}
	
	return state.metadata.LockInfo, nil
}