package migration

import (
	"context"
	
	"gorm.io/gorm"
)

// Migrator handles database schema migrations.
// Implementations can use various strategies (Atlas, golang-migrate, AutoMigrate, etc.)
type Migrator interface {
	// Migrate applies all pending migrations to the database
	Migrate(ctx context.Context, db *gorm.DB) error
	
	// Dialect returns the database dialect this migrator is configured for
	Dialect() string
}

