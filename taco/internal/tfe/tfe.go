package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// TfeHandler implements Terraform Cloud/Enterprise API.
// Uses TFEOperations interface (6 methods) - cannot create, list, delete, or manage versions.
type TfeHandler struct {
	authHandler *auth.Handler
	stateStore  domain.TFEOperations  // Scoped to TFE operations 
	rbacManager *rbac.RBACManager
    apiTokens   *auth.APITokenManager
}

// NewTFETokenHandler creates a new TFE handler.
// Accepts full repository for state ops and blob store for API token storage.
func NewTFETokenHandler(authHandler *auth.Handler, fullRepo domain.UnitRepository, blobStore storage.UnitStore, rbacManager *rbac.RBACManager) *TfeHandler {
	// Scope to TFE operations for state management (handler only uses Get, Upload, Lock methods)
	stateStore := domain.TFEOperations(fullRepo)
	
	return &TfeHandler{
		authHandler: authHandler,
		stateStore:  stateStore,
		rbacManager: rbacManager,
        apiTokens:   auth.NewAPITokenManagerFromStore(blobStore),  // Use blob store for token storage
	}
}
