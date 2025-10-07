package unit

import (
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/analytics"
    "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/domain"
    "github.com/diggerhq/digger/opentaco/internal/deps"
    "github.com/diggerhq/digger/opentaco/internal/rbac"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/diggerhq/digger/opentaco/internal/query"
    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "log"
    "context"

)

// Handler serves the management API (unit CRUD and locking)
type Handler struct {
    store       storage.UnitStore
    rbacManager *rbac.RBACManager
    signer      *auth.Signer
    queryStore  query.Store
}

func NewHandler(store storage.UnitStore, rbacManager *rbac.RBACManager, signer *auth.Signer, queryStore query.Store) *Handler {
    return &Handler{
        store:       store,
        rbacManager: rbacManager,
        signer:      signer,
        queryStore:  queryStore,
    }
}

type CreateUnitRequest struct {
    ID string `json:"id"`
}

type CreateUnitResponse struct {
    ID      string    `json:"id"`
    Created time.Time `json:"created"`
}

func (h *Handler) CreateUnit(c echo.Context) error {
    var req CreateUnitRequest
    if err := c.Bind(&req); err != nil {
        analytics.SendEssential("unit_create_failed_invalid_request")
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
    }

    if err := domain.ValidateUnitID(req.ID); err != nil {
        analytics.SendEssential("unit_create_failed_invalid_id")
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }
    id := domain.NormalizeUnitID(req.ID)

    metadata, err := h.store.Create(c.Request().Context(), id)
    if err != nil {
        if err == storage.ErrAlreadyExists {
            analytics.SendEssential("unit_create_failed_already_exists")
            return c.JSON(http.StatusConflict, map[string]string{"error": "Unit already exists"})
        }
        analytics.SendEssential("unit_create_failed_storage_error")
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create unit"})
    }

    analytics.SendEssential("unit_created")
    return c.JSON(http.StatusCreated, CreateUnitResponse{ID: metadata.ID, Created: metadata.Updated})
}

func (h *Handler) ListUnits(c echo.Context) error {
	ctx := c.Request().Context()
	prefix := c.QueryParam("prefix")

	// The RBAC logic is GONE. We just call the store.
	// The store (AuthorizingStore) returns a pre-filtered list or an error.
	unitsMetadata, err := h.store.List(ctx, prefix)
	if err != nil {
		if err.Error() == "unauthorized" || err.Error() == "forbidden" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		log.Printf("Error listing units: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list units"})
	}

	// The list is already filtered and secure. We just build the response.
	domainUnits := make([]*domain.Unit, 0, len(unitsMetadata))
	for _, u := range unitsMetadata {
		domainUnits = append(domainUnits, &domain.Unit{
			ID:       u.ID,
			Size:     u.Size,
			Updated:  u.Updated,
			Locked:   u.Locked,
			LockInfo: convertLockInfo(u.LockInfo),
		})
	}
	domain.SortUnitsByID(domainUnits)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"units": domainUnits,
		"count": len(domainUnits),
	})
}

// listFromStorage encapsulates the old storage-based path (including RBAC).
func (h *Handler) listFromStorage(ctx context.Context, c echo.Context, prefix string) error {
    items, err := h.store.List(ctx, prefix)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": "Failed to list units",
        })
    }

    unitIDs := make([]string, 0, len(items))
    unitMap := make(map[string]*storage.UnitMetadata, len(items))
    for _, s := range items {
        unitIDs = append(unitIDs, s.ID)
        unitMap[s.ID] = s
    }

    // Storage-based RBAC (manager-driven)
    if h.rbacManager != nil && h.signer != nil {
        principal, perr := h.getPrincipalFromToken(c)
        if perr != nil {
            if enabled, _ := h.rbacManager.IsEnabled(ctx); enabled {
                return c.JSON(http.StatusUnauthorized, map[string]string{
                    "error": "Failed to authenticate user",
                })
            }
            // RBAC not enabled -> show all units
        } else {
            filtered, ferr := h.rbacManager.FilterUnitsByReadAccess(ctx, principal, unitIDs)
            if ferr != nil {
                return c.JSON(http.StatusInternalServerError, map[string]string{
                    "error": "Failed to check permissions",
                })
            }
            unitIDs = filtered
        }
    }

    // Build response
    out := make([]*domain.Unit, 0, len(unitIDs))
    for _, id := range unitIDs {
        if s, ok := unitMap[id]; ok {
            out = append(out, &domain.Unit{
                ID:       s.ID,
                Size:     s.Size,
                Updated:  s.Updated,
                Locked:   s.Locked,
                LockInfo: convertLockInfo(s.LockInfo),
            })
        }
    }
    domain.SortUnitsByID(out)

    return c.JSON(http.StatusOK, map[string]interface{}{
        "units": out,
        "count": len(out),
    })
}


func (h *Handler) GetUnit(c echo.Context) error {
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
    if err := domain.ValidateUnitID(id); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }
    
    metadata, err := h.store.Get(c.Request().Context(), id)
    if err != nil {
        if err.Error() == "forbidden" {
            return c.JSON(http.StatusForbidden, map[string]string{"error": "Forbidden"})
        }
        if err == storage.ErrNotFound {
            return c.JSON(http.StatusNotFound, map[string]string{"error": "Unit not found"})
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get unit"})
    }
    return c.JSON(http.StatusOK, &domain.Unit{ID: metadata.ID, Size: metadata.Size, Updated: metadata.Updated, Locked: metadata.Locked, LockInfo: convertLockInfo(metadata.LockInfo)})
}

func (h *Handler) DeleteUnit(c echo.Context) error {
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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
    
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
    if err := domain.ValidateUnitID(id); err != nil {
        analytics.SendEssential("taco_unit_pull_failed")
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }
    data, err := h.store.Download(c.Request().Context(), id)
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
    
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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

    // Best-effort dependency graph update
    go deps.UpdateGraphOnWrite(c.Request().Context(), h.store, id, data)
    analytics.SendEssential("taco_unit_push_completed")
    return c.JSON(http.StatusOK, map[string]string{"message": "Unit uploaded successfully"})
}

type LockRequest struct {
    ID string `json:"id"`
    Who string `json:"who"`
    Version string `json:"version"`
}

func (h *Handler) LockUnit(c echo.Context) error {
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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

type UnlockRequest struct { ID string `json:"id"` }

func (h *Handler) UnlockUnit(c echo.Context) error {
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
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
    encodedID := c.Param("id")
    id := domain.DecodeUnitID(encodedID)
    if err := domain.ValidateUnitID(id); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
    }

    st, err := deps.ComputeUnitStatus(c.Request().Context(), h.store, id)
    if err != nil {
        // On errors, prefer a 200 with green/empty as per implementation notes
        return c.JSON(http.StatusOK, st)
    }
    return c.JSON(http.StatusOK, st)
}



// Helpers
func convertLockInfo(info *storage.LockInfo) *domain.Lock {
    if info == nil { return nil }
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
