package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/iac_utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/services"
	//"github.com/diggerhq/digger/next/middleware"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

type SetJobStatusRequest struct {
	Status          string                      `json:"status"`
	Timestamp       time.Time                   `json:"timestamp"`
	JobSummary      *iac_utils.IacSummary       `json:"job_summary"`
	Footprint       *iac_utils.IacPlanFootprint `json:"job_plan_footprint"`
	PrCommentUrl    string                      `json:"pr_comment_url"`
	TerraformOutput string                      `json:"terraform_output"`
}

func (d DiggerController) SetJobStatusForProject(c *gin.Context) {
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

	job, err := dbmodels.DB.GetDiggerJob(jobId)
	if err != nil {
		log.Printf("Error fetching job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching job"})
		return
	}

	batch, err := dbmodels.DB.GetDiggerBatch(job.BatchID)
	if err != nil {
		log.Printf("Error getting digger batch: %v ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error fetching batch"})
		return
	}

	switch request.Status {
	case "started":
		job.Status = int16(orchestrator_scheduler.DiggerJobStarted)
		err := dbmodels.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating job status"})
			return
		}

		client, _, err := utils.GetGithubClient(d.GithubClientProvider, batch.GithubInstallationID, batch.RepoFullName)
		if err != nil {
			log.Printf("Error Creating github client: %v", err)
		} else {
			_, workflowRunUrl, err := utils.GetWorkflowIdAndUrlFromDiggerJobId(client, batch.RepoOwner, batch.RepoName, job.DiggerJobID)
			if err != nil {
				log.Printf("Error getting workflow ID from job: %v", err)
			} else {
				job.WorkflowRunURL = workflowRunUrl
				err = dbmodels.DB.UpdateDiggerJob(job)
				if err != nil {
					log.Printf("Error updating digger job: %v", err)
				}
			}
		}
	case "succeeded":
		job.Status = int16(orchestrator_scheduler.DiggerJobSucceeded)

		log.Printf("terraform output: %v", job.TerraformOutput)

		job.TerraformOutput = request.TerraformOutput
		if request.Footprint != nil {
			job.PlanFootprint, err = json.Marshal(request.Footprint)
			if err != nil {
				log.Printf("Error marshalling plan footprint: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error marshalling plan footprint"})
			}
		}
		job.PrCommentURL = request.PrCommentUrl
		err := dbmodels.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
			return
		}

		// store digger job summary
		if request.JobSummary != nil {
			dbmodels.DB.UpdateDiggerJobSummary(job.DiggerJobSummaryID, request.JobSummary.ResourcesCreated, request.JobSummary.ResourcesUpdated, request.JobSummary.ResourcesDeleted)
		}

		// update the project drift status
		log.Printf("performing drift maybe for batch %v", batch.EventType)
		if batch.EventType == dbmodels.DiggerBatchDriftEvent {
			log.Printf("performing drift surely for batch %v", batch.EventType)
			err := services.SaveUpdatedDriftStatus(*batch, *job, request.TerraformOutput)
			if err != nil {
				log.Printf("error while updating project drift: %v", err)
			}
		}

	case "failed":
		job.Status = int16(orchestrator_scheduler.DiggerJobFailed)
		job.TerraformOutput = request.TerraformOutput
		err := dbmodels.DB.UpdateDiggerJob(job)
		if err != nil {
			log.Printf("Error updating job status: %v", request.Status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
			return
		}

		batch.Status = int16(orchestrator_scheduler.BatchJobFailed)
		batchErr := dbmodels.DB.GormDB.Save(batch).Error
		if batchErr != nil {
			log.Printf("error while updating batch status: %v", err)
		}

	default:
		log.Printf("Unexpected status %v", request.Status)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	job.StatusUpdatedAt = request.Timestamp
	err = dbmodels.DB.GormDB.Save(&job).Error
	if err != nil {
		log.Printf("Error saving update job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving job"})
		return
	}

	// get batch ID
	// check if all jobs have succeeded at this point
	// if so, perform merge of PR (if configured to do so)
	//batch := job.Batch
	err = dbmodels.DB.UpdateBatchStatus(batch)
	if err != nil {
		log.Printf("Error updating batch status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating batch status"})
		return
	}

	//err = AutomergePRforBatchIfEnabled(d.GithubClientProvider, batch)
	//if err != nil {
	//	log.Printf("Error merging PR with automerge option: %v", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Error merging PR with automerge option"})
	//}

	// return batch summary to client
	res, err := dbmodels.BatchToJsonStruct(*batch)
	if err != nil {
		log.Printf("Error getting batch details: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting batch details"})

	}

	c.JSON(http.StatusOK, res)
}

func UpdateCommentsForBatchGroup(gh utils.GithubClientProvider, batch *model.DiggerBatch, serializedJobs []orchestrator_scheduler.SerializedJob) error {
	diggerYmlString := batch.DiggerConfig
	diggerConfigYml, err := digger_config.LoadDiggerConfigYamlFromString(diggerYmlString)
	if err != nil {
		log.Printf("Error loading digger config from batch: %v", err)
		return fmt.Errorf("error loading digger config from batch: %v", err)
	}

	if diggerConfigYml.CommentRenderMode == nil ||
		*diggerConfigYml.CommentRenderMode != digger_config.CommentRenderModeGroupByModule {
		log.Printf("render mode is not group_by_module, skipping")
		return nil
	}

	if batch.BatchType != string(orchestrator_scheduler.DiggerCommandPlan) && batch.BatchType != string(orchestrator_scheduler.DiggerCommandApply) {
		log.Printf("command is not plan or apply, skipping")
		return nil
	}

	ghService, _, err := utils.GetGithubService(
		gh,
		batch.GithubInstallationID,
		batch.RepoFullName,
		batch.RepoOwner,
		batch.RepoName,
	)

	var sourceDetails []reporting.SourceDetails
	err = json.Unmarshal(batch.SourceDetails, &sourceDetails)
	if err != nil {
		log.Printf("failed to unmarshall sourceDetails: %v", err)
		return fmt.Errorf("failed to unmarshall sourceDetails: %v", err)
	}

	// project_name => terraform output
	projectToTerraformOutput := make(map[string]string)
	// TODO: add projectName as a field of Job
	for _, serialJob := range serializedJobs {
		job, err := models.DB.GetDiggerJob(serialJob.DiggerJobId)
		if err != nil {
			return fmt.Errorf("Could not get digger job: %v", err)
		}
		projectToTerraformOutput[serialJob.ProjectName] = job.TerraformOutput
	}

	for _, detail := range sourceDetails {
		reporter := reporting.SourceGroupingReporter{serializedJobs, int(batch.PrNumber), ghService}
		reporter.UpdateComment(sourceDetails, detail.SourceLocation, projectToTerraformOutput)
	}
	return nil
}
