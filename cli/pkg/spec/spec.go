package spec

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/digger"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/cli/pkg/utils"
	"github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/diggerhq/digger/libs/storage"
	"github.com/samber/lo"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func RunSpec(
	spec spec.Spec,
	vcsProvider spec.VCSProvider,
	jobProvider spec.JobSpecProvider,
	lockProvider spec.LockProvider,
	reporterProvider spec.ReporterProvider,
	backedProvider spec.BackendApiProvider,
	policyProvider spec.SpecPolicyProvider,
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

	orgService, err := vcsProvider.GetOrgService(spec.VCS)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get orgservice: %v", err), 1)
	}
	reporter, err := reporterProvider.GetReporter(fmt.Sprintf("%v for %v", spec.Job.JobType, job.ProjectName), spec.Reporter, prService, *spec.Job.PullRequestNumber)
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
	diggerConfig, _, _, err := digger_config.LoadDiggerConfig("./", false, changedFiles)
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

	workflow := diggerConfig.Workflows[job.ProjectWorkflow]
	stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars)
	job.StateEnvVars = lo.Assign(job.StateEnvVars, stateEnvVars)
	job.CommandEnvVars = lo.Assign(job.CommandEnvVars, commandEnvVars)

	jobs := []scheduler.Job{job}

	fullRepoName := fmt.Sprintf("%v-%v", spec.VCS.RepoOwner, spec.VCS.RepoName)
	_, err = backendApi.ReportProjectJobStatus(fullRepoName, spec.Job.ProjectName, spec.JobId, "started", time.Now(), nil, "", "")
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %v", err), 4)
	}

	commentId := spec.CommentId
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("failed to get comment ID: %v", err), 4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
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

func RunSpecManualCommand(
	spec spec.Spec,
	vcsProvider spec.VCSProvider,
	jobProvider spec.JobSpecProvider,
	lockProvider spec.LockProvider,
	reporterProvider spec.ReporterProvider,
	backedProvider spec.BackendApiProvider,
	policyProvider spec.SpecPolicyProvider,
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

	//prService, err := vcsProvider.GetPrService(spec.VCS)
	//if err != nil {
	//	usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get prservice: %v", err), 1)
	//}

	var prService ci.PullRequestService = ci.MockPullRequestManager{}
	//orgService, err := vcsProvider.GetOrgService(spec.VCS)
	//if err != nil {
	//	usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get orgservice: %v", err), 1)
	//}
	var orgService ci.OrgService = ci.MockPullRequestManager{}

	//reporter, err := reporterProvider.GetReporter(spec.Reporter, prService, *spec.Job.PullRequestNumber)
	//if err != nil {
	//	usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get reporter: %v", err), 1)
	//}
	reporter := reporting.StdOutReporter{}

	backendApi, err := backedProvider.GetBackendApi(spec.Backend)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get backend api: %v", err), 1)
	}

	// download zip artefact, git init and prepare for job execution
	tempDir, err := os.MkdirTemp("", "downloaded-zip-")
	if err != nil {
		log.Printf("failed to create temp dir: %w", err)
		os.Exit(1)
	}

	// downloading artefact zip , extracting, git init and then chdir to that directory for job execution
	absoluteFileName, err := backendApi.DownloadJobArtefact(tempDir)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not download job artefact: %v", err), 1)
	}
	zipPath := *absoluteFileName
	utils.ExtractZip(zipPath, tempDir)
	// Transforming: /var/temp/xxx/yyy/blabla.zip -> /var/temp/xxx/yyy/blabla
	gitLocation := strings.TrimSuffix(zipPath, ".zip")
	os.Chdir(gitLocation)
	cmd := exec.Command("git", "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	policyChecker, err := policyProvider.GetPolicyProvider(spec.Policy, spec.Backend.BackendHostname, spec.Backend.BackendOrganisationName, spec.Backend.BackendJobToken)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get policy provider: %v", err), 1)
	}

	//planStorage, err := PlanStorageProvider.GetPlanStorage(spec.VCS.RepoOwner, spec.VCS.RepoName, *spec.Job.PullRequestNumber)
	//if err != nil {
	//	usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not get plan storage: %v", err), 8)
	//}
	planStorage := storage.MockPlanStorage{}

	jobs := []scheduler.Job{job}

	//fullRepoName := fmt.Sprintf("%v-%v", spec.VCS.RepoOwner, spec.VCS.RepoName)
	//_, err = backendApi.ReportProjectJobStatus(fullRepoName, spec.Job.ProjectName, spec.JobId, "started", time.Now(), nil, "", "")
	//if err != nil {
	//	usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to report jobSpec status to backend. Exiting. %v", err), 4)
	//}

	noopBackendApi := backendapi.NoopApi{}

	commentId := spec.CommentId
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("failed to get comment ID: %v", err), 4)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Failed to get current dir. %s", err), 4)
	}

	commentUpdater := comment_summary.NoopCommentUpdater{}
	// do not change these placeholders as they are parsed by dgctl to stream logs
	log.Printf("<========= DIGGER RUNNING IN MANUAL MODE =========>")
	allAppliesSuccess, _, err := digger.RunJobs(jobs, prService, orgService, lock, reporter, planStorage, policyChecker, commentUpdater, noopBackendApi, spec.JobId, false, false, commentId, currentDir)
	log.Printf("<========= DIGGER COMPLETED =========>")
	if err != nil || allAppliesSuccess == false {
		usage.ReportErrorAndExit(spec.VCS.RepoOwner, "Terraform execution failed", 1)
	}

	usage.ReportErrorAndExit(spec.VCS.RepoOwner, "Digger finished successfully", 0)

	return nil
}
