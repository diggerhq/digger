package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config/terragrunt/tac"
	"github.com/diggerhq/digger/libs/git_utils"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/diggerhq/digger/backend/ci_backends"
	config2 "github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type IssueCommentHook func(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider) error

type DiggerController struct {
	CiBackendProvider                  ci_backends.CiBackendProvider
	GithubClientProvider               utils.GithubClientProvider
	GithubWebhookPostIssueCommentHooks []IssueCommentHook
}

func (d DiggerController) GithubAppWebHook(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	gh := d.GithubClientProvider
	slog.Info("Processing GitHub app webhook")

	appID := c.GetHeader("X-GitHub-Hook-Installation-Target-ID")

	_, _, webhookSecret, _, err := d.GithubClientProvider.FetchCredentials(appID)

	payload, err := github.ValidatePayload(c.Request, []byte(webhookSecret))
	if err != nil {
		slog.Error("Error validating GitHub app webhook's payload", "appID", appID, "error", err)
		c.String(http.StatusBadRequest, "Error validating github app webhook's payload")
		return
	}

	webhookType := github.WebHookType(c.Request)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		slog.Error("Failed to parse GitHub event", "webhookType", webhookType, "error", err)
		c.String(http.StatusInternalServerError, "Failed to parse Github Event")
		return
	}

	slog.Info("Received GitHub event",
		"eventType", reflect.TypeOf(event),
		"webhookType", webhookType,
	)

	appId64, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		slog.Error("Error converting appId string to int64", "appID", appID, "error", err)
		return
	}

	switch event := event.(type) {
	case *github.InstallationEvent:
		slog.Info("Processing InstallationEvent",
			"action", *event.Action,
			"installationId", *event.Installation.ID,
		)

		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event, appId64)
			if err != nil {
				slog.Error("Failed to handle installation deleted event", "error", err)
				c.String(http.StatusAccepted, "Failed to handle webhook event.")
				return
			}
		}
	case *github.PushEvent:
		slog.Info("Processing PushEvent",
			"repo", *event.Repo.FullName,
		)

		go handlePushEvent(gh, event, appId64)

	case *github.IssueCommentEvent:
		slog.Info("Processing IssueCommentEvent",
			"action", *event.Action,
			"repo", *event.Repo.FullName,
			"issueNumber", *event.Issue.Number,
		)

		if event.Sender.Type != nil && *event.Sender.Type == "Bot" {
			slog.Debug("Ignoring bot comment", "senderType", *event.Sender.Type)
			c.String(http.StatusOK, "OK")
			return
		}
		go handleIssueCommentEvent(gh, event, d.CiBackendProvider, appId64, d.GithubWebhookPostIssueCommentHooks)

	case *github.PullRequestEvent:
		slog.Info("Processing PullRequestEvent",
			"action", *event.Action,
			"repo", *event.Repo.FullName,
			"prNumber", *event.PullRequest.Number,
			"prId", *event.PullRequest.ID,
		)

		// run it as a goroutine to avoid timeouts
		go handlePullRequestEvent(gh, event, d.CiBackendProvider, appId64)

	default:
		slog.Debug("Unhandled event type", "eventType", reflect.TypeOf(event))
	}

	c.JSON(http.StatusAccepted, "ok")
}

func GithubAppSetup(c *gin.Context) {
	slog.Info("Setting up GitHub app")

	type githubWebhook struct {
		URL    string `json:"url"`
		Active bool   `json:"active"`
	}

	type githubAppRequest struct {
		Description           string            `json:"description"`
		Events                []string          `json:"default_events"`
		Name                  string            `json:"name"`
		Permissions           map[string]string `json:"default_permissions"`
		Public                bool              `json:"public"`
		RedirectURL           string            `json:"redirect_url"`
		CallbackUrls          []string          `json:"callback_urls"`
		RequestOauthOnInstall bool              `json:"request_oauth_on_install"`
		SetupOnUpdate         bool              `json:"setup_on_update"`
		URL                   string            `json:"url"`
		Webhook               *githubWebhook    `json:"hook_attributes"`
	}

	host := os.Getenv("HOSTNAME")
	manifest := &githubAppRequest{
		Name:        fmt.Sprintf("Digger app %v", rand.Int31()),
		Description: fmt.Sprintf("Digger hosted at %s", host),
		URL:         host,
		RedirectURL: fmt.Sprintf("%s/github/exchange-code", host),
		Public:      false,
		Webhook: &githubWebhook{
			Active: true,
			URL:    fmt.Sprintf("%s/github-app-webhook", host),
		},
		CallbackUrls:          []string{fmt.Sprintf("%s/github/callback", host)},
		SetupOnUpdate:         true,
		RequestOauthOnInstall: true,
		Events: []string{
			"check_run",
			"create",
			"delete",
			"issue_comment",
			"issues",
			"status",
			"pull_request_review_thread",
			"pull_request_review_comment",
			"pull_request_review",
			"pull_request",
			"push",
		},
		Permissions: map[string]string{
			"actions":          "write",
			"contents":         "write",
			"issues":           "write",
			"pull_requests":    "write",
			"repository_hooks": "write",
			"statuses":         "write",
			"administration":   "read",
			"checks":           "write",
			"members":          "read",
			"workflows":        "write",
		},
	}

	githubHostname := utils.GetGithubHostname()
	url := &url.URL{
		Scheme: "https",
		Host:   githubHostname,
		Path:   "/settings/apps/new",
	}

	// https://developer.github.com/apps/building-github-apps/creating-github-apps-using-url-parameters/#about-github-app-url-parameters
	githubOrg := os.Getenv("GITHUB_ORG")
	if githubOrg != "" {
		url.Path = fmt.Sprintf("organizations/%s%s", githubOrg, url.Path)
		slog.Debug("Using organization-specific GitHub app setup URL", "organization", githubOrg)
	}

	jsonManifest, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		slog.Error("Failed to serialize manifest", "error", err)
		c.Error(fmt.Errorf("failed to serialize manifest %s", err))
		return
	}

	slog.Info("Rendering GitHub app setup template",
		"targetUrl", url.String(),
		"appName", manifest.Name,
	)

	c.HTML(http.StatusOK, "github_setup.tmpl", gin.H{"Target": url.String(), "Manifest": string(jsonManifest)})
}

// GithubSetupExchangeCode handles the user coming back from creating their app
// A code query parameter is exchanged for this app's ID, key, and webhook_secret
// Implements https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest/#implementing-the-github-app-manifest-flow
func (d DiggerController) GithubSetupExchangeCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		slog.Warn("Missing code query parameter in GitHub setup callback")
		c.Error(fmt.Errorf("Ignoring callback, missing code query parameter"))
		return
	}

	slog.Info("Exchanging code for GitHub app credentials", "code", code)

	// TODO: to make tls verification configurable for debug purposes
	//var transport *http.Transport = nil
	//_, exists := os.LookupEnv("DIGGER_GITHUB_SKIP_TLS")
	//if exists {
	//	transport = &http.Transport{
	//		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	//	}
	//}

	client, err := d.GithubClientProvider.NewClient(nil)
	if err != nil {
		slog.Error("Could not create GitHub client", "error", err)
		c.Error(fmt.Errorf("could not create github client: %v", err))
		return
	}

	cfg, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
	if err != nil {
		slog.Error("Failed to exchange code for GitHub app", "error", err)
		c.Error(fmt.Errorf("Failed to exchange code for github app: %s", err))
		return
	}

	slog.Info("Successfully retrieved GitHub app credentials",
		"appName", *cfg.Name,
		"appId", cfg.GetID(),
	)

	PEM := cfg.GetPEM()
	PemBase64 := base64.StdEncoding.EncodeToString([]byte(PEM))
	c.HTML(http.StatusOK, "github_setup.tmpl", gin.H{
		"Target":        "",
		"Manifest":      "",
		"ID":            cfg.GetID(),
		"ClientID":      cfg.GetClientID(),
		"ClientSecret":  cfg.GetClientSecret(),
		"Key":           PEM,
		"KeyBase64":     PemBase64,
		"WebhookSecret": cfg.GetWebhookSecret(),
		"URL":           cfg.GetHTMLURL(),
	})
}

func createOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId int64, appId int64, defaultBranch string, cloneUrl string) (*models.Repo, *models.Organisation, error) {
	slog.Info("Creating or getting Digger repo for GitHub repo",
		slog.Group("githubRepo",
			slog.String("fullName", ghRepoFullName),
			slog.String("organization", ghRepoOrganisation),
			slog.String("name", ghRepoName),
			slog.String("url", ghRepoUrl),
		),
		"installationId", installationId,
	)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error fetching installation link", "installationId", installationId, "error", err)
		return nil, nil, err
	}

	orgId := link.OrganisationId
	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		slog.Error("Error fetching organisation", "orgId", orgId, "error", err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo models.Repo
	r := models.DB.GormDB.Unscoped().Where("organisation_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

	if r.Error != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Repo not found, will create a new one", "diggerRepoName", diggerRepoName)
		} else {
			slog.Error("Error fetching repo", "diggerRepoName", diggerRepoName, "error", err)
			return nil, nil, err
		}
	}

	if r.RowsAffected > 0 {
		slog.Info("Digger repo already exists, restoring if deleted", "diggerRepoName", diggerRepoName, "repoId", existingRepo.ID)
		existingRepo.DeletedAt = gorm.DeletedAt{}
		existingRepo.GithubAppId = appId
		existingRepo.GithubAppInstallationId = installationId
		existingRepo.CloneUrl = cloneUrl
		existingRepo.DefaultBranch = defaultBranch
		models.DB.GormDB.Save(&existingRepo)
		return &existingRepo, org, nil
	}

	slog.Info("Creating new Digger repo", "diggerRepoName", diggerRepoName, "orgId", orgId)
	repo, err := models.DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, `
generate_projects:
 include: "."
`, installationId, appId, defaultBranch, cloneUrl)
	if err != nil {
		slog.Error("Error creating Digger repo", "diggerRepoName", diggerRepoName, "error", err)
		return nil, nil, err
	}

	slog.Info("Created Digger repo", "repoId", repo.ID, "diggerRepoName", diggerRepoName)
	return repo, org, nil
}

func handleInstallationDeletedEvent(installation *github.InstallationEvent, appId int64) error {
	installationId := *installation.Installation.ID

	slog.Info("Handling installation deleted event",
		"installationId", installationId,
		"appId", appId,
	)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	_, err = models.DB.MakeGithubAppInstallationLinkInactive(link)
	if err != nil {
		slog.Error("Error making installation link inactive", "installationId", installationId, "error", err)
		return err
	}

	for _, repo := range installation.Repositories {
		repoFullName := *repo.FullName
		slog.Info("Removing installation for repo",
			"installationId", installationId,
			"repoFullName", repoFullName,
		)

		_, err := models.DB.GithubRepoRemoved(installationId, appId, repoFullName)
		if err != nil {
			slog.Error("Error removing GitHub repo",
				"installationId", installationId,
				"repoFullName", repoFullName,
				"error", err,
			)
			return err
		}
	}

	slog.Info("Successfully handled installation deleted event", "installationId", installationId)
	return nil
}

