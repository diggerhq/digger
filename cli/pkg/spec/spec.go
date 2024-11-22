package spec

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	backend2 "github.com/diggerhq/digger/libs/backendapi"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/samber/lo"
	"log"
	"os"
	"os/exec"
	"time"
)

func reportError(spec spec.Spec, backendApi backend2.Api, message string, err error) {
	log.Printf(message)
	_, reportingError := backendApi.ReportProjectJobStatus(spec.VCS.RepoName, spec.Job.ProjectName, spec.JobId, "failed", time.Now(), nil, "", "", "", nil)
	if reportingError != nil {
		usage.ReportErrorAndExit(spec.VCS.RepoOwner, fmt.Sprintf("Failed to run commands. %v", err), 5)
	}
	usage.ReportErrorAndExit(spec.VCS.Actor, message, 1)
}

func RunSpec(
	spec spec.Spec,
	vcsProvider spec.VCSProvider,
	jobProvider spec.JobSpecProvider,
	lockProvider spec.LockProvider,
	reporterProvider spec.ReporterProvider,
	backedProvider spec.BackendApiProvider,
	policyProvider spec.SpecPolicyProvider,
	PlanStorageProvider spec.PlanStorageProvider,
	variablesProvider spec.VariablesProvider,
	commentUpdaterProvider comment_summary.CommentUpdaterProvider,
) error {

	backendApi, err := backedProvider.GetBackendApi(spec.Backend)
	if err != nil {
		log.Printf("could not get backend api: %v", err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get backend api: %v", err), 1)
	}

	// for additional output reporting
	diggerOutPath := os.Getenv("DIGGER_OUT")
	if diggerOutPath == "" {
		diggerOutPath = os.Getenv("RUNNER_TEMP") + "/digger-out.log"
		os.Setenv("DIGGER_OUT", diggerOutPath)
	}

	if spec.Job.Commit != "" {
		// checking out to the commit ID
		log.Printf("fetching commit ID %v", spec.Job.Commit)
		fetchCmd := exec.Command("git", "fetch", "origin", spec.Job.Commit)
		fetchCmd.Stdout = os.Stdout
		fetchCmd.Stderr = os.Stderr
		err = fetchCmd.Run()
		if err != nil {
			msg := fmt.Sprintf("error while fetching commit SHA: %v", err)
			reportError(spec, backendApi, msg, err)
			usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("error while checking out to commit sha: %v", err), 1)
		}

		log.Printf("checking out to commit ID %v", spec.Job.Commit)
		cmd := exec.Command("git", "checkout", spec.Job.Commit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			msg := fmt.Sprintf("error while checking out to commit SHA: %v", err)
			reportError(spec, backendApi, msg, err)
			usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("error while checking out to commit sha: %v", err), 1)
		}
	}

	job, err := jobProvider.GetJob(spec.Job)
	if err != nil {
		message := fmt.Sprintf("could not get job: %v", err)
		reportError(spec, backendApi, message, err)
	}

	lock, err := lockProvider.GetLock(spec.Lock)
	if err != nil {
		message := fmt.Sprintf("could not get lock provider: %v", err)
		reportError(spec, backendApi, message, err)
	}

	prService, err := vcsProvider.GetPrService(spec.VCS)
	if err != nil {
		message := fmt.Sprintf("could not get pr service: %v", err)
		reportError(spec, backendApi, message, err)
	}

	orgService, err := vcsProvider.GetOrgService(spec.VCS)
	if err != nil {
		message := fmt.Sprintf("could not get org service: %v", err)
		reportError(spec, backendApi, message, err)
	}

	reporter, err := reporterProvider.GetReporter(fmt.Sprintf("%v for %v", spec.Job.JobType, job.ProjectName), spec.Reporter, prService, *spec.Job.PullRequestNumber)
	if err != nil {
		message := fmt.Sprintf("could not get reporter: %v", err)
		reportError(spec, backendApi, message, err)
	}

	policyChecker, err := policyProvider.GetPolicyProvider(spec.Policy, spec.Backend.BackendHostname, spec.Backend.BackendOrganisationName, spec.Backend.BackendJobToken)
	if err != nil {
		message := fmt.Sprintf("could not get policy provider: %v", err)
		reportError(spec, backendApi, message, err)
	}

	// TODO: render mode being passable from the spec as a string
	commentUpdater, err := commentUpdaterProvider.Get(spec.CommentUpdater.CommentUpdaterType)
	if err != nil {
		message := fmt.Sprintf("could not get comment updater: %v", err)
		reportError(spec, backendApi, message, err)
	}

	planStorage, err := PlanStorageProvider.GetPlanStorage(spec.VCS.RepoOwner, spec.VCS.RepoName, *spec.Job.PullRequestNumber)
	if err != nil {
		message := fmt.Sprintf("could not get planStorage: %v", err)
		reportError(spec, backendApi, message, err)
	}

	// get variables from the variables spec
	variablesMap, err := variablesProvider.GetVariables(spec.Variables)
	if err != nil {
		msg := fmt.Sprintf("could not get variables from provider: %v", err)
		reportError(spec, backendApi, msg, err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get variables from provider: %v", err), 1)
	}
	job.StateEnvVars = lo.Assign(job.StateEnvVars, variablesMap)
	job.CommandEnvVars = lo.Assign(job.CommandEnvVars, variablesMap)
	job.RunEnvVars = lo.Assign(job.RunEnvVars, variablesMap)

	jobs := []scheduler.Job{job}

	fullRepoName := fmt.Sprintf("%v-%v", spec.VCS.RepoOwner, spec.VCS.RepoName)
	_, err = backendApi.ReportProjectJobStatus(fullRepoName, spec.Job.ProjectName, spec.JobId, "started", time.Now(), nil, "", "", "", nil)
	if err != nil {
		message := fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %v", err)
		reportError(spec, backendApi, message, err)
	}

	commentId := spec.CommentId
	if err != nil {
		message := fmt.Sprintf("failed to get comment ID: %v", err)
		reportError(spec, backendApi, message, err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		message := fmt.Sprintf("Failed to get current dir. %v", err)
		reportError(spec, backendApi, message, err)
	}

	reportTerraformOutput := spec.Reporter.ReportTerraformOutput
	allAppliesSuccess, _, err := digger.RunJobs(jobs, prService, orgService, lock, reporter, planStorage, policyChecker, commentUpdater, backendApi, spec.JobId, true, reportTerraformOutput, commentId, currentDir)
	if !allAppliesSuccess || err != nil {
		serializedBatch, reportingError := backendApi.ReportProjectJobStatus(spec.VCS.RepoName, spec.Job.ProjectName, spec.JobId, "failed", time.Now(), nil, "", "", "", nil)
		if reportingError != nil {
			message := fmt.Sprintf("Failed run commands. %v", err)
			reportError(spec, backendApi, message, err)
		}
		commentUpdater.UpdateComment(serializedBatch.Jobs, serializedBatch.PrNumber, prService, commentId)
		digger.UpdateAggregateStatus(serializedBatch, prService)
		reportError(spec, backendApi, fmt.Sprintf("failed to run commands %v", err), err)
	}
	usage.ReportErrorAndExit(spec.VCS.RepoOwner, "Digger finished successfully", 0)

	return nil
}
