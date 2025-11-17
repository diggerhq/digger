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
	authHandler        *auth.Handler
	stateStore         domain.TFEOperations  // RBAC-wrapped for authenticated operations
	directStateStore   domain.TFEOperations  // Direct access for pre-authorized operations (signed URLs)
	rbacManager        *rbac.RBACManager
	apiTokens          *auth.APITokenManager
	identifierResolver domain.IdentifierResolver // For resolving org external IDs
	
	// TFE repositories for runs, plans, and configuration versions
	runRepo        domain.TFERunRepository
	planRepo       domain.TFEPlanRepository
	configVerRepo  domain.TFEConfigurationVersionRepository
	blobStore      storage.UnitStore
	unitRepo       domain.UnitRepository  // Direct access for locking during plan/apply
}

// NewTFETokenHandler creates a new TFE handler.
// Accepts wrapped (RBAC-enforced) and unwrapped (direct) repositories.
// The unwrapped repository is used for signed URL operations which are pre-authorized.
func NewTFETokenHandler(
	authHandler *auth.Handler,
	wrappedRepo domain.UnitRepository,
	unwrappedRepo domain.UnitRepository,
	blobStore storage.UnitStore,
	rbacManager *rbac.RBACManager,
	identifierResolver domain.IdentifierResolver,
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
) *TfeHandler {
	return &TfeHandler{
		authHandler:        authHandler,
		stateStore:         domain.TFEOperations(wrappedRepo),
		directStateStore:   domain.TFEOperations(unwrappedRepo),
		rbacManager:        rbacManager,
		apiTokens:          auth.NewAPITokenManagerFromStore(blobStore),
		identifierResolver: identifierResolver,
		runRepo:            runRepo,
		planRepo:           planRepo,
		configVerRepo:      configVerRepo,
		blobStore:          blobStore,
		unitRepo:           unwrappedRepo,  // Use unwrapped repo for direct lock access
	}
}
