package storage

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrLockConflict  = errors.New("lock conflict")
)

type UnitMetadata struct {
	ID       string    `json:"id"`
	Size     int64     `json:"size"`
	Updated  time.Time `json:"updated"`
	Locked   bool      `json:"locked"`
	LockInfo *LockInfo `json:"lock,omitempty"`
}

type VersionInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	S3Key     string    `json:"s3_key"`
}

type LockInfo struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

type UnitStore interface {
	// Unit operations
	Create(ctx context.Context, id string) (*UnitMetadata, error)
	Get(ctx context.Context, id string) (*UnitMetadata, error)
	List(ctx context.Context, prefix string) ([]*UnitMetadata, error)
	Delete(ctx context.Context, id string) error

	// Data operations
	Download(ctx context.Context, id string) ([]byte, error)
	Upload(ctx context.Context, id string, data []byte, lockID string) error

	// Lock operations
	Lock(ctx context.Context, id string, info *LockInfo) error
	Unlock(ctx context.Context, id string, lockID string) error
	GetLock(ctx context.Context, id string) (*LockInfo, error)

	// Version operations
	ListVersions(ctx context.Context, id string) ([]*VersionInfo, error)
	RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error
}

// S3Store extends UnitStore with S3-specific methods for RBAC integration
type S3Store interface {
	UnitStore
	GetS3Client() *s3.Client
	GetS3Bucket() string
	GetS3Prefix() string
	Key(parts ...string) string
}
