package spec

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	backend2 "github.com/diggerhq/digger/libs/backendapi"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/samber/lo"
	"log"
	"os"
	"os/exec"
	"time"
)

func reporterError(spec spec.Spec, backendApi backend2.Api, err error) {
	_, reportingError := backendApi.ReportProjectJobStatus(spec.VCS.RepoName, spec.Job.ProjectName, spec.JobId, "failed", time.Now(), nil, "", "")
	if reportingError != nil {
		usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed run commands. %s", err), 5)
	}
}

func RunSpecNext(
	spec spec.Spec,
	vcsProvider spec.VCSProvider,
	jobProvider spec.JobSpecProvider,
	lockProvider spec.LockProvider,
	reporterProvider spec.ReporterProvider,
	backedProvider spec.BackendApiProvider,
	policyProvider spec.SpecPolicyProvider,
	PlanStorageProvider spec.PlanStorageProvider,
	VariablesProvider spec.VariablesProvider,
	commentUpdaterProvider comment_summary.CommentUpdaterProvider,
) error {

	backendApi, err := backedProvider.GetBackendApi(spec.Backend)
	if err != nil {
		log.Printf("error getting backend api: %v", err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get backend api: %v", err), 1)
	}

	// checking out to the commit ID
	log.Printf("checking out to commit ID %v", spec.Job.Commit)
	cmd := exec.Command("git", "checkout", spec.Job.Commit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Printf("error while checking out to commit SHA: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("error while checking out to commit sha: %v", err), 1)
	}

	job, err := jobProvider.GetJob(spec.Job)
	if err != nil {
		log.Printf("error getting job: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get job: %v", err), 1)
	}

	// get variables from the variables spec
	variablesMap, err := VariablesProvider.GetVariables(spec.Variables)
	if err != nil {
		log.Printf("could not get variables from provider: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get variables from provider: %v", err), 1)
	}
	job.StateEnvVars = lo.Assign(job.StateEnvVars, variablesMap)
	job.CommandEnvVars = lo.Assign(job.CommandEnvVars, variablesMap)

	lock, err := lockProvider.GetLock(spec.Lock)
	if err != nil {
		log.Printf("error getting lock: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get lock: %v", err), 1)
	}

	prService, err := vcsProvider.GetPrService(spec.VCS)
	if err != nil {
		log.Printf("error getting prservice: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get prservice: %v", err), 1)
	}

	orgService, err := vcsProvider.GetOrgService(spec.VCS)
	if err != nil {
		log.Printf("error getting orgservice: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get orgservice: %v", err), 1)
	}
	reporter, err := reporterProvider.GetReporter(fmt.Sprintf("%v for %v", spec.Job.JobType, job.ProjectName), spec.Reporter, prService, *spec.Job.PullRequestNumber)
	if err != nil {
		log.Printf("error getting reporter: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get reporter: %v", err), 1)
	}

	policyChecker, err := policyProvider.GetPolicyProvider(spec.Policy, spec.Backend.BackendHostname, spec.Backend.BackendOrganisationName, spec.Backend.BackendJobToken)
	if err != nil {
		log.Printf("error getting policy provider: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get policy provider: %v", err), 1)
	}

	changedFiles, err := prService.GetChangedFiles(*spec.Job.PullRequestNumber)
	if err != nil {
		log.Printf("error getting changed files: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get changed files: %v", err), 1)
	}

	diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", false, changedFiles)
	if err != nil {
		log.Printf("error getting digger config: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to read Digger digger_config. %s", err), 4)
	}
	log.Printf("Digger digger_config read successfully\n")

	commentUpdater, err := commentUpdaterProvider.Get(*diggerConfig)
	if err != nil {
		log.Printf("error getting comment updater: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get comment updater: %v", err), 8)
	}

	planStorage, err := PlanStorageProvider.GetPlanStorage(spec.VCS.RepoOwner, spec.VCS.RepoName, *spec.Job.PullRequestNumber)
	if err != nil {
		log.Printf("error getting plan storage: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get plan storage: %v", err), 8)
	}

	jobs := []scheduler.Job{job}

	fullRepoName := fmt.Sprintf("%v-%v", spec.VCS.RepoOwner, spec.VCS.RepoName)
	_, err = backendApi.ReportProjectJobStatus(fullRepoName, spec.Job.ProjectName, spec.JobId, "started", time.Now(), nil, "", "")
	if err != nil {
		log.Printf("error getting project status: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %v", err), 4)
	}

	commentId := spec.CommentId

	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("error getting current directory: %v", err)
		reporterError(spec, backendApi, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	reportTerraformOutput := spec.Reporter.ReportTerraformOutput
	allAppliesSuccess, _, err := digger.RunJobs(jobs, prService, orgService, lock, reporter, planStorage, policyChecker, commentUpdater, backendApi, spec.JobId, true, reportTerraformOutput, commentId, currentDir)
	if !allAppliesSuccess || err != nil {
		serializedBatch, reportingError := backendApi.ReportProjectJobStatus(spec.VCS.RepoName, spec.Job.ProjectName, spec.JobId, "failed", time.Now(), nil, "", "")
		if reportingError != nil {
			usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed run commands. %s", err), 5)
		}
		commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, prService, commentId)
		digger.UpdateAggregateStatus(serializedBatch, prService)
		usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed to run commands. %s", err), 5)
	}
	usage.ReportErrorAndExit(spec.VCS.RepoOwner, "Digger finished successfully", 0)

	return nil
}
