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
	Name string `json:"name"`
}

type CreateUnitResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

func (h *Handler) CreateUnit(c echo.Context) error {
	startHandler := time.Now()
	
	// Extract request ID for end-to-end tracing
	requestID := c.Request().Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("backend-%d", time.Now().UnixNano())
	}
	
	c.Logger().Infof("[%s] 🔶 BACKEND: Received create unit request", requestID)
	
	var req CreateUnitRequest
	bindStart := time.Now()
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("[%s] ❌ BACKEND: Failed to bind request body - %v (+%dms)", requestID, err, time.Since(startHandler).Milliseconds())
		analytics.SendEssential("unit_create_failed_invalid_request")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	bindTime := time.Since(bindStart)
	
	c.Logger().Infof("[%s] 📋 BACKEND: Parsed request for unit '%s' (bind: %dms)", requestID, req.Name, bindTime.Milliseconds())

	if err := domain.ValidateUnitID(req.Name); err != nil {
		c.Logger().Errorf("[%s] ❌ BACKEND: Invalid unit name '%s' - %v (+%dms)", requestID, req.Name, err, time.Since(startHandler).Milliseconds())
		analytics.SendEssential("unit_create_failed_invalid_name")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	name := domain.NormalizeUnitID(req.Name)

	// Get org UUID from domain context (set by middleware for both JWT and webhook routes)
	ctx := c.Request().Context()
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		c.Logger().Errorf("[%s] ❌ BACKEND: Organization context missing (+%dms)", requestID, time.Since(startHandler).Milliseconds())
		analytics.SendEssential("unit_create_failed_no_org_context")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}
	
	c.Logger().Infof("[%s] 🗄️  BACKEND: Calling repository Create (org: %s, name: %s)", requestID, orgCtx.OrgID, name)

	createStart := time.Now()
	metadata, err := h.store.Create(ctx, orgCtx.OrgID, name)
	createTime := time.Since(createStart)
	
	if err != nil {
		if err == storage.ErrAlreadyExists {
			c.Logger().Warnf("[%s] ⚠️  BACKEND: Unit already exists (repo: %dms, total: %dms)", requestID, createTime.Milliseconds(), time.Since(startHandler).Milliseconds())
			analytics.SendEssential("unit_create_failed_already_exists")
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Unit already exists",
				"detail": fmt.Sprintf("A unit with name '%s' already exists in this organization", name),
			})
		}
		// Log the actual error for debugging
		c.Logger().Errorf("[%s] ❌ BACKEND: Repository error (repo: %dms, total: %dms) - %v", requestID, createTime.Milliseconds(), time.Since(startHandler).Milliseconds(), err)
		analytics.SendEssential("unit_create_failed_storage_error")
		// Surface the actual error message to help with debugging
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create unit",
			"detail": err.Error(),
		})
	}

	totalTime := time.Since(startHandler)
	
	// Log timing breakdown
	if totalTime.Milliseconds() > 3000 {
		c.Logger().Warnf("[%s] 🔥 BACKEND: VERY SLOW - total: %dms (bind: %dms, repo: %dms)", requestID, totalTime.Milliseconds(), bindTime.Milliseconds(), createTime.Milliseconds())
	} else if totalTime.Milliseconds() > 1000 {
		c.Logger().Warnf("[%s] ⚠️  BACKEND: SLOW - total: %dms (bind: %dms, repo: %dms)", requestID, totalTime.Milliseconds(), bindTime.Milliseconds(), createTime.Milliseconds())
	} else {
		c.Logger().Infof("[%s] ✅ BACKEND: Success - total: %dms (bind: %dms, repo: %dms)", requestID, totalTime.Milliseconds(), bindTime.Milliseconds(), createTime.Milliseconds())
	}

	analytics.SendEssential("unit_created")
	return c.JSON(http.StatusCreated, CreateUnitResponse{ID: metadata.ID, Created: metadata.Updated})
}

func (h *Handler) ListUnits(c echo.Context) error {
	ctx := c.Request().Context()
	prefix := c.QueryParam("prefix")

	// Get org UUID from domain context (set by middleware for both JWT and webhook routes)
	orgCtx, ok := domain.OrgFromContext(ctx)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Organization context missing"})
	}

	unitsMetadata, err := h.store.List(ctx, orgCtx.OrgID, prefix)
	if err != nil {
		if err.Error() == "unauthorized" || err.Error() == "forbidden" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		c.Logger().Errorf("Failed to list units for org '%s' with prefix '%s': %v", orgCtx.OrgID, prefix, err)
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"units": domainUnits,
		"count": len(domainUnits),
	})
}

