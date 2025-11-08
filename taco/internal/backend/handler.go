package backend

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/diggerhq/digger/opentaco/internal/analytics"
    "github.com/diggerhq/digger/opentaco/internal/deps"
    "github.com/diggerhq/digger/opentaco/internal/domain"
    "github.com/diggerhq/digger/opentaco/internal/logging"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
)

// Handler implements Terraform HTTP backend protocol.
type Handler struct {
    store domain.StateOperations  
}

func NewHandler(store domain.StateOperations) *Handler {
    return &Handler{
        store: store,
    }
}

// GetState handles GET requests for state retrieval
func (h *Handler) GetState(c echo.Context) error {
	logger := logging.FromContext(c)
	analytics.SendEssential("terraform_plan_started")
	
	id := extractID(c)
	
	logger.Info("Terraform backend GET state",
		"operation", "get_state",
		"state_id", id,
	)

	data, err := h.store.Download(c.Request().Context(), id)
	if err != nil {
		analytics.SendEssential("terraform_plan_failed")
		if err == storage.ErrNotFound {
			logger.Info("State not found",
				"operation", "get_state",
				"state_id", id,
			)
			return c.NoContent(http.StatusNotFound)
		}
		logger.Error("Failed to retrieve state",
			"operation", "get_state",
			"state_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to retrieve state",
		})
	}

	analytics.SendEssential("terraform_plan_completed")
	logger.Info("State retrieved successfully",
		"operation", "get_state",
		"state_id", id,
		"size_bytes", len(data),
	)
	return c.Blob(http.StatusOK, "application/json", data)
}

// UpdateState handles POST/PUT requests for state updates
func (h *Handler) UpdateState(c echo.Context) error {
    logger := logging.FromContext(c)
    analytics.SendEssential("terraform_apply_started")
    
    id := extractID(c)
    method := c.Request().Method
    
    logger.Info("Terraform backend update state",
		"operation", "update_state",
		"method", method,
		"state_id", id,
	)

	// Check if state exists - error if not found (no auto-creation)
	_, err := h.store.Get(c.Request().Context(), id)
	if err == storage.ErrNotFound {
		logger.Warn("Unit not found on state update",
			"operation", "update_state",
			"state_id", id,
		)
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
		})
	}
	if err != nil {
		logger.Error("Failed to check unit existence",
			"operation", "update_state",
			"state_id", id,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to check unit existence",
		})
	}

	// Read state data
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		analytics.SendEssential("terraform_apply_failed")
		logger.Error("Failed to read request body",
			"operation", "update_state",
			"state_id", id,
			"error", err,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Failed to read request body",
		})
	}

	// Get lock ID from header or query param (Terraform sends ?ID=...)
	lockID := c.Request().Header.Get("X-Terraform-Lock-ID")
	if lockID == "" {
		lockID = c.QueryParam("ID")
	}
	if lockID == "" {
		lockID = c.QueryParam("id")
	}

    logger.Info("Uploading state",
		"operation", "update_state",
		"state_id", id,
		"lock_id", lockID,
		"size_bytes", len(data),
	)

    // Upload state
    err = h.store.Upload(c.Request().Context(), id, data, lockID)
    if err != nil {
        analytics.SendEssential("terraform_apply_failed")
        if err == storage.ErrLockConflict {
			// Get current lock for details
			lock, _ := h.store.GetLock(c.Request().Context(), id)
			logger.Warn("State locked, cannot update",
				"operation", "update_state",
				"state_id", id,
				"requested_lock_id", lockID,
				"current_lock", lock,
			)
			if lock != nil {
				return c.JSON(http.StatusConflict, lock)
			}
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "State is locked",
			})
        }
        logger.Error("Failed to update state",
			"operation", "update_state",
			"state_id", id,
			"lock_id", lockID,
			"error", err,
		)
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": "Failed to update state",
        })
    }

    // Fire-and-forget graph update (best effort; never block/tank the write)
    go deps.UpdateGraphOnWrite(contextWithBackground(c), h.store, id, data)

    analytics.SendEssential("terraform_apply_completed")
    logger.Info("State updated successfully",
		"operation", "update_state",
		"state_id", id,
		"lock_id", lockID,
	)
    return c.NoContent(http.StatusOK)
}

