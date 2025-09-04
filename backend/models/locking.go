package models

import "gorm.io/gorm"

type DiggerLock struct {
	gorm.Model
	Resource       string `gorm:"index:idx_digger_locked_resource"`
	LockId         int    `gorm:"index:idx_digger_lock_id"`
	Organisation   *Organisation
	OrganisationID uint `gorm:"index:idx_digger_lock_id"`
}
