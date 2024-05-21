package controllers

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func ListProjects(c *gin.Context) {

}

func FindProjectsForRepo(c *gin.Context) {
	repo := c.Param("repo")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var projects []models.Project

	err := models.DB.GormDB.Preload("Organisation").Preload("Repo").
		Joins("LEFT JOIN repos ON projects.repo_id = repos.id").
		Joins("LEFT JOIN organisations ON projects.organisation_id = organisations.id").
		Where("repos.name = ? AND projects.organisation_id = ?", repo, orgId).Find(&projects).Error
	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		return
	}

	response := make([]interface{}, 0)

	for _, p := range projects {
		jsonStruct := p.MapToJsonStruct()
		response = append(response, jsonStruct)
	}

	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while marshalling response")
		return
	}

	c.JSON(http.StatusOK, response)

}

func FindProjectsForOrg(c *gin.Context) {
	requestedOrganisation := c.Param("organisationId")
	loggedInOrganisation, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if requestedOrganisation == "" {
		requestedOrganisation = fmt.Sprintf("%v", loggedInOrganisation)
	}

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("id = ?", requestedOrganisation).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Could not find organisation: "+requestedOrganisation)
		} else {
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	if org.ID != loggedInOrganisation {
		log.Printf("Organisation ID %v does not match logged in organisation ID %v", org.ID, loggedInOrganisation)
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var projects []models.Project

	err = models.DB.GormDB.Preload("Organisation").Preload("Repo").
		Joins("LEFT JOIN repos ON projects.repo_id = repos.id").
		Joins("LEFT JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", org.ID).Find(&projects).Error

	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		return
	}

	marshalledProjects := make([]interface{}, 0)

	for _, p := range projects {
		marshalled := p.MapToJsonStruct()
		marshalledProjects = append(marshalledProjects, marshalled)
	}

	response := make(map[string]interface{})
	response["projects"] = marshalledProjects

	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while marshalling response")
		return
	}

	c.JSON(http.StatusOK, response)
}

func ProjectDetails(c *gin.Context) {

	currentOrg, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	projectIdStr := c.Param("project_id")

	if projectIdStr == "" {
		c.String(http.StatusBadRequest, "ProjectId not specified")
		return
	}

	projectId, err := strconv.Atoi(projectIdStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ProjectId")
		return
	}

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err = models.DB.GormDB.Where("id = ?", currentOrg).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", currentOrg))
		} else {
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	project, err := models.DB.GetProject(uint(projectId))
	if err != nil {
		log.Printf("could not fetch project: %v", err)
		c.String(http.StatusInternalServerError, "Could not fetch project")
		return
	}

	if project.OrganisationID != org.ID {
		log.Printf("Forbidden access: not allowed to access projectID: %v logged in org: %v", project.OrganisationID, org.ID)
		c.String(http.StatusForbidden, "No access to this project")
		return
	}

	c.JSON(http.StatusOK, project.MapToJsonStruct())
}

type CreateProjectRequest struct {
	Name              string `json:"name"`
	ConfigurationYaml string `json:"configurationYaml"`
}

func ReportProjectsForRepo(c *gin.Context) {
	var request CreateProjectRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		return
	}

	repoName := c.Param("repo")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	var repo models.Repo

	err = models.DB.GormDB.Where("name = ? AND organisation_id = ?", repoName, orgId).First(&repo).Error

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			repo := models.Repo{
				Name:           repoName,
				OrganisationID: org.ID,
				Organisation:   org,
			}

			err = models.DB.GormDB.Create(&repo).Error

			if err != nil {
				log.Printf("Error creating repo: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating repo"})
				return
			}
		} else {
			log.Printf("Error fetching repo: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching repo"})
			return
		}
	}

	var project models.Project

	err = models.DB.GormDB.Where("name = ? AND organisation_id = ? AND repo_id = ?", request.Name, org.ID, repo.ID).First(&project).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			project := models.Project{
				Name:              request.Name,
				ConfigurationYaml: request.ConfigurationYaml,
				RepoID:            repo.ID,
				OrganisationID:    org.ID,
				Repo:              &repo,
				Organisation:      org,
			}

			err = models.DB.GormDB.Create(&project).Error

			if err != nil {
				log.Printf("Error creating project: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating project"})
				return
			}
			c.JSON(http.StatusOK, project.MapToJsonStruct())
		} else {
			log.Printf("Error fetching project: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching project"})
			return
		}
	}
}