// HandleLockUnlock handles LOCK and UNLOCK operations
func (h *Handler) HandleLockUnlock(c echo.Context) error {
	method := c.Request().Method

	switch method {
	case "LOCK":
		return h.lock(c)
	case "UNLOCK":
		return h.unlock(c)
	default:
		return c.JSON(http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
	}
}

func (h *Handler) lock(c echo.Context) error {
	logger := logging.FromContext(c)
	id := extractID(c)

	// Read lock info from body
	var lockInfo storage.LockInfo
	body, err := io.ReadAll(c.Request().Body)
	if err == nil && len(body) > 0 {
		json.Unmarshal(body, &lockInfo)
	}

	// Generate ID if not provided
	if lockInfo.ID == "" {
		lockInfo.ID = uuid.New().String()
	}
	if lockInfo.Who == "" {
		lockInfo.Who = "terraform"
	}
	if lockInfo.Version == "" {
		lockInfo.Version = "1.0.0"
	}
	lockInfo.Created = time.Now()

	logger.Info("Attempting to lock state",
		"operation", "lock",
		"state_id", id,
		"lock_id", lockInfo.ID,
		"who", lockInfo.Who,
		"version", lockInfo.Version,
	)

	// Attempt to lock
	err = h.store.Lock(c.Request().Context(), id, &lockInfo)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Warn("Unit not found on lock attempt",
				"operation", "lock",
				"state_id", id,
				"lock_id", lockInfo.ID,
			)
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
			})
		}

		if err == storage.ErrLockConflict {
			// Get current lock
			currentLock, _ := h.store.GetLock(c.Request().Context(), id)
			logger.Warn("Lock conflict - state already locked",
				"operation", "lock",
				"state_id", id,
				"requested_lock_id", lockInfo.ID,
				"current_lock", currentLock,
			)
			if currentLock != nil {
				return c.JSON(http.StatusLocked, currentLock)
			}
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "State is already locked",
			})
		}

		if err != nil {
			logger.Error("Failed to acquire lock",
				"operation", "lock",
				"state_id", id,
				"lock_id", lockInfo.ID,
				"error", err,
			)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to acquire lock",
			})
		}
	}

	logger.Info("Lock acquired successfully",
		"operation", "lock",
		"state_id", id,
		"lock_id", lockInfo.ID,
		"who", lockInfo.Who,
	)
	return c.JSON(http.StatusOK, lockInfo)
}

func (h *Handler) unlock(c echo.Context) error {
	logger := logging.FromContext(c)
	id := extractID(c)

	// Read lock ID from body
	var unlockReq struct {
		ID string `json:"ID"`
	}
	body, err := io.ReadAll(c.Request().Body)
	if err == nil && len(body) > 0 {
		json.Unmarshal(body, &unlockReq)
	}

	// Try header if not in body
	if unlockReq.ID == "" {
		unlockReq.ID = c.Request().Header.Get("X-Terraform-Lock-ID")
	}

	if unlockReq.ID == "" {
		logger.Warn("Unlock request missing lock ID",
			"operation", "unlock",
			"state_id", id,
		)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Lock ID required",
		})
	}

	logger.Info("Attempting to unlock state",
		"operation", "unlock",
		"state_id", id,
		"lock_id", unlockReq.ID,
	)

	err = h.store.Unlock(c.Request().Context(), id, unlockReq.ID)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Warn("State not found on unlock",
				"operation", "unlock",
				"state_id", id,
				"lock_id", unlockReq.ID,
			)
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		if err == storage.ErrLockConflict {
			logger.Warn("Lock ID mismatch on unlock",
				"operation", "unlock",
				"state_id", id,
				"lock_id", unlockReq.ID,
			)
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Lock ID mismatch",
			})
		}
		logger.Error("Failed to release lock",
			"operation", "unlock",
			"state_id", id,
			"lock_id", unlockReq.ID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to release lock",
		})
	}

	logger.Info("Lock released successfully",
		"operation", "unlock",
		"state_id", id,
		"lock_id", unlockReq.ID,
	)
	return c.NoContent(http.StatusOK)
}

func extractID(c echo.Context) string {
	// Get everything after /backend/
	path := c.Request().URL.Path
	prefix := "/v1/backend/"
	if idx := strings.Index(path, prefix); idx >= 0 {
		return path[idx+len(prefix):]
	}
	return c.Param("*")
}

// contextWithBackground returns a detached context for async operations
func contextWithBackground(c echo.Context) context.Context {
    // If request context is already done, use background
    if c == nil || c.Request() == nil || c.Request().Context().Err() != nil {
        return context.Background()
    }
    return c.Request().Context()
}
