package unit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/deps"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler serves the management API (unit CRUD and locking).
type Handler struct {
	store       domain.UnitManagement
	blobStore   storage.UnitStore      // For legacy deps like ComputeUnitStatus
	rbacManager *rbac.RBACManager
	signer      *auth.Signer
	queryStore  query.Store
	resolver    domain.IdentifierResolver // Resolves names/identifiers to UUIDs
}

func NewHandler(store domain.UnitManagement, blobStore storage.UnitStore, rbacManager *rbac.RBACManager, signer *auth.Signer, queryStore query.Store, resolver domain.IdentifierResolver) *Handler {
	return &Handler{
		store:       store,
		blobStore:   blobStore,
		rbacManager: rbacManager,
		signer:      signer,
		queryStore:  queryStore,
		resolver:    resolver,
	}
}

// resolveUnitIdentifier resolves a unit identifier (name or UUID) to its UUID
func (h *Handler) resolveUnitIdentifier(ctx context.Context, identifier string) (string, error) {
	// URL-decode first (Echo params may be URL-encoded)
	decoded, err := domain.DecodeURLPath(identifier)
	if err != nil {
		return "", err
	}
	
	normalized := domain.DecodeUnitID(decoded)
	
	// If already a UUID, return as-is
	if domain.IsUUID(normalized) {
		return normalized, nil
	}
	
	// If resolver is not available, return normalized name (will fail at repository layer)
	if h.resolver == nil {
		return normalized, nil
	}
	
	// Get org from context for resolution
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		// No org context, return normalized name
		return normalized, nil
	}
	
	// Resolve name to UUID using the identifier resolver
	// Note: ResolveUnit signature is (ctx, identifier, orgID)
	uuid, err := h.resolver.ResolveUnit(ctx, normalized, orgCtx.OrgID)
	if err != nil {
		// If resolution fails, return error
		return "", err
	}
	
	return uuid, nil
}

type CreateUnitRequest struct {
	Name                 string  `json:"name"`
	TFEAutoApply         *bool   `json:"tfe_auto_apply"`
	TFEExecutionMode     *string `json:"tfe_execution_mode"`
	TFETerraformVersion  *string `json:"tfe_terraform_version"`
	TFEEngine            *string `json:"tfe_engine"`
	TFEWorkingDirectory  *string `json:"tfe_working_directory"`
}

type CreateUnitResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

func (h *Handler) CreateUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	var req CreateUnitRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind request body",
			"operation", "create_unit",
			"error", err,
		)
		analytics.SendEssential("unit_create_failed_invalid_request")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := domain.ValidateUnitID(req.Name); err != nil {
		logger.Error("Invalid unit name",
			"operation", "create_unit",
			"name", req.Name,
			"error", err,
		)
		analytics.SendEssential("unit_create_failed_invalid_name")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	name := domain.NormalizeUnitID(req.Name)

	// Get org UUID from domain context (set by middleware for both JWT and webhook routes)
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		logger.Error("Organization context missing",
			"operation", "create_unit",
			"name", name,
		)
		analytics.SendEssential("unit_create_failed_no_org_context")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	logger.Info("Creating unit",
		"operation", "create_unit",
		"name", name,
		"org_id", orgCtx.OrgID,
		"tfe_execution_mode", req.TFEExecutionMode,
	)

	metadata, err := h.store.Create(ctx, orgCtx.OrgID, name)
	if err != nil {
		if err == storage.ErrAlreadyExists {
			logger.Warn("Unit already exists",
				"operation", "create_unit",
				"name", name,
				"org_id", orgCtx.OrgID,
			)
			analytics.SendEssential("unit_create_failed_already_exists")
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Unit already exists",
				"detail": fmt.Sprintf("A unit with name '%s' already exists in this organization", name),
			})
		}
		logger.Error("Failed to create unit",
			"operation", "create_unit",
			"name", name,
			"org_id", orgCtx.OrgID,
			"error", err,
		)
		analytics.SendEssential("unit_create_failed_storage_error")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create unit",
			"detail": err.Error(),
		})
	}

	// Update TFE fields if provided (after unit creation)
	if req.TFEAutoApply != nil || req.TFEExecutionMode != nil || req.TFETerraformVersion != nil || req.TFEWorkingDirectory != nil {
		if h.queryStore != nil {
			if err := h.queryStore.UpdateUnitTFESettings(ctx, metadata.ID, req.TFEAutoApply, req.TFEExecutionMode, req.TFETerraformVersion, req.TFEEngine, req.TFEWorkingDirectory); err != nil {
				logger.Warn("Failed to update TFE settings for unit",
					"operation", "create_unit",
					"unit_id", metadata.ID,
					"error", err,
				)
				// Don't fail the request, just log the warning
			} else {
				logger.Info("Updated TFE settings for unit",
					"operation", "create_unit",
					"unit_id", metadata.ID,
					"tfe_execution_mode", req.TFEExecutionMode,
				)
			}
		}
	}

	logger.Info("Unit created successfully",
		"operation", "create_unit",
		"name", name,
		"unit_id", metadata.ID,
		"org_id", orgCtx.OrgID,
	)
	analytics.SendEssential("unit_created")
	return c.JSON(http.StatusCreated, CreateUnitResponse{ID: metadata.ID, Created: metadata.Updated})
}

