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

	// Generate basic type-safe DAO API for struct `model.User` following conventions

	//g.ApplyBasic(
	//	// Generate struct `User` based on table `users`
	//	g.GenerateModel("users"),
	//	g.GenerateModel("organizations"),
	//	g.GenerateModel("digger_jobs"),
	//
	//	// Generate struct `Customer` based on table `customer` and generating options
	//	// customer table may have a tags column, it can be JSON type, gorm/gen tool can generate for your JSON data type
	//	g.GenerateModel("customers", gen.FieldType("tags", "datatypes.JSON")),
	//)
	g.ApplyBasic(
		// Generate structs from all tables of current database
		g.GenerateAllTable()...,
	)

	// need to make approver_user_id nullable and gorm gen does not generate it as a string pointer, for some reason
	g.ApplyBasic(
		g.GenerateModel("digger_runs",
			gen.FieldType("approver_user_id", "*string"),
			gen.FieldType("triggered_by_user_id", "*string"),
		),
	)

	g.ApplyBasic(
		g.GenerateModel("projects",
			gen.FieldType("team_id", "*int64"),
		),
	)

	// Generate the code
	g.Execute()
}
