package main

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// Dynamic SQL
type Querier interface {
	// SELECT * FROM @@table WHERE name = @name{{if role !=""}} AND role = @role{{end}}
	FilterWithNameAndRole(name, role string) ([]gen.T, error)
}

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "../models_generated",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode
	})

	dburl := os.Getenv("DB_URL")
	if dburl == "" {
		dburl = "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
	}
	gormdb, _ := gorm.Open(postgres.Open(dburl))
	g.UseDB(gormdb) // reuse your gorm db

	g.ApplyBasic(
		// Generate structs from all tables of current database
		g.GenerateAllTable()...,
	)

	// Generate the code
	g.Execute()
}