type UpdateUnitRequest struct {
	TFEAutoApply         *bool   `json:"tfe_auto_apply"`
	TFEExecutionMode     *string `json:"tfe_execution_mode"`
	TFETerraformVersion  *string `json:"tfe_terraform_version"`
	TFEEngine            *string `json:"tfe_engine"`
	TFEWorkingDirectory  *string `json:"tfe_working_directory"`
}

func (h *Handler) UpdateUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	identifier := c.Param("id")

	// Get org UUID from domain context
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		logger.Error("Organization context missing", "operation", "update_unit")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	// Resolve unit identifier to UUID
	unitID, err := h.resolveUnitIdentifier(ctx, identifier)
	if err != nil {
		logger.Error("Failed to resolve unit identifier",
			"operation", "update_unit",
			"identifier", identifier,
			"error", err)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}

	logger.Info("Updating unit",
		"operation", "update_unit",
		"unit_id", unitID,
		"org_id", orgCtx.OrgID)

	// Verify unit exists and user has access
	_, err = h.store.Get(ctx, unitID)
	if err != nil {
		logger.Error("Failed to get unit",
			"operation", "update_unit",
			"unit_id", unitID,
			"error", err)
		if err.Error() == "unauthorized" || err.Error() == "forbidden" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}

	// Parse request body
	var req UpdateUnitRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to parse request body",
			"operation", "update_unit",
			"unit_id", unitID,
			"error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Update TFE settings if any are provided
	if req.TFEAutoApply != nil || req.TFEExecutionMode != nil || req.TFETerraformVersion != nil || req.TFEWorkingDirectory != nil {
		if h.queryStore != nil {
			if err := h.queryStore.UpdateUnitTFESettings(ctx, unitID, req.TFEAutoApply, req.TFEExecutionMode, req.TFETerraformVersion, req.TFEEngine, req.TFEWorkingDirectory); err != nil {
				logger.Error("Failed to update TFE settings for unit",
					"operation", "update_unit",
					"unit_id", unitID,
					"error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "Failed to update unit settings",
					"detail": err.Error(),
				})
			}
		}
	}

	logger.Info("Unit updated successfully",
		"operation", "update_unit",
		"unit_id", unitID,
		"org_id", orgCtx.OrgID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id": unitID,
		"message": "Unit updated successfully",
	})
}

func (h *Handler) ListUnits(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	prefix := c.QueryParam("prefix")

	// Get org UUID from domain context (set by middleware for both JWT and webhook routes)
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		logger.Error("Organization context missing",
			"operation", "list_units",
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	logger.Info("Listing units",
		"operation", "list_units",
		"org_id", orgCtx.OrgID,
		"prefix", prefix,
	)

	unitsMetadata, err := h.store.List(ctx, orgCtx.OrgID, prefix)
	if err != nil {
		logger.Error("Failed to list units",
			"operation", "list_units",
			"org_id", orgCtx.OrgID,
			"prefix", prefix,
			"error", err,
		)
		if err.Error() == "unauthorized" || err.Error() == "forbidden" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list units",
			"detail": err.Error(),
		})
	}

	domainUnits := make([]*domain.Unit, 0, len(unitsMetadata))
	for _, u := range unitsMetadata {
		absoluteName := u.Name
		if u.OrgName != "" {
			absoluteName = domain.BuildAbsoluteName(u.OrgName, u.Name)
		}
		
		domainUnits = append(domainUnits, &domain.Unit{
			ID:           u.ID,
			Name:         u.Name,
			AbsoluteName: absoluteName,
			Size:         u.Size,
			Updated:      u.Updated,
			Locked:       u.Locked,
			LockInfo:     convertLockInfo(u.LockInfo),
		})
	}
	domain.SortUnitsByID(domainUnits)

	logger.Info("Units listed successfully",
		"operation", "list_units",
		"count", len(domainUnits),
	)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"units": domainUnits,
		"count": len(domainUnits),
	})
}

