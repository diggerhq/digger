package controllers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/ci_backends"
	config2 "github.com/diggerhq/digger/backend/config"
	locking2 "github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/apply_requirements"
	"github.com/diggerhq/digger/libs/ci/generic"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
)

func handleIssueCommentEvent(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64, postCommentHooks []IssueCommentHook) error {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in handleIssueCommentEvent", "error", r, slog.Group("stack"))
			fmt.Printf("Stack trace:\n%s\n", stack)
		}
	}()

	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	issueNumber := *payload.Issue.Number

	if payload.Installation == nil {
		slog.Error("Installation is nil in payload", "issueNumber", issueNumber)
		return fmt.Errorf("installation is missing from payload")
	}

	installationId = *payload.Installation.ID
	repoName = *payload.Repo.Name
	repoOwner = *payload.Repo.Owner.Login
	repoFullName = *payload.Repo.FullName
	cloneURL = *payload.Repo.CloneURL
	issueNumber = *payload.Issue.Number
	isDraft := payload.Issue.GetDraft()
	userCommentId := *payload.GetComment().ID
	actor := *payload.Sender.Login
	var vcsActorID string = ""
	if payload.Sender != nil && payload.Sender.Email != nil {
		vcsActorID = *payload.Sender.Email
	} else if payload.Sender != nil && payload.Sender.Login != nil {
		vcsActorID = *payload.Sender.Login
	}
	commentBody := *payload.Comment.Body
	defaultBranch := *payload.Repo.DefaultBranch
	isPullRequest := payload.Issue.IsPullRequest()
	labels := payload.Issue.Labels
	prLabelsStr := lo.Map(labels, func(label *github.Label, i int) string {
		return *label.Name
	})

	slog.Info("Processing issue comment event",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"issueNumber", issueNumber,
		"actor", actor,
		"isPullRequest", isPullRequest,
		"commentId", userCommentId,
		"installationId", installationId,
		"action", *payload.Action,
	)

	if !isPullRequest {
		slog.Info("Comment not on pull request, ignoring", "issueNumber", issueNumber)
		return nil
	}

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
			"issueNumber", issueNumber,
		)
		return fmt.Errorf("GitHub App installation not found for installation ID %d. Please ensure the GitHub App is properly installed on the repository and the installation process completed successfully", installationId)
	}
	orgId := link.OrganisationId

	if *payload.Action != "created" {
		slog.Info("Comment action is not 'created', ignoring",
			"action", *payload.Action,
			"issueNumber", issueNumber,
		)
		return nil
	}

	cleanedComment := strings.TrimSpace(strings.ToLower(commentBody))
	if !strings.HasPrefix(cleanedComment, "digger") {
		slog.Info("Comment is not a Digger command, ignoring",
			"issueNumber", issueNumber,
			"commentBody", commentBody,
		)
		return nil
	}

	ghService, _, ghServiceErr := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if ghServiceErr != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"issueNumber", issueNumber,
			"error", ghServiceErr,
		)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	commentReporterManager := utils.InitCommentReporterManager(ghService, issueNumber)
	if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
		_, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting....")
		if err != nil {
			slog.Error("Error initializing comment reporter",
				"issueNumber", issueNumber,
				"error", err,
			)
			return fmt.Errorf("error initializing comment reporter")
		}
	}

	slog.Info("Loading Digger config for PR",
		"issueNumber", issueNumber,
		"repoFullName", repoFullName,
		"orgId", orgId,
	)

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil || org == nil {
		slog.Error("Error getting organisation",
			"orgId", orgId,
			"error", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get organisation by ID"))
		return fmt.Errorf("error getting organisation")
	}

	diggerYmlStr, ghService, config, projectsGraph, prSourceBranch, commitSha, changedFiles, err := getDiggerConfigForPR(gh, orgId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneURL, issueNumber)
	if err != nil {		
		slog.Error("Error getting Digger config for PR",
			"issueNumber", issueNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	if config.DisableDiggerApplyComment && strings.HasPrefix(cleanedComment, "digger apply") {
		slog.Info("Digger configured to disable apply comment in PRs, ignoring comment", "DisableDiggerApplyComment", config.DisableDiggerApplyComment)
		if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
			commentReporterManager.UpdateComment("Digger configured to disable apply comment in PRs, ignoring comment")
		}
		return nil
	}

	// terraform code generator
	if os.Getenv("DIGGER_GENERATION_ENABLED") == "1" {
		slog.Info("Terraform code generation is enabled",
			"issueNumber", issueNumber,
			"repoFullName", repoFullName,
		)

		err = GenerateTerraformFromCode(payload, commentReporterManager, config, defaultBranch, ghService, repoOwner, repoName, commitSha, issueNumber, prSourceBranch)
		if err != nil {
			slog.Error("Terraform generation failed",
				"issueNumber", issueNumber,
				"repoFullName", repoFullName,
				"error", err,
			)
			return err
		}
	}

	commentIdStr := strconv.FormatInt(userCommentId, 10)
	err = ghService.CreateCommentReaction(commentIdStr, string(github2.GithubCommentEyesReaction))
	if err != nil {
		slog.Warn("Failed to create comment reaction",
			"commentId", commentIdStr,
			"error", err,
		)
	} else {
		slog.Debug("Added eyes reaction to comment", "commentId", commentIdStr)
	}

	commentReporter, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting.... config loaded successfully")
	if err != nil {
		slog.Error("Error initializing comment reporter",
			"issueNumber", issueNumber,
			"error", err,
		)
		return fmt.Errorf("error initializing comment reporter")
	}

	slog.Debug("Parsing Digger command from comment",
		"issueNumber", issueNumber,
		"comment", commentBody,
	)

	diggerCommand, err := scheduler.GetCommandFromComment(commentBody)
	if err != nil {
		slog.Error("Unknown Digger command in comment",
			"issueNumber", issueNumber,
			"comment", commentBody,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	slog.Info("Detected Digger command",
		"issueNumber", issueNumber,
		"command", *diggerCommand,
	)

	prBranchName, _, targetBranch, _, err := ghService.GetBranchName(issueNumber)
	if err != nil {
		slog.Error("Error getting branch name",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	slog.Debug("Retrieved PR branch name",
		"issueNumber", issueNumber,
		"branchName", prBranchName,
	)

	processEventResult, err := generic.ProcessIssueCommentEvent(issueNumber, config, projectsGraph, ghService)
	if err != nil {
		slog.Error("Error processing issue comment event",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}
	impactedProjectsSourceMapping := processEventResult.ImpactedProjectsSourceMapping
	allImpactedProjects := processEventResult.AllImpactedProjects

	// Persist detection run (append-only) for issue comment events using full impacted set
	var csha string
	if commitSha != nil {
		csha = *commitSha
	}
	recordDetectionRun(
		orgId,
		repoFullName,
		issueNumber,
		"issue_comment",
		"comment",
		csha,
		defaultBranch,
		targetBranch,
		prLabelsStr,
		changedFiles,
		allImpactedProjects,
		impactedProjectsSourceMapping,
	)

	impactedProjectsForComment, err := generic.FilterOutProjectsFromComment(allImpactedProjects, commentBody)
	if err != nil {
		slog.Error("Error filtering out projects from comment",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error filtering out projects from comment: %v", err))
		return fmt.Errorf("error filtering out projects from comment")
	}

	impactedProjectsForComment = generic.FilterTargetBranchForImpactedProjects(impactedProjectsForComment, defaultBranch, targetBranch)

	slog.Info("Issue comment event processed successfully",
		"issueNumber", issueNumber,
		"impactedProjectCount", len(impactedProjectsForComment),
		"allImpactedProjectsCount", len(allImpactedProjects),
	)

	jobs, coverAllImpactedProjects, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjectsForComment, allImpactedProjects, config.Workflows, prBranchName, defaultBranch, false)
	if err != nil {
		slog.Error("Error converting event to jobs",
			"issueNumber", issueNumber,
			"impactedProjectCount", len(impactedProjectsForComment),
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}

	slog.Info("Issue comment event converted to jobs successfully",
		"issueNumber", issueNumber,
		"jobCount", len(jobs),
	)


	// impacted projects should have already been populated in the database by here since the PR open would have
	// populated them, but just in case (disabled pr events or a long old pr before this change was deployed)
	// we will populate them if they did not exist
	dbImpactedProjects, err := models.DB.GetImpactedProjects(repoFullName, *commitSha)
	if len(dbImpactedProjects) == 0 {
		for _, impactedProject := range allImpactedProjects {
			_, err = models.DB.CreateImpactedProject(repoFullName, *commitSha, impactedProject.Name, &prBranchName, &issueNumber)
			if err != nil {
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error failed to update internal record of impacted projects %v", err))
				return err
			}
		}
	}

	// if flag set we dont allow more projects impacted than the number of changed files in PR (safety check)
	if config2.LimitByNumOfFilesChanged() {
		if len(impactedProjectsForComment) > len(changedFiles) {
			slog.Error("Number of impacted projects exceeds number of changed files",
				"issueNumber", issueNumber,
				"impactedProjectCount", len(impactedProjectsForComment),
				"changedFileCount", len(changedFiles),
			)

			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjectsForComment), len(changedFiles)))

			slog.Debug("Detailed event information",
				slog.Group("details",
					slog.Any("changedFiles", changedFiles),
					slog.Int("configLength", len(diggerYmlStr)),
					slog.Int("impactedProjectCount", len(impactedProjectsForComment)),
				),
			)

			return fmt.Errorf("error processing event")
		}
	}

	maxImpactedProjectsPerChange := config2.MaxImpactedProjectsPerChange()
	if len(impactedProjectsForComment) > maxImpactedProjectsPerChange {
		slog.Error("Number of impacted projects exceeds number of changed files",
			"prNumber", issueNumber,
			"impactedProjectCount", len(impactedProjectsForComment),
			"changedFileCount", len(changedFiles),
		)

		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds Max allowed ImpactedProjectsPerChange: %v, we set this limit to protect against hitting github API limits", len(impactedProjectsForComment), maxImpactedProjectsPerChange))

		slog.Debug("Detailed event information",
			slog.Group("details",
				slog.Any("changedFiles", changedFiles),
				slog.Int("configLength", len(diggerYmlStr)),
				slog.Int("impactedProjectCount", len(impactedProjectsForComment)),
			),
		)
		return fmt.Errorf("error processing event")
	}

	if !config.AllowDraftPRs && isDraft {
		slog.Info("Draft PRs are disabled, skipping",
			"issueNumber", issueNumber,
			"isDraft", isDraft,
		)

		if os.Getenv("DIGGER_REPORT_BEFORE_LOADING_CONFIG") == "1" {
			// This one is for aggregate reporting
			commentReporterManager.UpdateComment(":construction_worker: Ignoring event as it is a draft and draft PRs are configured to be ignored")
		}

		// special case to unlock all locks aquired by this PR
		if *diggerCommand == scheduler.DiggerCommandUnlock {
			err := models.DB.DeleteAllLocksAcquiredByPR(issueNumber, repoFullName, orgId)
			if err != nil {
				slog.Error("Failed to delete locks",
					"prNumber", issueNumber,
					"command", *diggerCommand,
					"error", err,
				)
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to delete locks: %v", err))
				return fmt.Errorf("failed to delete locks: %v", err)
			}
			commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		}

		return nil
	}

	// perform unlocking in backend
	if config.PrLocks {
		slog.Info("Processing PR locks for impacted projects",
			"issueNumber", issueNumber,
			"projectCount", len(impactedProjectsForComment),
			"command", *diggerCommand,
		)

		for _, project := range impactedProjectsForComment {
			prLock := locking.PullRequestLock{
				InternalLock: locking2.BackendDBLock{
					OrgId: orgId,
				},
				CIService:        ghService,
				Reporter:         reporting.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         issueNumber,
			}

			err = locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				slog.Error("Failed to perform lock action on project",
					"issueNumber", issueNumber,
					"projectName", project.Name,
					"command", *diggerCommand,
					"error", err,
				)
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// remove any dangling locks which are no longer in the list of impacted projects
	if *diggerCommand == scheduler.DiggerCommandUnlock {
		err := models.DB.DeleteAllLocksAcquiredByPR(issueNumber, repoFullName, orgId)
		if err != nil {
			slog.Error("Failed to delete locks",
				"prNumber", issueNumber,
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
			"issueNumber", issueNumber,
			"command", *diggerCommand,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	// Check for apply requirements
	if *diggerCommand == scheduler.DiggerCommandApply {
		err = apply_requirements.CheckApplyRequirements(ghService, impactedProjectsForComment, jobs, issueNumber, *prSourceBranch, targetBranch)
		if err != nil {
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not proceed with apply since apply requirements checks have failed: %v", err))
			return nil
		}
	}

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		slog.Error("Failed to comment initial status for jobs",
			"issueNumber", issueNumber,
			"jobCount", len(jobs),
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	if len(jobs) == 0 {
		slog.Info("No projects impacted, succeeding",
			"issueNumber", issueNumber,
			"command", *diggerCommand,
		)
		// This one is for aggregate reporting
		//err = utils.SetPRCommitStatusForJobs(ghService, issueNumber, jobs)
		_, _, err = utils.SetPRCheckForJobs(ghService, issueNumber, jobs, *commitSha)
		return nil
	}

	// If we reach here then we have created a comment that would have led to more events
	segment.Track(*org, repoOwner, vcsActorID, "github", "issue_digger_comment", map[string]string{"comment": commentBody})

	//err = utils.SetPRCommitStatusForJobs(ghService, issueNumber, jobs)
	batchCheckRunData, jobCheckRunDataMap, err := utils.SetPRCheckForJobs(ghService, issueNumber, jobs, *commitSha)
	if err != nil {
		slog.Error("Error setting status for PR",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: error setting status for PR: %v", err))
		return fmt.Errorf("error setting status for PR: %v", err)
	}

	slog.Debug("Preparing job and project maps",
		"issueNumber", issueNumber,
		"projectCount", len(impactedProjectsForComment),
		"jobCount", len(jobs),
	)

	impactedProjectsMap := make(map[string]digger_config.Project)
	for _, p := range impactedProjectsForComment {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]scheduler.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	reporterCommentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		slog.Error("Error parsing comment ID",
			"commentId", commentReporter.CommentId,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not handle commentId: %v", err))
		return fmt.Errorf("comment reporter error: %v", err)
	}

	var aiSummaryCommentId = ""
	if config.Reporting.AiSummary {
		slog.Info("Creating AI summary comment", "issueNumber", issueNumber)
		aiSummaryComment, err := ghService.PublishComment(issueNumber, "AI Summary will be posted here after completion")
		if err != nil {
			slog.Error("Could not post AI summary comment",
				"issueNumber", issueNumber,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not post ai comment summary comment id: %v", err))
			return fmt.Errorf("could not post ai summary comment: %v", err)
		}
		aiSummaryCommentId = aiSummaryComment.Id
		slog.Debug("Created AI summary comment", "commentId", aiSummaryCommentId)
	}


	reporterType := "lazy"
	if config.Reporting.CommentsEnabled == false {
		reporterType = "noop"
	}

	slog.Info("Converting jobs to Digger jobs",
		"issueNumber", issueNumber,
		"command", *diggerCommand,
		"jobCount", len(impactedProjectsJobMap),
	)

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, reporterType, "github", orgId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, installationId, *prSourceBranch, issueNumber, repoOwner, repoName, repoFullName, *commitSha, reporterCommentId, diggerYmlStr, 0, aiSummaryCommentId, config.ReportTerraformOutputs, coverAllImpactedProjects, nil, batchCheckRunData, jobCheckRunDataMap)
	if err != nil {
		slog.Error("Error converting jobs to Digger jobs",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	slog.Info("Successfully created batch for jobs",
		"issueNumber", issueNumber,
		"batchId", batchId,
	)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		slog.Error("Error getting Digger batch",
			"batchId", batchId,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not retrieve created batch: %v", err))
		return fmt.Errorf("error getting digger batch")
	}

	if config.CommentRenderMode == digger_config.CommentRenderModeGroupByModule &&
		(*diggerCommand == scheduler.DiggerCommandPlan || *diggerCommand == scheduler.DiggerCommandApply) {

		slog.Info("Using GroupByModule render mode for comments", "issueNumber", issueNumber)

		sourceDetails, err := reporting.PostInitialSourceComments(ghService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			slog.Error("Error posting initial source comments",
				"issueNumber", issueNumber,
				"error", err,
			)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
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

	slog.Info("Getting CI backend",
		"issueNumber", issueNumber,
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
			"issueNumber", issueNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	segment.Track(*org, repoOwner, vcsActorID, "github", "backend_trigger_job", map[string]string{
		"comment": commentBody,
	})

	slog.Info("Triggering Digger jobs",
		"issueNumber", issueNumber,
		"batchId", batchId,
		"repoFullName", repoFullName,
	)

	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, ghService, gh)
	if err != nil {
		slog.Error("Error triggering Digger jobs",
			"issueNumber", issueNumber,
			"batchId", batchId,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	if len(postCommentHooks) > 0 {
		slog.Info("Executing issue comment event post hooks",
			"hookCount", len(postCommentHooks),
			"issueNumber", issueNumber,
		)

		for i, hook := range postCommentHooks {
			slog.Debug("Executing post comment hook", "hookIndex", i, "issueNumber", issueNumber)
			err := hook(gh, payload, ciBackendProvider)
			if err != nil {
				slog.Error("Error in post comment hook",
					"hookIndex", i,
					"issueNumber", issueNumber,
					"error", err,
				)
				return fmt.Errorf("error during postevent hooks: %v", err)
			}
		}

		slog.Debug("Successfully executed all post comment hooks", "issueNumber", issueNumber)
	}

	slog.Info("Successfully processed issue comment event",
		"issueNumber", issueNumber,
		"batchId", batchId,
		"repoFullName", repoFullName,
	)

	return nil
}
