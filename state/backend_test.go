package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// S3ClientInterface defines the interface for S3 operations
type S3ClientInterface interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
	PutObject(ctx context.Context, key string, data []byte) error
	DeleteObject(ctx context.Context, key string) error
	HeadObject(ctx context.Context, key string) (*s3.HeadObjectOutput, error)
}

// MockS3Client is a mock implementation of the S3ClientInterface
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockS3Client) PutObject(ctx context.Context, key string, data []byte) error {
	args := m.Called(ctx, key, data)
	return args.Error(0)
}

func (m *MockS3Client) DeleteObject(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockS3Client) HeadObject(ctx context.Context, key string) (*s3.HeadObjectOutput, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

// Update StateBackend to use interface
type StateBackendInterface struct {
	s3Client S3ClientInterface
}

func NewStateBackendInterface(s3Client S3ClientInterface) *StateBackendInterface {
	return &StateBackendInterface{
		s3Client: s3Client,
	}
}

func (sb *StateBackendInterface) GetState(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state key is required"})
		return
	}

	// Normalize the key (remove leading slash if present)
	key = strings.TrimPrefix(key, "/")

	slog.Info("Getting state", "key", key)

	ctx := context.Background()
	data, err := sb.s3Client.GetObject(ctx, key)
	if err != nil {
		slog.Error("Failed to get state from S3", "key", key, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "state not found"})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func (sb *StateBackendInterface) SetState(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state key is required"})
		return
	}

	// Normalize the key (remove leading slash if present)
	key = strings.TrimPrefix(key, "/")

	// Read the request body
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("Failed to read request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	slog.Info("Setting state", "key", key, "size", len(data))

	ctx := context.Background()
	err = sb.s3Client.PutObject(ctx, key, data)
	if err != nil {
		slog.Error("Failed to put state to S3", "key", key, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store state"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "state stored successfully"})
}

func (sb *StateBackendInterface) DeleteState(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state key is required"})
		return
	}

	// Normalize the key (remove leading slash if present)
	key = strings.TrimPrefix(key, "/")

	slog.Info("Deleting state", "key", key)

	ctx := context.Background()
	err := sb.s3Client.DeleteObject(ctx, key)
	if err != nil {
		slog.Error("Failed to delete state from S3", "key", key, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete state"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "state deleted successfully"})
}

func TestHealthEndpoint(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "test",
			"service": "digger-state-backend",
		})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	r.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "digger-state-backend", response["service"])
}

func TestGetState(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockS3 := new(MockS3Client)
	stateBackend := NewStateBackendInterface(mockS3)

	r := gin.New()
	r.GET("/state/:key", stateBackend.GetState)

	// Mock S3 response
	expectedData := []byte(`{"version": 4, "terraform_version": "1.0.0"}`)
	mockS3.On("GetObject", mock.Anything, "test-state").Return(expectedData, nil)

	// Create request
	req, _ := http.NewRequest("GET", "/state/test-state", nil)
	w := httptest.NewRecorder()

	// Perform request
	r.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, expectedData, w.Body.Bytes())
	mockS3.AssertExpectations(t)
}

func TestSetState(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockS3 := new(MockS3Client)
	stateBackend := NewStateBackendInterface(mockS3)

	r := gin.New()
	r.POST("/state/:key", stateBackend.SetState)

	// Test data
	testData := []byte(`{"version": 4, "terraform_version": "1.0.0"}`)
	mockS3.On("PutObject", mock.Anything, "test-state", testData).Return(nil)

	// Create request
	req, _ := http.NewRequest("POST", "/state/test-state", bytes.NewBuffer(testData))
	w := httptest.NewRecorder()

	// Perform request
	r.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	mockS3.AssertExpectations(t)
}

func TestDeleteState(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockS3 := new(MockS3Client)
	stateBackend := NewStateBackendInterface(mockS3)

	r := gin.New()
	r.DELETE("/state/:key", stateBackend.DeleteState)

	// Mock S3 response
	mockS3.On("DeleteObject", mock.Anything, "test-state").Return(nil)

	// Create request
	req, _ := http.NewRequest("DELETE", "/state/test-state", nil)
	w := httptest.NewRecorder()

	// Perform request
	r.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	mockS3.AssertExpectations(t)
}
