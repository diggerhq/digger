package models

import (
	"log/slog"
	"os"

	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	GormDB *gorm.DB
}

var DEFAULT_ORG_NAME = "digger"

// var DB *gorm.DB
var DB *Database

func ConnectDatabase() {
	database, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{
		Logger: slogGorm.New(),
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
