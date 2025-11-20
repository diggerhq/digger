package controllers

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/google/go-github/v61/github"
)

func handleCheckRunActionEvent(gh utils.GithubClientProvider, identifier string, payload *github.CheckRunEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64) error {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in handlePullRequestEvent", "error", r, slog.Group("stack"))
			fmt.Printf("Stack trace:\n%s\n", stack)
		}
	}()

	//repoFullName := *payload.Repo.FullName
	//repoName := *payload.Repo.Name
	//repoOwner := *payload.Repo.Owner.Login
	//cloneUrl := *payload.Repo.CloneURL
	//
	//checkRunBatch, err := models.DB.GetDiggerBatchFromId(identifier)
	//if err != nil {
	//	slog.Error("Failed to find batch from identifier %v, err: %v", identifier, err)
	//	return fmt.Errorf("Failed to find batch from identifier %v, err: %v", identifier, err)
	//}
	//
	//installationId := checkRunBatch.GithubInstallationId
	//prNumber := checkRunBatch.PrNumber
	//
	//link, err := models.DB.GetGithubAppInstallationLink(installationId)
	//if err != nil {
	//	slog.Error("Error getting GitHub app installation link",
	//		"installationId", installationId,
	//		"error", err,
	//	)
	//	return fmt.Errorf("error getting github app link")
	//}
	//if link == nil {
	//	slog.Error("GitHub app installation link not found",
	//		"installationId", installationId,
	//		"prNumber", prNumber,
	//	)
	//	return fmt.Errorf("GitHub App installation not found for installation ID %d. Please ensure the GitHub App is properly installed on the repository and the installation process completed successfully", installationId)
	//}
	//orgId := link.OrganisationId
	//prLabelsStr := make([]string, 0)


	//diggerYmlStr, ghService, config, projectsGraph, prSourceBranch, commitSha, _, err := getDiggerConfigForPR(gh, orgId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneUrl, prNumber)
	//if err != nil {
	//	slog.Error("Error getting Digger config for PR",
	//		"issueNumber", prNumber,
	//		"repoFullName", repoFullName,
	//		"error", err,
	//	)
	//	return fmt.Errorf("error getting digger config")
	//}

	//batchId, _, err := utils.ConvertJobsToDiggerJobs(
	//	scheduler.DiggerCommandApply,
	//	"github",
	//	orgId,
	//	impactedProjectsJobMap,
	//	impactedProjectsMap,
	//	projectsGraph,
	//	installationId,
	//	*prSourceBranch,
	//	prNumber,
	//	repoOwner,
	//	repoName,
	//	repoFullName,
	//	*commitSha,
	//	reporterCommentId,
	//	diggerYmlStr,
	//	0,
	//	"",
	//	config.ReportTerraformOutputs,
	//	coverAllImpactedProjects,
	//	nil,
	//	batchCheckRunData,
	//	jobCheckRunDataMap,
	//)
	//
	//if err != nil {
	//	slog.Error("Error converting jobs to Digger jobs",
	//		"issueNumber", prNumber,
	//		"error", err,
	//	)
	//	commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
	//	return fmt.Errorf("error converting jobs")
	//}

	return nil
}