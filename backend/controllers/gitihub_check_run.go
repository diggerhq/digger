package controllers

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
)

func handleCheckRunActionEvent(gh utils.GithubClientProvider, identifier string, payload *github.CheckRunEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64) error {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in handlePullRequestEvent", "error", r, slog.Group("stack"))
			fmt.Printf("Stack trace:\n%s\n", stack)
		}
	}()

	repoFullName := *payload.Repo.FullName
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	cloneUrl := *payload.Repo.CloneURL
	actor := *payload.Sender.Login

	var checkRunBatch *models.DiggerBatch
	var checkedRunDiggerJobs []models.DiggerJob

	batchCheckApplyAllPrefix := string(utils.CheckedRunActionBatchApply)+":"
	if strings.HasPrefix(identifier, batchCheckApplyAllPrefix) {
		diggerBatchId := strings.ReplaceAll(identifier, batchCheckApplyAllPrefix, "")
		var err error
		checkRunBatch, err = models.DB.GetDiggerBatchFromId(diggerBatchId)
		if err != nil {
			slog.Error("Failed to find batch", "identifier", identifier, "error", err)
			return fmt.Errorf("Failed to find batch from identifier %v, err: %v", identifier, err)
		}
		checkedRunDiggerJobs, err = models.DB.GetDiggerJobsForBatch(checkRunBatch.ID)
		if err != nil {
			slog.Error("Failed to find jobs for batch", "batchId", checkRunBatch.ID, "error", err)
			return fmt.Errorf("Failed to find batch from identifier %v, err: %v", identifier, err)
		}
	}


	installationId := checkRunBatch.GithubInstallationId
	prNumber := checkRunBatch.PrNumber
	commitSha := checkRunBatch.CommitSha

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		slog.Error("Error getting GitHub app installation link",
			"installationId", installationId,
			"error", err,
		)
		return fmt.Errorf("error getting github app link")
	}
	if link == nil {
		slog.Error("GitHub app installation link not found",
			"installationId", installationId,
			"prNumber", prNumber,
		)
		return fmt.Errorf("GitHub App installation not found for installation ID %d. Please ensure the GitHub App is properly installed on the repository and the installation process completed successfully", installationId)
	}
	orgId := link.OrganisationId


	ghService, _, ghServiceErr := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if ghServiceErr != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"issueNumber", prNumber,
			"error", ghServiceErr,
		)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	prBranchName, _, _, _, err := ghService.GetBranchName(prNumber)


	diggerYmlStr, ghService, config, projectsGraph, err := GetDiggerConfigForBranchOrSha(gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, prBranchName, commitSha, nil, nil)
	if err != nil {
		slog.Error("Error getting Digger config for PR",
			"issueNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return fmt.Errorf("error getting digger config")
	}

	selectedProjects := lo.Filter(config.Projects, func(diggerYmlProject digger_config.Project, index int) bool {
		return lo.ContainsBy(checkedRunDiggerJobs, func(diggerJob models.DiggerJob) bool {
			return diggerJob.ProjectName == diggerYmlProject.Name
		})
	})

	jobs, err := generic.CreateJobsForProjects(selectedProjects, "digger apply", "check_run_action", repoFullName, actor, config.Workflows, &prNumber, &commitSha, "", checkRunBatch.BranchName, false)


	// just use noop since if someone clicks a button he shouldn't see comments on the PR (confusing)
	reporterType := "noop"


	impactedProjectsMap := make(map[string]digger_config.Project)
	for _, p := range selectedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]scheduler.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	batchCheckRunData, jobCheckRunDataMap, err := utils.SetPRCheckForJobs(ghService, prNumber, jobs, commitSha, repoName, repoOwner)
	if err != nil {
		slog.Error("Error setting status for PR",
			"prNumber", prNumber,
			"error", err,
		)
		return fmt.Errorf("error setting status for PR: %v", err)
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(
		scheduler.DiggerCommandApply,
		reporterType,
		"github",
		orgId,
		impactedProjectsJobMap,
		impactedProjectsMap,
		projectsGraph,
		installationId,
		prBranchName,
		prNumber,
		repoOwner,
		repoName,
		repoFullName,
		commitSha,
		nil,
		diggerYmlStr,
		0,
		"",
		config.ReportTerraformOutputs,
		false,
		nil,
		batchCheckRunData,
		jobCheckRunDataMap,
	)
	if err != nil {
		slog.Error("Error converting jobs to Digger jobs",
			"issueNumber", prNumber,
			"error", err,
		)
		return fmt.Errorf("error converting jobs")
	}

	ciBackend, err := ciBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: gh,
			GithubInstallationId: installationId,
			GithubAppId:          appId,
			RepoName:             repoName,
			RepoOwner:            repoOwner,
			RepoFullName:         repoFullName,
		},
	)
	if err != nil {
		slog.Error("Error getting CI backend",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return fmt.Errorf("error fetching ci backed %v", err)
	}


	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, prNumber, ghService, gh)
	if err != nil {
		slog.Error("Error triggering Digger jobs",
			"prNumber", prNumber,
			"batchId", batchId,
			"error", err,
		)
		return fmt.Errorf("error triggering Digger Jobs")
	}

	slog.Info("Successfully processed issue comment event",
		"prNumber", prNumber,
		"batchId", batchId,
		"repoFullName", repoFullName,
	)
	return nil
}