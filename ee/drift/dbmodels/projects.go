package dbmodels

import (
	"errors"
	"github.com/diggerhq/digger/ee/drift/model"
	"gorm.io/gorm"
	"log"
)

type DriftStatus string

var DriftStatusNewDrift = "new drift"
var DriftStatusNoDrift = "no drift"
var DriftStatusAcknowledgeDrift = "acknowledged drift"

// GetProjectByName return project for specified org and repo
// if record doesn't exist return nil
func (db *Database) GetProjectByName(orgId any, repo *model.Repo, name string) (*model.Project, error) {
	log.Printf("GetProjectByName, org id: %v, project name: %v\n", orgId, name)
	var project model.Project

	err := db.GormDB.
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Where("repos.id = ?", repo.ID).
		Where("projects.name = ?", name).First(&project).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &project, nil
}

func (db *Database) CreateProject(name string, repo *model.Repo) (*model.Project, error) {
	project := &model.Project{
		Name:        name,
		RepoID:      repo.ID,
		DriftStatus: DriftStatusNewDrift,
	}
	result := db.GormDB.Save(project)
	if result.Error != nil {
		log.Printf("Failed to create project: %v, error: %v\n", name, result.Error)
		return nil, result.Error
	}
	log.Printf("Project %s, (id: %v) has been created successfully\n", name, project.ID)
	return project, nil
}
