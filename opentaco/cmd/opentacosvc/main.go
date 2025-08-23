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

	"github.com/diggerhq/digger/opentaco/internal/api"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	var (
		port        = flag.String("port", "8080", "Server port")
		storageType = flag.String("storage", "memory", "Storage type: memory or file")
		dataDir     = flag.String("data-dir", ".devdata", "Data directory for file storage")
	)
	flag.Parse()

	// Initialize storage
	var store storage.StateStore
	switch *storageType {
	case "file":
		store = storage.NewFileStore(*dataDir)
		log.Printf("Using file storage in %s", *dataDir)
	default:
		store = storage.NewMemStore()
		log.Printf("Using in-memory storage")
	}

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

	// Initialize API
	api.RegisterRoutes(e, store)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("Starting OpenTaco service on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	
	log.Println("Server shutdown complete")
}