package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/diggerhq/digger/opentaco/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Default dialect
	dialect := "postgres"
	if len(os.Args) > 1 {
		dialect = os.Args[1]
	}

	// Use in-memory SQLite only to materialize GORM models
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	// Auto-migrate to generate schema from GORM models
	if err := db.AutoMigrate(models.Models...); err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}

	// Collect CREATE TABLE statements in creation order
	rows, err := sqlDB.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY rootpage")
	if err != nil {
		log.Fatalf("failed to query tables: %v", err)
	}
	defer rows.Close()

	var statements []string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			log.Fatalf("failed to scan row: %v", err)
		}
		if sql != "" {
			statements = append(statements, adaptSQL(sql, dialect))
		}
	}

	// Collect CREATE INDEX statements
	rows, err = sqlDB.Query("SELECT sql FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY rootpage")
	if err != nil {
		log.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			log.Fatalf("failed to scan row: %v", err)
		}
		if sql != "" {
			statements = append(statements, adaptSQL(sql, dialect))
		}
	}

	fmt.Println(strings.Join(statements, ";\n") + ";")
}

// adaptSQL converts SQLite DDL to the target dialect
func adaptSQL(sql, dialect string) string {
	switch dialect {
	case "postgres":
		// Identifiers use double quotes
		sql = strings.ReplaceAll(sql, "`", "\"")
		// Type adaptations
		sql = strings.ReplaceAll(sql, "AUTOINCREMENT", "")
		sql = strings.ReplaceAll(sql, "datetime", "timestamptz")
		sql = strings.ReplaceAll(sql, "numeric", "boolean")
		sql = strings.ReplaceAll(sql, "integer", "int")
		// DEFAULT literals
		sql = strings.ReplaceAll(sql, "DEFAULT \"allow\"", "DEFAULT 'allow'")
		sql = strings.ReplaceAll(sql, "DEFAULT \"\"", "DEFAULT ''")
	case "mysql":
		// Backticks are fine in MySQL, adjust types
		sql = strings.ReplaceAll(sql, "AUTOINCREMENT", "AUTO_INCREMENT")
		sql = strings.ReplaceAll(sql, "timestamptz", "datetime")
		sql = strings.ReplaceAll(sql, "numeric", "tinyint(1)")
		// TEXT limitations in MySQL
		sql = strings.ReplaceAll(sql, " text NOT NULL DEFAULT", " varchar(255) NOT NULL DEFAULT")
		sql = strings.ReplaceAll(sql, " text DEFAULT", " varchar(255) DEFAULT")
		sql = strings.ReplaceAll(sql, " text NOT NULL", " varchar(255) NOT NULL")
		// Common indexed fields sizing
		sql = strings.ReplaceAll(sql, "`action` varchar(255)", "`action` varchar(128)")
		sql = strings.ReplaceAll(sql, "`effect` varchar(255) NOT NULL DEFAULT", "`effect` varchar(8) NOT NULL DEFAULT")
	case "sqlite":
		// Already SQLite
	}
	return sql
}
