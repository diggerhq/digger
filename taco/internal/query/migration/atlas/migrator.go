package atlas

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/query/migration"
	atlas "ariga.io/atlas-go-sdk/atlasexec"
	"gorm.io/gorm"
)

//go:embed ../../../../../migrations
var migrationsFS embed.FS

// Migration directory paths (embedded in binary)
const (
	migrationsPostgres   = "migrations/postgres"
	migrationsMySQL      = "migrations/mysql"
	migrationsSQLite     = "migrations/sqlite"
	migrationsSQLServer  = "migrations/sqlserver"
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

	// Extract embedded migrations to temp directory
	tmpDir, err := m.extractMigrations(migrationDir)
	if err != nil {
		return fmt.Errorf("failed to extract migrations: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Get database URL from config
	dbURL, err := m.getDatabaseURL()
	if err != nil {
		return fmt.Errorf("failed to construct database URL: %w", err)
	}

	// Find atlas binary
	atlasPath := m.findAtlasBinary()

	// Apply migrations using Atlas SDK
	client, err := atlas.NewClient(".", atlasPath)
	if err != nil {
		return fmt.Errorf("failed to create atlas client: %w", err)
	}

	_, err = client.MigrateApply(ctx, &atlas.MigrateApplyParams{
		URL:    dbURL,
		DirURL: fmt.Sprintf("file://%s", tmpDir),
	})
	if err != nil {
		return fmt.Errorf("atlas migration failed: %w", err)
	}

	log.Printf("✅ %s migrations applied successfully", m.dialect)
	return nil
}

// findAtlasBinary looks for atlas in common locations
func (m *Migrator) findAtlasBinary() string {
	// Check if ATLAS_PATH is set
	if path := os.Getenv("ATLAS_PATH"); path != "" {
		return path
	}

	// Try common locations
	locations := []string{
		"atlas",                                           // In PATH
		"/usr/local/bin/atlas",                            // System install
		filepath.Join(os.Getenv("HOME"), "go/bin/atlas"),  // Go install (dev)
		"/go/bin/atlas",                                   // Docker container
	}

	for _, loc := range locations {
		if _, err := exec.LookPath(loc); err == nil {
			return loc
		}
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Default to "atlas" and let it fail with a clear error
	return "atlas"
}

func (m *Migrator) extractMigrations(dirPath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "atlas-migrations-*")
	if err != nil {
		return "", err
	}

	entries, err := migrationsFS.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := migrationsFS.ReadFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			return "", err
		}

		if err := os.WriteFile(filepath.Join(tmpDir, entry.Name()), content, 0644); err != nil {
			return "", err
		}
	}

	return tmpDir, nil
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
		absPath, err := filepath.Abs(cfg.Path)
		if err != nil {
			absPath = cfg.Path
		}
		return fmt.Sprintf("sqlite://%s", absPath), nil

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
