// Package queryfactory is responsible for constructing a query.Store.
// It is in a separate package from query to prevent circular dependencies,
// as it needs to import the various database-specific store packages.
package queryfactory

import (
	"fmt"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/mssql"
	"github.com/diggerhq/digger/opentaco/internal/query/mysql"
	"github.com/diggerhq/digger/opentaco/internal/query/postgres"
	"github.com/diggerhq/digger/opentaco/internal/query/sqlite"
	"gorm.io/gorm"
)

// NewQueryStore creates a new query.Store based on the provided configuration.
// NOTE: Migrations are NOT applied automatically. They must be applied via:
//   - Docker/Production: entrypoint.sh runs "atlas migrate apply" before starting
//   - Local Dev: Run "atlas migrate apply" manually before starting statesman
func NewQueryStore(cfg query.Config) (query.Store, error) {
	backend := strings.ToLower(cfg.Backend)

	// Step 1: Connect to database (get GORM DB instance)
	db, _, err := connectDatabase(backend, cfg)
	if err != nil {
		return nil, err
	}

	// Step 2: Create store (data access layer)
	// NOTE: Migrations must be applied externally (see function comment above)
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
