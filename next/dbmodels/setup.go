package dbmodels

import (
	"github.com/diggerhq/digger/next/models_generated"
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
)

type Database struct {
	GormDB *gorm.DB
	Query  *models_generated.Query
}

// var DB *gorm.DB
var DB *Database

func ConnectDatabase() {

	database, err := gorm.Open(postgres.Open(os.Getenv("DIGGER_DATABASE_URL")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
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
