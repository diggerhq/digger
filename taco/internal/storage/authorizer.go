package storage

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/principal"
	"github.com/diggerhq/digger/opentaco/internal/query"
)

// principalKey is a private type to prevent key collisions in context.
type principalKey string

const userPrincipalKey principalKey = "user"

// ContextWithPrincipal returns a new context with the given user principal.
func ContextWithPrincipal(ctx context.Context, p principal.Principal) context.Context {
	return context.WithValue(ctx, userPrincipalKey, p)
}

// principalFromContext retrieves the user principal from the context.
func principalFromContext(ctx context.Context) (principal.Principal, error) {
	p := ctx.Value(userPrincipalKey)
	if p == nil {
		return principal.Principal{}, errors.New("no user principal in context")
	}
	pr, ok := p.(principal.Principal)
	if !ok {
		return principal.Principal{}, errors.New("invalid user principal type in context")
	}
	return pr, nil
}

// AuthorizingStore is a decorator that enforces role-based access control
// on an underlying UnitStore.
type AuthorizingStore struct {
	nextStore  UnitStore   // The next store in the chain (e.g., OrchestratingStore)
	queryStore query.Store // Needed to perform the RBAC checks
}

// NewAuthorizingStore creates a new store that wraps another with an authorization layer.
func NewAuthorizingStore(next UnitStore, qs query.Store) UnitStore {
	return &AuthorizingStore{
		nextStore:  next,
		queryStore: qs,
	}
}

// List intercepts the call and returns only the units the user is permitted to see.
func (s *AuthorizingStore) List(ctx context.Context, prefix string) ([]*UnitMetadata, error) {
	principal, err := principalFromContext(ctx)
	if err != nil {
		log.Printf("DEBUG AuthorizingStore.List: Failed to get principal from context: %v", err)
		return nil, errors.New("unauthorized")
	}
	
	log.Printf("DEBUG AuthorizingStore.List: Got principal: %+v", principal)

	// Use the optimized query that fetches ONLY the units the user is allowed to see.
	units, err := s.queryStore.ListUnitsForUser(ctx, principal.Subject, prefix)
	if err != nil {
		log.Printf("DEBUG AuthorizingStore.List: ListUnitsForUser failed: %v", err)
		return nil, err
	}
	
	log.Printf("DEBUG AuthorizingStore.List: Found %d units for user %s", len(units), principal.Subject)


	metadata := make([]*UnitMetadata, len(units))
	for i, u := range units {
		log.Printf("DEBUG: DB Unit: Name=%s, Size=%d, UpdatedAt=%v, Locked=%v", u.Name, u.Size, u.UpdatedAt, u.Locked)
		
		var lockInfo *LockInfo
		if u.Locked {
			lockInfo = &LockInfo{
				ID:      u.LockID,
				Who:     u.LockWho,
				Created: u.LockCreated,
			}
		}
		metadata[i] = &UnitMetadata{
			ID:       u.Name,
			Size:     u.Size,
			Updated:  u.UpdatedAt,
			Locked:   u.Locked,
			LockInfo: lockInfo,
		}
		log.Printf("DEBUG: Mapped Metadata: ID=%s, Size=%d, Updated=%v", metadata[i].ID, metadata[i].Size, metadata[i].Updated)
	}
	
	return metadata, nil
}

// checkPermission is a new helper to centralize permission checks.
func (s *AuthorizingStore) checkPermission(ctx context.Context, action, unitID string) error {
	principal, err := principalFromContext(ctx)
	if err != nil {
		return errors.New("unauthorized")
	}

	allowed, err := s.queryStore.CanPerformAction(ctx, principal.Subject, action, unitID)
	if err != nil {
		log.Printf("RBAC check failed for user '%s', action '%s' on unit '%s': %v", principal.Subject, action, unitID, err)
		return errors.New("internal authorization error")
	}
	if !allowed {
		return errors.New("forbidden")
	}
	return nil
}

// Get checks for 'unit.read' permission.
func (s *AuthorizingStore) Get(ctx context.Context, id string) (*UnitMetadata, error) {
	if err := s.checkPermission(ctx, "unit.read", id); err != nil {
		return nil, err
	}
	return s.nextStore.Get(ctx, id)
}

// Download checks for 'unit.read' permission.
func (s *AuthorizingStore) Download(ctx context.Context, id string) ([]byte, error) {
	if err := s.checkPermission(ctx, "unit.read", id); err != nil {
		return nil, err
	}
	return s.nextStore.Download(ctx, id)
}

// Create checks for 'unit.write' permission.
func (s *AuthorizingStore) Create(ctx context.Context, id string) (*UnitMetadata, error) {
	if err := s.checkPermission(ctx, "unit.write", id); err != nil {
		return nil, err
	}
	return s.nextStore.Create(ctx, id)
}

// Upload checks for 'unit.write' permission.
func (s *AuthorizingStore) Upload(ctx context.Context, id string, data []byte, lockID string) error {
	if err := s.checkPermission(ctx, "unit.write", id); err != nil {
		return err
	}
	return s.nextStore.Upload(ctx, id, data, lockID)
}

// Delete checks for 'unit.delete' permission.
func (s *AuthorizingStore) Delete(ctx context.Context, id string) error {
	if err := s.checkPermission(ctx, "unit.delete", id); err != nil {
		return err
	}
	return s.nextStore.Delete(ctx, id)
}

// Lock checks for 'unit.lock' permission.
func (s *AuthorizingStore) Lock(ctx context.Context, id string, info *LockInfo) error {
	if err := s.checkPermission(ctx, "unit.lock", id); err != nil {
		return err
	}
	err := s.nextStore.Lock(ctx, id, info)
	if err != nil {
		return err
	}
	
	// Sync lock status to database
	if err := s.queryStore.SyncUnitLock(ctx, id, info.ID, info.Who, info.Created); err != nil {
		log.Printf("Warning: Failed to sync lock status for unit '%s': %v", id, err)
	}
	return nil
}

// Unlock checks for 'unit.lock' permission.
func (s *AuthorizingStore) Unlock(ctx context.Context, id string, lockID string) error {
	if err := s.checkPermission(ctx, "unit.lock", id); err != nil {
		return err
	}
	err := s.nextStore.Unlock(ctx, id, lockID)
	if err != nil {
		return err
	}
	
	// Sync unlock status to database
	if err := s.queryStore.SyncUnitUnlock(ctx, id); err != nil {
		log.Printf("Warning: Failed to sync unlock status for unit '%s': %v", id, err)
	}
	return nil
}

// --- Other Pass-through Methods with Read Checks ---
func (s *AuthorizingStore) GetLock(ctx context.Context, id string) (*LockInfo, error) {
	if err := s.checkPermission(ctx, "unit.read", id); err != nil {
		return nil, err
	}
	return s.nextStore.GetLock(ctx, id)
}

func (s *AuthorizingStore) ListVersions(ctx context.Context, id string) ([]*VersionInfo, error) {
	if err := s.checkPermission(ctx, "unit.read", id); err != nil {
		return nil, err
	}
	return s.nextStore.ListVersions(ctx, id)
}

func (s *AuthorizingStore) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
	if err := s.checkPermission(ctx, "unit.write", id); err != nil {
		return err
	}
	return s.nextStore.RestoreVersion(ctx, id, versionTimestamp, lockID)
}