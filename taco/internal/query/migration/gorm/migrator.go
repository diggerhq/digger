package gorm

import (
	"context"
	"log"

	"github.com/diggerhq/digger/opentaco/internal/query/migration"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// Migrator implements migration.Migrator using GORM's AutoMigrate
type Migrator struct {
	dialect string
}

// NewMigrator creates a new GORM AutoMigrate-based migrator
func NewMigrator(dialect string) migration.Migrator {
	return &Migrator{dialect: dialect}
}

func (m *Migrator) Dialect() string {
	return m.dialect
}

func (m *Migrator) Migrate(ctx context.Context, db *gorm.DB) error {
	log.Printf("Running AutoMigrate for %s...", m.dialect)
	if err := db.AutoMigrate(types.DefaultModels...); err != nil {
		return err
	}
	log.Printf("✅ AutoMigrate completed for %s", m.dialect)
	return nil
}

