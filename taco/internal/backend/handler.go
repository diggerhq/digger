package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/deps"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler implements Terraform HTTP backend protocol
type Handler struct {
	store storage.UnitStore
}

func NewHandler(store storage.UnitStore) *Handler {
	return &Handler{
		store: store,
	}
}

// GetState handles GET requests for state retrieval
func (h *Handler) GetState(c echo.Context) error {
	analytics.SendEssential("terraform_plan_started")

	id := extractID(c)

	data, err := h.store.Download(c.Request().Context(), id)
	if err != nil {
		analytics.SendEssential("terraform_plan_failed")
		if err == storage.ErrNotFound {
			return c.NoContent(http.StatusNotFound)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to retrieve state",
		})
	}

	analytics.SendEssential("terraform_plan_completed")
	return c.Blob(http.StatusOK, "application/json", data)
}

// UpdateState handles POST/PUT requests for state updates
func (h *Handler) UpdateState(c echo.Context) error {
	analytics.SendEssential("terraform_apply_started")

	fmt.Println("updating states ... ")

	id := extractID(c)

	// Check if state exists - error if not found (no auto-creation)
	_, err := h.store.Get(c.Request().Context(), id)
	if err == storage.ErrNotFound {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
		})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to check unit existence",
		})
	}

	// Read state data
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		analytics.SendEssential("terraform_apply_failed")
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

	// Upload state
	err = h.store.Upload(c.Request().Context(), id, data, lockID)
	if err != nil {
		analytics.SendEssential("terraform_apply_failed")
		if err == storage.ErrLockConflict {
			// Get current lock for details
			lock, _ := h.store.GetLock(c.Request().Context(), id)
			if lock != nil {
				return c.JSON(http.StatusConflict, lock)
			}
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "State is locked",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update state",
		})
	}

	// Fire-and-forget graph update (best effort; never block/tank the write)
	go deps.UpdateGraphOnWrite(contextForAsync(c), h.store, id, data)

	analytics.SendEssential("terraform_apply_completed")
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

	// Attempt to lock
	err = h.store.Lock(c.Request().Context(), id, &lockInfo)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
			})
		}

		if err == storage.ErrLockConflict {
			// Get current lock
			currentLock, _ := h.store.GetLock(c.Request().Context(), id)
			if currentLock != nil {
				return c.JSON(http.StatusLocked, currentLock)
			}
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "State is already locked",
			})
		}

		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to acquire lock",
			})
		}
	}

	return c.JSON(http.StatusOK, lockInfo)
}

func (h *Handler) unlock(c echo.Context) error {
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
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Lock ID required",
		})
	}

	err = h.store.Unlock(c.Request().Context(), id, unlockReq.ID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Lock ID mismatch",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to release lock",
		})
	}

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

func contextForAsync(c echo.Context) context.Context {
	if c == nil || c.Request() == nil {
		return context.Background()
	}
	// Keeps c.Request().Context() values, but removes its deadline/cancel.
	ctx := context.WithoutCancel(c.Request().Context())
	return ctx
}
