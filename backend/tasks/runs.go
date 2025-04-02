package main

import (
	"log/slog"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/github"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
)

func RunQueuesStateMachine(queueItem *models.DiggerRunQueueItem, service ci.PullRequestService, gh utils.GithubClientProvider) {
	dr := queueItem.DiggerRun

	runContext := slog.Group("runContext",
		"queueItemId", queueItem.ID,
		"runId", queueItem.DiggerRunId,
		"projectName", queueItem.DiggerRun.ProjectName,
		"status", queueItem.DiggerRun.Status)

	slog.Info("processing queue item", runContext)

	switch queueItem.DiggerRun.Status {
	case models.RunQueued:
		slog.Info("starting plan workflow", runContext)

		// trigger plan workflow (trigger the batch)
		job, err := models.DB.GetDiggerJobFromRunStage(dr.PlanStage)
		if err != nil {
			slog.Error("failed to get digger job from run stage",
				"error", err,
				"stageId", dr.PlanStage.ID,
				runContext)
			return
		}

		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}

		runName, err := services.GetRunNameFromJob(*job)
		if err != nil {
			slog.Error("could not get run name",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		spec, err := services.GetSpecFromJob(*job)
		if err != nil {
			slog.Error("could not get spec",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		vcsToken, err := services.GetVCSTokenFromJob(*job, gh)
		if err != nil {
			slog.Error("could not get vcs token",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

		// change status to RunPendingPlan
		slog.Info("updating run to planning state", runContext)
		dr.Status = models.RunPlanning
		err = models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			slog.Error("failed to update digger run",
				"error", err,
				runContext)
		}

	case models.RunPlanning:
		slog.Info("checking plan status", runContext)

		// Check the status of the batch
		batchStatus := orchestrator_scheduler.BatchJobSucceeded //dr.PlanStage.Batch.Status
		approvalRequired := true

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			slog.Info("plan failed, marking run as failed", runContext)
			dr.Status = models.RunFailed
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				slog.Error("failed to update digger run status to failed",
					"error", err,
					runContext)
			}
			err = models.DB.DequeueRunItem(queueItem)
			if err != nil {
				slog.Error("failed to dequeue run item",
					"error", err,
					runContext)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded {
			if approvalRequired {
				slog.Info("plan succeeded, approval required", runContext)
				dr.Status = models.RunPendingApproval
				err := models.DB.UpdateDiggerRun(&dr)
				if err != nil {
					slog.Error("failed to update digger run status to pending approval",
						"error", err,
						runContext)
				}
			} else {
				slog.Info("plan succeeded, auto-approving", runContext)
				dr.Status = models.RunApproved
				err := models.DB.UpdateDiggerRun(&dr)
				if err != nil {
					slog.Error("failed to update digger run status to approved",
						"error", err,
						runContext)
				}
			}
		}

	case models.RunPendingApproval:
		slog.Debug("run pending approval, no action needed", runContext)
		// do nothing

	case models.RunApproved:
		slog.Info("run approved, starting apply workflow", runContext)

		// trigger apply stage workflow
		job, err := models.DB.GetDiggerJobFromRunStage(dr.ApplyStage)
		if err != nil {
			slog.Error("failed to get digger job from apply stage",
				"error", err,
				"stageId", dr.ApplyStage.ID,
				runContext)
			return
		}

		client := service.(*github.GithubService).Client
		ciBackend := ci_backends.GithubActionCi{Client: client}

		runName, err := services.GetRunNameFromJob(*job)
		if err != nil {
			slog.Error("could not get run name",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		spec, err := services.GetSpecFromJob(*job)
		if err != nil {
			slog.Error("could not get spec",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		vcsToken, err := services.GetVCSTokenFromJob(*job, gh)
		if err != nil {
			slog.Error("could not get vcs token",
				"error", err,
				"jobId", job.ID,
				runContext)
			return
		}

		ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)

		slog.Info("updating run to applying state", runContext)
		dr.Status = models.RunApplying
		err = models.DB.UpdateDiggerRun(&dr)
		if err != nil {
			slog.Error("failed to update digger run status to applying",
				"error", err,
				runContext)
		}

	case models.RunApplying:
		slog.Info("checking apply status", runContext)

		// Check the status of the batch
		batchStatus := dr.PlanStage.Batch.Status

		// if failed then go straight to failed
		if batchStatus == orchestrator_scheduler.BatchJobFailed {
			slog.Info("apply failed, marking run as failed", runContext)
			dr.Status = models.RunFailed
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				slog.Error("failed to update digger run status to failed",
					"error", err,
					runContext)
			}
			err = models.DB.DequeueRunItem(queueItem)
			if err != nil {
				slog.Error("failed to dequeue run item",
					"error", err,
					runContext)
			}
		}

		// if successful then
		if batchStatus == orchestrator_scheduler.BatchJobSucceeded {
			slog.Info("apply succeeded, marking run as successful", runContext)
			dr.Status = models.RunSucceeded
			err := models.DB.UpdateDiggerRun(&dr)
			if err != nil {
				slog.Error("failed to update digger run status to succeeded",
					"error", err,
					runContext)
			}
		}

	case models.RunSucceeded:
		slog.Info("run succeeded, dequeuing item", runContext)
		// dequeue
		err := models.DB.DequeueRunItem(queueItem)
		if err != nil {
			slog.Error("failed to dequeue run item",
				"error", err,
				runContext)
		}

	case models.RunFailed:
		slog.Info("run failed, dequeuing item", runContext)
		// dequeue
		err := models.DB.DequeueRunItem(queueItem)
		if err != nil {
			slog.Error("failed to dequeue run item",
				"error", err,
				runContext)
		}

	default:
		slog.Warn("received unknown digger run status",
			"status", queueItem.DiggerRun.Status,
			runContext)
	}
}
