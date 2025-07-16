package models

import (
	"fmt"
	"log"
)

func (db *Database) LoadProjectsForOrg(orgId uint) ([]*Project, error) {
	orgProjects := make([]*Project, 0)
	var repos []*Repo
	err := db.GormDB.Where("organisation_id = ?", orgId).Find(&repos).Error
	if err != nil {
		log.Printf("could not find repos for org %v", orgId)
		return nil, fmt.Errorf("could not find repos for org %v", orgId)
	}
	for _, repo := range repos {
		var projects []*Project
		err := db.GormDB.Preload("Organisation").Preload("Repo").Where("repo_id = ?", repo.ID).Find(&projects).Error
		if err != nil {
			log.Printf("could not query projects for repo: %v", repo.ID)
			return nil, fmt.Errorf("could not query projects for repo: %v", repo.ID)
		}
		orgProjects = append(orgProjects, projects...)
	}
	return orgProjects, nil
}
