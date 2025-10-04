package query

import (
	"fmt"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/query/noop"
	"github.com/diggerhq/digger/opentaco/internal/query/sqlite"
)

// NewQueryStore creates a new query.Store based on the provided configuration.
func NewQueryStore(cfg Config) (Store, error) {
	backend := strings.ToLower(cfg.Backend)

	switch backend {
	case "sqlite", "":
		// Map our config struct to the one sqlite's New function expects.
		sqliteCfg := sqlite.Config{
			Path:              cfg.SQLite.Path,
			Cache:             cfg.SQLite.Cache,
			BusyTimeout:       cfg.SQLite.BusyTimeout,
			MaxOpenConns:      cfg.SQLite.MaxOpenConns,
			MaxIdleConns:      cfg.SQLite.MaxIdleConns,
			PragmaJournalMode: cfg.SQLite.PragmaJournalMode,
			PragmaForeignKeys: cfg.SQLite.PragmaForeignKeys,
			PragmaBusyTimeout: cfg.SQLite.PragmaBusyTimeout,
		}
		return sqlite.NewSQLiteQueryStore(sqliteCfg)
	case "off":
		return noop.NewNoOpQueryStore(), nil
	default:
		return nil, fmt.Errorf("unsupported TACO_QUERY_BACKEND value: %q", backend)
	}
}