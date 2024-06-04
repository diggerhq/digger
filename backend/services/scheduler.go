package services

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v61/github"
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
			ciBackend := ci_backends.GithubActionCi{Client: client}
			ScheduleJob(ciBackend, repoOwner, repoName, batchId, job)
		}

	}
	return nil
}

func ScheduleJob(ciBackend ci_backends.CiBackend, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob) error {
	maxConcurrencyForBatch := config.DiggerConfig.GetInt("max_concurrency_per_batch")
	if maxConcurrencyForBatch == 0 {
		// concurrency limits not set
		err := TriggerJob(ciBackend, repoOwner, repoName, batchId, job)
		if err != nil {
			log.Printf("Could not trigger job: %v", err)
			return err
		}
	} else {
		// concurrency limits set
		log.Printf("Scheduling job with concurrency limit: %v per batch", maxConcurrencyForBatch)
		jobs, err := models.DB.GetDiggerJobsForBatchWithStatus(*batchId, []orchestrator_scheduler.DiggerJobStatus{
			orchestrator_scheduler.DiggerJobTriggered,
			orchestrator_scheduler.DiggerJobStarted,
		})
		if err != nil {
			log.Printf("GetDiggerJobsForBatchWithStatus err: %v\n", err)
			return err
		}
		log.Printf("Length of jobs: %v", len(jobs))
		if len(jobs) >= maxConcurrencyForBatch {
			log.Printf("max concurrency for jobs reached: %v, queuing until more jobs succeed", len(jobs))
			job.Status = orchestrator_scheduler.DiggerJobQueuedForRun
			models.DB.UpdateDiggerJob(job)
			return nil
		} else {
			err := TriggerJob(ciBackend, repoOwner, repoName, batchId, job)
			if err != nil {
				log.Printf("Could not trigger job: %v", err)
				return err
			}
		}
	}
	return nil
}

func TriggerJob(ciBackend ci_backends.CiBackend, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob) error {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobID)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return err
	}

	if job.SerializedJobSpec == nil {
		log.Printf("GitHub job can't be nil")
		return fmt.Errorf("JobSpec is nil, skipping")
	}
	jobString := string(job.SerializedJobSpec)
	log.Printf("jobString: %v \n", jobString)

	err = ciBackend.TriggerWorkflow(repoOwner, repoName, *job, jobString, *batch.CommentId)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return err
	}

	job.Status = orchestrator_scheduler.DiggerJobTriggered
	err = models.DB.UpdateDiggerJob(job)
	if err != nil {
		log.Printf("failed to Update digger job state: %v\n", err)
		return err
	}

	return nil
}
