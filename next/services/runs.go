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
	"time"
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
		applyJob, err := dbmodels.DB.GetDiggerJobFromRunStage(*applyStage)
		planJob, err := dbmodels.DB.GetDiggerJobFromRunStage(*planStage)
		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}
		runName, err := GetRunNameFromJob(*planJob)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "Could not load run name"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get run name: %v", err)
		}

		err = RefreshVariableSpecForJob(planJob)
		if err != nil {
			log.Printf("could not get variable spec from job: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "Could not load variables"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get variable spec from job: %v", err)
		}

		// NOTE: We have to refresh both plan and apply jobs since we want to use exact same variables
		// in both of these jobs
		err = RefreshVariableSpecForJob(applyJob)
		if err != nil {
			log.Printf("could not get variable spec from job: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not load variables"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get variable spec from job: %v", err)
		}

		spec, err := GetSpecFromJob(*planJob)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not prepare job spec for triggering"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get spec: %v", err)
		}

		vcsToken, err := GetVCSTokenFromJob(*planJob, gh)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not fetch VCS token (hint: is your app installed for repo?)"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}

			return fmt.Errorf("could not get vcs token: %v", err)
		}

		err = dbmodels.DB.RefreshDiggerJobTokenExpiry(planJob)
		if err != nil {
			log.Printf("could not refresh job token expiry: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not refresh digger token (likely an internal error)"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not refresh job token from expiry: %v", err)
		}

		err = ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)
		if err != nil {
			log.Printf("ERROR: Failed to trigger for Digger Run queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = fmt.Sprintf("could not trigger workflow, internal error: %v", err)
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}

			return fmt.Errorf("ERROR: Failed to trigger for Digger Run queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}

		// change status to RunPendingPlan
		log.Printf("Updating run queueItem item to planning state")
		dr.Status = string(dbmodels.RunPlanning)
		err = dbmodels.DB.UpdateDiggerRun(dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	case string(dbmodels.RunPlanning):
		// Check the status of the batch
		batch, err := dbmodels.DB.GetDiggerBatch(planStage.BatchID)
		if err != nil {
			log.Printf("could not get plan batch: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not find digger batch"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get plan batch: %v", err)
		}
		batchStatus := batch.Status

		// if failed then go straight to failed
		if batchStatus == int16(orchestrator_scheduler.BatchJobFailed) {
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "The job failed to run, please check action logs for more details"
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
			err = dbmodels.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				return fmt.Errorf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

		// if successful then
		if batchStatus == int16(orchestrator_scheduler.BatchJobSucceeded) {
			project, err := dbmodels.DB.GetProject(dr.ProjectID)
			if err != nil {
				log.Printf("could not get project: %v", err)
				return fmt.Errorf("could not get project: %v", err)
			}

			if project.AutoApprove {
				dr.Status = string(dbmodels.RunApproved)
				dr.ApprovalAuthor = "autoapproved"
				dr.ApprovalDate = time.Now()
				dr.IsApproved = true
				err := dbmodels.DB.UpdateDiggerRun(dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
					return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				}
			} else {
				dr.Status = string(dbmodels.RunPendingApproval)
				err := dbmodels.DB.UpdateDiggerRun(dr)
				if err != nil {
					log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
					return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
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
			log.Printf("could not get job: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not get job from run stage"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get run name: %v", err)
		}
		runName, err := GetRunNameFromJob(*job)
		if err != nil {
			log.Printf("could not get run name: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not get run name"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get run name: %v", err)
		}

		spec, err := GetSpecFromJob(*job)
		if err != nil {
			log.Printf("could not get spec: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could get spec from job"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get spec: %v", err)
		}

		vcsToken, err := GetVCSTokenFromJob(*job, gh)
		if err != nil {
			log.Printf("could not get vcs token: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not fetch vcs token (hint: is the app still installed?)"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}

			return fmt.Errorf("could not get spec: %v", err)
		}

		err = dbmodels.DB.RefreshDiggerJobTokenExpiry(job)
		if err != nil {
			log.Printf("could not refresh job token expiry: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not refresh expiry token"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not refresh job token from expiry: %v", err)
		}

		err = ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)
		if err != nil {
			log.Printf("could not trigger workflow for apply queueItem: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = fmt.Sprintf("could not trigger workflow: %v", err)
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("ERROR: failed to trigger workflow: %v", err)
		}

		dr.Status = string(dbmodels.RunApplying)
		err = dbmodels.DB.UpdateDiggerRun(dr)
		if err != nil {
			log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}

	case string(dbmodels.RunApplying):
		// Check the status of the batch
		batch, err := dbmodels.DB.GetDiggerBatch(applyStage.BatchID)
		if err != nil {
			log.Printf("could not get apply batch: %v", err)
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "could not get apply batch"
			err = dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("Error: could not update digger status to failed: %v", err)
			}
			return fmt.Errorf("could not get apply batch: %v", err)
		}
		batchStatus := batch.Status

		// if failed then go straight to failed
		if batchStatus == int16(orchestrator_scheduler.BatchJobFailed) {
			dr.Status = string(dbmodels.RunFailed)
			dr.FailureReason = "the job failed to run, please refer to action logs for details"
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
			err = dbmodels.DB.DequeueRunItem(queueItem)
			if err != nil {
				log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				return fmt.Errorf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

		// if successful then
		if batchStatus == int16(orchestrator_scheduler.BatchJobSucceeded) {
			dr.Status = string(dbmodels.RunSucceeded)
			err := dbmodels.DB.UpdateDiggerRun(dr)
			if err != nil {
				log.Printf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
				return fmt.Errorf("ERROR: Failed to update Digger Run for queueID: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			}
		}

	case string(dbmodels.RunSucceeded):
		// dequeue
		err := dbmodels.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			return fmt.Errorf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	case string(dbmodels.RunFailed):
		// dequeue
		err := dbmodels.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			return fmt.Errorf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	case string(dbmodels.RunDiscarded):
		// dequeue
		err := dbmodels.DB.DequeueRunItem(queueItem)
		if err != nil {
			log.Printf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
			return fmt.Errorf("ERROR: Failed to delete queueItem item: %v [%v %v]", queueItem.ID, queueItem.DiggerRunID, dr.ProjectName)
		}
	default:
		log.Printf("WARN: Recieived unknown DiggerRunStatus: %v", dr.Status)
	}

	return nil
}
