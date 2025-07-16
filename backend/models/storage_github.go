package models

import (
	"errors"
	"gorm.io/gorm"
	"log"
	"strings"
)

func CreateOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId string, githubAppId int64, defaultBranch string, cloneUrl string) (*Repo, *Organisation, error) {
	link, err := DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		log.Printf("Error fetching installation link: %v", err)
		return nil, nil, err
	}
	orgId := link.OrganisationId
	org, err := DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation by id: %v, error: %v\n", orgId, err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo Repo
	r := DB.GormDB.Unscoped().Where("organisation_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

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

	repo, err := DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, "", "", 0, "", "")
	//repo, err := DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, "", installationId, githubAppId, accountId, login, defaultBranch, cloneUrl)
	if err != nil {
		log.Printf("Error creating digger repo: %v", err)
		return nil, nil, err
	}
	log.Printf("Created digger repo: %v", repo)
	return repo, org, nil
}
