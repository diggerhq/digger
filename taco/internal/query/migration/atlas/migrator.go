package atlas

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/migration"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/database/sqlserver"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"
)

//go:embed migrations
var migrationsFS embed.FS

// Migration directory paths (embedded in binary)
const (
	migrationsPostgres  = "migrations/postgres"
	migrationsMySQL     = "migrations/mysql"
	migrationsSQLite    = "migrations/sqlite"
	migrationsSQLServer = "migrations/sqlserver"
)

// Migrator implements migration.Migrator using Atlas
type Migrator struct {
	dialect string
	config  query.Config
}

// NewMigrator creates a new Atlas-based migrator
func NewMigrator(dialect string, cfg query.Config) migration.Migrator {
	return &Migrator{
		dialect: dialect,
		config:  cfg,
	}
}

func (m *Migrator) Dialect() string {
	return m.dialect
}

func (m *Migrator) Migrate(ctx context.Context, db *gorm.DB) error {
	migrationDir := m.getMigrationDir()
	if migrationDir == "" {
		return fmt.Errorf("no migrations available for dialect: %s", m.dialect)
	}

	log.Printf("Applying %s migrations from %s...", m.dialect, migrationDir)

	// Get database URL from config
	dbURL, err := m.getDatabaseURL()
	if err != nil {
		return fmt.Errorf("failed to construct database URL: %w", err)
	}

	// Get the migration subdirectory from embedded FS
	migrationFS, err := fs.Sub(migrationsFS, migrationDir)
	if err != nil {
		return fmt.Errorf("failed to access migration directory %s: %w", migrationDir, err)
	}

	// Create source instance from embedded filesystem
	sourceDriver, err := iofs.New(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create migrate instance
	migrator, err := migrate.NewWithSourceInstance("iofs", sourceDriver, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	// Apply all pending migrations
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Printf("✅ %s migrations already up to date", m.dialect)
	} else {
		log.Printf("✅ %s migrations applied successfully", m.dialect)
	}
	
	return nil
}

func (m *Migrator) getDatabaseURL() (string, error) {
	switch m.dialect {
	case "postgres":
		cfg := m.config.Postgres
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode), nil

	case "mysql":
		cfg := m.config.MySQL
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName), nil

	case "sqlite":
		cfg := m.config.SQLite
		absPath := cfg.Path
		if !filepath.IsAbs(absPath) {
			var err error
			absPath, err = filepath.Abs(cfg.Path)
			if err != nil {
				absPath = cfg.Path
			}
		}
		// golang-migrate uses sqlite3:// scheme
		return fmt.Sprintf("sqlite3://%s", absPath), nil

	case "sqlserver":
		cfg := m.config.MSSQL
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName), nil

	default:
		return "", fmt.Errorf("unsupported database dialect: %s", m.dialect)
	}
}

func (m *Migrator) getMigrationDir() string {
	switch m.dialect {
	case "postgres":
		return migrationsPostgres
	case "mysql":
		return migrationsMySQL
	case "sqlite":
		return migrationsSQLite
	case "sqlserver":
		return migrationsSQLServer
	default:
		return ""
	}
}
