package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var (
	ErrOrgExists    = errors.New("organization already exists")
	ErrOrgNotFound  = errors.New("organization not found")
	ErrInvalidOrgID = errors.New("invalid organization ID format")
)

// OrgIDPattern defines valid organization ID format: alphanumeric, hyphens, underscores
var OrgIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$`)

// ============================================
// Domain Models
// ============================================

// Organization represents an organization in the domain layer
// This is the domain model, separate from database entities
type Organization struct {
	ID        int64
	OrgID     string
	Name      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ============================================
// Repository Interface
// ============================================

// OrganizationRepository defines the interface for organization data access
// Implementations live in the repositories package
type OrganizationRepository interface {
	Create(ctx context.Context, orgID, name, createdBy string) (*Organization, error)
	Get(ctx context.Context, orgID string) (*Organization, error)
	List(ctx context.Context) ([]*Organization, error)
	Delete(ctx context.Context, orgID string) error

	WithTransaction(ctx context.Context, fn func(ctx context.Context, txRepo OrganizationRepository) error) error
}

// ============================================
// User Management
// ============================================

// User represents a user in the domain layer
type User struct {
	ID        int64
	Subject   string // Unique identifier (email, auth0 ID, etc.)
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	// Create or get a user (idempotent)
	EnsureUser(ctx context.Context, subject, email string) (*User, error)

	// Get user by subject
	Get(ctx context.Context, subject string) (*User, error)

	// Get user by email
	GetByEmail(ctx context.Context, email string) (*User, error)

	// List all users
	List(ctx context.Context) ([]*User, error)
}

// ============================================
// Domain Validation
// ============================================

// ValidateOrgID checks if an organization ID is valid
// This is pure domain logic with no infrastructure dependencies
func ValidateOrgID(orgID string) error {
	if len(orgID) < 3 {
		return fmt.Errorf("%w: must be at least 3 characters", ErrInvalidOrgID)
	}
	if len(orgID) > 50 {
		return fmt.Errorf("%w: must be at most 50 characters", ErrInvalidOrgID)
	}
	if !OrgIDPattern.MatchString(orgID) {
		return fmt.Errorf("%w: must contain only letters, numbers, hyphens, and underscores", ErrInvalidOrgID)
	}
	return nil
}
