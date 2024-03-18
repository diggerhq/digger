package controllers

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func FindProjectsForOrganisation(c *gin.Context) {
	requestedOrganisation := c.Param("organisation")
	loggedInOrganisation, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("name = ?", requestedOrganisation).First(&org).Error
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

	response := make([]interface{}, 0)

	for _, p := range projects {
		marshalled := p.MapToJsonStruct()
		response = append(response, marshalled)
	}

	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while marshalling response")
		return
	}

	c.JSON(http.StatusOK, response)
}

func CreateNewRun(c *gin.Context) {
	requestedOrganisation := c.Param("organisation")
	loggedInOrganisation, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	//project := c.Param("projectid")

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("name = ?", requestedOrganisation).First(&org).Error
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

	installationId, repoName, repoOwner, repoFullName, cloneURL, prNumber := 47778898, "digger_demo_multiworkflow", "diggerhq", "diggerhq/digger_demo_multiworkflow", "https://github.com/diggerhq/digger_demo_multiworkflow.git", 3
	diggerYmlStr, ghService, config, projectsGraph, branch, err := getDiggerConfig(gh, int64(installationId), repoFullName, repoOwner, repoName, cloneURL, prNumber)
	if err != nil {
		log.Printf("getDiggerConfig error: %v", err)
		fmt.Errorf("Error getting digger config %v", err)
		c.String(http.StatusForbidden, "Error getting digger config")
		return
	}

	var impactedProjects []digger_config.Project
	changedFiles, err := ghService.GetChangedFiles(prNumber)

	if err != nil {
		return nil, prNumber, fmt.Errorf("could not get changed files")
	}
	impactedProjects = config.GetModifiedProjects(changedFiles)

	jobsForImpactedProjects, _, err := dg_github.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, config.Workflows)
	if err != nil {
		log.Printf("Error converting event to jobsForImpactedProjects: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
		return fmt.Errorf("error converting event to jobsForImpactedProjects")
	}

	//err = utils.TriggerGithubWorkflow(client, repoOwner, repoName, *job, jobString, *batch.CommentId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return
	}

	response := struct {
		status string `json:"status"`
	}{
		status: "success",
	}

	c.JSON(http.StatusOK, response)
}
