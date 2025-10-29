package mssql

import (
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/common"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a MS SQL Server connection
func Connect(cfg query.MSSQLConfig) (*gorm.DB, error) {
	// DSN format: sqlserver://username:password@host:port?database=dbname
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), 
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mssql: %w", err)
	}

	return db, nil
}

// NewMSSQLStore creates a new MS SQL-backed query store.
// Assumes migrations have already been applied.
func NewMSSQLStore(db *gorm.DB) (query.Store, error) {
	return common.NewSQLStore(db)
}
