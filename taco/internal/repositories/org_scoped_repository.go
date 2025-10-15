package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

var (
	// ErrOrgRequired is returned when an operation requires org context but it's missing
	ErrOrgRequired = errors.New("organization context required")
	// ErrOrgMismatch is returned when a unit doesn't belong to the user's organization
	ErrOrgMismatch = errors.New("unit does not belong to organization")
)

// orgScopedRepository wraps a repository and enforces org-based isolation
// This follows the decorator pattern used by authorizingRepository for RBAC
// All operations automatically validate org ownership or scope to org namespace
type orgScopedRepository struct {
	underlying domain.UnitRepository
	orgService *domain.OrgService
}

// NewOrgScopedRepository creates a repository wrapper that enforces org isolation
// This decorator ensures:
// - List operations are automatically filtered to org namespace
// - All other operations validate that units belong to the org
// - Org context is required in all requests (fails fast if missing)
func NewOrgScopedRepository(repo domain.UnitRepository, orgService *domain.OrgService) domain.UnitRepository {
	return &orgScopedRepository{
		underlying: repo,
		orgService: orgService,
	}
}

// validateOrgAccess checks if the operation is allowed for the org in context
// Returns ErrOrgRequired if org context is missing
// Returns ErrOrgMismatch if unit doesn't belong to the org
func (r *orgScopedRepository) validateOrgAccess(ctx context.Context, unitID string) error {
	org, ok := domain.OrgFromContext(ctx)
	if !ok {
		return ErrOrgRequired
	}
	
	if err := r.orgService.ValidateUnitBelongsToOrg(unitID, org.OrgID); err != nil {
		return ErrOrgMismatch
	}
	
	return nil
}

// ============================================
// StateOperations Implementation (6 methods)
// ============================================

func (r *orgScopedRepository) Get(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return nil, err
	}
	return r.underlying.Get(ctx, id)
}

func (r *orgScopedRepository) Download(ctx context.Context, id string) ([]byte, error) {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return nil, err
	}
	return r.underlying.Download(ctx, id)
}

func (r *orgScopedRepository) GetLock(ctx context.Context, id string) (*storage.LockInfo, error) {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return nil, err
	}
	return r.underlying.GetLock(ctx, id)
}

func (r *orgScopedRepository) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return err
	}
	return r.underlying.Upload(ctx, id, data, lockID)
}

func (r *orgScopedRepository) Lock(ctx context.Context, id string, info *storage.LockInfo) error {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return err
	}
	return r.underlying.Lock(ctx, id, info)
}

func (r *orgScopedRepository) Unlock(ctx context.Context, id string, lockID string) error {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return err
	}
	return r.underlying.Unlock(ctx, id, lockID)
}

// ============================================
// UnitManagement Additional Methods (5 methods)
// ============================================

func (r *orgScopedRepository) Create(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return nil, err
	}
	return r.underlying.Create(ctx, id)
}

// List automatically filters results to the organization's namespace
// This is the key method for org isolation - users can only see their org's units
func (r *orgScopedRepository) List(ctx context.Context, prefix string) ([]*storage.UnitMetadata, error) {
	org, ok := domain.OrgFromContext(ctx)
	if !ok {
		return nil, ErrOrgRequired
	}
	
	// Automatically scope the prefix to the organization
	// This ensures even admins can only see their org's units
	orgScopedPrefix := r.orgService.GetOrgScopedPrefix(org.OrgID, prefix)
	
	return r.underlying.List(ctx, orgScopedPrefix)
}

func (r *orgScopedRepository) Delete(ctx context.Context, id string) error {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return err
	}
	return r.underlying.Delete(ctx, id)
}

func (r *orgScopedRepository) ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error) {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return nil, err
	}
	return r.underlying.ListVersions(ctx, id)
}

func (r *orgScopedRepository) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	if err := r.validateOrgAccess(ctx, id); err != nil {
		return err
	}
	return r.underlying.RestoreVersion(ctx, id, versionTimestamp, lockID)
}

