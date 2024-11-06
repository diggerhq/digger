package hooks

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	ce_controllers "github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var DriftReconcilliationHook ce_controllers.IssueCommentHook = func(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider) error {
	log.Printf("handling the drift reconcilliation hook")
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	issueTitle := *payload.Issue.Title
	issueNumber := *payload.Issue.Number
	userCommentId := *payload.GetComment().ID
	actor := *payload.Sender.Login
	commentBody := *payload.Comment.Body
	defaultBranch := *payload.Repo.DefaultBranch
	isPullRequest := payload.Issue.IsPullRequest()

	if isPullRequest {
		log.Printf("Comment is not an issue, ignoring")
		return nil
	}

	// checking that the title of the issue matches regex
	var projectName string
	re := regexp.MustCompile(`^Drift detected in project:\s*(\S+)`)
	matches := re.FindStringSubmatch(issueTitle)
	if len(matches) > 1 {
		projectName = matches[1]
	} else {
		log.Printf("does not look like a drift issue, ignoring")
	}

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	orgId := link.OrganisationId

	if *payload.Action != "created" {
		log.Printf("comment is not of type 'created', ignoring")
		return nil
	}

	allowedCommands := []string{"digger apply", "digger unlock"}
	if !lo.Contains(allowedCommands, strings.TrimSpace(*payload.Comment.Body)) {
		log.Printf("comment is not in allowed commands, ignoring")
		log.Printf("allowed commands: %v", allowedCommands)
		return nil
	}

	diggerYmlStr, ghService, config, projectsGraph, err := ce_controllers.GetDiggerConfigForBranch(gh, installationId, repoFullName, repoOwner, repoName, cloneURL, defaultBranch, nil)
	if err != nil {
		log.Printf("Error loading digger.yml: %v", err)
		return fmt.Errorf("error loading digger.yml")
	}

	commentIdStr := strconv.FormatInt(userCommentId, 10)
	err = ghService.CreateCommentReaction(commentIdStr, string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	diggerCommand, err := scheduler.GetCommandFromComment(*payload.Comment.Body)
	if err != nil {
		log.Printf("unknown digger command in comment: %v", *payload.Comment.Body)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	// attempting to lock for performing drift apply command
	prLock := dg_locking.PullRequestLock{
		InternalLock: locking.BackendDBLock{
			OrgId: orgId,
		},
		CIService:        ghService,
		Reporter:         comment_updater.NoopReporter{},
		ProjectName:      projectName,
		ProjectNamespace: repoFullName,
		PrNumber:         issueNumber,
	}
	err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
	if err != nil {
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", projectName, err))
		return fmt.Errorf("failed perform lock action on project: %v %v", projectName, err)
	}

	if *diggerCommand == scheduler.DiggerCommandUnlock {
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	// === if we get here its a "digger apply command and we are already locked for this project ====
	// perform apply here then unlock the project
	commentReporter, err := utils.InitCommentReporter(ghService, issueNumber, ":construction_worker: Digger starting....")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	impactedProjects := config.GetProjects(projectName)
	jobs, _, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjects, nil, config.Workflows, defaultBranch, defaultBranch)
	if err != nil {
		log.Printf("Error converting event to jobs: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}
	log.Printf("GitHub IssueComment event converted to Jobs successfully\n")

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	impactedProjectsMap := make(map[string]digger_config.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]scheduler.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	reporterCommentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		log.Printf("strconv.ParseInt error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: could not handle commentId: %v", err))
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, "github", orgId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, installationId, defaultBranch, issueNumber, repoOwner, repoName, repoFullName, "", reporterCommentId, diggerYmlStr, 0)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	ciBackend, err := ciBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: gh,
			GithubInstallationId: installationId,
			RepoName:             repoName,
			RepoOwner:            repoOwner,
			RepoFullName:         repoFullName,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	err = ce_controllers.TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, ghService, gh)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	// === now unlocking the project ===
	err = dg_locking.PerformLockingActionFromCommand(prLock, scheduler.DiggerCommandUnlock)
	if err != nil {
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", projectName, err))
		return fmt.Errorf("failed perform lock action on project: %v %v", projectName, err)
	}

	return nil
}
