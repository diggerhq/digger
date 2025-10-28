package controllers

import (
	"github.com/diggerhq/digger/backend/models"
    "github.com/diggerhq/digger/drift/middleware"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/gin-gonic/gin"
)

type SetJobStatusRequest struct {
	Status          string                      `json:"status"`
	Timestamp       time.Time                   `json:"timestamp"`
	JobSummary      *iac_utils.IacSummary       `json:"job_summary"`
	Footprint       *iac_utils.IacPlanFootprint `json:"job_plan_footprint"`
	PrCommentUrl    string                      `json:"pr_comment_url"`
	TerraformOutput string                      `json:"terraform_output"`
}

func (mc MainController) SetJobStatusForProject(c *gin.Context) {
	jobId := c.Param("jobId")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		slog.Warn("Organisation ID not found in context", "jobId", jobId)
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

	log.Printf("settings status for job: %v, new status: %v, job summary: %v", jobId, request.Status, request.TerraformOutput, request.JobSummary)

	job, err := models.DB.GetDiggerCiJob(jobId)
	if err != nil {
		log.Printf("could not get digger ci job, err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting digger job"})
		return
	}

	switch request.Status {
	case string(orchestrator_scheduler.DiggerJobStartedString):
		job.Status = orchestrator_scheduler.DiggerJobStarted
		err := models.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}
	case string(orchestrator_scheduler.DiggerJobSucceededString):
		job.Status = orchestrator_scheduler.DiggerJobSucceeded
		job.TerraformOutput = request.TerraformOutput
		err := models.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}

		job, err = models.DB.UpdateDiggerJobSummary(job.DiggerJobID, request.JobSummary.ResourcesCreated, request.JobSummary.ResourcesUpdated, request.JobSummary.ResourcesDeleted)
		if err != nil {
			log.Printf("Error updating job summary: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job summary"})
			return
		}
		project, err := models.DB.GetProjectByName(orgId, job.Batch.RepoFullName, job.ProjectName)
		if err != nil {
			log.Printf("Error retrieving project: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving project"})
			return

		}
		summary := job.DiggerJobSummary
		err = ProjectDriftStateMachineApply(*project, job.TerraformOutput, summary.ResourcesCreated, summary.ResourcesUpdated, summary.ResourcesDeleted)
		if err != nil {
			log.Printf("error while checking drifted project")
		}

	case string(orchestrator_scheduler.DiggerJobFailedString):
		job.Status = orchestrator_scheduler.DiggerJobFailed
		job.TerraformOutput = request.TerraformOutput
		err := models.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", request.Status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
			return
		}

	default:
		log.Printf("Unexpected status %v", request.Status)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	job.UpdatedAt = request.Timestamp
	err = models.DB.GormDB.Save(job).Error
	if err != nil {
		log.Printf("Error saving update job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func ProjectDriftStateMachineApply(project models.Project, tfplan string, resourcesCreated uint, resourcesUpdated uint, resourcesDeleted uint) error {
	isEmptyPlan := resourcesCreated == 0 && resourcesUpdated == 0 && resourcesDeleted == 0
	wasEmptyPlan := project.DriftToCreate == 0 && project.DriftToUpdate == 0 && project.DriftToDelete == 0
	if isEmptyPlan {
		project.DriftStatus = models.DriftStatusNoDrift
	}
	if !isEmptyPlan && wasEmptyPlan {
		project.DriftStatus = models.DriftStatusNewDrift
	}
	if !isEmptyPlan && !wasEmptyPlan {
		if project.DriftTerraformPlan != tfplan {
			if project.DriftStatus == models.DriftStatusAcknowledgeDrift {
				project.DriftStatus = models.DriftStatusNewDrift
			}
		}
	}

	project.DriftTerraformPlan = tfplan
	project.DriftToCreate = resourcesCreated
	project.DriftToUpdate = resourcesUpdated
	project.DriftToDelete = resourcesDeleted
	project.LatestDriftCheck = time.Now()
	result := models.DB.GormDB.Save(&project)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("project %v, (name: %v) has been updated successfully\n", project.ID, project.Name)
	return nil
}
