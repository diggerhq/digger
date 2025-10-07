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
)

// NewQueryStore creates a new query.Store based on the provided configuration.
func NewQueryStore(cfg query.Config) (query.Store, error) {
	backend := strings.ToLower(cfg.Backend)

	switch backend {
	case "sqlite", "":
		return sqlite.NewSQLiteQueryStore(cfg.SQLite)
	case "postgres":
		return postgres.NewPostgresStore(cfg.Postgres)
	case "mssql":
		return mssql.NewMSSQLStore(cfg.MSSQL)
	case "mysql":
		return mysql.NewMySQLStore(cfg.MySQL)
	default:
		return nil, fmt.Errorf("unsupported TACO_QUERY_BACKEND value: %q (supported: sqlite, postgres, mssql, mysql)", backend)
	}
}
