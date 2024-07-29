package models

import (
	"gorm.io/gorm"
)

type JobArtefact struct {
	gorm.Model
	JobTokenID  uint
	JobToken    JobToken
	Filename    string
	Contents    []byte `gorm:"type:bytea"`
	Size        int64
	ContentType string
}
