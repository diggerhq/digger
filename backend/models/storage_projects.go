package models

import (
	"fmt"
	"log"
)

func (db *Database) LoadProjectsForOrg(orgId uint) ([]*Project, error) {
	var projects []*Project
	err := db.GormDB.Preload("Organisation").Where("organisation_id = ?", orgId).Find(&projects).Error
	if err != nil {
		log.Printf("could not query projects for org: %v", orgId)
		return nil, fmt.Errorf("could not query projects for org: %v", orgId)
	}
	return projects, nil
}
