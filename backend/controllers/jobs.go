package controllers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-substrate/strate/backend/middleware"
	"github.com/go-substrate/strate/backend/models"
	orchestrator_scheduler "github.com/go-substrate/strate/libs/scheduler"
	"gorm.io/gorm"
)

func GetJobsForRepoApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)
	repoId := c.Param("repo_id")

	if repoId == "" {
		log.Printf("missing parameter: repo_full_name")
		c.String(http.StatusBadRequest, "missing parameter: repo_full_name")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("could not find organisation: %v err: %v", organisationId, err)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			log.Printf("could not fetch organisation: %v err: %v", organisationId, err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	repo, err := models.DB.GetRepoById(org.ID, repoId)
	if err != nil {
		log.Printf("could not fetch repo details %v", err)
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching jobs from database")
		return
	}

	jobsRes, err := models.DB.GetJobsByRepoName(org.ID, repo.RepoFullName)
	if err != nil {
		log.Printf("could not fetch job details")
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching jobs from database")
		return
	}

	// update the values of "status" accordingly
	for i, j := range jobsRes {
		statusInt, err := strconv.Atoi(j.Status)
		if err != nil {
			log.Printf("could not convert status to string: job id: %v status: %v", j.ID, j.Status)
			continue
		}
		statusI := orchestrator_scheduler.DiggerJobStatus(statusInt)
		jobsRes[i].Status = statusI.ToString()
	}

	response := make(map[string]interface{})
	response["repo"] = repo
	response["jobs"] = jobsRes

	c.JSON(http.StatusOK, response)
}
