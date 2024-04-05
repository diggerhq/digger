package main

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/github"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
)

func RunQueuesStateMachine(queueItem *models.DiggerRunQueueItem, service orchestrator.PullRequestService) {
	dr := queueItem.DiggerRun
	switch queueItem.DiggerRun.Status {
	case models.RunQueued:
		// trigger plan workflow (trigger the batch)

		repoOwner := dr.Repo.RepoOrganisation
		repoName := dr.Repo.RepoName
		job, err := models.DB.GetDiggerJobFromRunStage(dr.PlanStage)
		jobSpec := string(job.SerializedJobSpec)
		commentId := int64(2037675659)
		utils.TriggerGithubWorkflow(service.(*github.GithubService).Client, repoOwner, repoName, *job, jobSpec, commentId)

		// change status to RunPendingPlan
		log.Printf("Updating run queueItem item to planning state")
		dr.Status = models.RunPlanning
		err = models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}
	case models.RunPlanning:
		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobSucceeded //dr.PlanStage.Batch.Status
		approvalRequired := true

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			dr.Status = models.RunFailed
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
			}
			err = models.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded {
			if approvalRequired {
				dr.Status = models.RunPendingApproval
				err := models.DB.UpdateDiggerRun(&dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
				}
			} else {
				dr.Status = models.RunApproved
				err := models.DB.UpdateDiggerRun(&dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
				}
			}
		}

	case models.RunPendingApproval:
		// do nothing
	case models.RunApproved:
		// trigger apply stage workflow
		repoOwner := dr.Repo.RepoOrganisation
		repoName := dr.Repo.RepoName
		job, err := models.DB.GetDiggerJobFromRunStage(dr.ApplyStage)
		jobSpec := string(job.SerializedJobSpec)
		commentId := int64(2037675659)
		utils.TriggerGithubWorkflow(service.(*github.GithubService).Client, repoOwner, repoName, *job, jobSpec, commentId)

		dr.Status = models.RunApplying
		err = models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}

	case models.RunApplying:
		// Check the status of the batch
		batchStatus := dr.PlanStage.Batch.Status

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			dr.Status = models.RunFailed
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
			}
			err = models.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded {
			dr.Status = models.RunSucceeded
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
			}
		}

	case models.RunSucceeded:
		// dequeue
		err := models.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}
	case models.RunFailed:
		// dequeue
		err := models.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}
	default:
		log.Printf("WARN: Recieived unknown DiggerRunStatus: %v", queueItem.DiggerRun.Status)
	}
}
