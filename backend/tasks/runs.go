package main

import (
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"log"
)

func RunQueuesStateMachine(queue *models.DiggerRunQueue) {
	switch queue.DiggerRun.Status {
	case models.RunQueued:
		// trigger plan workflow (trigger the batch)
		// .....
		// change status to RunPendingPlan
		_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunPlanning)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
		}
	case models.RunPlanning:
		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobStarted
		approvalRequired := true

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunFailed)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
			err = models.DB.DequeueRunItem(queue)
			if err != nil {
				log.Printf("ERROR: Failed to delete queue item: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded && approvalRequired {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunPendingApproval)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		} else {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunApproved)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		}

	case models.RunPendingApproval:
		// do nothing
	case models.RunApproved:
		// trigger apply stage workflow
		// ...
		_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunPendingApproval)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
		}

	case models.RunApplying:
		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobStarted

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunFailed)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
			err = models.DB.DequeueRunItem(queue)
			if err != nil {
				log.Printf("ERROR: Failed to delete queue item: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunPendingApproval)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		} else {
			_, err := models.DB.UpdateDiggerRun(&queue.DiggerRun, models.RunApproved)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
			}
		}

	case models.RunSucceeded:
		// dequeue
		err := models.DB.DequeueRunItem(queue)
		if err != nil {
			log.Printf("ERROR: Failed to delete queue item: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
		}
	case models.RunFailed:
		// dequeue
		err := models.DB.DequeueRunItem(queue)
		if err != nil {
			log.Printf("ERROR: Failed to delete queue item: %v [%v %v]", queue.ID, queue.DiggerRunId, queue.ProjectId)
		}
	default:
		log.Printf("WARN: Recieived unknown DiggerRunStatus: %v", queue.DiggerRun.Status)
	}
}
