package models

import (
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
)

type Database struct {
	GormDB *gorm.DB
}

var DEFAULT_ORG_NAME = "digger"

// var DB *gorm.DB
var DB *Database

func ConnectDatabase() {

	database, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		panic("Failed to connect to database!")
	}

	DB = &Database{GormDB: database}

	// data and fixtures added
	orgNumberOne, err := DB.GetOrganisation(DEFAULT_ORG_NAME)
	if orgNumberOne == nil {
		log.Print("No default found, creating default organisation")
		DB.CreateOrganisation("digger", "", DEFAULT_ORG_NAME)
	}

}
