package main

import (
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
)

func RunQueuesStateMachine(queueItem *models.DiggerRunQueueItem, CIBackend ci_backends.CiBackend) {
	dr := queueItem.DiggerRun
	switch queueItem.DiggerRun.Status {
	case models.RunQueued:
		// trigger plan workflow (trigger the batch)
		// .....
		// change status to RunPendingPlan
		log.Printf("Updating run queueItem item to planning state")
		dr.Status = models.RunPlanning
		err := models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}
	case models.RunPlanning:
		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobSucceeded
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
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded && approvalRequired {
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

	case models.RunPendingApproval:
		// do nothing
	case models.RunApproved:
		// trigger apply stage workflow
		// ...
		dr.Status = models.RunApplying
		err := models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunId, queueItem.DiggerRun.ProjectName)
		}

	case models.RunApplying:
		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobSucceeded

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
