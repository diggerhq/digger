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

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/queryfactory"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
	"github.com/diggerhq/digger/opentaco/internal/token_service"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// change this random number to bump version of token service: 1
func main() {
	var (
		port = flag.String("port", "8081", "Server port")
	)
	flag.Parse()

	// Load configuration from environment variables into our struct.
	var queryCfg query.Config
	err := envconfig.Process("opentaco", &queryCfg) // The prefix "OPENTACO" will be used for all vars.
	if err != nil {
		log.Fatalf("Failed to process configuration: %v", err)
	}

	// --- Initialize Stores ---

	// Create the database query store using the dedicated factory.
	queryStore, err := queryfactory.NewQueryStore(queryCfg)
	if err != nil {
		log.Fatalf("Failed to initialize query backend: %v", err)
	}
	defer queryStore.Close()

	log.Printf("Query backend initialized: %s", queryCfg.Backend)

	// Get the underlying *gorm.DB from the query store
	db := repositories.GetDBFromQueryStore(queryStore)
	if db == nil {
		log.Fatalf("Query store does not provide GetDB method")
	}

	// Create token repository
	tokenRepo := token_service.NewTokenRepository(db)
	log.Println("Token repository initialized")

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.RequestID())
	e.Use(echomiddleware.Gzip())
	e.Use(echomiddleware.Secure())
	e.Use(echomiddleware.CORS())

	// Register routes
	token_service.RegisterRoutes(e, tokenRepo)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("Starting Token Service on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server shutdown complete")
}

