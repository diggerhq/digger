package mssql

import (
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/common"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewMSSQLStore creates a new MS SQL-backed query store.
// Its only job is to establish the DB connection and pass it to the common SQLStore.
func NewMSSQLStore(cfg query.MSSQLConfig) (query.Store, error) {
	// DSN format: sqlserver://username:password@host:port?database=dbname
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Or Silent
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mssql: %w", err)
	}

	// Hand off to the common, dialect-aware SQLStore engine.
	return common.NewSQLStore(db)
}
