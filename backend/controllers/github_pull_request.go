package controllers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"slices"
	"strconv"

	"github.com/diggerhq/digger/backend/ci_backends"
	config2 "github.com/diggerhq/digger/backend/config"
	locking2 "github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
)

func handlePullRequestEvent(gh utils.GithubClientProvider, payload *github.PullRequestEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64) error {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in handlePullRequestEvent", "error", r, slog.Group("stack"))
			fmt.Printf("Stack trace:\n%s\n", stack)
		}
	}()

	if os.Getenv("DIGGER_IGNORE_PULL_REQUEST_EVENTS") == "1" {
		slog.Debug("Ignoring pull request event as DIGGER_IGNORE_PULL_REQUEST_EVENTS is set")
		return nil
	}

	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	prNumber := *payload.PullRequest.Number
	isDraft := payload.PullRequest.GetDraft()
	commitSha := payload.PullRequest.Head.GetSHA()
	branch := payload.PullRequest.Head.GetRef()
	action := *payload.Action
	labels := payload.PullRequest.Labels
	var vcsActorId string = ""
	if payload.Sender != nil && payload.Sender.Email != nil {
		vcsActorId = *payload.Sender.Email
	} else if payload.Sender != nil && payload.Sender.Login != nil {
		vcsActorId = *payload.Sender.Login
	}
	prLabelsStr := lo.Map(labels, func(label *github.Label, i int) string {
		return *label.Name
	})

	slog.Info("Processing pull request event",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"prNumber", prNumber,
		"action", action,
		"branch", branch,
		"commitSha", commitSha,
		"isDraft", isDraft,
		"labels", prLabelsStr,
		"installationId", installationId,
	)

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
		)
		return fmt.Errorf("github app installation link not found")
	}
	organisationId := link.OrganisationId

	ghService, _, ghServiceErr := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if ghServiceErr != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", ghServiceErr,
		)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	if !slices.Contains([]string{"closed", "opened", "reopened", "synchronize", "converted_to_draft"}, action) {
		slog.Info("Ignoring event with action not requiring processing", "action", action, "prNumber", prNumber)
		return nil
	}

	commentReporterManager := utils.InitCommentReporterManager(ghService, prNumber)
	if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
		_, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting....")
		if err != nil {
			slog.Error("Error initializing comment reporter",
				"prNumber", prNumber,
				"error", err,
			)
			return fmt.Errorf("error initializing comment reporter")
		}
	}

	org, err := models.DB.GetOrganisationById(organisationId)
	if err != nil || org == nil {
		slog.Error("Error getting organisation",
			"orgId", organisationId,
			"error", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get organisation by ID"))
		return fmt.Errorf("error getting organisation")
	}

	diggerYmlStr, ghService, config, projectsGraph, _, _, changedFiles, err := getDiggerConfigForPR(gh, organisationId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneURL, prNumber)
	if err != nil {
		slog.Error("Error getting Digger config for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error loading digger config: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	slog.Info("Successfully loaded Digger config",
		"prNumber", prNumber,
		"repoFullName", repoFullName,
		"configLength", len(diggerYmlStr),
	)

	impactedProjects, impactedProjectsSourceMapping, _, err := github2.ProcessGitHubPullRequestEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		slog.Error("Error processing GitHub pull request event",
			"prNumber", prNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, coverAllImpactedProjects, err := github2.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config, false)
	if err != nil {
		slog.Error("Error converting event to jobs",
			"prNumber", prNumber,
			"impactedProjectCount", len(impactedProjects),
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
		return fmt.Errorf("error converting event to jobsForImpactedProjects")
	}

	if len(jobsForImpactedProjects) == 0 {
		// do not report if no projects are impacted to minimise noise in the PR thread
		// TODO use status checks instead: https://github.com/diggerhq/digger/issues/1135
		slog.Info("No projects impacted; not starting any jobs",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
		)
		if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
			// This one is for aggregate reporting
			commentReporterManager.UpdateComment(":construction_worker: No projects impacted")
		}
		err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
		return nil
	}

	// if flag set we dont allow more projects impacted than the number of changed files in PR (safety check)
	if config2.LimitByNumOfFilesChanged() {
		if len(impactedProjects) > len(changedFiles) {
			slog.Error("Number of impacted projects exceeds number of changed files",
				"prNumber", prNumber,
				"impactedProjectCount", len(impactedProjects),
				"changedFileCount", len(changedFiles),
			)

			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjects), len(changedFiles)))

			slog.Debug("Detailed event information",
				slog.Group("details",
					slog.Any("changedFiles", changedFiles),
					slog.Int("configLength", len(diggerYmlStr)),
					slog.Int("impactedProjectCount", len(impactedProjects)),
				),
			)

			return fmt.Errorf("error processing event")
		}
	}

	maxImpactedProjectsPerChange := config2.MaxImpactedProjectsPerChange()
	if len(impactedProjects) > maxImpactedProjectsPerChange {
		slog.Error("Number of impacted projects exceeds number of changed files",
			"prNumber", prNumber,
			"impactedProjectCount", len(impactedProjects),
			"changedFileCount", len(changedFiles),
		)

		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds Max allowed ImpactedProjectsPerChange: %v, we set this limit to protect against hitting github API limits", len(impactedProjects), maxImpactedProjectsPerChange))

		slog.Debug("Detailed event information",
			slog.Group("details",
				slog.Any("changedFiles", changedFiles),
				slog.Int("configLength", len(diggerYmlStr)),
				slog.Int("impactedProjectCount", len(impactedProjects)),
			),
		)
		return fmt.Errorf("error processing event")
	}

	diggerCommand, err := scheduler.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		slog.Error("Could not determine Digger command from job",
			"prNumber", prNumber,
			"commands", jobsForImpactedProjects[0].Commands,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	if *diggerCommand == scheduler.DiggerCommandNoop {
		slog.Info("Job is of type noop, no actions to perform",
			"prNumber", prNumber,
			"command", *diggerCommand,
		)
		return nil
	}

	// special case for when a draft pull request is opened and ignore PRs is set to true we DO NOT want to lock the projects
	if !config.AllowDraftPRs && isDraft && action == "opened" {
		slog.Info("Draft PRs are disabled, skipping PR",
			"prNumber", prNumber,
			"isDraft", isDraft,
		)
		if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
			// This one is for aggregate reporting
			commentReporterManager.UpdateComment(":construction_worker: Ignoring event as it is a draft and draft PRs are configured to be ignored")
		}
		return nil
	}

	// perform locking/unlocking in backend
	if config.PrLocks {
		slog.Info("Processing PR locks for impacted projects",
			"prNumber", prNumber,
			"projectCount", len(impactedProjects),
			"command", *diggerCommand,
		)

		for _, project := range impactedProjects {
			prLock := locking.PullRequestLock{
				InternalLock: locking2.BackendDBLock{
					OrgId: organisationId,
				},
				CIService:        ghService,
				Reporter:         reporting.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         prNumber,
			}

			err = locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				slog.Error("Failed to perform lock action on project",
					"prNumber", prNumber,
					"projectName", project.Name,
					"command", *diggerCommand,
					"error", err,
				)
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed to perform lock action on project: %v, %v", project.Name, err)
			}
		}
	}

	// remove any dangling locks which are no longer in the list of impacted projects
	if *diggerCommand == scheduler.DiggerCommandUnlock {
		err := models.DB.DeleteAllLocksAcquiredByPR(prNumber, repoFullName, organisationId)
		if err != nil {
			slog.Error("Failed to delete locks",
				"prNumber", prNumber,
				"command", *diggerCommand,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to delete locks: %v", err))
			return fmt.Errorf("failed to delete locks: %v", err)
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == scheduler.DiggerCommandUnlock ||
		*diggerCommand == scheduler.DiggerCommandLock {
		slog.Info("Lock/unlock command completed successfully",
			"prNumber", prNumber,
			"command", *diggerCommand,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	if !config.AllowDraftPRs && isDraft {
		slog.Info("Draft PRs are disabled, skipping PR",
			"prNumber", prNumber,
			"isDraft", isDraft,
		)
		if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
			// This one is for aggregate reporting
			commentReporterManager.UpdateComment(":construction_worker: Ignoring event as it is a draft and draft PRs are configured to be ignored")
		}
		return nil
	}

	// a pull request has led to jobs which would be triggered (ignoring closed event by here)
	segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request", map[string]string{})

	commentReporter, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting... Config loaded successfully")
	if err != nil {
		slog.Error("Error initializing comment reporter",
			"prNumber", prNumber,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		return fmt.Errorf("error initializing comment reporter")
	}

	err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
	if err != nil {
		slog.Error("Error setting status for PR",
			"prNumber", prNumber,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: error setting status for PR: %v", err))
		return fmt.Errorf("error setting status for PR: %v", err)
	}

	nLayers, _ := scheduler.CountUniqueLayers(jobsForImpactedProjects)
	slog.Debug("Number of layers",
		"prNumber", prNumber,
		"nLayers", nLayers,
		"respectLayers", config.RespectLayers,
	)
	if config.RespectLayers && nLayers > 1 {
		slog.Debug("Respecting layers",
			"prNumber", prNumber)
		err = utils.ReportLayersTableForJobs(commentReporter, jobsForImpactedProjects)
		if err != nil {
			slog.Error("Failed to comment initial status for jobs",
				"prNumber", prNumber,
				"jobCount", len(jobsForImpactedProjects),
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
			return fmt.Errorf("failed to comment initial status for jobs")
		}
		slog.Debug("not performing plan since there are multiple layers and respect_layers is enabled")
		return nil
	} else {
		err = utils.ReportInitialJobsStatus(commentReporter, jobsForImpactedProjects)
		if err != nil {
			slog.Error("Failed to comment initial status for jobs",
				"prNumber", prNumber,
				"jobCount", len(jobsForImpactedProjects),
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
			return fmt.Errorf("failed to comment initial status for jobs")
		}
	}

	slog.Debug("Preparing job and project maps",
		"prNumber", prNumber,
		"projectCount", len(impactedProjects),
		"jobCount", len(jobsForImpactedProjects),
	)

	impactedProjectsMap := make(map[string]digger_config.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]scheduler.Job)
	for _, j := range jobsForImpactedProjects {
		impactedJobsMap[j.ProjectName] = j
	}

	commentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		slog.Error("Error parsing comment ID",
			"commentId", commentReporter.CommentId,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not handle commentId: %v", err))
	}

	var aiSummaryCommentId = ""
	if config.Reporting.AiSummary {
		slog.Info("Creating AI summary comment", "prNumber", prNumber)
		aiSummaryComment, err := ghService.PublishComment(prNumber, "AI Summary will be posted here after completion")
		if err != nil {
			slog.Error("Could not post AI summary comment",
				"prNumber", prNumber,
				"error", err,
			)
			segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not post ai comment summary comment id: %v", err))
			return fmt.Errorf("could not post ai summary comment: %v", err)
		}
		aiSummaryCommentId = aiSummaryComment.Id
		slog.Debug("Created AI summary comment", "commentId", aiSummaryCommentId)
	}

	slog.Info("Converting jobs to Digger jobs",
		"prNumber", prNumber,
		"command", *diggerCommand,
		"jobCount", len(impactedJobsMap),
	)

	if config.RespectLayers {

	}
	batchId, _, err := utils.ConvertJobsToDiggerJobs(
		*diggerCommand,
		models.DiggerVCSGithub,
		organisationId,
		impactedJobsMap,
		impactedProjectsMap,
		projectsGraph,
		installationId,
		branch,
		prNumber,
		repoOwner,
		repoName,
		repoFullName,
		commitSha,
		commentId,
		diggerYmlStr,
		0,
		aiSummaryCommentId,
		config.ReportTerraformOutputs,
		coverAllImpactedProjects,
		nil,
	)
	if err != nil {
		slog.Error("Error converting jobs to Digger jobs",
			"prNumber", prNumber,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	slog.Info("Successfully created batch for jobs",
		"prNumber", prNumber,
		"batchId", batchId,
	)

	if config.CommentRenderMode == digger_config.CommentRenderModeGroupByModule {
		slog.Info("Using GroupByModule render mode for comments", "prNumber", prNumber)

		sourceDetails, err := reporting.PostInitialSourceComments(ghService, prNumber, impactedProjectsSourceMapping)
		if err != nil {
			slog.Error("Error posting initial source comments",
				"prNumber", prNumber,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}

		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			slog.Error("Error getting Digger batch",
				"batchId", batchId,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			slog.Error("Error marshalling source details",
				"batchId", batchId,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}

		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			slog.Error("Error updating Digger batch",
				"batchId", batchId,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}

		slog.Debug("Successfully updated batch with source details", "batchId", batchId)
	}

	segment.Track(*org, repoOwner, vcsActorId, "github", "backend_trigger_job", map[string]string{})

	slog.Info("Getting CI backend",
		"prNumber", prNumber,
		"repoFullName", repoFullName,
	)

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
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	slog.Info("Triggering Digger jobs",
		"prNumber", prNumber,
		"batchId", batchId,
		"repoFullName", repoFullName,
	)

	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, prNumber, ghService, gh)
	if err != nil {
		slog.Error("Error triggering Digger jobs",
			"prNumber", prNumber,
			"batchId", batchId,
			"error", err,
		)
		segment.Track(*org, repoOwner, vcsActorId, "github", "pull_request_ERROR", map[string]string{"error": err.Error()})
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	slog.Info("Successfully processed pull request event",
		"prNumber", prNumber,
		"batchId", batchId,
		"repoFullName", repoFullName,
	)

	return nil
}
