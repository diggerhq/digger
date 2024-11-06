package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/services"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
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

type TriggerRunAssumingUserRequest struct {
	UserId    string `json:"user_id"`
	ProjectId string `json:"project_id"`
}

func (d DiggerController) TriggerRunForProjectAssumingUser(c *gin.Context) {
	var request TriggerRunAssumingUserRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload received"})
		return
	}
	projectId := request.ProjectId
	userId := request.UserId

	p := dbmodels.DB.Query.Project
	project, err := dbmodels.DB.Query.Project.Where(p.ID.Eq(projectId)).First()
	if err != nil {
		log.Printf("could not find project %v: %v", projectId, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not find project"})
		return
	}

	//branch := project.Branch
	projectName := project.Name

	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.ID.Eq(project.RepoID)).First()
	if err != nil {
		log.Printf("could not find repo: %v for project %v: %v", project.RepoID, project.ID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not find repo"})
		return
	}

	orgId := repo.OrganizationID
	repoId := repo.ID
	//repoFullName := repo.RepoFullName
	//repoOwner := repo.RepoOrganisation
	//repoName := repo.RepoName

	appInstallation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(orgId, repo.RepoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("error retrieving app installation")
		c.JSON(http.StatusBadRequest, gin.H{"error": "error retrieving app installation"})
		return
	}
	installationId := appInstallation.GithubInstallationID

	planBatchId, commitSha, err := services.CreateJobAndBatchForProjectFromBranch(d.GithubClientProvider, projectId, "digger plan", dbmodels.DiggerBatchManualTriggerEvent, scheduler.DiggerCommandPlan)
	if err != nil {
		log.Printf("Error creating plan batch: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating plan batch jobs"})
		return
	}

	applyBatchId, _, err := services.CreateJobAndBatchForProjectFromBranch(d.GithubClientProvider, projectId, "digger apply", dbmodels.DiggerBatchManualTriggerEvent, scheduler.DiggerCommandApply)
	if err != nil {
		log.Printf("Error creating apply batch: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating apply batch jobs"})
		return
	}

	planStage, err := dbmodels.DB.CreateDiggerRunStage(*planBatchId)
	if err != nil {
		log.Printf("Error creating digger run stage: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating digger run stage (plan)"})
		return
	}

	applyStage, err := dbmodels.DB.CreateDiggerRunStage(*applyBatchId)
	if err != nil {
		log.Printf("Error creating digger run stage: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating digger run stage (apply)"})
		return
	}

	diggerRun, err := dbmodels.DB.CreateDiggerRun("user", 0, dbmodels.RunQueued, *commitSha, "", installationId, repoId, projectId, projectName, dbmodels.PlanAndApply, planStage.ID, applyStage.ID, &userId)
	if err != nil {
		log.Printf("Error creating digger run: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating digger run"})
		return
	}

	_, err = dbmodels.DB.CreateDiggerRunQueueItem(diggerRun.ID, projectId)
	if err != nil {
		log.Printf("Error creating queuing run: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error adding run to queue"})
		return
	}
}
