package services

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
	"log"
	"runtime/debug"
	"time"
)

func DiggerJobCompleted(client *github.Client, batchId *uuid.UUID, parentJob *models.DiggerJob, repoFullName string, repoOwner string, repoName string, workflowFileName string, gh utils.GithubClientProvider) error {
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
			ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, batchId, job, gh)
		}

	}
	return nil
}

func ScheduleJob(ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob, gh utils.GithubClientProvider) error {
	maxConcurrencyForBatch := config.DiggerConfig.GetInt("max_concurrency_per_batch")
	if maxConcurrencyForBatch == 0 {
		// concurrency limits not set
		err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
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
			err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
			if err != nil {
				log.Printf("Could not trigger job: %v", err)
				return err
			}
		}
	}
	return nil
}

func TriggerJob(gh utils.GithubClientProvider, ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId *uuid.UUID, job *models.DiggerJob) error {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobID)

	if job.SerializedJobSpec == nil {
		log.Printf("Jobspec can't be nil")
		return fmt.Errorf("JobSpec is nil, skipping")
	}
	jobString := string(job.SerializedJobSpec)
	log.Printf("jobString: %v \n", jobString)

	runName, err := GetRunNameFromJob(*job)
	if err != nil {
		log.Printf("could not get run name: %v", err)
		return fmt.Errorf("could not get run name %v", err)
	}

	spec, err := GetSpecFromJob(*job)
	if err != nil {
		log.Printf("could not get spec: %v", err)
		return fmt.Errorf("could not get spec %v", err)
	}

	vcsToken, err := GetVCSTokenFromJob(*job, gh)
	if err != nil {
		log.Printf("could not get vcs token: %v", err)
		return fmt.Errorf("could not get vcs token: %v", err)
	}

	err = ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)
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

	go UpdateWorkflowUrlForJob(job, ciBackend, spec, gh)

	return nil
}

// This is meant to run asyncronously since it queries for job url
// in case of github we don't get it immediately but with some delay
func UpdateWorkflowUrlForJob(job *models.DiggerJob, ciBackend ci_backends.CiBackend, spec *spec.Spec, gh utils.GithubClientProvider) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in UpdateWorkflowUrlForJob handler: %v", r)
			log.Printf("\n=== PANIC RECOVERED ===\n")
			log.Printf("Error: %v\n", r)
			log.Printf("Stack Trace:\n%s", string(debug.Stack()))
			log.Printf("=== END PANIC ===\n")
		}
	}()

	batch := job.Batch
	// for now we only perform this update for github
	if batch.VCS != models.DiggerVCSGithub {
		return
	}
	for n := 0; n < 30; n++ {
		time.Sleep(1 * time.Second)
		workflowUrl, err := ciBackend.GetWorkflowUrl(*spec)
		if err != nil {
			log.Printf("DiggerJobId %v: error while attempting to fetch workflow url: %v", job.DiggerJobID, err)
		} else {
			if workflowUrl == "#" || workflowUrl == "" {
				log.Printf("DiggerJobId %v: got blank workflow url as response, ignoring", job.DiggerJobID)
			} else {
				job.WorkflowRunUrl = &workflowUrl
				err = models.DB.UpdateDiggerJob(job)
				if err != nil {
					log.Printf("DiggerJobId %v: Error updating digger job: %v", job.DiggerJobID, err)
					continue
				} else {
					log.Printf("DiggerJobId %v: successfully updated workflow run url to: %v for DiggerJobID: %v", job.DiggerJobID, workflowUrl, job.DiggerJobID)
				}

				// refresh the batch from DB to get accurate results
				batch, err = models.DB.GetDiggerBatch(&job.Batch.ID)
				if err != nil {
					log.Printf("DiggerJobId %v: Error getting batch: %v", job.DiggerJobID, err)
					continue
				}
				res, err := batch.MapToJsonStruct()
				if err != nil {
					log.Printf("DiggerJobId %v: Error getting batch details: %v", job.DiggerJobID, err)
					continue
				}
				// TODO: make this abstract and extracting the right "prService" based on VCS
				client, _, err := utils.GetGithubService(gh, batch.GithubInstallationId, spec.VCS.RepoFullname, spec.VCS.RepoOwner, spec.VCS.RepoName)
				err = comment_updater.BasicCommentUpdater{}.UpdateComment(res.Jobs, *spec.Job.PullRequestNumber, client, spec.CommentId)
				if err != nil {
					log.Printf("diggerJobId: %v error whilst updating comment %v", job.DiggerJobID, err)
					continue
				}
				return
			}
		}
	}

}
