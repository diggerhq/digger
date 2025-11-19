package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrLockConflict  = errors.New("lock conflict")
	ErrForbidden     = errors.New("forbidden")
	ErrUnauthorized  = errors.New("unauthorized")
)

type UnitMetadata struct {
    ID       string    `json:"id"`
    Name     string    `json:"name"`
    OrgID    string    `json:"org_id"`
    OrgName  string    `json:"org_name"`
    Size     int64     `json:"size"`
    Updated  time.Time `json:"updated"`
    Locked   bool      `json:"locked"`
    LockInfo *LockInfo `json:"lock,omitempty"`
    
    // TFE workspace settings (nullable for non-TFE usage)
    TFEAutoApply        *bool   `json:"tfe_auto_apply,omitempty"`
    TFETerraformVersion *string `json:"tfe_terraform_version,omitempty"`
    TFEEngine           *string `json:"tfe_engine,omitempty"` // 'terraform' or 'tofu'
    TFEWorkingDirectory *string `json:"tfe_working_directory,omitempty"`
    TFEExecutionMode    *string `json:"tfe_execution_mode,omitempty"` // 'remote', 'local', 'agent'
    LockID              string  `json:"lock_id,omitempty"`
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
	DownloadBlob(ctx context.Context, key string) ([]byte, error)     // For non-state files (no /terraform.tfstate suffix)
	UploadBlob(ctx context.Context, key string, data []byte) error     // For non-state files (no lock checks)
	
	// Lock operations
	Lock(ctx context.Context, id string, info *LockInfo) error
	Unlock(ctx context.Context, id string, lockID string) error
	GetLock(ctx context.Context, id string) (*LockInfo, error)
	
	// Version operations
    ListVersions(ctx context.Context, id string) ([]*VersionInfo, error)
    RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error
}
