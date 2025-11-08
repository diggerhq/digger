package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/diggerhq/digger/opentaco/cmd/token_service/query"
	querytypes "github.com/diggerhq/digger/opentaco/cmd/token_service/query/types"
	"github.com/diggerhq/digger/opentaco/internal/logging"
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

	// Initialize structured logging first (before any log statements)
	logging.Init("opentaco-token-service")
	slog.Info("Starting OpenTaco Token Service")

	// Load configuration from environment variables with "opentaco_token" prefix
	var dbCfg query.Config
	err := envconfig.Process("opentaco_token", &dbCfg)
	if err != nil {
		slog.Error("Failed to process token service database configuration", "error", err)
		os.Exit(1)
	}

	// --- Initialize Token Service Database ---

	// Create the database connection for token service
	db, err := query.NewDB(dbCfg)
	if err != nil {
		slog.Error("Failed to initialize token service database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("Failed to get underlying sql.DB", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	slog.Info("Token service database initialized", "backend", dbCfg.Backend)

	// Auto-migrate token models (ensures schema exists)
	// Note: In Docker, migrations are applied via Atlas in entrypoint.sh
	// This AutoMigrate is primarily for local development convenience
	if err := db.AutoMigrate(querytypes.TokenModels...); err != nil {
		slog.Error("Failed to auto-migrate token models", "error", err)
		os.Exit(1)
	}
	slog.Info("Token service database schema verified")

	// Create token repository
	tokenRepo := token_service.NewTokenRepository(db)
	slog.Info("Token repository initialized")

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
		slog.Info("Starting Token Service", "address", addr, "port", *port)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("Server startup failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	slog.Info("Shutting down server gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server shutdown complete")
}

