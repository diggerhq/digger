package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type StateBackend struct {
	s3Client *S3Client
}

func NewStateBackend(s3Client *S3Client) *StateBackend {
	return &StateBackend{
		s3Client: s3Client,
	}
}

// GetState retrieves a state file from S3
func (sb *StateBackend) GetState(c *gin.Context) {
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

// SetState stores a state file to S3
func (sb *StateBackend) SetState(c *gin.Context) {
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

// DeleteState removes a state file from S3
func (sb *StateBackend) DeleteState(c *gin.Context) {
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

// HeadState checks if a state file exists in S3
func (sb *StateBackend) HeadState(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state key is required"})
		return
	}

	// Normalize the key (remove leading slash if present)
	key = strings.TrimPrefix(key, "/")

	slog.Info("Checking state existence", "key", key)

	ctx := context.Background()
	result, err := sb.s3Client.HeadObject(ctx, key)
	if err != nil {
		slog.Error("Failed to head state from S3", "key", key, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "state not found"})
		return
	}

	// Set headers from S3 response
	if result.ContentLength != nil {
		c.Header("Content-Length", fmt.Sprintf("%d", *result.ContentLength))
	}
	if result.LastModified != nil {
		c.Header("Last-Modified", result.LastModified.Format(http.TimeFormat))
	}
	if result.ETag != nil {
		c.Header("ETag", *result.ETag)
	}

	c.Status(http.StatusOK)
}
