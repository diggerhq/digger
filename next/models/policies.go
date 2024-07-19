package models

import "gorm.io/gorm"

const (
	POLICY_TYPE_ACCESS = "access"
	POLICY_TYPE_PLAN   = "plan"
	POLICY_TYPE_DRIFT  = "drift"
)

type Policy struct {
	gorm.Model
	Project        *Project
	ProjectID      *uint
	Policy         string
	Type           string
	CreatedBy      *User
	CreatedByID    *uint
	Organisation   *Organisation
	OrganisationID uint
	Repo           *Repo
	RepoID         *uint
}
