package controllers

import (
	"log"
	"net/http"
	"time"

	"github.com/diggerhq/digger/ee/drift/dbmodels"
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
	TerraformOutput string                      `json:"terraform_output""`
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

	log.Printf("settings tatus for job: %v, new status: %v, tfout: %v, job summary: %v", jobId, request.Status, request.TerraformOutput, request.JobSummary)

	job, err := dbmodels.DB.GetDiggerCiJob(jobId)
	if err != nil {
		log.Printf("could not get digger ci job, err: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting digger job"})
		return
	}

	switch request.Status {
	case string(dbmodels.DiggerJobStarted):
		job.Status = string(dbmodels.DiggerJobStarted)
		err := dbmodels.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}
	case string(dbmodels.DiggerJobSucceeded):
		job.Status = string(dbmodels.DiggerJobSucceeded)
		job.TerraformOutput = request.TerraformOutput
		job.ResourcesCreated = int32(request.JobSummary.ResourcesCreated)
		job.ResourcesUpdated = int32(request.JobSummary.ResourcesUpdated)
		job.ResourcesDeleted = int32(request.JobSummary.ResourcesDeleted)
		err := dbmodels.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}

		project, err := dbmodels.DB.GetProjectById(job.ProjectID)
		if err != nil {
			log.Printf("Error retriving project: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving project"})
			return

		}
		err = ProjectDriftStateMachineApply(*project, job.TerraformOutput, job.ResourcesCreated, job.ResourcesUpdated, job.ResourcesDeleted)
		if err != nil {
			log.Printf("error while checking drifted project")
		}

	case string(dbmodels.DiggerJobFailed):
		job.Status = string(dbmodels.DiggerJobFailed)
		job.TerraformOutput = request.TerraformOutput
		err := dbmodels.DB.UpdateDiggerJob(job)
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
	err = dbmodels.DB.GormDB.Save(job).Error
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
		project.DriftStatus = dbmodels.DriftStatusNoDrift
	}
	if !isEmptyPlan && wasEmptyPlan {
		project.DriftStatus = dbmodels.DriftStatusNewDrift
	}
	if !isEmptyPlan && !wasEmptyPlan {
		if project.DriftTerraformPlan != tfplan {
			if project.DriftStatus == dbmodels.DriftStatusAcknowledgeDrift {
				project.DriftStatus = dbmodels.DriftStatusNewDrift
			}
		}
	}

	project.DriftTerraformPlan = tfplan
	project.ToCreate = resourcesCreated
	project.ToUpdate = resourcesUpdated
	project.ToDelete = resourcesDeleted
	result := dbmodels.DB.GormDB.Save(&project)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("project %v, (name: %v) has been updated successfully\n", project.ID, project.Name)
	return nil
}
