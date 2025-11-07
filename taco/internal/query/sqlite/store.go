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

// Connect establishes a SQLite connection
func Connect(cfg query.SQLiteConfig) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %v", err)
	}

	dsn := fmt.Sprintf("file:%s?cache=%s", cfg.Path, cfg.Cache)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
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

	// Configure connection pool settings from config (use SQLITE_MAX_OPEN_CONNS, etc.)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}

// NewSQLiteQueryStore creates a new SQLite-backed query store.
// Assumes migrations have already been applied.
func NewSQLiteQueryStore(db *gorm.DB) (query.Store, error) {
	return common.NewSQLStore(db)
}

