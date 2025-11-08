package query

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB creates a new GORM database connection based on the config
func NewDB(cfg Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	switch cfg.Backend {
	case "sqlite":
		dsn := fmt.Sprintf("file:%s?cache=%s", cfg.SQLite.Path, cfg.SQLite.Cache)
		db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to sqlite: %w", err)
		}

		// Apply SQLite-specific PRAGMAs
		if err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s;", cfg.SQLite.PragmaJournalMode)).Error; err != nil {
			return nil, fmt.Errorf("apply journal_mode: %w", err)
		}
		if err := db.Exec(fmt.Sprintf("PRAGMA foreign_keys = %s;", cfg.SQLite.PragmaForeignKeys)).Error; err != nil {
			return nil, fmt.Errorf("apply foreign_keys: %w", err)
		}
		if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %s;", cfg.SQLite.PragmaBusyTimeout)).Error; err != nil {
			return nil, fmt.Errorf("apply busy_timeout: %w", err)
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get underlying sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(cfg.SQLite.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.SQLite.MaxIdleConns)

	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
			cfg.Postgres.Host, cfg.Postgres.User, cfg.Postgres.Password,
			cfg.Postgres.DBName, cfg.Postgres.Port, cfg.Postgres.SSLMode)
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to postgres: %w", err)
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get underlying sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)

	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Host,
			cfg.MySQL.Port, cfg.MySQL.DBName, cfg.MySQL.Charset)
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to mysql: %w", err)
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get underlying sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)

	case "mssql":
		dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			cfg.MSSQL.User, cfg.MSSQL.Password, cfg.MSSQL.Host,
			cfg.MSSQL.Port, cfg.MSSQL.DBName)
		db, err = gorm.Open(sqlserver.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to mssql: %w", err)
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get underlying sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(cfg.MSSQL.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MSSQL.MaxIdleConns)

	default:
		return nil, fmt.Errorf("unsupported database backend: %s", cfg.Backend)
	}

	return db, nil
}

