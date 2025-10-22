// Package queryfactory is responsible for constructing a query.Store.
// It is in a separate package from query to prevent circular dependencies,
// as it needs to import the various database-specific store packages.
package queryfactory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/migration/atlas"
	gormmigration "github.com/diggerhq/digger/opentaco/internal/query/migration/gorm"
	"github.com/diggerhq/digger/opentaco/internal/query/mssql"
	"github.com/diggerhq/digger/opentaco/internal/query/mysql"
	"github.com/diggerhq/digger/opentaco/internal/query/postgres"
	"github.com/diggerhq/digger/opentaco/internal/query/sqlite"
	"gorm.io/gorm"
)

// NewQueryStore creates a new query.Store based on the provided configuration.
// It handles both database connection and schema migration.
func NewQueryStore(cfg query.Config) (query.Store, error) {
	backend := strings.ToLower(cfg.Backend)

	// Step 1: Connect to database (get GORM DB instance)
	db, dialect, err := connectDatabase(backend, cfg)
	if err != nil {
		return nil, err
	}

	// Step 2: Run migrations
	if err := runMigrations(db, dialect, cfg); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	// Step 3: Create store (data access layer)
	return createStore(db, backend)
}

// connectDatabase establishes the database connection
func connectDatabase(backend string, cfg query.Config) (*gorm.DB, string, error) {
	switch backend {
	case "sqlite", "":
		db, err := sqlite.Connect(cfg.SQLite)
		return db, "sqlite", err
	case "postgres":
		db, err := postgres.Connect(cfg.Postgres)
		return db, "postgres", err
	case "mssql":
		db, err := mssql.Connect(cfg.MSSQL)
		return db, "sqlserver", err
	case "mysql":
		db, err := mysql.Connect(cfg.MySQL)
		return db, "mysql", err
	default:
		return nil, "", fmt.Errorf("unsupported OPENTACO_QUERY_BACKEND value: %q (supported: sqlite, postgres, mssql, mysql)", backend)
	}
}

// runMigrations applies schema migrations with fallback strategy
func runMigrations(db *gorm.DB, dialect string, cfg query.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Try Atlas migrations first
	atlasMigrator := atlas.NewMigrator(dialect, cfg)
	
	log.Printf("Attempting Atlas migrations for %s...", dialect)
	if err := atlasMigrator.Migrate(ctx, db); err != nil {
		log.Printf("⚠️  Atlas migration failed: %v", err)
		log.Printf("Falling back to AutoMigrate for %s...", dialect)
		
		// Fallback to GORM AutoMigrate
		gormMigrator := gormmigration.NewMigrator(dialect)
		if err := gormMigrator.Migrate(ctx, db); err != nil {
			return fmt.Errorf("both Atlas and AutoMigrate failed: %w", err)
		}
	}

	return nil
}

// createStore creates the actual store implementation
func createStore(db *gorm.DB, backend string) (query.Store, error) {
	switch backend {
	case "sqlite", "":
		return sqlite.NewSQLiteQueryStore(db)
	case "postgres":
		return postgres.NewPostgresStore(db)
	case "mssql":
		return mssql.NewMSSQLStore(db)
	case "mysql":
		return mysql.NewMySQLStore(db)
	default:
		return nil, fmt.Errorf("unsupported backend: %q", backend)
	}
}