func (h *Handler) GetUnit(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	metadata, err := h.store.Get(ctx, id)
	if err != nil {
		if err.Error() == "forbidden" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Forbidden"})
		}
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
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
	})
}

func (h *Handler) DeleteUnit(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.store.Delete(c.Request().Context(), id); err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete unit"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DownloadUnit(c echo.Context) error {
	analytics.SendEssential("taco_unit_pull_started")

	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		analytics.SendEssential("taco_unit_pull_failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		analytics.SendEssential("taco_unit_pull_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	data, err := h.store.Download(ctx, id)
	if err != nil {
		analytics.SendEssential("taco_unit_pull_failed")
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to download unit"})
	}
	analytics.SendEssential("taco_unit_pull_completed")
	return c.Blob(http.StatusOK, "application/json", data)
}

func (h *Handler) UploadUnit(c echo.Context) error {
	analytics.SendEssential("taco_unit_push_started")

	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		analytics.SendEssential("taco_unit_push_failed")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
	}
	lockID := c.QueryParam("if_locked_by")
	if err := h.store.Upload(c.Request().Context(), id, data, lockID); err != nil {
		analytics.SendEssential("taco_unit_push_failed")
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock conflict"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload unit"})
	}
	// TODO: This graph update function does not currently work correctly,
	// commenting out for now until this functionality is fixed
	// Best-effort dependency graph update
	//go deps.UpdateGraphOnWrite(c.Request().Context(), h.store, id, data)
	analytics.SendEssential("taco_unit_push_completed")
	return c.JSON(http.StatusOK, map[string]string{"message": "Unit uploaded successfully"})
}

type LockRequest struct {
	ID      string `json:"id"`
	Who     string `json:"who"`
	Version string `json:"version"`
}

func (h *Handler) LockUnit(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}
	
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req LockRequest
	if err := c.Bind(&req); err != nil {
		req.ID = uuid.New().String()
		req.Who = "opentaco"
		req.Version = "1.0.0"
	}
	lockInfo := &storage.LockInfo{ID: req.ID, Who: req.Who, Version: req.Version, Created: time.Now()}
	if err := h.store.Lock(c.Request().Context(), id, lockInfo); err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			currentLock, _ := h.store.GetLock(c.Request().Context(), id)
			return c.JSON(http.StatusConflict, convertLockInfo(currentLock))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to lock unit"})
	}
	return c.JSON(http.StatusOK, convertLockInfo(lockInfo))
}

type UnlockRequest struct {
	ID string `json:"id"`
}

func (h *Handler) UnlockUnit(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
	}
	
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	var req UnlockRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Lock ID required"})
	}
	if err := h.store.Unlock(c.Request().Context(), id, req.ID); err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock ID mismatch"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to unlock unit"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Unit unlocked successfully"})
}

// Version operations

type ListVersionsResponse struct {
	UnitID   string            `json:"unit_id"`
	Versions []*domain.Version `json:"versions"`
	Count    int               `json:"count"`
}

func (h *Handler) ListVersions(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	versions, err := h.store.ListVersions(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
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
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var req RestoreVersionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if err := h.store.RestoreVersion(c.Request().Context(), id, req.Timestamp, req.LockID); err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Lock conflict"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to restore version"})
	}

	return c.JSON(http.StatusOK, RestoreVersionResponse{UnitID: id, Timestamp: req.Timestamp, Message: "Version restored"})
}

// GetUnitStatus computes and returns the dependency status for a given unit ID
func (h *Handler) GetUnitStatus(c echo.Context) error {
	ctx := c.Request().Context()
	encodedID := c.Param("id")
	id, err := h.resolveUnitIdentifier(ctx, encodedID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found",
			"detail": err.Error(),
		})
	}
	if err := domain.ValidateUnitID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	st, err := deps.ComputeUnitStatus(c.Request().Context(), h.blobStore, id)
	if err != nil {
		// On errors, prefer a 200 with green/empty as per implementation notes
		return c.JSON(http.StatusOK, st)
	}
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
