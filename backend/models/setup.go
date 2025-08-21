package models

import (
	"github.com/diggerhq/digger/backend/logging"
	sloggorm "github.com/imdatngo/slog-gorm/v2"
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"time"
)

type Database struct {
	GormDB *gorm.DB
}

var DEFAULT_ORG_NAME = "digger"

// var DB *gorm.DB
var DB *Database

func ConnectDatabase() {

	slogger := logging.Default()

	cfg := sloggorm.NewConfig(slogger.Handler()).
		WithGroupKey("db").
		WithSlowThreshold(time.Second).
		WithIgnoreRecordNotFoundError(true)

	if os.Getenv("DIGGER_LOG_LEVEL") == "DEBUG" {
		cfg.WithTraceAll(true)
	}
	glogger := sloggorm.NewWithConfig(cfg)
	database, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{
		Logger: glogger,
	})
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		panic("Failed to connect to database!")
	}

	DB = &Database{GormDB: database}

	// data and fixtures added
	orgNumberOne, err := DB.GetOrganisation(DEFAULT_ORG_NAME)
	if orgNumberOne == nil {
		slog.Info("No default organization found, creating default organisation", "name", DEFAULT_ORG_NAME)
		_, err := DB.CreateOrganisation("digger", "", DEFAULT_ORG_NAME)
		if err != nil {
			slog.Error("Failed to create default organization", "error", err)
		}
	}
}
