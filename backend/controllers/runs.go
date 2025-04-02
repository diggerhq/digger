package controllers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RunsForProject(c *gin.Context) {
	currentOrg, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	projectIdStr := c.Param("project_id")

	if projectIdStr == "" {
		slog.Warn("ProjectId not specified")
		c.String(http.StatusBadRequest, "ProjectId not specified")
		return
	}

	projectId, err := strconv.Atoi(projectIdStr)
	if err != nil {
		slog.Warn("Invalid ProjectId", "projectIdStr", projectIdStr, "error", err)
		c.String(http.StatusBadRequest, "Invalid ProjectId")
		return
	}

	if !exists {
		slog.Warn("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err = models.DB.GormDB.Where("id = ?", currentOrg).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", currentOrg)
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", currentOrg))
		} else {
			slog.Error("Error fetching organisation", "organisationId", currentOrg, "error", err)
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	project, err := models.DB.GetProject(uint(projectId))
	if err != nil {
		slog.Error("Could not fetch project", "projectId", projectId, "error", err)
		c.String(http.StatusInternalServerError, "Could not fetch project")
		return
	}

	if project.OrganisationID != org.ID {
		slog.Warn("Forbidden access: not allowed to access project",
			"projectOrgId", project.OrganisationID,
			"loggedInOrgId", org.ID,
			"projectId", projectId)
		c.String(http.StatusForbidden, "No access to this project")
		return
	}

	runs, err := models.DB.ListDiggerRunsForProject(project.Name, project.Repo.ID)
	if err != nil {
		slog.Error("Could not fetch runs", "projectId", projectId, "repoId", project.Repo.ID, "error", err)
		c.String(http.StatusInternalServerError, "Could not fetch runs")
		return
	}

	serializedRuns := make([]interface{}, 0)
	for _, run := range runs {
		serializedRun, err := run.MapToJsonStruct()
		if err != nil {
			slog.Error("Could not unmarshal run", "runId", run.ID, "error", err)
			c.String(http.StatusInternalServerError, "Could not unmarshal runs")
			return
		}
		serializedRuns = append(serializedRuns, serializedRun)
	}

	slog.Info("Successfully fetched runs for project",
		"projectId", projectId,
		"projectName", project.Name,
		"repoId", project.Repo.ID,
		"runCount", len(runs))

	response := make(map[string]interface{})
	response["runs"] = serializedRuns
	c.JSON(http.StatusOK, response)
}

func RunDetails(c *gin.Context) {
	currentOrg, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	runIdStr := c.Param("run_id")

	if runIdStr == "" {
		slog.Warn("RunID not specified")
		c.String(http.StatusBadRequest, "RunID not specified")
		return
	}

	runId, err := strconv.Atoi(runIdStr)
	if err != nil {
		slog.Warn("Invalid RunId", "runIdStr", runIdStr, "error", err)
		c.String(http.StatusBadRequest, "Invalid RunId")
		return
	}

	if !exists {
		slog.Warn("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err = models.DB.GormDB.Where("id = ?", currentOrg).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", currentOrg)
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", currentOrg))
		} else {
			slog.Error("Error fetching organisation", "organisationId", currentOrg, "error", err)
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	run, err := models.DB.GetDiggerRun(uint(runId))
	if err != nil {
		slog.Error("Could not fetch run", "runId", runId, "error", err)
		c.String(http.StatusBadRequest, "Could not fetch run, please check that it exists")
		return
	}

	if run.Repo.OrganisationID != org.ID {
		slog.Warn("Forbidden access: not allowed to access run",
			"runRepoOrgId", run.Repo.OrganisationID,
			"loggedInOrgId", org.ID,
			"runId", runId)
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	response, err := run.MapToJsonStruct()
	if err != nil {
		slog.Error("Could not unmarshal run data", "runId", runId, "error", err)
		c.String(http.StatusInternalServerError, "Could not unmarshall data")
		return
	}

	slog.Info("Successfully fetched run details", "runId", runId)
	c.JSON(http.StatusOK, response)
}

func ApproveRun(c *gin.Context) {
	currentOrg, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	runIdStr := c.Param("run_id")

	if runIdStr == "" {
		slog.Warn("RunID not specified")
		c.String(http.StatusBadRequest, "RunID not specified")
		return
	}

	runId, err := strconv.Atoi(runIdStr)
	if err != nil {
		slog.Warn("Invalid RunId", "runIdStr", runIdStr, "error", err)
		c.String(http.StatusBadRequest, "Invalid RunId")
		return
	}

	if !exists {
		slog.Warn("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err = models.DB.GormDB.Where("id = ?", currentOrg).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", currentOrg)
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", currentOrg))
		} else {
			slog.Error("Error fetching organisation", "organisationId", currentOrg, "error", err)
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	run, err := models.DB.GetDiggerRun(uint(runId))
	if err != nil {
		slog.Error("Could not fetch run", "runId", runId, "error", err)
		c.String(http.StatusBadRequest, "Could not fetch run, please check that it exists")
		return
	}

	if run.Repo.OrganisationID != org.ID {
		slog.Warn("Forbidden access: not allowed to approve run",
			"runRepoOrgId", run.Repo.OrganisationID,
			"loggedInOrgId", org.ID,
			"runId", runId)
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	if run.Status != models.RunPendingApproval {
		slog.Warn("Run status not ready for approval", "runId", run.ID, "status", run.Status)
		c.String(http.StatusBadRequest, "Approval not possible for run (%v) because status is %v", run.ID, run.Status)
		return
	}

	if run.IsApproved == false {
		run.ApprovalAuthor = "a_user"
		run.IsApproved = true
		run.ApprovalDate = time.Now()
		err := models.DB.UpdateDiggerRun(run)
		if err != nil {
			slog.Error("Could not update run approval status", "runId", runId, "error", err)
			c.String(http.StatusInternalServerError, "Could not update approval")
			return
		}
		slog.Info("Run approved successfully", "runId", runId)
	} else {
		slog.Info("Run has already been approved", "runId", runId)
	}

	response, err := run.MapToJsonStruct()
	if err != nil {
		slog.Error("Could not unmarshal run data", "runId", runId, "error", err)
		c.String(http.StatusInternalServerError, "Could not unmarshall data")
		return
	}

	c.JSON(http.StatusOK, response)
}
