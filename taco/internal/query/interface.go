package query

import (
	"context"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"time"
)

type QueryStore interface {
	Close() error
	IsEnabled() bool
}

type UnitQuery interface {
	ListUnits(ctx context.Context, prefix string) ([]types.Unit, error)
	GetUnit(ctx context.Context, id string) (*types.Unit, error)
	SyncEnsureUnit(ctx context.Context, unitName string) error
	SyncUnitMetadata(ctx context.Context, unitName string, size int64, updated time.Time) error
	SyncUnitLock(ctx context.Context, unitName string, lockID, lockWho string, lockCreated time.Time) error
	SyncUnitUnlock(ctx context.Context, unitName string) error
	SyncDeleteUnit(ctx context.Context, unitName string) error
}

type RBACQuery interface {
	FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error)
	ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error)
	CanPerformAction(ctx context.Context, userSubject string, action string, resourceID string) (bool, error)
	HasRBACRoles(ctx context.Context) (bool, error)
	
	SyncPermission(ctx context.Context, permission interface{}) error
	SyncRole(ctx context.Context, role interface{}) error
	SyncUser(ctx context.Context, user interface{}) error
}

type Store interface {
	QueryStore
	UnitQuery
	RBACQuery
}


