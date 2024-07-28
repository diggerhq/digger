package services

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/github"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	nextutils "github.com/diggerhq/digger/next/utils"

	"log"
)

func RunQueuesStateMachine(queueItem *model.DiggerRunQueueItem, service ci.PullRequestService, gh nextutils.GithubClientProvider) error {
	dr, err := dbmodels.DB.GetDiggerRun(queueItem.DiggerRunID)
	if err != nil {
		log.Printf("could not get digger run: %v", err)
		return fmt.Errorf("could not get digger run: %v", err)
	}
	planStage, err := dbmodels.DB.GetDiggerRunStage(dr.PlanStageID)
	if err != nil {
		log.Printf("could not get digger plan stage: %v", err)
		return fmt.Errorf("could not get digger plan stage: %v", err)
	}
	applyStage, err := dbmodels.DB.GetDiggerRunStage(dr.ApplyStageID)
	if err != nil {
		log.Printf("could not get digger apply stage: %v", err)
		return fmt.Errorf("could not get digger apply stage: %v", err)
	}

	switch dr.Status {
	case string(dbmodels.RunQueued):
		// trigger plan workflow (trigger the batch)
		job, err := dbmodels.DB.GetDiggerJobFromRunStage(*planStage)
		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}
		runName, err := GetRunNameFromJob(*job)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return fmt.Errorf("could not get run name: %v", err)
		}

		spec, err := GetSpecFromJob(*job)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			return fmt.Errorf("could not get spec: %v", err)
		}

		vcsToken, err := GetVCSTokenFromJob(*job, gh)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			return fmt.Errorf("could not get vcs token: %v", err)
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

		// change status to RunPendingPlan
		log.Printf("Updating run queueItem item to planning state")
		dr.Status = string(dbmodels.RunPlanning)
		err = dbmodels.DB.UpdateDiggerRun(dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	case string(dbmodels.RunPlanning):
		// Check the status of the batch
		batch, err := dbmodels.DB.GetDiggerBatch(planStage.BatchID)
		if err != nil {
			log.Printf("could not get plan batch: %v", err)
			return fmt.Errorf("could not get plan batch: %v", err)

		}
		batchStatus := batch.Status
		approvalRequired := true

		// if failed then go straight to failed
		if batchStatus == int16(orchestrator_scheduler.BatchJobFailed) {
			dr.Status = string(dbmodels.RunFailed)
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
			err = dbmodels.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

		// if successful then
		if batchStatus == int16(orchestrator_scheduler.BatchJobSucceeded) {
			if approvalRequired {
				dr.Status = string(dbmodels.RunPendingApproval)
				err := dbmodels.DB.UpdateDiggerRun(dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				}
			} else {
				dr.Status = string(dbmodels.RunApproved)
				err := dbmodels.DB.UpdateDiggerRun(dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				}
			}
		}

	case string(dbmodels.RunPendingApproval):
		// do nothing
	case string(dbmodels.RunApproved):
		// trigger apply stage workflow
		job, err := dbmodels.DB.GetDiggerJobFromRunStage(*applyStage)
		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return fmt.Errorf("could not get run name: %v", err)
		}
		runName, err := GetRunNameFromJob(*job)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			return fmt.Errorf("could not get run name: %v", err)
		}

		spec, err := GetSpecFromJob(*job)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			return fmt.Errorf("could not get spec: %v", err)
		}

		vcsToken, err := GetVCSTokenFromJob(*job, gh)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			return fmt.Errorf("could not get spec: %v", err)
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

		dr.Status = string(dbmodels.RunApplying)
		err = dbmodels.DB.UpdateDiggerRun(dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}

	case string(dbmodels.RunApplying):
		// Check the status of the batch
		batch, err := dbmodels.DB.GetDiggerBatch(applyStage.BatchID)
		if err != nil {
			log.Printf("could not get apply batch: %v", err)
			return fmt.Errorf("could not get apply batch: %v", err)
		}
		batchStatus := batch.Status

		// if failed then go straight to failed
		if batchStatus == int16(orchestrator_scheduler.BatchJobFailed) {
			dr.Status = string(dbmodels.RunFailed)
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
			err = dbmodels.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

		// if successful then
		if batchStatus == int16(orchestrator_scheduler.BatchJobSucceeded) {
			dr.Status = string(dbmodels.RunSucceeded)
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

	case string(dbmodels.RunSucceeded):
		// dequeue
		err := dbmodels.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	case string(dbmodels.RunFailed):
		// dequeue
		err := dbmodels.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	default:
		log.Printf("WARN: Recieived unknown DiggerRunStatus: %v", dr.Status)
	}

	return nil
}
