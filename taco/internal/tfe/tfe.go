package tfe

import (
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
)

type TfeHandler struct {
	authHandler *auth.Handler
	stateStore  storage.UnitStore
	rbacManager *rbac.RBACManager
    apiTokens   *auth.APITokenManager
}

func NewTFETokenHandler(authHandler *auth.Handler, stateStore storage.UnitStore, rbacManager *rbac.RBACManager) *TfeHandler {
	return &TfeHandler{
		authHandler: authHandler,
		stateStore:  stateStore,
		rbacManager: rbacManager,
        apiTokens:   auth.NewAPITokenManagerFromStore(stateStore),
	}
}
