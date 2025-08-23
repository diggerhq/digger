package api

import (
	"io"
	"net/http"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type StateHandler struct {
	store storage.StateStore
}

func NewStateHandler(store storage.StateStore) *StateHandler {
	return &StateHandler{
		store: store,
	}
}

type CreateStateRequest struct {
	ID string `json:"id"`
}

type CreateStateResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

func (h *StateHandler) CreateState(c echo.Context) error {
	var req CreateStateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Validate and normalize ID
	if err := domain.ValidateStateID(req.ID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	
	id := domain.NormalizeStateID(req.ID)

	// Create state
	metadata, err := h.store.Create(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrAlreadyExists {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "State already exists",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create state",
		})
	}

	return c.JSON(http.StatusCreated, CreateStateResponse{
		ID:      metadata.ID,
		Created: metadata.Updated,
	})
}

func (h *StateHandler) ListStates(c echo.Context) error {
	prefix := c.QueryParam("prefix")

	states, err := h.store.List(c.Request().Context(), prefix)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list states",
		})
	}

	// Convert to domain states
	var domainStates []*domain.State
	for _, s := range states {
		domainStates = append(domainStates, &domain.State{
			ID:       s.ID,
			Size:     s.Size,
			Updated:  s.Updated,
			Locked:   s.Locked,
			LockInfo: convertLockInfo(s.LockInfo),
		})
	}

	// Sort by ID
	domain.SortStatesByID(domainStates)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"states": domainStates,
		"count":  len(domainStates),
	})
}

func (h *StateHandler) GetState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	metadata, err := h.store.Get(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get state",
		})
	}

	return c.JSON(http.StatusOK, &domain.State{
		ID:       metadata.ID,
		Size:     metadata.Size,
		Updated:  metadata.Updated,
		Locked:   metadata.Locked,
		LockInfo: convertLockInfo(metadata.LockInfo),
	})
}

func (h *StateHandler) DeleteState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	err := h.store.Delete(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete state",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *StateHandler) DownloadState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	data, err := h.store.Download(c.Request().Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to download state",
		})
	}

	return c.Blob(http.StatusOK, "application/json", data)
}

func (h *StateHandler) UploadState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Read body
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Failed to read request body",
		})
	}

	// Get lock ID from query param
	lockID := c.QueryParam("if_locked_by")

	// Upload
	err = h.store.Upload(c.Request().Context(), id, data, lockID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Lock conflict",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to upload state",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "State uploaded successfully",
	})
}

type LockRequest struct {
	ID      string `json:"id"`
	Who     string `json:"who"`
	Version string `json:"version"`
}

func (h *StateHandler) LockState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	var req LockRequest
	if err := c.Bind(&req); err != nil {
		// Generate default lock info
		req.ID = uuid.New().String()
		req.Who = "opentaco"
		req.Version = "1.0.0"
	}

	lockInfo := &storage.LockInfo{
		ID:      req.ID,
		Who:     req.Who,
		Version: req.Version,
		Created: time.Now(),
	}

	err := h.store.Lock(c.Request().Context(), id, lockInfo)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "State not found",
			})
		}
		if err == storage.ErrLockConflict {
			// Get current lock info
			currentLock, _ := h.store.GetLock(c.Request().Context(), id)
			return c.JSON(http.StatusConflict, convertLockInfo(currentLock))
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to lock state",
		})
	}

	return c.JSON(http.StatusOK, convertLockInfo(lockInfo))
}

type UnlockRequest struct {
	ID string `json:"id"`
}

func (h *StateHandler) UnlockState(c echo.Context) error {
	encodedID := c.Param("id")
	id := domain.DecodeStateID(encodedID)
	
	if err := domain.ValidateStateID(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	var req UnlockRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Lock ID required",
		})
	}

	err := h.store.Unlock(c.Request().Context(), id, req.ID)
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
			"error": "Failed to unlock state",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "State unlocked successfully",
	})
}

// Helper functions

func convertLockInfo(info *storage.LockInfo) *domain.Lock {
	if info == nil {
		return nil
	}
	return &domain.Lock{
		ID:      info.ID,
		Who:     info.Who,
		Version: info.Version,
		Created: info.Created,
	}
}