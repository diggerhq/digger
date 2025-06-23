package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

var Version = "dev"

func main() {
	// Initialize logging
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Starting Digger HTTP State Backend", "version", Version)

	// Initialize S3 client
	s3Client, err := NewS3Client()
	if err != nil {
		slog.Error("Failed to initialize S3 client", "error", err)
		os.Exit(1)
	}

	// Create state backend service
	stateBackend := NewStateBackend(s3Client)

	// Setup HTTP server
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sloggin.New(logger))

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": Version,
			"service": "digger-state-backend",
		})
	})

	// State endpoints
	stateGroup := r.Group("/state")
	{
		stateGroup.GET("/:key", stateBackend.GetState)
		stateGroup.POST("/:key", stateBackend.SetState)
		stateGroup.DELETE("/:key", stateBackend.DeleteState)
		stateGroup.HEAD("/:key", stateBackend.HeadState)
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("HTTP State Backend server starting", "port", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
