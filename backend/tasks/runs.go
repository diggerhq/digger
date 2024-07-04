package main

import (
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/github"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"log"
)

func RunQueuesStateMachine(queueItem *models.DiggerRunQueueItem, service ci.PullRequestService) {
	dr := queueItem.DiggerRun
	switch queueItem.DiggerRun.Status {
	case models.RunQueued:
		// trigger plan workflow (trigger the batch)
		job, err := models.DB.GetDiggerJobFromRunStage(dr.PlanStage)
		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}
		runName, err := services.GetRunNameFromJob(*job)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return
		}

		spec, err := services.GetSpecFromJob(*job)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			return
		}

		vcsToken, err := services.GetVCSTokenFromJob(*job)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			return
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

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
		job, err := models.DB.GetDiggerJobFromRunStage(dr.ApplyStage)
		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return
		}
		runName, err := services.GetRunNameFromJob(*job)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return
		}

		spec, err := services.GetSpecFromJob(*job)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			return
		}

		vcsToken, err := services.GetVCSTokenFromJob(*job)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			return
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

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
