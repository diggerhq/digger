package dbmodels

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/model"
	"github.com/diggerhq/digger/next/dbmodels"
	"gorm.io/gorm"
	"log"
)

type DriftStatus string

var DriftStatusNewDrift = "new drift"
var DriftStatusNoDrift = "no drift"
var DriftStatusAcknowledgeDrift = "acknowledged drift"

func (db *Database) GetProjectById(projectId string) (*model.Project, error) {
	log.Printf("GetProjectById, projectId: %v\n", projectId)
	var project model.Project

	err := db.GormDB.Where("id = ?", projectId).First(&project).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("could not find project")
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &project, nil
}

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

func (db *Database) LoadProjectsForOrg(orgId string) ([]*model.Project, error) {
	orgProjects := make([]*model.Project, 0)
	p := db.Query.Project
	r := db.Query.Repo
	repos, err := dbmodels.DB.Query.Project.Select(r.OrganisationID.Eq(orgId)).Find()
	if err != nil {
		log.Printf("could not find repos for org %v", orgId)
		return nil, fmt.Errorf("could not find repos for org %v", orgId)
	}
	for _, repo := range repos {
		projects, err := db.Query.Project.Select(p.RepoID.Eq(repo.ID)).Find()
		if err != nil {
			log.Printf("could not query projects for repo: %v", repo.ID)
			return nil, fmt.Errorf("could not query projects for repo: %v", repo.ID)
		}
		orgProjects = append(orgProjects, projects...)
	}
	return orgProjects, nil
}
