package services

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/google/go-github/v55/github"
	"github.com/google/uuid"
	"log"
)

func DiggerJobCompleted(client *github.Client, batchId *uuid.UUID, parentJob *models.DiggerJob, repoOwner string, repoName string, workflowFileName string) error {
	log.Printf("DiggerJobCompleted parentJobId: %v", parentJob.DiggerJobId)

	jobLinksForParent, err := models.DB.GetDiggerJobParentLinksByParentId(&parentJob.DiggerJobId)
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

			if parentJob.Status != models.DiggerJobSucceeded {
				allParentJobsAreComplete = false
				break
			}

		}

		if allParentJobsAreComplete {
			job, err := models.DB.GetDiggerJob(jobLink.DiggerJobId)
			if err != nil {
				return err
			}
			TriggerJob(client, repoOwner, repoName, batchId, job, workflowFileName)
		}

	}
	return nil
}

func TriggerJob(client *github.Client, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob, workflowFileName string) {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobId)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return
	}

	if job.SerializedJob == nil {
		log.Printf("GitHub job can't be nil")
	}
	jobString := string(job.SerializedJob)
	log.Printf("jobString: %v \n", jobString)

	err = utils.TriggerGithubWorkflow(client, repoOwner, repoName, *job, jobString, *batch.CommentId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return
	}
}
