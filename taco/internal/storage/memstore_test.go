package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemStore_Create(t *testing.T) {
	store := NewMemStore()
	ctx := context.Background()

	// Test creating a new state
	metadata, err := store.Create(ctx, "test/state")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metadata.ID != "test/state" {
		t.Errorf("expected ID 'test/state', got %s", metadata.ID)
	}

	// Test creating duplicate state
	_, err = store.Create(ctx, "test/state")
	if err != ErrAlreadyExists {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestMemStore_Lock(t *testing.T) {
	store := NewMemStore()
	ctx := context.Background()

	// Create a state first
	_, err := store.Create(ctx, "test/state")
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	// Lock the state
	lockInfo := &LockInfo{
		ID:      "lock-123",
		Who:     "test",
		Version: "1.0.0",
		Created: time.Now(),
	}

	err = store.Lock(ctx, "test/state", lockInfo)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Try to lock again
	err = store.Lock(ctx, "test/state", &LockInfo{ID: "lock-456"})
	if !errors.Is(err, ErrLockConflict) {
		t.Errorf("expected ErrLockConflict, got %v", err)
	}

	// Unlock with wrong ID
	err = store.Unlock(ctx, "test/state", "wrong-id")
	if !errors.Is(err, ErrLockConflict) {
		t.Errorf("expected ErrLockConflict, got %v", err)
	}

	// Unlock with correct ID
	err = store.Unlock(ctx, "test/state", "lock-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMemStore_Upload(t *testing.T) {
	store := NewMemStore()
	ctx := context.Background()

	// Create a state
	_, err := store.Create(ctx, "test/state")
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	// Upload data
	data := []byte(`{"version": 4, "terraform_version": "1.0.0"}`)
	err = store.Upload(ctx, "test/state", data, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Download and verify
	downloaded, err := store.Download(ctx, "test/state")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(downloaded) != string(data) {
		t.Errorf("expected data %s, got %s", string(data), string(downloaded))
	}

	// Get metadata and check size
	metadata, err := store.Get(ctx, "test/state")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metadata.Size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), metadata.Size)
	}
}