func (h *Handler) GetUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	
	logger.Info("🔍 GetUnit called",
		"operation", "get_unit",
		"encoded_id", encodedID,
		"headers", c.Request().Header,
	)
	
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution",
			"operation", "get_unit",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	
	logger.Info("🔍 Unit identifier resolved",
		"operation", "get_unit",
		"resolved_id", id,
	)
	
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID",
			"operation", "get_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	logger.Info("🔍 Getting unit from store",
		"operation", "get_unit",
		"unit_id", id,
	)

	metadata, err := h.store.Get(ctx, id)
	if err != nil {
		logger.Error("🔍 Store.Get failed",
			"operation", "get_unit",
			"unit_id", id,
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"error_string", err.Error(),
		)
		if err.Error() == "forbidden" {
			logger.Warn("Forbidden access to unit",
				"operation", "get_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Forbidden"})
		}
		if err == storage.ErrNotFound {
			logger.Info("Unit not found",
				"operation", "get_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		logger.Error("Failed to get unit",
			"operation", "get_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get unit"})
	}
	
	absoluteName := metadata.Name
	if metadata.OrgName != "" {
		absoluteName = domain.BuildAbsoluteName(metadata.OrgName, metadata.Name)
	}
	
	return c.JSON(http.StatusOK, &domain.Unit{
		ID:           metadata.ID,
		Name:         metadata.Name,
		AbsoluteName: absoluteName,
		Size:         metadata.Size,
		Updated:      metadata.Updated,
		Locked:       metadata.Locked,
		LockInfo:     convertLockInfo(metadata.LockInfo),
		
		// Include TFE workspace settings
		TFEAutoApply:        metadata.TFEAutoApply,
		TFETerraformVersion: metadata.TFETerraformVersion,
		TFEEngine:           metadata.TFEEngine,
		TFEWorkingDirectory: metadata.TFEWorkingDirectory,
		TFEExecutionMode:    metadata.TFEExecutionMode,
	})
}

func (h *Handler) DeleteUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for delete",
			"operation", "delete_unit",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for delete",
			"operation", "delete_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	
	logger.Info("Deleting unit",
		"operation", "delete_unit",
		"unit_id", id,
	)
	
	if err := h.store.Delete(c.Request().Context(), id); err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for delete",
				"operation", "delete_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		logger.Error("Failed to delete unit",
			"operation", "delete_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete unit"})
	}
	
	logger.Info("Unit deleted successfully",
		"operation", "delete_unit",
		"unit_id", id,
	)
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DownloadUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	analytics.SendEssential("taco_unit_pull_started")

	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for download",
			"operation", "download_unit",
			"identifier", encodedID,
			"error", err,
		)
		analytics.SendEssential("taco_unit_pull_failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for download",
			"operation", "download_unit",
			"unit_id", id,
			"error", err,
		)
		analytics.SendEssential("taco_unit_pull_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	
	logger.Info("Downloading unit",
		"operation", "download_unit",
		"unit_id", id,
	)
	
	data, err := h.store.Download(ctx, id)
	if err != nil {
		analytics.SendEssential("taco_unit_pull_failed")
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for download",
				"operation", "download_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		logger.Error("Failed to download unit",
			"operation", "download_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to download unit"})
	}
	
	logger.Info("Unit downloaded successfully",
		"operation", "download_unit",
		"unit_id", id,
		"size_bytes", len(data),
	)
	analytics.SendEssential("taco_unit_pull_completed")
	return c.Blob(http.StatusOK, "application/json", data)
}

