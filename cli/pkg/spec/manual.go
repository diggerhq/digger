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
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/diggerhq/digger/libs/storage"
	"log"
	"os"
	"os/exec"
)

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
		log.Printf("failed to create temp dir: %v", err)
		os.Exit(1)
	}

	// downloading artefact zip , extracting, git init and then chdir to that directory for job execution
	absoluteFileName, err := backendApi.DownloadJobArtefact(tempDir)
	if err != nil {
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("could not download job artefact: %v", err), 1)
	}
	zipPath := *absoluteFileName
	err = utils.ExtractZip(zipPath, tempDir)
	if err != nil {
		log.Printf("ExtractZip err: %v", err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("artefact zip extraction err: %v", err), 1)

	}

	// remove the zipPath
	err = os.Remove(zipPath)
	if err != nil {
		log.Printf("os.Remove err: %v", err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("zip path removal err: %v", err), 1)
	}

	// Navigating to our diractory
	err = os.Chdir(tempDir)
	if err != nil {
		log.Printf("Chdir err: %v", err)
		usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("Chdir err: %v", err), 1)
	}

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
