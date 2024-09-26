package dbmodels

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/model"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"gorm.io/gorm"
	"log"
)

// GetRepo returns digger repo by organisationId and repo name (diggerhq-digger)
// it will return an empty object if record doesn't exist in database
func (db *Database) GetRepo(orgIdKey any, repoName string) (*model.Repo, error) {
	var repo model.Repo

	err := db.GormDB.Where("organisation_id = ? AND repos.name=?", orgIdKey, repoName).First(&repo).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Printf("Failed to find digger repo for orgId: %v, and repoName: %v, error: %v\n", orgIdKey, repoName, err)
		return nil, err
	}
	return &repo, nil
}

func (db *Database) RefreshProjectsFromRepo(orgId string, config configuration.DiggerConfigYaml, repo *model.Repo) error {
	log.Printf("UpdateRepoDiggerConfig, repo: %v\n", repo)

	err := db.GormDB.Transaction(func(tx *gorm.DB) error {
		for _, dc := range config.Projects {
			projectName := dc.Name
			p, err := db.GetProjectByName(orgId, repo, projectName)
			if err != nil {
				return fmt.Errorf("error retriving project by name: %v", err)
			}
			if p == nil {
				_, err := db.CreateProject(projectName, repo)
				if err != nil {
					return fmt.Errorf("could not create project: %v", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error while updating projects from config: %v", err)
	}
	return nil
}