func (h *Handler) UploadUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	analytics.SendEssential("taco_unit_push_started")

	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for upload",
			"operation", "upload_unit",
			"identifier", encodedID,
			"error", err,
		)
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for upload",
			"operation", "upload_unit",
			"unit_id", id,
			"error", err,
		)
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		logger.Error("Failed to read request body for upload",
			"operation", "upload_unit",
			"unit_id", id,
			"error", err,
		)
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
	}
	lockID := c.QueryParam("if_locked_by")
	
	logger.Info("Uploading unit",
		"operation", "upload_unit",
		"unit_id", id,
		"lock_id", lockID,
		"size_bytes", len(data),
	)
	
	if err := h.store.Upload(c.Request().Context(), id, data, lockID); err != nil {
		analytics.SendEssential("taco_unit_push_failed")
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for upload",
				"operation", "upload_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			logger.Warn("Lock conflict on upload",
				"operation", "upload_unit",
				"unit_id", id,
				"lock_id", lockID,
			)
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock conflict"})
		}
		logger.Error("Failed to upload unit",
			"operation", "upload_unit",
			"unit_id", id,
			"lock_id", lockID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload unit"})
	}
	// TODO: This graph update function does not currently work correctly,
	// commenting out for now until this functionality is fixed
	// Best-effort dependency graph update
	//go deps.UpdateGraphOnWrite(c.Request().Context(), h.store, id, data)
	
	logger.Info("Unit uploaded successfully",
		"operation", "upload_unit",
		"unit_id", id,
	)
	analytics.SendEssential("taco_unit_push_completed")
	return c.JSON(http.StatusOK, map[string]string{"message": "Unit uploaded successfully"})
}

type LockRequest struct {
	ID      string `json:"id"`
	Who     string `json:"who"`
	Version string `json:"version"`
}

func (h *Handler) LockUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for lock",
			"operation", "lock_unit",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}
	
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for lock",
			"operation", "lock_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req LockRequest
	if err := c.Bind(&req); err != nil {
		req.ID = uuid.New().String()
		req.Who = "opentaco"
		req.Version = "1.0.0"
	}
	lockInfo := &storage.LockInfo{ID: req.ID, Who: req.Who, Version: req.Version, Created: time.Now()}
	
	logger.Info("Locking unit",
		"operation", "lock_unit",
		"unit_id", id,
		"lock_id", lockInfo.ID,
		"who", lockInfo.Who,
	)
	
	if err := h.store.Lock(c.Request().Context(), id, lockInfo); err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for lock",
				"operation", "lock_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			currentLock, _ := h.store.GetLock(c.Request().Context(), id)
			logger.Warn("Lock conflict",
				"operation", "lock_unit",
				"unit_id", id,
				"requested_lock_id", lockInfo.ID,
				"current_lock", currentLock,
			)
			return c.JSON(http.StatusConflict, convertLockInfo(currentLock))
		}
		logger.Error("Failed to lock unit",
			"operation", "lock_unit",
			"unit_id", id,
			"lock_id", lockInfo.ID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to lock unit"})
	}
	
	logger.Info("Unit locked successfully",
		"operation", "lock_unit",
		"unit_id", id,
		"lock_id", lockInfo.ID,
	)
	return c.JSON(http.StatusOK, convertLockInfo(lockInfo))
}

type UnlockRequest struct {
	ID string `json:"id"`
}

func (h *Handler) UnlockUnit(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for unlock",
			"operation", "unlock_unit",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}
	
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for unlock",
			"operation", "unlock_unit",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req UnlockRequest
	if err := c.Bind(&req); err != nil {
		logger.Warn("Lock ID required for unlock",
			"operation", "unlock_unit",
			"unit_id", id,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Lock ID required"})
	}
	
	logger.Info("Unlocking unit",
		"operation", "unlock_unit",
		"unit_id", id,
		"lock_id", req.ID,
	)
	
	if err := h.store.Unlock(c.Request().Context(), id, req.ID); err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for unlock",
				"operation", "unlock_unit",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			logger.Warn("Lock ID mismatch on unlock",
				"operation", "unlock_unit",
				"unit_id", id,
				"lock_id", req.ID,
			)
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock ID mismatch"})
		}
		logger.Error("Failed to unlock unit",
			"operation", "unlock_unit",
			"unit_id", id,
			"lock_id", req.ID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to unlock unit"})
	}
	
	logger.Info("Unit unlocked successfully",
		"operation", "unlock_unit",
		"unit_id", id,
		"lock_id", req.ID,
	)
	return c.JSON(http.StatusOK, map[string]string{"message": "Unit unlocked successfully"})
}

// Version operations

type ListVersionsResponse struct {
	UnitID   string            `json:"unit_id"`
	Versions []*domain.Version `json:"versions"`
	Count    int               `json:"count"`
}

