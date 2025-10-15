package domain

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// ============================================
// Core State Operations
// ============================================

// StateOperations defines the common operations most handlers need.
// This is the practical interface for backend, S3-compat, and TFE handlers.
// Includes read, write, and lock operations but NOT list or admin operations.
type StateOperations interface {
	// Read operations
	Get(ctx context.Context, id string) (*storage.UnitMetadata, error)
	Download(ctx context.Context, id string) ([]byte, error)
	GetLock(ctx context.Context, id string) (*storage.LockInfo, error)
	
	// Write operations
	Upload(ctx context.Context, id string, data []byte, lockID string) error
	
	// Lock operations
	Lock(ctx context.Context, id string, info *storage.LockInfo) error
	Unlock(ctx context.Context, id string, lockID string) error
}

// ============================================
// Management Operations
// ============================================

// UnitManagement extends StateOperations with admin/management operations.
// This is for the unit management API that needs full CRUD + versioning.
type UnitManagement interface {
	StateOperations
	
	// Admin operations
	Create(ctx context.Context, id string) (*storage.UnitMetadata, error)
	List(ctx context.Context, prefix string) ([]*storage.UnitMetadata, error)
	Delete(ctx context.Context, id string) error
	
	// Version operations
	ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error)
	RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error
}

// ============================================
// TFE-Specific Operations
// ============================================

// TFEOperations defines what TFE handler needs.
// TFE needs read/write/lock operations only
type TFEOperations interface {
	StateOperations
}

// ============================================
// Full Repository Interface
// ============================================

// UnitRepository provides all unit storage and management operations.
// This is the primary interface that concrete repositories implement.
// Handlers receive scoped interfaces (StateOperations, TFEOperations, UnitManagement)
// depending on what operations they need.
type UnitRepository interface {
	UnitManagement
}

// ============================================
// API Response Models
// ============================================
// These types define the API contract for JSON responses.
// They are separate from internal storage types to allow the API format
// to evolve independently from storage implementation.

// Unit represents a Terraform state unit in API responses
type Unit struct {
	ID       string `json:"id"`
	Size     int64  `json:"size"`
	Updated  time.Time `json:"updated"`
	Locked   bool   `json:"locked"`
	LockInfo *Lock  `json:"lock_info,omitempty"`
}

// Lock represents a Terraform state lock in API responses
type Lock struct {
	ID      string    `json:"id"`
	Who     string    `json:"who"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

// Version represents a state version in API responses
type Version struct {
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
}

// SortUnitsByID sorts units by their ID for consistent API responses
func SortUnitsByID(units []*Unit) {
	sort.Slice(units, func(i, j int) bool {
		return units[i].ID < units[j].ID
	})
}

// ============================================
// Utility Functions
// ============================================

// ValidateUnitID validates that a unit ID is safe and doesn't contain path traversal
func ValidateUnitID(id string) error {
	normalized := NormalizeUnitID(id)
	if normalized == "" {
		return errors.New("unit ID cannot be empty")
	}
	if strings.Contains(id, "..") {
		return errors.New("unit ID cannot contain '..'")
	}
	return nil
}

// NormalizeUnitID normalizes a unit ID by removing leading/trailing slashes and collapsing multiple slashes
func NormalizeUnitID(id string) string {
	s := strings.TrimSpace(id)
	s = strings.Trim(s, "/")
	
	// Collapse multiple slashes
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	
	return s
}

// DecodeUnitID decodes a URL-encoded unit ID (currently just normalizes)
func DecodeUnitID(encoded string) string {
	return NormalizeUnitID(encoded)
}

