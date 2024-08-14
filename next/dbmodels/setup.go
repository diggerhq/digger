package dbmodels

import (
	"github.com/diggerhq/digger/next/models_generated"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log/slog"
	"os"
)

type Database struct {
	GormDB *gorm.DB
	Query  *models_generated.Query
}

// var DB *gorm.DB
var DB *Database

func ConnectDatabase() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("gorm", true)
	gormLogger := slogGorm.New(
		slogGorm.WithHandler(logger.Handler()),
		slogGorm.WithTraceAll(),
		slogGorm.SetLogLevel(slogGorm.DefaultLogType, slog.LevelInfo),
		slogGorm.WithContextValue("gorm", "true"),
	)

	database, err := gorm.Open(postgres.Open(os.Getenv("DIGGER_DATABASE_URL")), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		panic("Failed to connect to database!")
	}

	query := models_generated.Use(database)
	DB = &Database{
		Query:  query,
		GormDB: database,
	}

}
