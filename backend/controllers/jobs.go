package controllers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
)

func GetJobsForRepoApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)
	repoId := c.Param("repo_id")

	if repoId == "" {
		slog.Warn("Missing parameter", "parameter", "repo_full_name")
		c.String(http.StatusBadRequest, "missing parameter: repo_full_name")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "error", err)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Could not fetch organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	repo, err := models.DB.GetRepoById(org.ID, repoId)
	if err != nil {
		slog.Error("Could not fetch repo details", "repoId", repoId, "orgId", org.ID, "error", err)
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching jobs from database")
		return
	}

	jobsRes, err := models.DB.GetJobsByRepoName(org.ID, repo.RepoFullName)
	if err != nil {
		slog.Error("Could not fetch job details", "repoFullName", repo.RepoFullName, "orgId", org.ID, "error", err)
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching jobs from database")
		return
	}

	// update the values of "status" accordingly
	for i, j := range jobsRes {
		statusInt, err := strconv.Atoi(j.Status)
		if err != nil {
			slog.Error("Could not convert status to string", "jobId", j.ID, "status", j.Status, "error", err)
			continue
		}
		statusI := orchestrator_scheduler.DiggerJobStatus(statusInt)
		jobsRes[i].Status = statusI.ToString()
	}

	slog.Info("Successfully fetched jobs for repo",
		"repoId", repoId,
		"repoFullName", repo.RepoFullName,
		"orgId", org.ID,
		"jobCount", len(jobsRes))

	response := make(map[string]interface{})
	response["repo"] = repo
	response["jobs"] = jobsRes

	c.JSON(http.StatusOK, response)
}
