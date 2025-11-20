// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Default to sqlite if no argument provided
	dialect := "sqlite"
	if len(os.Args) > 1 {
		dialect = os.Args[1]
	}

	var db *gorm.DB
	var err error

	// Create an in-memory database for schema generation
	switch dialect {
	case "postgres":
		// Use SQLite in postgres mode for schema generation since we just need the structure
		db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	case "mysql":
		db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	case "sqlite":
		db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	default:
		log.Fatalf("Unknown dialect: %s", dialect)
	}

	if err != nil {
		log.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Auto-migrate to generate schema
	err = db.AutoMigrate(types.DefaultModels...)
	if err != nil {
		log.Fatalf("Failed to auto-migrate: %v", err)
	}

	// Get raw SQL connection
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get sql.DB: %v", err)
	}

	// Query sqlite_master to get CREATE statements in creation order (rootpage preserves order)
	rows, err := sqlDB.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY rootpage")
	if err != nil {
		log.Fatalf("Failed to query schema: %v", err)
	}
	defer rows.Close()

	var statements []string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		if sql != "" {
			statements = append(statements, adaptSQL(sql, dialect))
		}
	}

	// Also get indexes
	rows, err = sqlDB.Query("SELECT sql FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY rootpage")
	if err != nil {
		log.Fatalf("Failed to query indexes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		if sql != "" {
			statements = append(statements, adaptSQL(sql, dialect))
		}
	}

	// Output all statements
	fmt.Println(strings.Join(statements, ";\n") + ";")
}

// adaptSQL adapts SQLite SQL to target dialect
func adaptSQL(sql, dialect string) string {
	switch dialect {
	case "postgres":
		// PostgreSQL uses double quotes for identifiers, single quotes for strings
		sql = strings.ReplaceAll(sql, "`", "\"")
		sql = strings.ReplaceAll(sql, "AUTOINCREMENT", "")
		sql = strings.ReplaceAll(sql, "datetime", "timestamptz")
		sql = strings.ReplaceAll(sql, "numeric", "boolean")
		sql = strings.ReplaceAll(sql, "integer", "int")
		// Fix DEFAULT clause - PostgreSQL uses single quotes for string literals
		sql = strings.ReplaceAll(sql, "DEFAULT \"allow\"", "DEFAULT 'allow'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"active\"", "DEFAULT 'active'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"terraform\"", "DEFAULT 'terraform'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"pending\"", "DEFAULT 'pending'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"cli\"", "DEFAULT 'cli'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"\"", "DEFAULT ''")
		// PostgreSQL doesn't have longtext, use text instead
		sql = strings.ReplaceAll(sql, "longtext", "text")
	case "mysql":
		// MySQL uses backticks (already correct from SQLite)
		sql = strings.ReplaceAll(sql, "AUTOINCREMENT", "AUTO_INCREMENT")
		sql = strings.ReplaceAll(sql, "timestamptz", "datetime")
		sql = strings.ReplaceAll(sql, "numeric", "tinyint(1)")
		// MySQL has limitations with TEXT (no DEFAULT, no index without key length)
		// Convert most text fields to varchar(255), keep only truly large fields as text
		sql = strings.ReplaceAll(sql, " text NOT NULL DEFAULT", " varchar(255) NOT NULL DEFAULT")
		sql = strings.ReplaceAll(sql, " text DEFAULT", " varchar(255) DEFAULT")
		sql = strings.ReplaceAll(sql, " text NOT NULL", " varchar(255) NOT NULL")
		sql = strings.ReplaceAll(sql, " text,", " text,") // Keep text for description, resource_patterns
		sql = strings.ReplaceAll(sql, "`action` varchar(255)", "`action` varchar(128)") // Smaller for indexed fields
		sql = strings.ReplaceAll(sql, "`effect` varchar(255) NOT NULL DEFAULT", "`effect` varchar(8) NOT NULL DEFAULT")
	case "sqlite":
		// No adaptation needed
	}
	return sql
}
