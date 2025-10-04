package query

import (
	"context"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
)

type QueryStore interface {
	Close() error
	IsEnabled() bool
}

type UnitQuery interface {
	ListUnits(ctx context.Context, prefix string) ([]types.Unit, error)
	GetUnit(ctx context.Context, id string) (*types.Unit, error)
	SyncEnsureUnit(ctx context.Context, unitName string) error
	SyncDeleteUnit(ctx context.Context, unitName string) error
}

type RBACQuery interface {
	FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error)
	ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error)
}

type Store interface {
	QueryStore
	UnitQuery
	RBACQuery
}