func handlePushEvent(gh utils.GithubClientProvider, payload *github.PushEvent, appId int64) error {
	slog.Debug("Handling push event", "appId", appId, "payload", payload)
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			slog.Error("Recovered from panic in handlePushEvent", "error", r)
			fmt.Printf("Stack trace:\n%s\n", stack)
		}
	}()

	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	ref := *payload.Ref
	defaultBranch := *payload.Repo.DefaultBranch

	loadProjectsOnPush := os.Getenv("DIGGER_LOAD_PROJECTS_ON_PUSH")

	if loadProjectsOnPush == "true" {
		if strings.HasSuffix(ref, defaultBranch) {
			slog.Debug("Loading projects from GitHub repo (push event)", "loadProjectsOnPush", loadProjectsOnPush, "ref", ref, "defaultBranch", defaultBranch)
			err := services.LoadProjectsFromGithubRepo(gh, strconv.FormatInt(installationId, 10), repoFullName, repoOwner, repoName, cloneURL, defaultBranch)
			if err != nil {
				slog.Error("Failed to load projects from GitHub repo", "error", err)
			}
		}
	} else {
		slog.Debug("Skipping loading projects from GitHub repo", "loadProjectsOnPush", loadProjectsOnPush)
	}

	repoCacheEnabled := os.Getenv("DIGGER_CONFIG_REPO_CACHE_ENABLED")
	if repoCacheEnabled == "1" && strings.HasSuffix(ref, defaultBranch) {
		go func() {
			if err := sendProcessCacheRequest(repoFullName, defaultBranch, installationId); err != nil {
				slog.Error("Failed to process cache request", "error", err, "repoFullName", repoFullName)
			}
		}()
	}

	return nil
}

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

	// here we check if pr was closed and automatic deletion is enabled, to avoid errors when
	// pr is merged and the branch does not exist we handle that gracefully
	if action == "closed" {
		slog.Debug("Handling closed PR action", "prNumber", prNumber)
		// we sleep for 1 second to give github time to delete the branch
		time.Sleep(1 * time.Second)

		branchName, _, err := ghService.GetBranchName(prNumber)
		if err != nil {
			slog.Error("Could not retrieve PR details", "prNumber", prNumber, "error", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Could not retrieve PR details, error: %v", err))
			return fmt.Errorf("Could not retrieve PR details: %v", err)
		}

		branchExists, err := ghService.CheckBranchExists(branchName)
		if err != nil {
			slog.Error("Could not check if branch exists", "prNumber", prNumber, "branchName", branchName, "error", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Could not check if branch exists, error: %v", err))
			return fmt.Errorf("Could not check if branch exists: %v", err)
		}

		if !branchExists {
			slog.Info("Branch no longer exists, ignoring PR closed event",
				"prNumber", prNumber,
				"branchName", branchName,
			)
			return nil
		}
	}

	if !slices.Contains([]string{"closed", "opened", "reopened", "synchronize", "converted_to_draft"}, action) {
		slog.Info("Ignoring event with action not requiring processing", "action", action, "prNumber", prNumber)
		return nil
	}

	commentReporterManager := utils.InitCommentReporterManager(ghService, prNumber)
	if _, exists := os.LookupEnv("DIGGER_REPORT_BEFORE_LOADING_CONFIG"); exists {
		_, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting....")
		if err != nil {
			slog.Error("Error initializing comment reporter",
				"prNumber", prNumber,
				"error", err,
			)
			return fmt.Errorf("error initializing comment reporter")
		}
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

	impactedProjects, impactedProjectsSourceMapping, _, err := dg_github.ProcessGitHubPullRequestEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		slog.Error("Error processing GitHub pull request event",
			"prNumber", prNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, coverAllImpactedProjects, err := dg_github.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config, false)
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
		// This one is for aggregate reporting
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

	diggerCommand, err := orchestrator_scheduler.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		slog.Error("Could not determine Digger command from job",
			"prNumber", prNumber,
			"commands", jobsForImpactedProjects[0].Commands,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	if *diggerCommand == orchestrator_scheduler.DiggerCommandNoop {
		slog.Info("Job is of type noop, no actions to perform",
			"prNumber", prNumber,
			"command", *diggerCommand,
		)
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
			prLock := dg_locking.PullRequestLock{
				InternalLock: locking.BackendDBLock{
					OrgId: organisationId,
				},
				CIService:        ghService,
				Reporter:         comment_updater.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         prNumber,
			}

			err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
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
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock {
		err := models.DB.DeleteAllLocksAquiredByPR(prNumber, organisationId)
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
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock ||
		*diggerCommand == orchestrator_scheduler.DiggerCommandLock {
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
		return nil
	}

	commentReporter, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting... Config loaded successfully")
	if err != nil {
		slog.Error("Error initializing comment reporter",
			"prNumber", prNumber,
			"error", err,
		)
		return fmt.Errorf("error initializing comment reporter")
	}

	err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
	if err != nil {
		slog.Error("Error setting status for PR",
			"prNumber", prNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: error setting status for PR: %v", err))
		return fmt.Errorf("error setting status for PR: %v", err)
	}

	nLayers, _ := orchestrator_scheduler.CountUniqueLayers(jobsForImpactedProjects)
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

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]orchestrator_scheduler.Job)
	for _, j := range jobsForImpactedProjects {
		impactedJobsMap[j.ProjectName] = j
	}

	commentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		slog.Error("Error parsing comment ID",
			"commentId", commentReporter.CommentId,
			"error", err,
		)
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
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	slog.Info("Successfully created batch for jobs",
		"prNumber", prNumber,
		"batchId", batchId,
	)

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule {
		slog.Info("Using GroupByModule render mode for comments", "prNumber", prNumber)

		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, prNumber, impactedProjectsSourceMapping)
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

	segment.Track(strconv.Itoa(int(organisationId)), "backend_trigger_job")

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

func GetDiggerConfigForBranch(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, changedFiles []string, taConfig *tac.AtlantisConfig) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	slog.Info("Getting Digger config for branch",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"installationId", installationId,
		"branch", branch,
		"changedFileCount", len(changedFiles),
	)

	ghService, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	err = git_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, "", func(dir string) error {
		slog.Debug("Reading Digger config from cloned repository", "directory", dir)

		diggerYmlStr, err = dg_configuration.ReadDiggerYmlFileContents(dir)
		if err != nil {
			slog.Error("Could not load Digger config file",
				"directory", dir,
				"error", err,
			)
			return err
		}

		slog.Debug("Successfully read digger.yml file", "configLength", len(diggerYmlStr))

		config, _, dependencyGraph, _, err = dg_configuration.LoadDiggerConfig(dir, true, changedFiles, taConfig)
		if err != nil {
			slog.Error("Error loading and parsing Digger config",
				"directory", dir,
				"error", err,
			)
			return err
		}
		return nil
	})

	if err != nil {
		slog.Error("Error cloning and loading config",
			"repoFullName", repoFullName,
			"branch", branch,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error cloning and loading config %v", err)
	}

	projectCount := 0
	if config != nil {
		projectCount = len(config.Projects)
	}

	slog.Info("Digger config loaded successfully",
		"repoFullName", repoFullName,
		"branch", branch,
		"projectCount", projectCount,
	)

	return diggerYmlStr, ghService, config, dependencyGraph, nil
}

// TODO: Refactor this func to receive ghService as input
func getDiggerConfigForPR(gh utils.GithubClientProvider, orgId uint, prLabels []string, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], *string, *string, []string, error) {
	slog.Info("Getting Digger config for PR",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"orgId", orgId,
		"prNumber", prNumber,
		"installationId", installationId,
		"labels", prLabels,
	)

	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var prBranch string
	prBranch, prCommitSha, err := ghService.GetBranchName(prNumber)
	if err != nil {
		slog.Error("Error getting branch name for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting branch name")
	}

	slog.Debug("Retrieved PR details",
		"prNumber", prNumber,
		"branch", prBranch,
		"commitSha", prCommitSha,
	)

	changedFiles, err := ghService.GetChangedFiles(prNumber)
	if err != nil {
		slog.Error("Error getting changed files for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting changed files")
	}

	slog.Debug("Retrieved changed files for PR",
		"prNumber", prNumber,
		"fileCount", len(changedFiles),
	)

	// check if items should be loaded from cache
	useCache := false
	var taConfig *tac.AtlantisConfig = nil
	if val, _ := os.LookupEnv("DIGGER_CONFIG_REPO_CACHE_ENABLED"); val == "1" && !slices.Contains(prLabels, "digger:no-cache") {
		useCache = true
		slog.Info("Attempting to load config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"prNumber", prNumber,
		)

		_, _, _, taConfigTemp, err := retrieveConfigFromCache(orgId, repoFullName)
		if err != nil {
			slog.Info("Could not load from cache, falling back to live loading",
				"orgId", orgId,
				"repoFullName", repoFullName,
				"error", err,
			)
		} else {
			slog.Info("Successfully loaded config from cache",
				"orgId", orgId,
				"repoFullName", repoFullName,
			)
			taConfig = taConfigTemp
		}
	}

	if !useCache {
		slog.Debug("Cache disabled or skipped due to labels",
			"cacheEnabled", os.Getenv("DIGGER_CONFIG_REPO_CACHE_ENABLED") == "1",
			"hasNoCacheLabel", slices.Contains(prLabels, "digger:no-cache"),
		)
	}

	slog.Info("Loading config from repository",
		"repoFullName", repoFullName,
		"branch", prBranch,
		"prNumber", prNumber,
	)

	diggerYmlStr, ghService, config, dependencyGraph, err := GetDiggerConfigForBranch(gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, prBranch, changedFiles, taConfig)
	if err != nil {
		slog.Error("Error loading Digger config from repository",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"branch", prBranch,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error loading digger.yml: %v", err)
	}

	return diggerYmlStr, ghService, config, dependencyGraph, &prBranch, &prCommitSha, changedFiles, nil
}

func retrieveConfigFromCache(orgId uint, repoFullName string) (string, *dg_configuration.DiggerConfig, *graph.Graph[string, dg_configuration.Project], *tac.AtlantisConfig, error) {
	slog.Debug("Retrieving config from cache",
		"orgId", orgId,
		"repoFullName", repoFullName,
	)

	repoCache, err := models.DB.GetRepoCache(orgId, repoFullName)
	if err != nil {
		slog.Info("Failed to load repo cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to load repo cache: %v", err)
	}

	var config dg_configuration.DiggerConfig
	err = json.Unmarshal(repoCache.DiggerConfig, &config)
	if err != nil {
		slog.Error("Failed to unmarshal config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to unmarshal config from cache: %v", err)
	}

	var taConfig tac.AtlantisConfig
	err = json.Unmarshal(repoCache.TerragruntAtlantisConfig, &taConfig)
	if err != nil {
		slog.Error("Failed to unmarshal config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to unmarshal config from cache: %v", err)
	}

	slog.Debug("Creating project dependency graph from cached config",
		"orgId", orgId,
		"repoFullName", repoFullName,
		"projectCount", len(config.Projects),
	)

	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(config.Projects)
	if err != nil {
		slog.Error("Error creating dependency graph from cached config",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error creating dependency graph from cached config: %v", err)
	}

	slog.Info("Successfully retrieved config from cache",
		"orgId", orgId,
		"repoFullName", repoFullName,
		"projectCount", len(config.Projects),
	)

	return repoCache.DiggerYmlStr, &config, &projectsGraph, &taConfig, nil
}

func GetRepoByInstllationId(installationId int64, repoOwner string, repoName string) (*models.Repo, error) {
	slog.Debug("Getting repo by installation ID",
		"installationId", installationId,
		"repoOwner", repoOwner,
		"repoName", repoName,
	)

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		slog.Error("Error getting GitHub app installation link",
			"installationId", installationId,
			"error", err,
		)
		return nil, fmt.Errorf("error getting github app link")
	}

	if link == nil {
		slog.Error("Failed to find GitHub app installation link",
			"installationId", installationId,
		)
		return nil, fmt.Errorf("error getting github app installation link")
	}

	diggerRepoName := repoOwner + "-" + repoName
	slog.Debug("Looking up repo in database",
		"orgId", link.Organisation.ID,
		"diggerRepoName", diggerRepoName,
	)

	repo, err := models.DB.GetRepo(link.Organisation.ID, diggerRepoName)
	if err != nil {
		slog.Error("Error getting repo",
			"orgId", link.Organisation.ID,
			"diggerRepoName", diggerRepoName,
			"error", err,
		)
	} else if repo != nil {
		slog.Debug("Found repo",
			"repoId", repo.ID,
			"diggerRepoName", diggerRepoName,
		)
	}

	return repo, err
}

func getBatchType(jobs []orchestrator_scheduler.Job) orchestrator_scheduler.DiggerBatchType {
	allJobsContainApply := lo.EveryBy(jobs, func(job orchestrator_scheduler.Job) bool {
		return lo.Contains(job.Commands, "digger apply")
	})

	batchType := orchestrator_scheduler.BatchTypePlan
	if allJobsContainApply {
		batchType = orchestrator_scheduler.BatchTypeApply
	}

	slog.Debug("Determined batch type",
		"jobCount", len(jobs),
		"allContainApply", allJobsContainApply,
		"batchType", batchType,
	)

	return batchType
}

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
	isDraft := payload.Issue.GetDraft()
	userCommentId := *payload.GetComment().ID
	actor := *payload.Sender.Login
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
	orgId := link.OrganisationId

	if *payload.Action != "created" {
		slog.Info("Comment action is not 'created', ignoring",
			"action", *payload.Action,
			"issueNumber", issueNumber,
		)
		return nil
	}

	if !strings.HasPrefix(commentBody, "digger") {
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
			"error", ghServiceErr,
		)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	commentReporterManager := utils.InitCommentReporterManager(ghService, issueNumber)
	if _, exists := os.LookupEnv("DIGGER_REPORT_BEFORE_LOADING_CONFIG"); exists {
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

	diggerYmlStr, ghService, config, projectsGraph, branch, commitSha, changedFiles, err := getDiggerConfigForPR(gh, orgId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneURL, issueNumber)
	if err != nil {
		slog.Error("Error getting Digger config for PR",
			"issueNumber", issueNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	// terraform code generator
	if os.Getenv("DIGGER_GENERATION_ENABLED") == "1" {
		slog.Info("Terraform code generation is enabled",
			"issueNumber", issueNumber,
			"repoFullName", repoFullName,
		)

		err = GenerateTerraformFromCode(payload, commentReporterManager, config, defaultBranch, ghService, repoOwner, repoName, commitSha, issueNumber, branch)
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
	err = ghService.CreateCommentReaction(commentIdStr, string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		slog.Warn("Failed to create comment reaction",
			"commentId", commentIdStr,
			"error", err,
		)
	} else {
		slog.Debug("Added eyes reaction to comment", "commentId", commentIdStr)
	}

	if !config.AllowDraftPRs && isDraft {
		slog.Info("Draft PRs are disabled, skipping",
			"issueNumber", issueNumber,
			"isDraft", isDraft,
		)
		return nil
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

	diggerCommand, err := orchestrator_scheduler.GetCommandFromComment(commentBody)
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

	prBranchName, _, err := ghService.GetBranchName(issueNumber)
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

	impactedProjectsForComment, err := generic.FilterOutProjectsFromComment(allImpactedProjects, commentBody)
	if err != nil {
		slog.Error("Error filtering out projects from comment",
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error filtering out projects from comment: %v", err))
		return fmt.Errorf("error filtering out projects from comment")
	}

	slog.Info("Issue comment event processed successfully",
		"issueNumber", issueNumber,
		"impactedProjectCount", len(impactedProjectsForComment),
		"allImpactedProjectsCount", len(allImpactedProjects),
	)

	jobs, coverAllImpactedProjects, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjectsForComment, allImpactedProjects, config.Workflows, prBranchName, defaultBranch)
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

	// perform unlocking in backend
	if config.PrLocks {
		slog.Info("Processing PR locks for impacted projects",
			"issueNumber", issueNumber,
			"projectCount", len(impactedProjectsForComment),
			"command", *diggerCommand,
		)

		for _, project := range impactedProjectsForComment {
			prLock := dg_locking.PullRequestLock{
				InternalLock: locking.BackendDBLock{
					OrgId: orgId,
				},
				CIService:        ghService,
				Reporter:         comment_updater.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         issueNumber,
			}

			err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
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
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock {
		err := models.DB.DeleteAllLocksAquiredByPR(issueNumber, orgId)
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
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock ||
		*diggerCommand == orchestrator_scheduler.DiggerCommandLock {
		slog.Info("Lock/unlock command completed successfully",
			"issueNumber", issueNumber,
			"command", *diggerCommand,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
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
		err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
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

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjectsForComment {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]orchestrator_scheduler.Job)
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

	slog.Info("Converting jobs to Digger jobs",
		"issueNumber", issueNumber,
		"command", *diggerCommand,
		"jobCount", len(impactedProjectsJobMap),
	)

	batchId, _, err := utils.ConvertJobsToDiggerJobs(
		*diggerCommand,
		"github",
		orgId,
		impactedProjectsJobMap,
		impactedProjectsMap,
		projectsGraph,
		installationId,
		*branch,
		issueNumber,
		repoOwner,
		repoName,
		repoFullName,
		*commitSha,
		reporterCommentId,
		diggerYmlStr,
		0,
		aiSummaryCommentId,
		config.ReportTerraformOutputs,
		coverAllImpactedProjects,
		nil,
	)
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

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == orchestrator_scheduler.DiggerCommandPlan || *diggerCommand == orchestrator_scheduler.DiggerCommandApply) {

		slog.Info("Using GroupByModule render mode for comments", "issueNumber", issueNumber)

		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			slog.Error("Error posting initial source comments",
				"issueNumber", issueNumber,
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

	segment.Track(strconv.Itoa(int(orgId)), "backend_trigger_job")

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

func GenerateTerraformFromCode(payload *github.IssueCommentEvent, commentReporterManager utils.CommentReporterManager, config *dg_configuration.DiggerConfig, defaultBranch string, ghService *dg_github.GithubService, repoOwner string, repoName string, commitSha *string, issueNumber int, branch *string) error {
	if !strings.HasPrefix(*payload.Comment.Body, "digger generate") {
		return nil
	}

	slog.Info("Processing Terraform generation request",
		"issueNumber", issueNumber,
		slog.Group("repository",
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"branch", *branch,
		"defaultBranch", defaultBranch,
		"comment", *payload.Comment.Body,
	)

	projectName := ci.ParseProjectName(*payload.Comment.Body)
	if projectName == "" {
		slog.Error("Missing project name in generate command",
			"issueNumber", issueNumber,
			"comment", *payload.Comment.Body,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: generate requires argument -p <project_name>"))
		return fmt.Errorf("generate requires argument -p <project_name>")
	}

	slog.Debug("Looking for project in config",
		"projectName", projectName,
		"issueNumber", issueNumber,
	)

	project := config.GetProject(projectName)
	if project == nil {
		slog.Error("Project not found in digger.yml",
			"projectName", projectName,
			"issueNumber", issueNumber,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf("could not find project %v in digger.yml", projectName))
		return fmt.Errorf("could not find project %v in digger.yml", projectName)
	}

	slog.Info("Found project in configuration",
		"projectName", projectName,
		"projectDir", project.Dir,
		"issueNumber", issueNumber,
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Successfully loaded project"))

	generationEndpoint := os.Getenv("DIGGER_GENERATION_ENDPOINT")
	if generationEndpoint == "" {
		slog.Error("Generation endpoint not configured",
			"issueNumber", issueNumber,
			"projectName", projectName,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: server does not have generation endpoint configured, please verify"))
		return fmt.Errorf("server does not have generation endpoint configured, please verify")
	}
	apiToken := os.Getenv("DIGGER_GENERATION_API_TOKEN")

	// Get all code content from the repository at a specific commit
	getCodeFromCommit := func(ghService *dg_github.GithubService, repoOwner, repoName string, commitSha *string, projectDir string) (string, error) {
		const MaxPatchSize = 1024 * 1024 // 1MB limit

		slog.Debug("Getting code from commit",
			"commitSha", *commitSha,
			"repoOwner", repoOwner,
			"repoName", repoName,
			"projectDir", projectDir,
		)

		// Get the commit's changes compared to default branch
		comparison, _, err := ghService.Client.Repositories.CompareCommits(
			context.Background(),
			repoOwner,
			repoName,
			defaultBranch,
			*commitSha,
			nil,
		)
		if err != nil {
			slog.Error("Error comparing commits",
				"commitSha", *commitSha,
				"defaultBranch", defaultBranch,
				"error", err,
			)
			return "", fmt.Errorf("error comparing commits: %v", err)
		}

		slog.Debug("Retrieved commit comparison",
			"filesChanged", len(comparison.Files),
			"commitSha", *commitSha,
		)

		var appCode strings.Builder
		for _, file := range comparison.Files {
			if file.Patch == nil {
				continue // Skip files without patches
			}

			slog.Debug("Processing file patch",
				"filename", *file.Filename,
				"additions", *file.Additions,
				"patchSize", len(*file.Patch),
			)

			if *file.Additions > 0 {
				lines := strings.Split(*file.Patch, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
						appCode.WriteString(strings.TrimPrefix(line, "+"))
						appCode.WriteString("\n")
					}
				}
			}
			appCode.WriteString("\n")
		}

		if appCode.Len() == 0 {
			slog.Error("No code changes found in commit",
				"commitSha", *commitSha,
				"filesChecked", len(comparison.Files),
			)
			return "", fmt.Errorf("no code changes found in commit %s. Please ensure the PR contains added or modified code", *commitSha)
		}

		slog.Info("Extracted code changes from commit",
			"commitSha", *commitSha,
			"codeLength", appCode.Len(),
		)

		return appCode.String(), nil
	}

	appCode, err := getCodeFromCommit(ghService, repoOwner, repoName, commitSha, project.Dir)
	if err != nil {
		slog.Error("Failed to get code content",
			"projectName", projectName,
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get code content: %v", err))
		return fmt.Errorf("error getting code content: %v", err)
	}

	slog.Debug("Successfully loaded code from commit",
		"issueNumber", issueNumber,
		"codeLength", len(appCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Successfully loaded code from commit"))

	slog.Info("Generating Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"endpoint", generationEndpoint,
		"codeLength", len(appCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf("Generating terraform..."))
	terraformCode, err := utils.GenerateTerraformCode(appCode, generationEndpoint, apiToken)
	if err != nil {
		slog.Error("Failed to generate Terraform code",
			"projectName", projectName,
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not generate terraform code: %v", err))
		return fmt.Errorf("could not generate terraform code: %v", err)
	}

	slog.Info("Successfully generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"codeLength", len(terraformCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Generated terraform"))

	// Committing the generated Terraform code to the repository
	slog.Info("Preparing to commit generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"branch", *branch,
	)

	baseTree, _, err := ghService.Client.Git.GetTree(context.Background(), repoOwner, repoName, *commitSha, false)
	if err != nil {
		slog.Error("Failed to get base tree",
			"commitSha", *commitSha,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get base tree: %v", err))
		return fmt.Errorf("error getting base tree: %v", err)
	}

	tfFilePath := filepath.Join(project.Dir, fmt.Sprintf("generated_%v.tf", issueNumber))

	// Create a new tree with the new file
	treeEntries := []*github.TreeEntry{
		{
			Path:    github.String(tfFilePath),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(terraformCode),
		},
	}

	slog.Debug("Creating new Git tree",
		"baseSHA", *baseTree.SHA,
		"tfFilePath", tfFilePath,
	)

	newTree, _, err := ghService.Client.Git.CreateTree(context.Background(), repoOwner, repoName, *baseTree.SHA, treeEntries)
	if err != nil {
		slog.Error("Failed to create new tree",
			"baseSHA", *baseTree.SHA,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to create new tree: %v", err))
		return fmt.Errorf("error creating new tree: %v", err)
	}

	// Create the commit
	commitMsg := fmt.Sprintf("Add generated Terraform code for %v", projectName)
	commit := &github.Commit{
		Message: &commitMsg,
		Tree:    newTree,
		Parents: []*github.Commit{{SHA: commitSha}},
	}

	slog.Debug("Creating new commit",
		"message", commitMsg,
		"treeSHA", *newTree.SHA,
		"parentSHA", *commitSha,
	)

	newCommit, _, err := ghService.Client.Git.CreateCommit(context.Background(), repoOwner, repoName, commit, nil)
	if err != nil {
		slog.Error("Failed to create commit",
			"message", commitMsg,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to commit Terraform file: %v", err))
		return fmt.Errorf("error committing Terraform file: %v", err)
	}

	// Update the reference to point to the new commit
	refName := fmt.Sprintf("refs/heads/%s", *branch)
	ref := &github.Reference{
		Ref: github.String(refName),
		Object: &github.GitObject{
			SHA: newCommit.SHA,
		},
	}

	slog.Debug("Updating branch reference",
		"ref", refName,
		"newCommitSHA", *newCommit.SHA,
	)

	_, _, err = ghService.Client.Git.UpdateRef(context.Background(), repoOwner, repoName, ref, false)
	if err != nil {
		slog.Error("Failed to update branch reference",
			"ref", refName,
			"newCommitSHA", *newCommit.SHA,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to update branch reference: %v", err))
		return fmt.Errorf("error updating branch reference: %v", err)
	}

	slog.Info("Successfully committed generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"branch", *branch,
		"commitSHA", *newCommit.SHA,
		"filePath", tfFilePath,
	)

	commentReporterManager.UpdateComment(":white_check_mark: Successfully generated and committed Terraform code")
	return nil
}

func TriggerDiggerJobs(ciBackend ci_backends.CiBackend, repoFullName string, repoOwner string, repoName string, batchId *uuid.UUID, prNumber int, prService ci.PullRequestService, gh utils.GithubClientProvider) error {
	slog.Info("Triggering Digger jobs for batch",
		"batchId", batchId,
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"prNumber", prNumber,
	)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		slog.Error("Failed to get Digger batch",
			"batchId", batchId,
			"error", err,
		)
		return fmt.Errorf("failed to get digger batch, %v", err)
	}

	diggerJobs, err := models.DB.GetPendingParentDiggerJobs(batchId)
	if err != nil {
		slog.Error("Failed to get pending Digger jobs",
			"batchId", batchId,
			"error", err,
		)
		return fmt.Errorf("failed to get pending digger jobs, %v", err)
	}

	slog.Info("Retrieved pending jobs for batch",
		"batchId", batchId,
		"jobCount", len(diggerJobs),
		"batchType", batch.BatchType,
	)

	for i, job := range diggerJobs {
		slog.Debug("Processing job",
			"jobIndex", i+1,
			"jobCount", len(diggerJobs),
			"jobId", job.DiggerJobID,
			"batchId", batchId,
		)

		if job.SerializedJobSpec == nil {
			slog.Error("GitHub job specification is nil",
				"jobId", job.DiggerJobID,
				"batchId", batchId,
			)
			return fmt.Errorf("GitHub job can't be nil")
		}

		slog.Debug("Scheduling job",
			"jobId", job.DiggerJobID,
			"specLength", len(job.SerializedJobSpec),
			"batchId", batchId,
		)

		// TODO: make workflow file name configurable
		err = services.ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, batchId, &job, gh)
		if err != nil {
			slog.Error("Failed to trigger CI workflow",
				"jobId", job.DiggerJobID,
				"batchId", batchId,
				"error", err,
			)
			return fmt.Errorf("failed to trigger CI workflow, %v", err)
		}

		slog.Info("Successfully scheduled job",
			"jobId", job.DiggerJobID,
			"batchId", batchId,
			"jobNumber", i+1,
			"totalJobs", len(diggerJobs),
		)
	}

	slog.Info("Successfully triggered all Digger jobs",
		"batchId", batchId,
		"jobCount", len(diggerJobs),
	)

	return nil
}

// CreateDiggerWorkflowWithPullRequest for specified repo it will create a new branch 'digger/configure' and a pull request to default branch
// in the pull request it will try to add .github/workflows/digger_workflow.yml file with workflow for digger
func CreateDiggerWorkflowWithPullRequest(org *models.Organisation, client *github.Client, githubRepo string) error {
	slog.Info("Creating Digger workflow with pull request",
		"githubRepo", githubRepo,
		"orgId", org.ID,
		"orgName", org.Name,
	)

	ctx := context.Background()
	if strings.Index(githubRepo, "/") == -1 {
		slog.Error("GitHub repo is in wrong format",
			"githubRepo", githubRepo,
		)
		return fmt.Errorf("githubRepo is in a wrong format: %v", githubRepo)
	}

	githubRepoSplit := strings.Split(githubRepo, "/")
	if len(githubRepoSplit) != 2 {
		slog.Error("GitHub repo is in wrong format",
			"githubRepo", githubRepo,
			"splitCount", len(githubRepoSplit),
		)
		return fmt.Errorf("githubRepo is in a wrong format: %v", githubRepo)
	}

	repoOwner := githubRepoSplit[0]
	repoName := githubRepoSplit[1]

	slog.Debug("Parsed repository information",
		"owner", repoOwner,
		"name", repoName,
	)

	// check if workflow file exist already in default branch, if it does, do nothing
	// else try to create a branch and PR
	workflowFilePath := ".github/workflows/digger_workflow.yml"

	repo, _, err := client.Repositories.Get(ctx, repoOwner, repoName)
	if err != nil {
		slog.Error("Failed to get repository",
			"owner", repoOwner,
			"name", repoName,
			"error", err,
		)
		return fmt.Errorf("failed to get repository: %w", err)
	}

	defaultBranch := *repo.DefaultBranch
	slog.Debug("Retrieved repository information",
		"owner", repoOwner,
		"name", repoName,
		"defaultBranch", defaultBranch,
	)

	defaultBranchRef, _, err := client.Git.GetRef(ctx, repoOwner, repoName, "refs/heads/"+defaultBranch)
	if err != nil {
		slog.Error("Failed to get default branch reference",
			"owner", repoOwner,
			"name", repoName,
			"defaultBranch", defaultBranch,
			"error", err,
		)
		return fmt.Errorf("failed to get default branch reference: %w", err)
	}

	branch := "digger/configure"
	refName := fmt.Sprintf("refs/heads/%s", branch)
	branchRef := &github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: defaultBranchRef.Object.SHA,
		},
	}

	slog.Debug("Checking if workflow file already exists",
		"workflowPath", workflowFilePath,
		"owner", repoOwner,
		"name", repoName,
		"ref", *defaultBranchRef.Ref,
	)

	opts := &github.RepositoryContentGetOptions{Ref: *defaultBranchRef.Ref}
	contents, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, workflowFilePath, opts)
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			slog.Error("Failed to get contents of workflow file",
				"path", workflowFilePath,
				"owner", repoOwner,
				"name", repoName,
				"error", err,
			)
			return fmt.Errorf("failed to get contents of the file %v", workflowFilePath)
		}

		slog.Debug("Workflow file does not exist, will create it",
			"path", workflowFilePath,
		)
	}

	// workflow file doesn't already exist, we can create it
	if contents == nil {
		slog.Info("Creating new workflow file and branch",
			"branch", branch,
			"path", workflowFilePath,
			"owner", repoOwner,
			"name", repoName,
		)

		// trying to create a new branch
		_, _, err := client.Git.CreateRef(ctx, repoOwner, repoName, branchRef)
		if err != nil {
			// if branch already exist, do nothing
			if strings.Contains(err.Error(), "Reference already exists") {
				slog.Info("Branch already exists, no action needed",
					"branch", branch,
					"owner", repoOwner,
					"name", repoName,
				)
				return nil
			}

			slog.Error("Failed to create branch",
				"branch", branch,
				"owner", repoOwner,
				"name", repoName,
				"error", err,
			)
			return fmt.Errorf("failed to create a branch, %w", err)
		}

		// TODO: move to a separate config
		jobName := "Digger Workflow"
		setupAws := false
		disableLocking := false
		diggerHostname := os.Getenv("DIGGER_CLOUD_HOSTNAME")
		diggerOrg := org.Name

		workflowFileContents := fmt.Sprintf(`on:
  workflow_dispatch:
    inputs:
      job:
        required: true
      id:
        description: 'run identifier'
        required: false
jobs:
  build:
    name: %v
    runs-on: ubuntu-latest
    steps:
      - name: digger run
        uses: diggerhq/digger@develop
        with:
          setup-aws: %v
          disable-locking: %v
          digger-token: ${{ secrets.DIGGER_TOKEN }}
          digger-hostname: '%v'
          digger-organisation: '%v'
        env:
          GITHUB_CONTEXT: ${{ toJson(github) }}
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`, jobName, setupAws, disableLocking, diggerHostname, diggerOrg)

		commitMessage := "Configure Digger workflow"
		var req github.RepositoryContentFileOptions
		req.Content = []byte(workflowFileContents)
		req.Message = &commitMessage
		req.Branch = &branch

		slog.Debug("Creating workflow file commit",
			"path", workflowFilePath,
			"branch", branch,
			"owner", repoOwner,
			"name", repoName,
		)

		_, _, err = client.Repositories.CreateFile(ctx, repoOwner, repoName, workflowFilePath, &req)
		if err != nil {
			slog.Error("Failed to create workflow file",
				"path", workflowFilePath,
				"branch", branch,
				"owner", repoOwner,
				"name", repoName,
				"error", err,
			)
			return fmt.Errorf("failed to create digger workflow file, %w", err)
		}

		prTitle := "Configure Digger"
		pullRequest := &github.NewPullRequest{
			Title: &prTitle,
			Head:  &branch,
			Base:  &defaultBranch,
		}

		slog.Info("Creating pull request for workflow", "title", prTitle, "head", branch, "base", defaultBranch, "owner", repoOwner, "name", repoName)

		pr, _, err := client.PullRequests.Create(ctx, repoOwner, repoName, pullRequest)
		if err != nil {
			slog.Error("Failed to create pull request",
				"title", prTitle,
				"head", branch,
				"base", defaultBranch,
				"owner", repoOwner,
				"name", repoName,
				"error", err,
			)
			return fmt.Errorf("failed to create a pull request for digger/configure, %w", err)
		}

		slog.Info("Successfully created Digger workflow pull request",
			"prNumber", pr.GetNumber(),
			"prUrl", pr.GetHTMLURL(),
			"owner", repoOwner,
			"name", repoName,
		)
	} else {
		slog.Info("Workflow file already exists, no action needed",
			"path", workflowFilePath,
			"owner", repoOwner,
			"name", repoName,
		)
	}

	return nil
}

func (d DiggerController) GithubAppCallbackPage(c *gin.Context) {
	installationId := c.Request.URL.Query()["installation_id"][0]
	//setupAction := c.Request.URL.Query()["setup_action"][0]
	code := c.Request.URL.Query()["code"][0]
	appId := c.Request.URL.Query().Get("state")

	slog.Info("Processing GitHub app callback", "installationId", installationId, "appId", appId)

	clientId, clientSecret, _, _, err := d.GithubClientProvider.FetchCredentials(appId)
	if err != nil {
		slog.Error("Could not fetch credentials for GitHub app", "appId", appId, "error", err)
		c.String(http.StatusInternalServerError, "could not find credentials for github app")
		return
	}

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		slog.Error("Failed to parse installation ID",
			"installationId", installationId,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to parse installation_id.")
		return
	}

	slog.Debug("Validating GitHub callback", "installationId", installationId64, "clientId", clientId)

	result, installation, err := validateGithubCallback(d.GithubClientProvider, clientId, clientSecret, code, installationId64)
	if !result {
		slog.Error("Failed to validate installation ID",
			"installationId", installationId64,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to validate installation_id.")
		return
	}

	// TODO: Lookup org in GithubAppInstallation by installationID if found use that installationID otherwise
	// create a new org for this installationID
	// retrieve org for current orgID
	installationIdInt64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		slog.Error("Failed to parse installation ID as int64",
			"installationId", installationId,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "installationId could not be parsed"})
		return
	}

	slog.Debug("Looking up GitHub app installation link", "installationId", installationIdInt64)

	var link *models.GithubAppInstallationLink
	link, err = models.DB.GetGithubAppInstallationLink(installationIdInt64)
	if err != nil {
		slog.Error("Error getting GitHub app installation link",
			"installationId", installationIdInt64,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error getting github app link"})
		return
	}

	if link == nil {
		slog.Info("No existing link found, creating new organization and link",
			"installationId", installationId,
		)

		name := fmt.Sprintf("dggr-def-%v", uuid.NewString()[:8])
		externalId := uuid.NewString()

		slog.Debug("Creating new organization",
			"name", name,
			"externalId", externalId,
		)

		org, err := models.DB.CreateOrganisation(name, "digger", externalId)
		if err != nil {
			slog.Error("Error creating organization",
				"name", name,
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error with CreateOrganisation"})
			return
		}

		slog.Debug("Creating GitHub installation link",
			"orgId", org.ID,
			"installationId", installationId64,
		)

		link, err = models.DB.CreateGithubInstallationLink(org, installationId64)
		if err != nil {
			slog.Error("Error creating GitHub installation link",
				"orgId", org.ID,
				"installationId", installationId64,
				"error", err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error with CreateGithubInstallationLink"})
			return
		}

		slog.Info("Created new organization and installation link",
			"orgId", org.ID,
			"installationId", installationId64,
		)
	} else {
		slog.Info("Found existing installation link",
			"orgId", link.OrganisationId,
			"installationId", installationId64,
		)
	}

	org := link.Organisation
	orgId := link.OrganisationId

	// create a github installation link (org ID matched to installation ID)
	_, err = models.DB.CreateGithubInstallationLink(org, installationId64)
	if err != nil {
		slog.Error("Error creating GitHub installation link",
			"orgId", orgId,
			"installationId", installationId64,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating GitHub installation"})
		return
	}

	slog.Debug("Getting GitHub client",
		"appId", *installation.AppID,
		"installationId", installationId64,
	)

	client, _, err := d.GithubClientProvider.Get(*installation.AppID, installationId64)
	if err != nil {
		slog.Error("Error retrieving GitHub client",
			"appId", *installation.AppID,
			"installationId", installationId64,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	// we get repos accessible to this installation
	slog.Debug("Listing repositories for installation", "installationId", installationId64)

	opt := &github.ListOptions{Page: 1, PerPage: 100}
	listRepos, _, err := client.Apps.ListRepos(context.Background(), opt)
	if err != nil {
		slog.Error("Failed to list existing repositories",
			"installationId", installationId64,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to list existing repos: %v", err)
		return
	}
	repos := listRepos.Repositories

	slog.Info("Retrieved repositories for installation",
		"installationId", installationId64,
		"repoCount", len(repos),
	)

	// resets all existing installations (soft delete)
	slog.Debug("Resetting existing GitHub installations",
		"installationId", installationId,
	)

	var AppInstallation models.GithubAppInstallation
	err = models.DB.GormDB.Model(&AppInstallation).Where("github_installation_id=?", installationId).Update("status", models.GithubAppInstallDeleted).Error
	if err != nil {
		slog.Error("Failed to update GitHub installations",
			"installationId", installationId,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to update github installations: %v", err)
		return
	}

	// reset all existing repos (soft delete)
	slog.Debug("Soft deleting existing repositories",
		"orgId", orgId,
	)

	var ExistingRepos []models.Repo
	err = models.DB.GormDB.Delete(ExistingRepos, "organisation_id=?", orgId).Error
	if err != nil {
		slog.Error("Could not delete repositories",
			"orgId", orgId,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "could not delete repos: %v", err)
		return
	}

	// here we mark repos that are available one by one
	slog.Info("Adding repositories to organization",
		"orgId", orgId,
		"repoCount", len(repos),
	)

	for i, repo := range repos {
		repoFullName := *repo.FullName
		repoOwner := strings.Split(*repo.FullName, "/")[0]
		repoName := *repo.Name
		repoUrl := fmt.Sprintf("https://%v/%v", utils.GetGithubHostname(), repoFullName)

		slog.Debug("Processing repository",
			"index", i+1,
			"repoFullName", repoFullName,
			"repoOwner", repoOwner,
			"repoName", repoName,
		)

		_, err := models.DB.GithubRepoAdded(
			installationId64,
			*installation.AppID,
			*installation.Account.Login,
			*installation.Account.ID,
			repoFullName,
		)
		if err != nil {
			slog.Error("Error recording GitHub repository",
				"repoFullName", repoFullName,
				"error", err,
			)
			c.String(http.StatusInternalServerError, "github repos added error: %v", err)
			return
		}

		cloneUrl := *repo.CloneURL
		defaultBranch := *repo.DefaultBranch

		_, _, err = createOrGetDiggerRepoForGithubRepo(repoFullName, repoOwner, repoName, repoUrl, installationId64, *installation.AppID, defaultBranch, cloneUrl)
		if err != nil {
			slog.Error("Error creating or getting Digger repo",
				"repoFullName", repoFullName,
				"error", err,
			)
			c.String(http.StatusInternalServerError, "createOrGetDiggerRepoForGithubRepo error: %v", err)
			return
		}
	}

	slog.Info("GitHub app callback processed successfully",
		"installationId", installationId64,
		"orgId", orgId,
		"repoCount", len(repos),
	)

	c.HTML(http.StatusOK, "github_success.tmpl", gin.H{})
}

func (d DiggerController) GithubReposPage(c *gin.Context) {
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		slog.Warn("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	slog.Info("Fetching GitHub repositories for organisation", "orgId", orgId)

	link, err := models.DB.GetGithubInstallationLinkForOrg(orgId)
	if err != nil {
		slog.Error("Failed to get GitHub installation link for organisation",
			"orgId", orgId,
			"error", err,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	slog.Debug("Found GitHub installation link",
		"orgId", orgId,
		"installationId", link.GithubInstallationId,
	)

	installations, err := models.DB.GetGithubAppInstallations(link.GithubInstallationId)
	if err != nil {
		slog.Error("Failed to get GitHub app installations",
			"installationId", link.GithubInstallationId,
			"error", err,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	if len(installations) == 0 {
		slog.Warn("No GitHub installations found",
			"installationId", link.GithubInstallationId,
			"orgId", orgId,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	slog.Debug("Found GitHub installations",
		"count", len(installations),
		"appId", installations[0].GithubAppId,
		"installationId", installations[0].GithubInstallationId,
	)

	gh := d.GithubClientProvider
	client, _, err := gh.Get(installations[0].GithubAppId, installations[0].GithubInstallationId)
	if err != nil {
		slog.Error("Failed to create GitHub client",
			"appId", installations[0].GithubAppId,
			"installationId", installations[0].GithubInstallationId,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GitHub client"})
		return
	}

	slog.Debug("Successfully created GitHub client",
		"appId", installations[0].GithubAppId,
		"installationId", installations[0].GithubInstallationId,
	)

	opts := &github.ListOptions{}
	repos, _, err := client.Apps.ListRepos(context.Background(), opts)
	if err != nil {
		slog.Error("Failed to list GitHub repositories",
			"installationId", installations[0].GithubInstallationId,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list GitHub repos."})
		return
	}

	slog.Info("Successfully retrieved GitHub repositories",
		"orgId", orgId,
		"repoCount", len(repos.Repositories),
	)

	c.HTML(http.StatusOK, "github_repos.tmpl", gin.H{"Repos": repos.Repositories})
}

// why this validation is needed: https://roadie.io/blog/avoid-leaking-github-org-data/
// validation based on https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app , step 3
func validateGithubCallback(githubClientProvider utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, *github.Installation, error) {
	slog.Debug("Validating GitHub callback",
		"clientId", clientId,
		"installationId", installationId,
	)

	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := utils.GetGithubHostname()
	reqURL := fmt.Sprintf("https://%v/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s", githubHostname, clientId, clientSecret, code)

	slog.Debug("Creating OAuth access token request", "hostname", githubHostname)

	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		slog.Error("Could not create HTTP request",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not create HTTP request: %v", err)
	}
	req.Header.Set("accept", "application/json")

	slog.Debug("Sending OAuth token request")
	res, err := httpClient.Do(req)
	if err != nil {
		slog.Error("Request to OAuth access token endpoint failed",
			"error", err,
		)
		return false, nil, fmt.Errorf("request to login/oauth/access_token failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		slog.Error("OAuth token request returned non-200 status",
			"statusCode", res.StatusCode,
			"status", res.Status,
		)
		return false, nil, fmt.Errorf("OAuth token request failed with status: %s", res.Status)
	}

	var t OAuthAccessResponse
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		slog.Error("Could not parse JSON response from OAuth token request",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not parse JSON response: %v", err)
	}

	if t.AccessToken == "" {
		slog.Error("OAuth response contained empty access token")
		return false, nil, fmt.Errorf("received empty access token in OAuth response")
	}

	slog.Debug("Successfully obtained OAuth access token")

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: t.AccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	//tc := &http.Client{
	//	Transport: &oauth2.Transport{
	//		Base:   httpClient.Transport,
	//		Source: oauth2.ReuseTokenSource(nil, ts),
	//	},
	//}

	client, err := githubClientProvider.NewClient(tc)
	if err != nil {
		slog.Error("Could not create GitHub client",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not create github client: %v", err)
	}

	slog.Debug("Listing user installations to validate installation ID",
		"installationIdToMatch", installationId,
	)

	installationIdMatch := false
	// list all installations for the user
	var matchedInstallation *github.Installation
	installations, _, err := client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		slog.Error("Could not retrieve user installations",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not retrieve installations: %v", err)
	}

	slog.Debug("Retrieved user installations",
		"count", len(installations),
	)

	for _, v := range installations {
		slog.Debug("Checking installation",
			"installationId", *v.ID,
			"targetId", installationId,
		)

		if *v.ID == installationId {
			matchedInstallation = v
			installationIdMatch = true

			slog.Info("Found matching installation",
				"installationId", *v.ID,
				"appId", *v.AppID,
				"account", *v.Account.Login,
			)
		}
	}

	if !installationIdMatch {
		slog.Warn("Installation ID does not match any installation for the authenticated user",
			"installationId", installationId,
			"availableInstallations", len(installations),
		)
		return false, nil, fmt.Errorf("installationId %v doesn't match any ID for specified user", installationId)
	}

	slog.Info("Successfully validated GitHub callback",
		"installationId", installationId,
		"appId", *matchedInstallation.AppID,
	)

	return true, matchedInstallation, nil
}
