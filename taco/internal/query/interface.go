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
	ListUnits(ctx context.Context, prefix string) ([]types.Unit,error)
	GetUnit(ctx context.Context, id string) (*types.Unit, error)
    SyncCreateUnit(ctx context.Context, unitName string) error
    SyncDeleteUnit(ctx context.Context, unitName string) error
    SyncUnitExists(ctx context.Context, unitName string) error
}


type RBACQuery interface {
	FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error)
	ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error)
}


func SupportsUnitQuery(store QueryStore) (UnitQuery, bool) {
	q, ok := store.(UnitQuery)

	return q,ok 

}


func SupportsRBACQuery(store QueryStore) (RBACQuery, bool) {
	q,ok := store.(RBACQuery)

	return q,ok 
}

