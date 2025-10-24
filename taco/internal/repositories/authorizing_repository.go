package repositories

import (
	"context"
	"log/slog"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// authorizingRepository wraps a repository with RBAC enforcement for all operations.
// This decorator checks permissions before every operation.
//
// IMPORTANT: RBAC checks require database access (permissions, roles, user assignments).
// If the database is unavailable, all operations will fail closed (deny access).
// This is a security-first design - better to deny access than to incorrectly allow it.
type authorizingRepository struct {
	underlying domain.UnitRepository
	rbac       *rbac.RBACManager
}

// NewAuthorizingRepository wraps a repository with RBAC enforcement.
// All operations will be checked against RBAC rules stored in the database.
// If the database is unavailable, operations will fail (fail-closed behavior).
func NewAuthorizingRepository(repo domain.UnitRepository, rbacMgr *rbac.RBACManager) domain.UnitRepository {
	return &authorizingRepository{
		underlying: repo,
		rbac:       rbacMgr,
	}
}

// ============================================
// StateOperations (6 methods)
// ============================================

func (a *authorizingRepository) Get(ctx context.Context, id string) (*storage.UnitMetadata, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitRead, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, storage.ErrForbidden
	}

	return a.underlying.Get(ctx, id)
}

func (a *authorizingRepository) Download(ctx context.Context, id string) ([]byte, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitRead, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, storage.ErrForbidden
	}

	return a.underlying.Download(ctx, id)
}

func (a *authorizingRepository) GetLock(ctx context.Context, id string) (*storage.LockInfo, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitRead, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, storage.ErrForbidden
	}

	return a.underlying.GetLock(ctx, id)
}

func (a *authorizingRepository) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitWrite, id)
	if err != nil {
		return err
	}
	if !allowed {
		return storage.ErrForbidden
	}

	return a.underlying.Upload(ctx, id, data, lockID)
}

func (a *authorizingRepository) Lock(ctx context.Context, id string, info *storage.LockInfo) error {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitLock, id)
	if err != nil {
		return err
	}
	if !allowed {
		return storage.ErrForbidden
	}

	return a.underlying.Lock(ctx, id, info)
}

func (a *authorizingRepository) Unlock(ctx context.Context, id string, lockID string) error {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitLock, id)
	if err != nil {
		return err
	}
	if !allowed {
		return storage.ErrForbidden
	}

	return a.underlying.Unlock(ctx, id, lockID)
}

// ============================================
// UnitManagement additional methods (5 methods)
// ============================================

func (a *authorizingRepository) Create(ctx context.Context, orgID string, name string) (*storage.UnitMetadata, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitWrite, name)
	if err != nil {
		slog.Error("RBAC check error",
			"operation", "Create",
			"principal", principal.Subject,
			"action", "unit:write",
			"resource", name,
			"error", err)
		return nil, err
	}
	if !allowed {
		slog.Warn("RBAC check denied",
			"operation", "Create",
			"principal", principal.Subject,
			"action", "unit:write",
			"resource", name)
		return nil, storage.ErrForbidden
	}
	
	slog.Debug("RBAC check allowed",
		"operation", "Create",
		"principal", principal.Subject,
		"action", "unit:write",
		"resource", name)

	return a.underlying.Create(ctx, orgID, name)
}

func (a *authorizingRepository) List(ctx context.Context, orgID string, prefix string) ([]*storage.UnitMetadata, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	// Get all units
	allUnits, err := a.underlying.List(ctx, orgID, prefix)
	if err != nil {
		return nil, err
	}

	// Filter by read access
	unitIDs := make([]string, len(allUnits))
	for i, u := range allUnits {
		unitIDs[i] = u.ID
	}

	filteredIDs, err := a.rbac.FilterUnitsByReadAccess(ctx, principal, unitIDs)
	if err != nil {
		return nil, err
	}

	// Build result with only authorized units
	filtered := make([]*storage.UnitMetadata, 0, len(filteredIDs))
	idSet := make(map[string]bool, len(filteredIDs))
	for _, id := range filteredIDs {
		idSet[id] = true
	}
	for _, u := range allUnits {
		if idSet[u.ID] {
			filtered = append(filtered, u)
		}
	}

	return filtered, nil
}

func (a *authorizingRepository) Delete(ctx context.Context, id string) error {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitDelete, id)
	if err != nil {
		return err
	}
	if !allowed {
		return storage.ErrForbidden
	}

	return a.underlying.Delete(ctx, id)
}

func (a *authorizingRepository) ListVersions(ctx context.Context, id string) ([]*storage.VersionInfo, error) {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return nil, storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitRead, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, storage.ErrForbidden
	}

	return a.underlying.ListVersions(ctx, id)
}

func (a *authorizingRepository) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	principal, ok := rbac.PrincipalFromContext(ctx)
	if !ok {
		return storage.ErrUnauthorized
	}

	allowed, err := a.rbac.Can(ctx, principal, rbac.ActionUnitWrite, id)
	if err != nil {
		return err
	}
	if !allowed {
		return storage.ErrForbidden
	}

	return a.underlying.RestoreVersion(ctx, id, versionTimestamp, lockID)
}
