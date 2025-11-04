package postgres

import (
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/common"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a PostgreSQL connection
func Connect(cfg query.PostgresConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true, // Use prepared statements for better performance
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
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

// NewPostgresStore creates a new PostgreSQL-backed query store.
// Assumes migrations have already been applied.
func NewPostgresStore(db *gorm.DB) (query.Store, error) {
	return common.NewSQLStore(db)
}