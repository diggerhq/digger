package dbmodels

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/model"
	"gorm.io/gorm"
	"log"
)

func (db *Database) GetOrganisationById(orgId any) (*model.Organisation, error) {
	log.Printf("GetOrganisationById, orgId: %v, type: %T \n", orgId, orgId)
	org := model.Organisation{}
	err := db.GormDB.Where("id = ?", orgId).First(&org).Error
	if err != nil {
		return nil, fmt.Errorf("Error fetching organisation: %v\n", err)
	}
	return &org, nil
}

func (db *Database) CreateGithubInstallationLink(orgId string, installationId string) (*model.GithubAppInstallationLink, error) {
	l := model.GithubAppInstallationLink{}
	// check if there is already a link to another org, and throw an error in this case
	result := db.GormDB.Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallationLinkActive).Find(&l)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	if result.RowsAffected > 0 {
		if l.OrganisationID != orgId {
			return nil, fmt.Errorf("GitHub app installation %v already linked to another org ", installationId)
		}
		log.Printf("installation %v has been linked to the org %v already.", installationId, orgId)
		// record already exist, do nothing
		return &l, nil
	}

	var list []model.GithubAppInstallationLink
	// if there are other installation for this org, we need to make them inactive
	result = db.GormDB.Where("github_installation_id <> ? AND organisation_id = ? AND status=?", installationId, orgId, GithubAppInstallationLinkActive).Find(&list)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	for _, item := range list {
		item.Status = string(GithubAppInstallationLinkInactive)
		db.GormDB.Save(&item)
	}

	link := model.GithubAppInstallationLink{OrganisationID: orgId, GithubInstallationID: installationId, Status: string(GithubAppInstallationLinkActive)}
	result = db.GormDB.Save(&link)
	if result.Error != nil {
		return nil, result.Error
	}
	log.Printf("GithubAppInstallationLink (org: %v, installationId: %v) has been created successfully\n", orgId, installationId)
	return &link, nil
}

func (db *Database) CreateRepo(name string, repoFullName string, repoOrganisation string, repoName string, repoUrl string, org *model.Organisation, diggerConfig string, githubInstallationId string, githubAppId int64, accountId int64, login string) (*model.Repo, error) {
	var repo model.Repo
	// check if repo exist already, do nothing in this case
	result := db.GormDB.Where("name = ? AND organisation_id=?", name, org.ID).Find(&repo)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	if result.RowsAffected > 0 {
		// record already exist, do nothing
		return &repo, nil
	}
	repo = model.Repo{
		Name:                 name,
		OrganisationID:       org.ID,
		DiggerConfig:         diggerConfig,
		RepoFullName:         repoFullName,
		RepoOrganisation:     repoOrganisation,
		RepoName:             repoName,
		RepoURL:              repoUrl,
		GithubInstallationID: githubInstallationId,
		GithubAppID:          githubAppId,
		AccountID:            accountId,
		Login:                login,
	}
	result = db.GormDB.Save(&repo)
	if result.Error != nil {
		log.Printf("Failed to create repo: %v, error: %v\n", name, result.Error)
		return nil, result.Error
	}
	log.Printf("Repo %s, (id: %v) has been created successfully\n", name, repo.ID)
	return &repo, nil
}
