package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email      string `gorm:"uniqueIndex"`
	ExternalId string
	OrgId      *uint
	Username   string `gorm:"uniqueIndex:idx_user"`
}