func RunHistoryForProject(c *gin.Context) {
	repoName := c.Param("repo")
	projectName := c.Param("project")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	var repo models.Repo

	err = models.DB.GormDB.Where("name = ? AND organisation_id = ?", repoName, orgId).First(&repo).Error

	if err != nil {
		log.Printf("Error fetching repo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching repo"})
		return
	}

	var project models.Project

	err = models.DB.GormDB.Where("name = ? AND repo_id = ? AND organisation_id", projectName, repo.ID, org.ID).First(&project).Error

	if err != nil {
		log.Printf("Error fetching project: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching project"})
		return
	}

	var runHistory []models.ProjectRun

	err = models.DB.GormDB.Where("project_id = ?", project.ID).Find(&runHistory).Error

	if err != nil {
		log.Printf("Error fetching run history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching run history"})
		return
	}

	response := make([]interface{}, 0)

	for _, r := range runHistory {
		response = append(response, r.MapToJsonStruct())
	}

	c.JSON(http.StatusOK, response)
}

type JobSummary struct {
	ResourcesCreated uint `json:"resources_created"`
	ResourcesUpdated uint `json:"resources_updated"`
	ResourcesDeleted uint `json:"resources_deleted"`
}

type SetJobStatusRequest struct {
	Status       string      `json:"status"`
	Timestamp    time.Time   `json:"timestamp"`
	JobSummary   *JobSummary `json:"job_summary"`
	PrCommentUrl string      `json:"pr_comment_url"`
}

func SetJobStatusForProject(c *gin.Context) {
	jobId := c.Param("jobId")

	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var request SetJobStatusRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	job, err := models.DB.GetDiggerJob(jobId)

	if err != nil {
		log.Printf("Error fetching job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching job"})
		return
	}

	switch request.Status {
	case "started":
		job.Status = orchestrator_scheduler.DiggerJobStarted
		client, _, err := utils.GetGithubClient(&utils.DiggerGithubRealClientProvider{}, job.Batch.GithubInstallationId, job.Batch.RepoFullName)
		if err != nil {
			log.Printf("Error Creating github client: %v", err)
		} else {
			_, workflowRunUrl, err := utils.GetWorkflowIdAndUrlFromDiggerJobId(client, job.Batch.RepoOwner, job.Batch.RepoName, job.DiggerJobID)
			if err != nil {
				log.Printf("Error getting workflow ID from job: %v", err)
			} else {
				job.WorkflowRunUrl = &workflowRunUrl
				err = models.DB.UpdateDiggerJob(job)
				if err != nil {
					log.Printf("Error updating digger job: %v", err)
				}
			}
		}
	case "succeeded":
		job.Status = orchestrator_scheduler.DiggerJobSucceeded
		job.PRCommentUrl = request.PrCommentUrl
		err := models.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Unexpected status %v", request.Status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
			return
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic while executing goroutine dispatching digger jobs: %v ", r)
				}
			}()
			ghClientProvider := &utils.DiggerGithubRealClientProvider{}
			installationLink, err := models.DB.GetGithubInstallationLinkForOrg(orgId)
			if err != nil {
				log.Printf("Error fetching installation link: %v", err)
				return
			}

			installations, err := models.DB.GetGithubAppInstallations(installationLink.GithubInstallationId)
			if err != nil {
				log.Printf("Error fetching installation: %v", err)
				return
			}

			if len(installations) == 0 {
				log.Printf("No installations found for installation id %v", installationLink.GithubInstallationId)
				return
			}

			jobLink, err := models.DB.GetDiggerJobLink(jobId)

			if err != nil {
				log.Printf("Error fetching job link: %v", err)
				return
			}

			workflowFileName := "digger_workflow.yml"

			if !strings.Contains(jobLink.RepoFullName, "/") {
				log.Printf("Repo full name %v does not contain a slash", jobLink.RepoFullName)
				return
			}

			repoFullNameSplit := strings.Split(jobLink.RepoFullName, "/")
			client, _, err := ghClientProvider.Get(installations[0].GithubAppId, installationLink.GithubInstallationId)
			err = services.DiggerJobCompleted(client, &job.Batch.ID, job, repoFullNameSplit[0], repoFullNameSplit[1], workflowFileName)
			if err != nil {
				log.Printf("Error triggering job: %v", err)
				return
			}
		}()

		// store digger job summary
		if request.JobSummary != nil {
			models.DB.UpdateDiggerJobSummary(job.DiggerJobID, request.JobSummary.ResourcesCreated, request.JobSummary.ResourcesUpdated, request.JobSummary.ResourcesDeleted)
		}

	case "failed":
		job.Status = orchestrator_scheduler.DiggerJobFailed
	default:
		log.Printf("Unexpected status %v", request.Status)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}
	job.StatusUpdatedAt = request.Timestamp
	err = models.DB.GormDB.Save(&job).Error
	if err != nil {
		log.Printf("Error saving update job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	// get batch ID
	// check if all jobs have succeeded at this point
	// if so, perform merge of PR (if configured to do so)
	batch := job.Batch
	err = models.DB.UpdateBatchStatus(batch)
	if err != nil {
		log.Printf("Error updating batch status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating batch status"})
		return
	}

	err = AutomergePRforBatchIfEnabled(&utils.DiggerGithubRealClientProvider{}, batch)
	if err != nil {
		log.Printf("Error merging PR with automerge option: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error merging PR with automerge option"})
	}

	// return batch summary to client
	res, err := batch.MapToJsonStruct()
	if err != nil {
		log.Printf("Error getting batch details: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting batch details"})

	}

	c.JSON(http.StatusOK, res)
}

