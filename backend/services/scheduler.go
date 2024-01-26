package services

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v58/github"
	"github.com/google/uuid"
	"log"
)

func DiggerJobCompleted(client *github.Client, batchId *uuid.UUID, parentJob *models.DiggerJob, repoOwner string, repoName string, workflowFileName string) error {
	log.Printf("DiggerJobCompleted parentJobId: %v", parentJob.DiggerJobID)

	jobLinksForParent, err := models.DB.GetDiggerJobParentLinksByParentId(&parentJob.DiggerJobID)
	if err != nil {
		return err
	}

	for _, jobLink := range jobLinksForParent {
		jobLinksForChild, err := models.DB.GetDiggerJobParentLinksChildId(&jobLink.DiggerJobId)
		if err != nil {
			return err
		}
		allParentJobsAreComplete := true

		for _, jobLinkForChild := range jobLinksForChild {
			parentJob, err := models.DB.GetDiggerJob(jobLinkForChild.ParentDiggerJobId)
			if err != nil {
				return err
			}

			if parentJob.Status != orchestrator_scheduler.DiggerJobSucceeded {
				allParentJobsAreComplete = false
				break
			}

		}

		if allParentJobsAreComplete {
			job, err := models.DB.GetDiggerJob(jobLink.DiggerJobId)
			if err != nil {
				return err
			}
			TriggerJob(client, repoOwner, repoName, batchId, job)
		}

	}
	return nil
}

func TriggerJob(client *github.Client, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob) {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobID)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return
	}

	if job.SerializedJobSpec == nil {
		log.Printf("GitHub job can't be nil")
	}
	jobString := string(job.SerializedJobSpec)
	log.Printf("jobString: %v \n", jobString)

	err = utils.TriggerGithubWorkflow(client, repoOwner, repoName, *job, jobString, *batch.CommentId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return
	}

	_, workflowRunUrl, err := utils.GetWorkflowIdAndUrlFromDiggerJobId(client, repoOwner, repoName, job.DiggerJobID)
	if err != nil {
		log.Printf("failed to find workflow url: %v\n", err)
	}

	job.Status = orchestrator_scheduler.DiggerJobTriggered
	job.WorkflowRunUrl = &workflowRunUrl
	err = models.DB.UpdateDiggerJob(job)
	if err != nil {
		log.Printf("failed to Update digger job state: %v\n", err)
	}
}
