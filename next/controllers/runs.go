package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/services"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"log"
)

func processSingleQueueItem(gh next_utils.GithubClientProvider, queueItem model.DiggerRunQueueItem) error {
	dr, err := dbmodels.DB.GetDiggerRun(queueItem.DiggerRunID)
	if err != nil {
		log.Printf("could not get queue item: %v", err)
		return fmt.Errorf("could not get queue item: %v", err)
	}

	repo, err := dbmodels.DB.GetRepoById(dr.RepoID)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		return fmt.Errorf("could not get repo: %v", err)
	}

	installation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(repo.OrganizationID, repo.RepoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("could not get github installation: %v", err)
		return fmt.Errorf("could not get github installation: %v", err)
	}

	repoFullName := repo.RepoFullName
	repoOwner := repo.RepoOrganisation
	repoName := repo.RepoName
	service, _, err := next_utils.GetGithubService(gh, installation.GithubInstallationID, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("failed to get github service for DiggerRun ID: %v: %v", dr.ID, err)
		return fmt.Errorf("failed to get github service for DiggerRun ID: %v: %v", dr.ID, err)
	}
	err = services.RunQueuesStateMachine(&queueItem, service, gh)
	return err
}

func (d DiggerController) ProcessRunQueueItems(c *gin.Context) {
	runQueues, err := dbmodels.DB.GetFirstRunQueueForEveryProject()
	if err != nil {
		log.Printf("Error fetching Latest queueItem runs: %v", err)
		return
	}

	for _, queueItem := range runQueues {
		err := processSingleQueueItem(d.GithubClientProvider, queueItem)
		if err != nil {
			log.Printf("single queue item error: %v", err)
			continue
		}
	}
}
