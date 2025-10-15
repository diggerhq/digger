package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestBackendHandler demonstrates the testability improvement.
// Before: Had to mock 11 UnitStore methods
// After: Only mock 6 StateOperations methods (or less!)
func TestBackendHandler_GetState(t *testing.T) {
	// Setup: Create a minimal mock (only 1 method!)
	mock := &domain.MockStateOperations{
		DownloadFunc: func(ctx context.Context, id string) ([]byte, error) {
			if id == "myapp/prod" {
				return []byte(`{"version": 4, "terraform_version": "1.5.0"}`), nil
			}
			return nil, nil
		},
	}

	// Create handler with scoped interface
	handler := NewHandler(mock)

	// Setup HTTP test
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/backend/myapp/prod", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/v1/backend/*")
	c.SetParamNames("*")
	c.SetParamValues("myapp/prod")

	// Execute
	err := handler.GetState(c)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "terraform_version")
}

// TestBackendHandler_UpdateState demonstrates you only mock what you need
func TestBackendHandler_UpdateState(t *testing.T) {
	// Setup: Mock only Get and Upload (2 methods!)
	called := false
	mock := &domain.MockStateOperations{
		GetFunc: func(ctx context.Context, id string) (*storage.UnitMetadata, error) {
			return &storage.UnitMetadata{ID: id, Size: 100, Updated: time.Now()}, nil
		},
		UploadFunc: func(ctx context.Context, id string, data []byte, lockID string) error {
			called = true
			assert.Equal(t, "myapp/staging", id)
			assert.Contains(t, string(data), "new_state")
			return nil
		},
	}

	handler := NewHandler(mock)

	// Setup HTTP test
	e := echo.New()
	body := strings.NewReader(`{"version": 4, "new_state": true}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/backend/myapp/staging", body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/v1/backend/*")

	// Execute
	err := handler.UpdateState(c)

	// Assert
	assert.NoError(t, err)
	assert.True(t, called, "Upload should have been called")
}

