package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/common"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewSQLiteQueryStore creates a new SQLite-backed query store.
func NewSQLiteQueryStore(cfg query.SQLiteConfig) (query.Store, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %v", err)
	}

	dsn := fmt.Sprintf("file:%s?cache=%s", cfg.Path, cfg.Cache)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Or Silent
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %v", err)
	}

	// Apply SQLite-specific PRAGMAs
	if err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", strings.ToUpper(cfg.PragmaJournalMode))).Error; err != nil {
		return nil, fmt.Errorf("apply journal_mode: %w", err)
	}
	if err := db.Exec(fmt.Sprintf("PRAGMA foreign_keys = %s;", strings.ToUpper(cfg.PragmaForeignKeys))).Error; err != nil {
		return nil, fmt.Errorf("apply foreign_keys: %w", err)
	}
	if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %s;", cfg.PragmaBusyTimeout)).Error; err != nil {
		return nil, fmt.Errorf("apply busy_timeout: %w", err)
	}

	// Create the common SQLStore with our configured DB object, breaking the cycle.
	return common.NewSQLStore(db)
}

