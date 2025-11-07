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
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// Database connection helpers (direct connections without QueryStore overhead)

func connectPostgres(cfg query.PostgresConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode)
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	
	// Configure connection pool
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	
	return db, nil
}

func connectMySQL(cfg query.MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
	
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}
	
	// Configure connection pool
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	
	return db, nil
}

func connectSQLite(cfg query.SQLiteConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s?cache=%s&_busy_timeout=%d&_journal_mode=%s&_foreign_keys=%s",
		cfg.Path, cfg.Cache, int(cfg.BusyTimeout.Milliseconds()), 
		cfg.PragmaJournalMode, cfg.PragmaForeignKeys)
	
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite: %w", err)
	}
	
	// Configure connection pool for SQLite
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	
	return db, nil
}

