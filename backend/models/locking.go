package models

import "gorm.io/gorm"

type DiggerLock struct {
	gorm.Model
	Resource       string `gorm:"index:idx_digger_locked_resource"`
	LockId         int
	Organisation   *Organisation
	OrganisationID uint
}
