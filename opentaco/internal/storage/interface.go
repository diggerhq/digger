package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrLockConflict = errors.New("lock conflict")
)

type StateMetadata struct {
	ID       string    `json:"id"`
	Size     int64     `json:"size"`
	Updated  time.Time `json:"updated"`
	Locked   bool      `json:"locked"`
	LockInfo *LockInfo `json:"lock,omitempty"`
}

type LockInfo struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

type StateStore interface {
	// State operations
	Create(ctx context.Context, id string) (*StateMetadata, error)
	Get(ctx context.Context, id string) (*StateMetadata, error)
	List(ctx context.Context, prefix string) ([]*StateMetadata, error)
	Delete(ctx context.Context, id string) error
	
	// Data operations
	Download(ctx context.Context, id string) ([]byte, error)
	Upload(ctx context.Context, id string, data []byte, lockID string) error
	
	// Lock operations
	Lock(ctx context.Context, id string, info *LockInfo) error
	Unlock(ctx context.Context, id string, lockID string) error
	GetLock(ctx context.Context, id string) (*LockInfo, error)
}