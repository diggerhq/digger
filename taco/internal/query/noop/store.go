package noop

import (
	"context"
	"errors"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
)

// NoOpQueryStore provides a disabled query backend that satisfies the Store interface.
type NoOpQueryStore struct{}

func NewNoOpQueryStore() *NoOpQueryStore {
	return &NoOpQueryStore{}
}

func (n *NoOpQueryStore) Close() error {
	return nil
}

func (n *NoOpQueryStore) IsEnabled() bool {
	return false
}

var errDisabled = errors.New("query store is disabled")

// UnitQuery implementation (no-op)
func (n *NoOpQueryStore) ListUnits(ctx context.Context, prefix string) ([]types.Unit, error) {
	return nil, errDisabled
}
func (n *NoOpQueryStore) GetUnit(ctx context.Context, id string) (*types.Unit, error) {
	return nil, errDisabled
}
func (n *NoOpQueryStore) SyncEnsureUnit(ctx context.Context, unitName string) error {
	return errDisabled
}
func (n *NoOpQueryStore) SyncDeleteUnit(ctx context.Context, unitName string) error {
	return errDisabled
}

// RBACQuery implementation (no-op)
func (n *NoOpQueryStore) FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error) {
	return nil, errDisabled
}
func (n *NoOpQueryStore) ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error) {
	return nil, errDisabled
}
