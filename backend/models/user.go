package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email          string `gorm:"uniqueIndex"`
	ExternalSource string `gorm:"uniqueIndex:idx_user_external_source"`
	ExternalId     string `gorm:"uniqueIndex:idx_user_external_source"`
	// the default org currently in use by this user
	OrganisationId *uint
	Organisation   Organisation
	Username       string `gorm:"uniqueIndex:idx_user"`
}
