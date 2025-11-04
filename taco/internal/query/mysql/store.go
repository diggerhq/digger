package mysql

import (
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/common"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a MySQL connection
func Connect(cfg query.MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.Charset)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true, // Use prepared statements for better performance
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	// Configure connection pool for production workloads
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Optimize connection pool settings
	sqlDB.SetMaxOpenConns(25)                      // Max concurrent connections
	sqlDB.SetMaxIdleConns(10)                      // Keep connections warm
	sqlDB.SetConnMaxLifetime(5 * time.Minute)      // Recycle connections every 5 min
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)     // Close idle connections after 10 min

	return db, nil
}

// NewMySQLStore creates a new MySQL-backed query store.
// Assumes migrations have already been applied.
func NewMySQLStore(db *gorm.DB) (query.Store, error) {
	return common.NewSQLStore(db)
}