type CreateProjectRunRequest struct {
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Status    string    `json:"status"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
}

func CreateRunForProject(c *gin.Context) {
	repoName := c.Param("repo")
	projectName := c.Param("projectName")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	var repo models.Repo

	err = models.DB.GormDB.Where("name = ? AND organisation_id = ?", repoName, orgId).First(&repo).Error

	if err != nil {
		log.Printf("Error fetching repo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching repo"})
		return
	}

	var project models.Project

	err = models.DB.GormDB.Where("name = ? AND repo_id = ? AND organisation_id = ?", projectName, repo.ID, org.ID).First(&project).Error

	if err != nil {
		log.Printf("Error fetching project: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching project"})
		return
	}

	var request CreateProjectRunRequest

	err = c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	run := models.ProjectRun{
		StartedAt: request.StartedAt.UnixMilli(),
		EndedAt:   request.EndedAt.UnixMilli(),
		Status:    request.Status,
		Command:   request.Command,
		Output:    request.Output,
		ProjectID: project.ID,
		Project:   &project,
	}

	err = models.DB.GormDB.Create(&run).Error

	if err != nil {
		log.Printf("Error creating run: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating run"})
		return
	}

	c.JSON(http.StatusOK, run.MapToJsonStruct())
}

func AutomergePRforBatchIfEnabled(gh utils.GithubClientProvider, batch *models.DiggerBatch) error {
	diggerYmlString := batch.DiggerConfig
	diggerConfigYml, err := digger_config.LoadDiggerConfigYamlFromString(diggerYmlString)
	if err != nil {
		log.Printf("Error loading digger config from batch: %v", err)
		return fmt.Errorf("error loading digger config from batch: %v", err)
	}

	var automerge bool
	if diggerConfigYml.AutoMerge != nil {
		automerge = *diggerConfigYml.AutoMerge
	} else {
		automerge = false
	}
	if batch.Status == orchestrator_scheduler.BatchJobSucceeded && batch.BatchType == orchestrator_scheduler.BatchTypeApply && automerge == true {
		ghService, _, err := utils.GetGithubService(
			gh,
			batch.GithubInstallationId,
			batch.RepoFullName,
			batch.RepoOwner,
			batch.RepoName,
		)
		if err != nil {
			log.Printf("Error getting github service: %v", err)
			return fmt.Errorf("error getting github service: %v", err)
		}
		err = ghService.MergePullRequest(batch.PrNumber)
		if err != nil {
			log.Printf("Error merging pull request: %v", err)
			return fmt.Errorf("error merging pull request: %v", err)
		}
	}
	return nil
}
