package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
)

// TfeHandler implements Terraform Cloud/Enterprise API.
// Uses TFEOperations interface (7 methods) - cannot list, delete, or manage versions.
type TfeHandler struct {
	authHandler *auth.Handler
	stateStore  domain.TFEOperations  // Scoped to TFE operations only!
	rbacManager *rbac.RBACManager
    apiTokens   *auth.APITokenManager
}

// NewTFETokenHandler creates a new TFE handler.
// Accepts full repository (not scoped interface) because API tokens need storage.
func NewTFETokenHandler(authHandler *auth.Handler, fullRepo domain.UnitRepository, rbacManager *rbac.RBACManager) *TfeHandler {
	// Scope to TFE operations for state management (handler only uses Create, Get, Upload, Lock methods)
	stateStore := domain.TFEOperations(fullRepo)
	
	return &TfeHandler{
		authHandler: authHandler,
		stateStore:  stateStore,
		rbacManager: rbacManager,
        apiTokens:   auth.NewAPITokenManagerFromStore(fullRepo),  // Full repo for token storage
	}
}
