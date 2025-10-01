package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/api"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// change this random number to bump version of statesman: 429
func main() {
	var (
		port        = flag.String("port", "8080", "Server port")
		authDisable = flag.Bool("auth-disable", false, "Disable auth enforcement (default: false)")
		storageType = flag.String("storage", "s3", "Storage type: s3 or memory (default: s3 with fallback to memory)")
		s3Bucket    = flag.String("s3-bucket", os.Getenv("OPENTACO_S3_BUCKET"), "S3 bucket for state storage")
		s3Prefix    = flag.String("s3-prefix", os.Getenv("OPENTACO_S3_PREFIX"), "S3 key prefix (optional)")
		s3Region    = flag.String("s3-region", os.Getenv("OPENTACO_S3_REGION"), "S3 region (optional; uses AWS defaults if empty)")
	)
	flag.Parse()

	// Initialize storage
	var store storage.UnitStore
	switch *storageType {
	case "s3":
		if *s3Bucket == "" {
			log.Printf("WARNING: S3 storage selected but bucket not provided. Falling back to in-memory storage.")
			store = storage.NewMemStore()
			log.Printf("Using in-memory storage")
			break
		}
		s, err := storage.NewS3Store(context.Background(), *s3Bucket, *s3Prefix, *s3Region)
		if err != nil {
			log.Printf("WARNING: failed to initialize S3 store: %v. Falling back to in-memory storage.", err)
			store = storage.NewMemStore()
			log.Printf("Using in-memory storage")
		} else {
			store = s
			log.Printf("Using S3 storage: bucket=%s prefix=%s region=%s", *s3Bucket, *s3Prefix, *s3Region)
		}
	default:
		store = storage.NewMemStore()
		log.Printf("Using in-memory storage")
	}

	// Initialize analytics with system ID management (always create system ID)
	analytics.InitGlobalWithSystemID("production", store)

	// Initialize system ID synchronously during startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to preload existing system ID and user email first
	if err := analytics.PreloadSystemID(ctx); err == nil {
		log.Printf("System ID and user email loaded from storage")
	} else {
		// If preload fails, create new system ID
		if systemID, err := analytics.GetOrCreateSystemID(ctx); err == nil {
			log.Printf("System ID created: %s", systemID)
		} else {
			log.Printf("System ID not available: %v", err)
		}
	}
	analytics.SendEssential("service_startup")

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Gzip())
	e.Use(middleware.Secure())
	e.Use(middleware.CORS())

	api.RegisterRoutes(e, store, !*authDisable)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("Starting OpenTaco service on %s", addr)
		analytics.SendEssential("server_started")
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			analytics.SendEssential("server_startup_failed")
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	analytics.SendEssential("server_shutdown_initiated")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		analytics.SendEssential("server_shutdown_failed")
		log.Fatalf("Server shutdown failed: %v", err)
	}

	analytics.SendEssential("server_shutdown_complete")
	log.Println("Server shutdown complete")
}
