package dbmodels

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/model"
	"gorm.io/gorm"
	"log"
	"strings"
)

type GithubAppInstallationLinkStatus string

const (
	GithubAppInstallationLinkActive   GithubAppInstallationLinkStatus = "active"
	GithubAppInstallationLinkInactive GithubAppInstallationLinkStatus = "inactive"
)

func (db *Database) GetGithubInstallationLinkForInstallationId(installationId string) (*model.GithubAppInstallationLink, error) {
	l := model.GithubAppInstallationLink{}
	err := db.GormDB.Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallationLinkActive).Find(&l).Error
	if err != nil {
		return nil, err
	}
	if l.ID == "" {
		return nil, fmt.Errorf("github installation link not found")
	}
	return &l, nil
}

func CreateOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId string, githubAppId int64, accountId int64, login string) (*model.Repo, *model.Organisation, error) {
	link, err := DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		log.Printf("Error fetching installation link: %v", err)
		return nil, nil, err
	}
	orgId := link.OrganisationID
	org, err := DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation by id: %v, error: %v\n", orgId, err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo model.Repo
	r := DB.GormDB.Unscoped().Where("organization_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

	if r.Error != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("repo not found, will proceed with repo creation")
		} else {
			log.Printf("Error fetching repo: %v", err)
			return nil, nil, err
		}
	}

	if r.RowsAffected > 0 {
		existingRepo.DeletedAt = gorm.DeletedAt{}
		DB.GormDB.Save(&existingRepo)
		log.Printf("Digger repo already exists: %v", existingRepo)
		return &existingRepo, org, nil
	}

	repo, err := DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, "", installationId, githubAppId, accountId, login)
	if err != nil {
		log.Printf("Error creating digger repo: %v", err)
		return nil, nil, err
	}
	log.Printf("Created digger repo: %v", repo)
	return repo, org, nil
}
