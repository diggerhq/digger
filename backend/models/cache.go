package models

import (
	"gorm.io/gorm"
)

// storing repo cache such as digger.yml configuration
type RepoCache struct {
	gorm.Model
	OrgId        uint
	RepoFullName string
	DiggerYmlStr string
	DiggerConfig []byte `gorm:"type:bytea"`
}
