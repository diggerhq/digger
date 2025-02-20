package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email          string `gorm:"uniqueIndex"`
	ExternalSource string `gorm:"uniqueIndex:idx_user_external_source"`
	ExternalId     string `gorm:"uniqueIndex:idx_user_external_source"`
	OrgId          *uint
	Username       string `gorm:"uniqueIndex:idx_user"`
}