func (h *Handler) ListVersions(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for list versions",
			"operation", "list_versions",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for list versions",
			"operation", "list_versions",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	logger.Info("Listing versions",
		"operation", "list_versions",
		"unit_id", id,
	)

	versions, err := h.store.ListVersions(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for list versions",
				"operation", "list_versions",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		logger.Error("Failed to list versions",
			"operation", "list_versions",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list versions"})
	}

	domainVersions := make([]*domain.Version, len(versions))
	for i, v := range versions {
		domainVersions[i] = &domain.Version{
			Timestamp: v.Timestamp,
			Hash:      v.Hash,
			Size:      v.Size,
		}
	}

	logger.Info("Versions listed successfully",
		"operation", "list_versions",
		"unit_id", id,
		"count", len(domainVersions),
	)
	return c.JSON(http.StatusOK, ListVersionsResponse{
		UnitID:   id,
		Versions: domainVersions,
		Count:    len(domainVersions),
	})
}

type RestoreVersionRequest struct {
	Timestamp time.Time `json:"timestamp"`
	LockID    string    `json:"lock_id,omitempty"`
}

type RestoreVersionResponse struct {
	UnitID    string    `json:"unit_id"`
	Timestamp time.Time `json:"restored_timestamp"`
	Message   string    `json:"message"`
}

func (h *Handler) RestoreVersion(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for restore version",
			"operation", "restore_version",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for restore version",
			"operation", "restore_version",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var req RestoreVersionRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Invalid request body for restore version",
			"operation", "restore_version",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	logger.Info("Restoring version",
		"operation", "restore_version",
		"unit_id", id,
		"timestamp", req.Timestamp,
		"lock_id", req.LockID,
	)

	if err := h.store.RestoreVersion(c.Request().Context(), id, req.Timestamp, req.LockID); err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Unit not found for restore version",
				"operation", "restore_version",
				"unit_id", id,
			)
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			logger.Warn("Lock conflict on restore version",
				"operation", "restore_version",
				"unit_id", id,
				"lock_id", req.LockID,
			)
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock conflict"})
		}
		logger.Error("Failed to restore version",
			"operation", "restore_version",
			"unit_id", id,
			"timestamp", req.Timestamp,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore version"})
	}

	logger.Info("Version restored successfully",
		"operation", "restore_version",
		"unit_id", id,
		"timestamp", req.Timestamp,
	)
	return c.JSON(http.StatusOK, RestoreVersionResponse{UnitID: id, Timestamp: req.Timestamp, Message: "Version restored"})
}

// GetUnitStatus computes and returns the dependency status for a given unit ID
func (h *Handler) GetUnitStatus(c echo.Context) error {
	logger := logging.FromContext(c)
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		logger.Warn("Unit not found during resolution for unit status",
			"operation", "get_unit_status",
			"identifier", encodedID,
			"error", err,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		logger.Warn("Invalid unit ID for unit status",
			"operation", "get_unit_status",
			"unit_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	logger.Info("Getting unit status",
		"operation", "get_unit_status",
		"unit_id", id,
	)

	st, err := deps.ComputeUnitStatus(c.Request().Context(), h.blobStore, id)
	if err != nil {
		logger.Warn("Error computing unit status, returning default",
			"operation", "get_unit_status",
			"unit_id", id,
			"error", err,
		)
		// On errors, prefer a 200 with green/empty as per implementation notes
		return c.JSON(http.StatusOK, st)
	}
	
	logger.Info("Unit status retrieved successfully",
		"operation", "get_unit_status",
		"unit_id", id,
		"status", st,
	)
	return c.JSON(http.StatusOK, st)
}

// Helpers
func convertLockInfo(info *storage.LockInfo) *domain.Lock {
	if info == nil {
		return nil
	}
	return &domain.Lock{ID: info.ID, Who: info.Who, Version: info.Version, Created: info.Created}
}

// getPrincipalFromToken extracts principal information from the bearer token
func (h *Handler) getPrincipalFromToken(c echo.Context) (rbac.Principal, error) {
	authz := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if h.signer == nil {
		return rbac.Principal{}, echo.NewHTTPError(http.StatusInternalServerError, "auth not configured")
	}

	claims, err := h.signer.VerifyAccess(token)
	if err != nil {
		return rbac.Principal{}, echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
	}

	return rbac.Principal{
		Subject: claims.Subject,
		Email:   claims.Email,
		Roles:   claims.Roles,
		Groups:  claims.Groups,
	}, nil
}
