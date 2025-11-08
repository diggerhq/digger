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

	"github.com/diggerhq/digger/opentaco/cmd/token_service/query"
	querytypes "github.com/diggerhq/digger/opentaco/cmd/token_service/query/types"
	"github.com/diggerhq/digger/opentaco/internal/token_service"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// change this random number to bump version of token service: 2
func main() {
	var (
		port = flag.String("port", "8081", "Server port")
	)
	flag.Parse()

	// Load configuration from environment variables with "opentaco_token" prefix
	var dbCfg query.Config
	err := envconfig.Process("opentaco_token", &dbCfg)
	if err != nil {
		log.Fatalf("Failed to process token service database configuration: %v", err)
	}

	// --- Initialize Token Service Database ---

	// Create the database connection for token service
	db, err := query.NewDB(dbCfg)
	if err != nil {
		log.Fatalf("Failed to initialize token service database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	defer sqlDB.Close()

	log.Printf("Token service database initialized: %s", dbCfg.Backend)

	// Auto-migrate token models (ensures schema exists)
	// Note: In Docker, migrations are applied via Atlas in entrypoint.sh
	// This AutoMigrate is primarily for local development convenience
	if err := db.AutoMigrate(querytypes.TokenModels...); err != nil {
		log.Fatalf("Failed to auto-migrate token models: %v", err)
	}
	log.Println("Token service database schema verified")

	// Create token repository
	tokenRepo := token_service.NewTokenRepository(db)
	log.Println("Token repository initialized")

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	// Use incoming X-Request-ID if provided, otherwise generate new one
	e.Use(echomiddleware.RequestIDWithConfig(echomiddleware.RequestIDConfig{
		TargetHeader: echo.HeaderXRequestID,
	}))
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

