package controllers

import (
	"github.com/diggerhq/digger/backend/models"
	dbmodels2 "github.com/diggerhq/digger/backend/models/dbmodels"
	"log"
	"net/http"
	"time"

	"github.com/diggerhq/digger/ee/drift/model"
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
	//orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	//if !exists {
	//	c.String(http.StatusForbidden, "Not allowed to access this resource")
	//	return
	//}

	var request SetJobStatusRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	log.Printf("settings status for job: %v, new status: %v, tfout: %v, job summary: %v", jobId, request.Status, request.TerraformOutput, request.JobSummary)

	job, err := dbmodels2.DB.GetDiggerCiJob(jobId)
	if err != nil {
		log.Printf("could not get digger ci job, err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting digger job"})
		return
	}

	switch request.Status {
	case string(models.DiggerJobStarted):
		job.Status = string(models.DiggerJobStarted)
		err := dbmodels2.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}
	case string(models.DiggerJobSucceeded):
		job.Status = string(models.DiggerJobSucceeded)
		job.TerraformOutput = request.TerraformOutput
		job.ResourcesCreated = int32(request.JobSummary.ResourcesCreated)
		job.ResourcesUpdated = int32(request.JobSummary.ResourcesUpdated)
		job.ResourcesDeleted = int32(request.JobSummary.ResourcesDeleted)
		err := dbmodels2.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}

		project, err := dbmodels2.DB.GetProjectById(job.ProjectID)
		if err != nil {
			log.Printf("Error retrieving project: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving project"})
			return

		}
		err = ProjectDriftStateMachineApply(*project, job.TerraformOutput, job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
		if err != nil {
			log.Printf("error while checking drifted project")
		}

	case string(models.DiggerJobFailed):
		job.Status = string(models.DiggerJobFailed)
		job.TerraformOutput = request.TerraformOutput
		err := dbmodels2.DB.UpdateDiggerJob(job)
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
	err = dbmodels2.DB.GormDB.Save(job).Error
	if err != nil {
		log.Printf("Error saving update job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func ProjectDriftStateMachineApply(project model.Project, tfplan string, resourcesCreated int32, resourcesUpdated int32, resourcesDeleted int32) error {
	isEmptyPlan := resourcesCreated == 0 && resourcesUpdated == 0 && resourcesDeleted == 0
	wasEmptyPlan := project.ToCreate == 0 && project.ToUpdate == 0 && project.ToDelete == 0
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
	project.ToCreate = resourcesCreated
	project.ToUpdate = resourcesUpdated
	project.ToDelete = resourcesDeleted
	result := dbmodels2.DB.GormDB.Save(&project)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("project %v, (name: %v) has been updated successfully\n", project.ID, project.Name)
	return nil
}
