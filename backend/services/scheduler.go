package services

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
)

func DiggerJobCompleted(client *github.Client, batchId *uuid.UUID, parentJob *models.DiggerJob, repoFullName string, repoOwner string, repoName string, workflowFileName string, gh utils.GithubClientProvider) error {
	slog.Info("Job completed", "parentJobId", parentJob.DiggerJobID)

	jobLinksForParent, err := models.DB.GetDiggerJobParentLinksByParentId(&parentJob.DiggerJobID)
	if err != nil {
		slog.Error("Failed to get parent job links", "parentJobId", parentJob.DiggerJobID, "error", err)
		return err
	}

	for _, jobLink := range jobLinksForParent {
		jobLinksForChild, err := models.DB.GetDiggerJobParentLinksChildId(&jobLink.DiggerJobId)
		if err != nil {
			slog.Error("Failed to get child job links", "childJobId", jobLink.DiggerJobId, "error", err)
			return err
		}
		allParentJobsAreComplete := true

		for _, jobLinkForChild := range jobLinksForChild {
			parentJob, err := models.DB.GetDiggerJob(jobLinkForChild.ParentDiggerJobId)
			if err != nil {
				slog.Error("Failed to get parent job", "parentJobId", jobLinkForChild.ParentDiggerJobId, "error", err)
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
				slog.Error("Failed to get job", "jobId", jobLink.DiggerJobId, "error", err)
				return err
			}

			slog.Info("All parent jobs completed, scheduling job",
				"jobId", job.DiggerJobID,
				slog.Group("repository",
					slog.String("fullName", repoFullName),
					slog.String("owner", repoOwner),
					slog.String("name", repoName),
				),
			)

			ciBackend := ci_backends.GithubActionCi{Client: client}
			ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, batchId, job, gh)
		}
	}
	return nil
}

func ScheduleJob(ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob, gh utils.GithubClientProvider) error {
	maxConcurrencyForBatch := config.DiggerConfig.GetInt("max_concurrency_per_batch")

	if maxConcurrencyForBatch == 0 {
		// concurrency limits not set
		slog.Info("Scheduling job without concurrency limit", "jobId", job.DiggerJobID, "batchId", batchId)
		err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
		if err != nil {
			slog.Error("Could not trigger job", "jobId", job.DiggerJobID, "error", err)
			return err
		}
	} else {
		// concurrency limits set
		slog.Info("Scheduling job with concurrency limit",
			"jobId", job.DiggerJobID,
			"batchId", batchId,
			"maxConcurrency", maxConcurrencyForBatch)

		jobs, err := models.DB.GetDiggerJobsForBatchWithStatus(*batchId, []orchestrator_scheduler.DiggerJobStatus{
			orchestrator_scheduler.DiggerJobTriggered,
			orchestrator_scheduler.DiggerJobStarted,
		})
		if err != nil {
			slog.Error("Failed to get jobs for batch", "batchId", batchId, "error", err)
			return err
		}

		slog.Debug("Current running jobs for batch", "count", len(jobs), "batchId", batchId)

		if len(jobs) >= maxConcurrencyForBatch {
			slog.Info("Maximum concurrency reached, queueing job",
				"jobId", job.DiggerJobID,
				"currentJobCount", len(jobs),
				"maxConcurrency", maxConcurrencyForBatch)

			job.Status = orchestrator_scheduler.DiggerJobQueuedForRun
			models.DB.UpdateDiggerJob(job)
			return nil
		} else {
			err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
			if err != nil {
				slog.Error("Could not trigger job", "jobId", job.DiggerJobID, "error", err)
				return err
			}
		}
	}
	return nil
}

func TriggerJob(gh utils.GithubClientProvider, ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob) error {
	slog.Info("Triggering job",
		"jobId", job.DiggerJobID,
		slog.Group("repository",
			slog.String("fullName", repoFullname),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
	)

	if job.SerializedJobSpec == nil {
		slog.Error("Job spec is nil", "jobId", job.DiggerJobID)
		return fmt.Errorf("JobSpec is nil, skipping")
	}

	jobString := string(job.SerializedJobSpec)
	slog.Debug("Job specification", "jobId", job.DiggerJobID, "jobSpec", jobString)

	runName, err := GetRunNameFromJob(*job)
	if err != nil {
		slog.Error("Could not get run name", "jobId", job.DiggerJobID, "error", err)
		return fmt.Errorf("could not get run name %v", err)
	}

	spec, err := GetSpecFromJob(*job)
	if err != nil {
		slog.Error("Could not get spec", "jobId", job.DiggerJobID, "error", err)
		return fmt.Errorf("could not get spec %v", err)
	}

	vcsToken, err := GetVCSTokenFromJob(*job, gh)
	if err != nil {
		slog.Error("Could not get VCS token", "jobId", job.DiggerJobID, "error", err)
		return fmt.Errorf("could not get vcs token: %v", err)
	}

	err = ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)
	if err != nil {
		slog.Error("Failed to trigger workflow", "jobId", job.DiggerJobID, "error", err)
		return err
	}

	job.Status = orchestrator_scheduler.DiggerJobTriggered
	err = models.DB.UpdateDiggerJob(job)
	if err != nil {
		slog.Error("Failed to update job status", "jobId", job.DiggerJobID, "error", err)
		return err
	}

	slog.Info("Job successfully triggered", "jobId", job.DiggerJobID, "status", job.Status)
	go UpdateWorkflowUrlForJob(job, ciBackend, spec)

	return nil
}

// This is meant to run asynchronously since it queries for job url
func UpdateWorkflowUrlForJob(job *models.DiggerJob, ciBackend ci_backends.CiBackend, spec *spec.Spec) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in UpdateWorkflowUrlForJob",
				"jobId", job.DiggerJobID,
				"error", r,
				"stackTrace", stack,
			)
		}
	}()

	batch := job.Batch
	// for now we only perform this update for github
	if batch.VCS != models.DiggerVCSGithub {
		slog.Debug("Skipping workflow URL update for non-GitHub VCS", "jobId", job.DiggerJobID, "vcs", batch.VCS)
		return
	}

	slog.Info("Starting workflow URL update", "jobId", job.DiggerJobID)

	for n := 0; n < 30; n++ {
		time.Sleep(1 * time.Second)
		workflowUrl, err := ciBackend.GetWorkflowUrl(*spec)
		if err != nil {
			slog.Debug("Error fetching workflow URL",
				"jobId", job.DiggerJobID,
				"attempt", n+1,
				"error", err)
		} else {
			if workflowUrl == "#" || workflowUrl == "" {
				slog.Debug("Received blank workflow URL", "jobId", job.DiggerJobID, "attempt", n+1)
			} else {
				job.WorkflowRunUrl = &workflowUrl
				err = models.DB.UpdateDiggerJob(job)
				if err != nil {
					slog.Error("Failed to update job with workflow URL",
						"jobId", job.DiggerJobID,
						"url", workflowUrl,
						"error", err)
					continue
				} else {
					slog.Info("Successfully updated workflow URL",
						"jobId", job.DiggerJobID,
						"url", workflowUrl)
				}
				return
			}
		}
	}

	slog.Warn("Failed to obtain workflow URL after multiple attempts", "jobId", job.DiggerJobID)
	// if we get to here its highly likely that the workflow job entirely failed to start for some reason
}
