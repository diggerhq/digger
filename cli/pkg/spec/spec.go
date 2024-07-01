package spec

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/diggerhq/digger/libs/spec"
	"log"
	"os"
	"strconv"
	"time"
)

func RunSpec(
	spec spec.Spec,
	vcsProvider spec.VCSProvider,
	jobProvider spec.JobSpecProvider,
	lockProvider spec.LockProvider,
	reporterProvider spec.ReporterProvider,
	backedProvider spec.BackendApiProvider,
	policyProvider spec.PolicyProvider,
	PlanStorageProvider spec.PlanStorageProvider,
	commentUpdaterProvider comment_summary.CommentUpdaterProvider,
) error {

	job, err := jobProvider.GetJob(spec.Job)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get job: %v", err), 1)
	}

	lock, err := lockProvider.GetLock(spec.Lock)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get job: %v", err), 1)

	}

	prService, err := vcsProvider.GetPrService(spec.VCS)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get prservice: %v", err), 1)
	}

	reporter, err := reporterProvider.GetReporter(spec.Reporter, prService, *spec.Job.PullRequestNumber)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get reporter: %v", err), 1)
	}

	backendApi, err := backedProvider.GetBackendApi(spec.Backend)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get backend api: %v", err), 1)
	}

	policyChecker, err := policyProvider.GetPolicyProvider(spec.Policy, spec.Backend.BackendHostname, spec.Backend.BackendOrganisationName, spec.Backend.BackendJobToken)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get policy provider: %v", err), 1)
	}

	changedFiles, err := prService.GetChangedFiles(*spec.Job.PullRequestNumber)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get changed files: %v", err), 1)
	}
	diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", true, changedFiles)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	commentUpdater, err := commentUpdaterProvider.Get(*diggerConfig)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get comment updater: %v", err), 8)
	}

	planStorage, err := PlanStorageProvider.GetPlanStorage(spec.VCS.RepoOwner, spec.VCS.RepoName, *spec.Job.PullRequestNumber)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get plan storage: %v", err), 8)
	}

	jobs := []orchestrator.Job{job}

	fullRepoName := fmt.Sprintf("%v-%v", spec.VCS.RepoOwner, spec.VCS.RepoName)
	_, err = backendApi.ReportProjectJobStatus(fullRepoName, spec.Job.ProjectName, spec.JobId, "started", time.Now(), nil, "", "")
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %v", err), 4)
	}

	commentId64, err := strconv.ParseInt(spec.CommentId, 10, 64)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("failed to get comment ID: %v", err), 4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	// TODO: do not require conversion to gh service
	ghService := prService.(orchestrator_github.GithubService)
	allAppliesSuccess, _, err := digger.RunJobs(jobs, prService, ghService, lock, reporter, planStorage, policyChecker, commentUpdater, backendApi, spec.JobId, true, false, commentId64, currentDir)
	if !allAppliesSuccess || err != nil {
		serializedBatch, reportingError := backendApi.ReportProjectJobStatus(spec.VCS.RepoName, spec.Job.ProjectName, spec.JobId, "failed", time.Now(), nil, "", "")
		if reportingError != nil {
			usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed run commands. %s", err), 5)
		}
		commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, prService, commentId64)
		digger.UpdateAggregateStatus(serializedBatch, prService)
		usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed to run commands. %s", err), 5)
	}
	usage.ReportErrorAndExit(spec.VCS.RepoOwner, "Digger finished successfully", 0)

	return nil
}
