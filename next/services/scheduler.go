package services

import (
	"fmt"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"log"
	"os"
	"strconv"
)

func ScheduleJob(ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId string, job *model.DiggerJob, gh utils.GithubClientProvider) error {
	maxConcurrencyForBatch, err := strconv.Atoi(os.Getenv("MAX_DIGGER_CONCURRENCY_PER_BATCH"))
	if err != nil {
		log.Printf("WARN: could not get max concurrency for batch, setting it to 0: %v", err)
		maxConcurrencyForBatch = 0
	}
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
		jobs, err := dbmodels.DB.GetDiggerJobsForBatchWithStatus(batchId, []orchestrator_scheduler.DiggerJobStatus{
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
			job.Status = int16(orchestrator_scheduler.DiggerJobQueuedForRun)
			dbmodels.DB.UpdateDiggerJob(job)
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

func TriggerJob(gh utils.GithubClientProvider, ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId string, job *model.DiggerJob) error {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobID)

	if job.JobSpec == nil {
		log.Printf("Jobspec can't be nil")
		return fmt.Errorf("JobSpec is nil, skipping")
	}
	jobString := string(job.JobSpec)
	log.Printf("jobString: %v \n", jobString)

	runName, err := GetRunNameFromJob(*job)
	if err != nil {
		log.Printf("could not get run name: %v", err)
		return fmt.Errorf("could not get run name %v", err)
	}

	err = RefreshVariableSpecForJob(job)
	if err != nil {
		log.Printf("could not get variable spec from job: %v", err)
		return fmt.Errorf("could not get variable spec from job: %v", err)
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

	job.Status = int16(orchestrator_scheduler.DiggerJobTriggered)
	err = dbmodels.DB.UpdateDiggerJob(job)
	if err != nil {
		log.Printf("failed to Update digger job state: %v\n", err)
		return err
	}

	return nil
}
