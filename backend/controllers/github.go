package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/diggerhq/digger/backend/ci_backends"
	config2 "github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/generic"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
	"golang.org/x/oauth2"
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
	log.Printf("GithubAppWebHook")

	appID := c.GetHeader("X-GitHub-Hook-Installation-Target-ID")

	_, _, webhookSecret, _, err := d.GithubClientProvider.FetchCredentials(appID)

	payload, err := github.ValidatePayload(c.Request, []byte(webhookSecret))
	if err != nil {
		log.Printf("Error validating github app webhook's payload: %v", err)
		c.String(http.StatusBadRequest, "Error validating github app webhook's payload")
		return
	}

	webhookType := github.WebHookType(c.Request)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		log.Printf("Failed to parse Github Event. :%v\n", err)
		c.String(http.StatusInternalServerError, "Failed to parse Github Event")
		return
	}

	log.Printf("github event type: %v\n", reflect.TypeOf(event))

	appId64, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		log.Printf("Error converting appId string to int64: %v", err)
		return
	}

	switch event := event.(type) {
	case *github.InstallationEvent:
		log.Printf("InstallationEvent, action: %v\n", *event.Action)
		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event, appId64)
			if err != nil {
				c.String(http.StatusAccepted, "Failed to handle webhook event.")
				return
			}
		}
	case *github.IssueCommentEvent:
		log.Printf("IssueCommentEvent, action: %v\n", *event.Action)
		if event.Sender.Type != nil && *event.Sender.Type == "Bot" {
			c.String(http.StatusOK, "OK")
			return
		}
		go handleIssueCommentEvent(gh, event, d.CiBackendProvider, appId64, d.GithubWebhookPostIssueCommentHooks)
	case *github.PullRequestEvent:
		log.Printf("Got pull request event for %d", *event.PullRequest.ID)
		// run it as a goroutine to avoid timeouts
		go handlePullRequestEvent(gh, event, d.CiBackendProvider, appId64)
	default:
		log.Printf("Unhandled event, event type %v", reflect.TypeOf(event))
	}

	c.JSON(http.StatusAccepted, "ok")
}

func GithubAppSetup(c *gin.Context) {

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
	}

	jsonManifest, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		c.Error(fmt.Errorf("failed to serialize manifest %s", err))
		return
	}

	c.HTML(http.StatusOK, "github_setup.tmpl", gin.H{"Target": url.String(), "Manifest": string(jsonManifest)})
}

// GithubSetupExchangeCode handles the user coming back from creating their app
// A code query parameter is exchanged for this app's ID, key, and webhook_secret
// Implements https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest/#implementing-the-github-app-manifest-flow
func (d DiggerController) GithubSetupExchangeCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.Error(fmt.Errorf("Ignoring callback, missing code query parameter"))
	}

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
		c.Error(fmt.Errorf("could not create github client: %v", err))
	}
	cfg, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
	if err != nil {
		c.Error(fmt.Errorf("Failed to exchange code for github app: %s", err))
		return
	}
	log.Printf("Found credentials for GitHub app %v with id %d", *cfg.Name, cfg.GetID())

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

func createOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId int64) (*models.Repo, *models.Organisation, error) {
	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		log.Printf("Error fetching installation link: %v", err)
		return nil, nil, err
	}
	orgId := link.OrganisationId
	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation by id: %v, error: %v\n", orgId, err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo models.Repo
	r := models.DB.GormDB.Unscoped().Where("organisation_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

	if r.Error != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("repo not found, will proceed with repo creation")
		} else {
			log.Printf("Error fetching repo: %v", err)
			return nil, nil, err
		}
	}

	if r.RowsAffected > 0 {
		existingRepo.DeletedAt = gorm.DeletedAt{}
		models.DB.GormDB.Save(&existingRepo)
		log.Printf("Digger repo already exists: %v", existingRepo)
		return &existingRepo, org, nil
	}

	repo, err := models.DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, `
generate_projects:
 include: "."
`)
	if err != nil {
		log.Printf("Error creating digger repo: %v", err)
		return nil, nil, err
	}
	log.Printf("Created digger repo: %v", repo)
	return repo, org, nil
}

func handleInstallationDeletedEvent(installation *github.InstallationEvent, appId int64) error {
	installationId := *installation.Installation.ID
	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		return err
	}
	_, err = models.DB.MakeGithubAppInstallationLinkInactive(link)
	if err != nil {
		return err
	}

	for _, repo := range installation.Repositories {
		repoFullName := *repo.FullName
		log.Printf("Removing an installation %d for repo: %s", installationId, repoFullName)
		_, err := models.DB.GithubRepoRemoved(installationId, appId, repoFullName)
		if err != nil {
			return err
		}
	}
	return nil
}

func handlePullRequestEvent(gh utils.GithubClientProvider, payload *github.PullRequestEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64) error {
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

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	organisationId := link.OrganisationId

	ghService, _, ghServiceErr := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if ghServiceErr != nil {
		log.Printf("GetGithubService error: %v", err)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	// here we check if pr was closed and automatic deletion is enabled, to avoid errors when
	// pr is merged and the branch does not exist we handle that gracefully
	if action == "closed" {
		branchName, _, err := ghService.GetBranchName(prNumber)
		if err != nil {
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Could not retrieve PR details, error: %v", err))
			log.Printf("Could not retrieve PR details error: %v", err)
			return fmt.Errorf("Could not retrieve PR details: %v", err)
		}
		branchExists, err := ghService.CheckBranchExists(branchName)
		if err != nil {
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Could not check if branch exists, error: %v", err))
			log.Printf("Could not check if branch exists, error: %v", err)
			return fmt.Errorf("Could not check if branch exists: %v", err)

		}
		if !branchExists {
			log.Printf("automating branch deletion is configured, ignoring pr closed event")
			return nil
		}
	}

	if !slices.Contains([]string{"closed", "opened", "reopened", "synchronize", "converted_to_draft"}, action) {
		log.Printf("The action %v is not one that we should act on, ignoring webhook event", action)
		return nil
	}

	commentReporterManager := utils.InitCommentReporterManager(ghService, prNumber)
	if _, exists := os.LookupEnv("DIGGER_REPORT_BEFORE_LOADING_CONFIG"); exists {
		_, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting....")
		if err != nil {
			log.Printf("Error initializing comment reporter: %v", err)
			return fmt.Errorf("error initializing comment reporter")
		}
	}

	diggerYmlStr, ghService, config, projectsGraph, _, _, changedFiles, err := getDiggerConfigForPR(gh, organisationId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneURL, prNumber)
	if err != nil {
		log.Printf("getDiggerConfigForPR error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error loading digger config: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	impactedProjects, impactedProjectsSourceMapping, _, err := dg_github.ProcessGitHubPullRequestEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, _, err := dg_github.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config, false)
	if err != nil {
		log.Printf("Error converting event to jobsForImpactedProjects: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
		return fmt.Errorf("error converting event to jobsForImpactedProjects")
	}

	if len(jobsForImpactedProjects) == 0 {
		// do not report if no projects are impacted to minimise noise in the PR thread
		// TODO use status checks instead: https://github.com/diggerhq/digger/issues/1135
		log.Printf("No projects impacted; not starting any jobs")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
		return nil
	}

	// if flag set we dont allow more projects impacted than the number of changed files in PR (safety check)
	if config2.LimitByNumOfFilesChanged() {
		if len(impactedProjects) > len(changedFiles) {
			log.Printf("Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjects), len(changedFiles))
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjects), len(changedFiles)))
			log.Printf("Information about the event:")
			log.Printf("GH payload: %v", payload)
			log.Printf("PR changed files: %v", changedFiles)
			log.Printf("digger.yml STR: %v", diggerYmlStr)
			log.Printf("Parsed config: %v", config)
			log.Printf("Dependency graph:")
			spew.Dump(projectsGraph)
			log.Printf("Impacted Projects: %v", impactedProjects)
			log.Printf("Impacted Project jobs: %v", jobsForImpactedProjects)
			return fmt.Errorf("error processing event")
		}
	}
	diggerCommand, err := orchestrator_scheduler.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		log.Printf("could not determine digger command from job: %v", jobsForImpactedProjects[0].Commands)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	if *diggerCommand == orchestrator_scheduler.DiggerCommandNoop {
		log.Printf("job is of type noop, no actions top perform")
		return nil
	}

	// perform locking/unlocking in backend
	if config.PrLocks {
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
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed to perform lock action on project: %v, %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock ||
		*diggerCommand == orchestrator_scheduler.DiggerCommandLock {
		commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("Draft PRs are disabled, skipping PR: %v", prNumber)
		return nil
	}

	commentReporter, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting... Config loaded successfully")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	err = utils.ReportInitialJobsStatus(commentReporter, jobsForImpactedProjects)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

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
		log.Printf("strconv.ParseInt error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not handle commentId: %v", err))
	}
	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, models.DiggerVCSGithub, organisationId, impactedJobsMap, impactedProjectsMap, projectsGraph, installationId, branch, prNumber, repoOwner, repoName, repoFullName, commitSha, commentId, diggerYmlStr, 0)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule {
		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, prNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}
		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(organisationId)), "backend_trigger_job")

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
		log.Printf("GetCiBackend error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, prNumber, ghService, gh)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	return nil
}

func GetDiggerConfigForBranch(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, changedFiles []string) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	ghService, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return "", nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, func(dir string) error {
		diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
		diggerYmlStr = string(diggerYmlBytes)
		config, _, dependencyGraph, err = dg_configuration.LoadDiggerConfig(dir, true, changedFiles)
		if err != nil {
			log.Printf("Error loading digger config: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("Error cloning and loading config: %v", err)
		return "", nil, nil, nil, fmt.Errorf("error cloning and loading config %v", err)
	}

	log.Printf("Digger config loadded successfully\n")
	return diggerYmlStr, ghService, config, dependencyGraph, nil
}

// TODO: Refactor this func to receive ghService as input
func getDiggerConfigForPR(gh utils.GithubClientProvider, orgId uint, prLabels []string, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], *string, *string, []string, error) {
	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var prBranch string
	prBranch, prCommitSha, err := ghService.GetBranchName(prNumber)
	if err != nil {
		log.Printf("Error getting branch name: %v", err)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting branch name")
	}

	changedFiles, err := ghService.GetChangedFiles(prNumber)
	if err != nil {
		log.Printf("Error getting changed files: %v", err)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting changed files")
	}

	// check if items should be loaded from cache
	if val, _ := os.LookupEnv("DIGGER_CONFIG_REPO_CACHE_ENABLED"); val == "1" && !slices.Contains(prLabels, "digger:no-cache") {
		diggerYmlStr, config, dependencyGraph, err := retrieveConfigFromCache(orgId, repoFullName)
		if err != nil {
			log.Printf("could not load from cache")
		} else {
			log.Printf("successfully loaded from cache")
			return diggerYmlStr, ghService, config, *dependencyGraph, &prBranch, &prCommitSha, changedFiles, nil
		}
	}

	diggerYmlStr, ghService, config, dependencyGraph, err := GetDiggerConfigForBranch(gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, prBranch, changedFiles)
	if err != nil {
		log.Printf("Error loading digger.yml: %v", err)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error loading digger.yml: %v", err)
	}

	return diggerYmlStr, ghService, config, dependencyGraph, &prBranch, &prCommitSha, changedFiles, nil
}

func retrieveConfigFromCache(orgId uint, repoFullName string) (string, *dg_configuration.DiggerConfig, *graph.Graph[string, dg_configuration.Project], error) {
	repoCache, err := models.DB.GetRepoCache(orgId, repoFullName)
	if err != nil {
		log.Printf("Error: failed to load repoCache, going to try live load %v", err)
		return "", nil, nil, fmt.Errorf("")
	}
	var config dg_configuration.DiggerConfig
	err = json.Unmarshal(repoCache.DiggerConfig, &config)
	if err != nil {
		log.Printf("Error: failed to load repoCache unmarshall config %v", err)
		return "", nil, nil, fmt.Errorf("failed to load repoCache unmarshall config %v", err)
	}

	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(config.Projects)
	if err != nil {
		log.Printf("error retrieving graph of dependencies: %v", err)
		return "", nil, nil, fmt.Errorf("error retrieving graph of dependencies: %v", err)
	}

	return repoCache.DiggerYmlStr, &config, &projectsGraph, nil
}

func GetRepoByInstllationId(installationId int64, repoOwner string, repoName string) (*models.Repo, error) {
	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return nil, fmt.Errorf("error getting github app link")
	}

	if link == nil {
		log.Printf("Failed to find GithubAppInstallationLink for installationId: %v", installationId)
		return nil, fmt.Errorf("error getting github app installation link")
	}

	diggerRepoName := repoOwner + "-" + repoName
	repo, err := models.DB.GetRepo(link.Organisation.ID, diggerRepoName)
	return repo, nil
}

func getBatchType(jobs []orchestrator_scheduler.Job) orchestrator_scheduler.DiggerBatchType {
	allJobsContainApply := lo.EveryBy(jobs, func(job orchestrator_scheduler.Job) bool {
		return lo.Contains(job.Commands, "digger apply")
	})
	if allJobsContainApply == true {
		return orchestrator_scheduler.BatchTypeApply
	} else {
		return orchestrator_scheduler.BatchTypePlan
	}
}

func handleIssueCommentEvent(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider, appId int64, postCommentHooks []IssueCommentHook) error {
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

	if !isPullRequest {
		log.Printf("comment not on pullrequest, ignroning")
		return nil
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

	if !strings.HasPrefix(*payload.Comment.Body, "digger") {
		log.Printf("comment is not a Digger command, ignoring")
		return nil
	}

	ghService, _, ghServiceErr := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if ghServiceErr != nil {
		log.Printf("GetGithubService error: %v", err)
		return fmt.Errorf("error getting ghService to post error comment")
	}
	commentReporterManager := utils.InitCommentReporterManager(ghService, issueNumber)
	if _, exists := os.LookupEnv("DIGGER_REPORT_BEFORE_LOADING_CONFIG"); exists {
		_, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting....")
		if err != nil {
			log.Printf("Error initializing comment reporter: %v", err)
			return fmt.Errorf("error initializing comment reporter")
		}
	}

	diggerYmlStr, ghService, config, projectsGraph, branch, commitSha, changedFiles, err := getDiggerConfigForPR(gh, orgId, prLabelsStr, installationId, repoFullName, repoOwner, repoName, cloneURL, issueNumber)
	if err != nil {
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		log.Printf("getDiggerConfigForPR error: %v", err)
		return fmt.Errorf("error getting digger config")
	}

	commentIdStr := strconv.FormatInt(userCommentId, 10)
	err = ghService.CreateCommentReaction(commentIdStr, string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", issueNumber)
		return nil
	}

	commentReporter, err := commentReporterManager.UpdateComment(":construction_worker: Digger starting.... config loaded successfully")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	diggerCommand, err := orchestrator_scheduler.GetCommandFromComment(*payload.Comment.Body)
	if err != nil {
		log.Printf("unknown digger command in comment: %v", *payload.Comment.Body)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	prBranchName, _, err := ghService.GetBranchName(issueNumber)
	if err != nil {
		log.Printf("GetBranchName error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	impactedProjects, impactedProjectsSourceMapping, requestedProject, _, err := generic.ProcessIssueCommentEvent(issueNumber, *payload.Comment.Body, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}
	log.Printf("GitHub IssueComment event processed successfully\n")

	jobs, _, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjects, requestedProject, config.Workflows, prBranchName, defaultBranch)
	if err != nil {
		log.Printf("Error converting event to jobs: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}
	log.Printf("GitHub IssueComment event converted to Jobs successfully\n")

	// if flag set we dont allow more projects impacted than the number of changed files in PR (safety check)
	if config2.LimitByNumOfFilesChanged() {
		if len(impactedProjects) > len(changedFiles) {
			log.Printf("Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjects), len(changedFiles))
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: Error the number impacted projects %v exceeds number of changed files: %v", len(impactedProjects), len(changedFiles)))
			log.Printf("Information about the event:")
			log.Printf("GH payload: %v", payload)
			log.Printf("PR changed files: %v", changedFiles)
			log.Printf("digger.yml STR: %v", diggerYmlStr)
			log.Printf("Parsed config: %v", config)
			log.Printf("Dependency graph:")
			spew.Dump(projectsGraph)
			log.Printf("Impacted Projects: %v", impactedProjects)
			log.Printf("Impacted Project jobs: %v", jobs)
			return fmt.Errorf("error processing event")
		}
	}

	// perform unlocking in backend
	if config.PrLocks {
		for _, project := range impactedProjects {
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
				commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == orchestrator_scheduler.DiggerCommandUnlock ||
		*diggerCommand == orchestrator_scheduler.DiggerCommandLock {
		commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	if len(jobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]orchestrator_scheduler.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	reporterCommentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		log.Printf("strconv.ParseInt error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not handle commentId: %v", err))
		return fmt.Errorf("comment reporter error: %v", err)
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, "github", orgId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, installationId, *branch, issueNumber, repoOwner, repoName, repoFullName, *commitSha, reporterCommentId, diggerYmlStr, 0)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == orchestrator_scheduler.DiggerCommandPlan || *diggerCommand == orchestrator_scheduler.DiggerCommandApply) {

		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			commentReporterManager.UpdateComment(fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(orgId)), "backend_trigger_job")

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
		log.Printf("GetCiBackend error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}
	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, ghService, gh)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	log.Printf("executing issue comment event post hooks:")
	for _, hook := range postCommentHooks {
		err := hook(gh, payload, ciBackendProvider)
		if err != nil {
			log.Printf("handleIssueCommentEvent post hook error: %v", err)
			return fmt.Errorf("error during postevent hooks: %v", err)
		}
	}
	return nil
}

func TriggerDiggerJobs(ciBackend ci_backends.CiBackend, repoFullName string, repoOwner string, repoName string, batchId *uuid.UUID, prNumber int, prService ci.PullRequestService, gh utils.GithubClientProvider) error {
	_, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("failed to get digger batch, %v\n", err)
		return fmt.Errorf("failed to get digger batch, %v\n", err)
	}
	diggerJobs, err := models.DB.GetPendingParentDiggerJobs(batchId)

	if err != nil {
		log.Printf("failed to get pending digger jobs, %v\n", err)
		return fmt.Errorf("failed to get pending digger jobs, %v\n", err)
	}

	log.Printf("number of diggerJobs:%v\n", len(diggerJobs))

	for _, job := range diggerJobs {
		if job.SerializedJobSpec == nil {
			return fmt.Errorf("GitHub job can't be nil")
		}
		jobString := string(job.SerializedJobSpec)
		log.Printf("jobString: %v \n", jobString)

		// TODO: make workflow file name configurable
		err = services.ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, batchId, &job, gh)
		if err != nil {
			log.Printf("failed to trigger CI workflow, %v\n", err)
			return fmt.Errorf("failed to trigger CI workflow, %v\n", err)
		}
	}
	return nil
}

// CreateDiggerWorkflowWithPullRequest for specified repo it will create a new branch 'digger/configure' and a pull request to default branch
// in the pull request it will try to add .github/workflows/digger_workflow.yml file with workflow for digger
func CreateDiggerWorkflowWithPullRequest(org *models.Organisation, client *github.Client, githubRepo string) error {
	ctx := context.Background()
	if strings.Index(githubRepo, "/") == -1 {
		return fmt.Errorf("githubRepo is in a wrong format: %v", githubRepo)
	}
	githubRepoSplit := strings.Split(githubRepo, "/")
	if len(githubRepoSplit) != 2 {
		return fmt.Errorf("githubRepo is in a wrong format: %v", githubRepo)
	}
	repoOwner := githubRepoSplit[0]
	repoName := githubRepoSplit[1]

	// check if workflow file exist already in default branch, if it does, do nothing
	// else try to create a branch and PR

	workflowFilePath := ".github/workflows/digger_workflow.yml"
	repo, _, _ := client.Repositories.Get(ctx, repoOwner, repoName)
	defaultBranch := *repo.DefaultBranch

	defaultBranchRef, _, _ := client.Git.GetRef(ctx, repoOwner, repoName, "refs/heads/"+defaultBranch) // or "refs/heads/main"
	branch := "digger/configure"
	refName := fmt.Sprintf("refs/heads/%s", branch)
	branchRef := &github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: defaultBranchRef.Object.SHA,
		},
	}

	opts := &github.RepositoryContentGetOptions{Ref: *defaultBranchRef.Ref}
	contents, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, workflowFilePath, opts)
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			log.Printf("failed to get contents of the file %v", err)
			return fmt.Errorf("failed to get contents of the file %v", workflowFilePath)
		}
	}

	// workflow file doesn't already exist, we can create it
	if contents == nil {
		// trying to create a new branch
		_, _, err := client.Git.CreateRef(ctx, repoOwner, repoName, branchRef)
		if err != nil {
			// if branch already exist, do nothing
			if strings.Contains(err.Error(), "Reference already exists") {
				log.Printf("Branch %v already exist, do nothing\n", branchRef)
				return nil
			}
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

		_, _, err = client.Repositories.CreateFile(ctx, repoOwner, repoName, workflowFilePath, &req)
		if err != nil {
			return fmt.Errorf("failed to create digger workflow file, %w", err)
		}

		prTitle := "Configure Digger"
		pullRequest := &github.NewPullRequest{Title: &prTitle,
			Head: &branch, Base: &defaultBranch}
		_, _, err = client.PullRequests.Create(ctx, repoOwner, repoName, pullRequest)
		if err != nil {
			return fmt.Errorf("failed to create a pull request for digger/configure, %w", err)
		}
	}
	return nil
}

func (d DiggerController) GithubAppCallbackPage(c *gin.Context) {
	installationId := c.Request.URL.Query()["installation_id"][0]
	//setupAction := c.Request.URL.Query()["setup_action"][0]
	code := c.Request.URL.Query()["code"][0]
	appId := c.Request.URL.Query().Get("state")

	clientId, clientSecret, _, _, err := d.GithubClientProvider.FetchCredentials(appId)
	if err != nil {
		log.Printf("could not fetch credentials for the app: %v", err)
		c.String(500, "could not find credentials for github app")
		return
	}

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		log.Printf("err: %v", err)
		c.String(http.StatusInternalServerError, "Failed to parse installation_id.")
		return
	}

	result, installation, err := validateGithubCallback(d.GithubClientProvider, clientId, clientSecret, code, installationId64)
	if !result {
		log.Printf("Failed to validated installation id, %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to validate installation_id.")
		return
	}

	// TODO: Lookup org in GithubAppInstallation by installationID if found use that installationID otherwise
	// create a new org for this installationID
	// retrieve org for current orgID
	installationIdInt64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		log.Printf("strconv.ParseInt error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "installationId could not be parsed"})
		return
	}

	var link *models.GithubAppInstallationLink
	link, err = models.DB.GetGithubAppInstallationLink(installationIdInt64)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error getting github app link"})
		return
	}

	if link == nil {
		log.Printf("Failed to find GithubAppInstallationLink create a link and an org %v", installationId)
		name := fmt.Sprintf("dggr-def-%v", uuid.NewString()[:8])
		externalId := uuid.NewString()
		org, err := models.DB.CreateOrganisation(name, "digger", externalId)
		if err != nil {
			log.Printf("Error with CreateOrganisation: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error with CreateOrganisation"})
			return
		}
		link, err = models.DB.CreateGithubInstallationLink(org, installationId64)
		if err != nil {
			log.Printf("Error with CreateGithubInstallationLink: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error with CreateGithubInstallationLink"})
			return
		}
	}

	org := link.Organisation
	orgId := link.OrganisationId

	// create a github installation link (org ID matched to installation ID)
	_, err = models.DB.CreateGithubInstallationLink(org, installationId64)
	if err != nil {
		log.Printf("Error saving CreateGithubInstallationLink to database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating GitHub installation"})
		return
	}

	client, _, err := d.GithubClientProvider.Get(*installation.AppID, installationId64)
	if err != nil {
		log.Printf("Error retrieving github client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return

	}

	// we get repos accessible to this installation
	listRepos, _, err := client.Apps.ListRepos(context.Background(), nil)
	if err != nil {
		log.Printf("Failed to validated list existing repos, %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to list existing repos: %v", err)
		return
	}
	repos := listRepos.Repositories

	// resets all existing installations (soft delete)
	var AppInstallation models.GithubAppInstallation
	err = models.DB.GormDB.Model(&AppInstallation).Where("github_installation_id=?", installationId).Update("status", models.GithubAppInstallDeleted).Error
	if err != nil {
		log.Printf("Failed to update github installations: %v", err)
		c.String(http.StatusInternalServerError, "Failed to update github installations: %v", err)
		return
	}

	// reset all existing repos (soft delete)
	var ExistingRepos []models.Repo
	err = models.DB.GormDB.Delete(ExistingRepos, "organisation_id=?", orgId).Error
	if err != nil {
		log.Printf("could not delete repos: %v", err)
		c.String(http.StatusInternalServerError, "could not delete repos: %v", err)
		return
	}

	// here we mark repos that are available one by one
	for _, repo := range repos {
		repoFullName := *repo.FullName
		repoOwner := strings.Split(*repo.FullName, "/")[0]
		repoName := *repo.Name
		repoUrl := fmt.Sprintf("https://%v/%v", utils.GetGithubHostname(), repoFullName)
		_, err := models.DB.GithubRepoAdded(installationId64, *installation.AppID, *installation.Account.Login, *installation.Account.ID, repoFullName)
		if err != nil {
			log.Printf("github repos added error: %v", err)
			c.String(http.StatusInternalServerError, "github repos added error: %v", err)
			return
		}

		_, _, err = createOrGetDiggerRepoForGithubRepo(repoFullName, repoOwner, repoName, repoUrl, installationId64)
		if err != nil {
			log.Printf("createOrGetDiggerRepoForGithubRepo error: %v", err)
			c.String(http.StatusInternalServerError, "createOrGetDiggerRepoForGithubRepo error: %v", err)
			return
		}
	}

	c.HTML(http.StatusOK, "github_success.tmpl", gin.H{})
}

func (d DiggerController) GithubReposPage(c *gin.Context) {
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		log.Printf("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	link, err := models.DB.GetGithubInstallationLinkForOrg(orgId)
	if err != nil {
		log.Printf("GetGithubInstallationLinkForOrg error: %v\n", err)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	installations, err := models.DB.GetGithubAppInstallations(link.GithubInstallationId)
	if err != nil {
		log.Printf("GetGithubAppInstallations error: %v\n", err)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	if len(installations) == 0 {
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	gh := d.GithubClientProvider
	client, _, err := gh.Get(installations[0].GithubAppId, installations[0].GithubInstallationId)
	if err != nil {
		log.Printf("failed to create github client, %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GitHub client"})
		return
	}

	opts := &github.ListOptions{}
	repos, _, err := client.Apps.ListRepos(context.Background(), opts)
	if err != nil {
		log.Printf("GetGithubAppInstallations error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list GitHub repos."})
		return
	}
	c.HTML(http.StatusOK, "github_repos.tmpl", gin.H{"Repos": repos.Repositories})
}

// why this validation is needed: https://roadie.io/blog/avoid-leaking-github-org-data/
// validation based on https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app , step 3
func validateGithubCallback(githubClientProvider utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, *github.Installation, error) {
	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := utils.GetGithubHostname()
	reqURL := fmt.Sprintf("https://%v/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s", githubHostname, clientId, clientSecret, code)
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return false, nil, fmt.Errorf("could not create HTTP request: %v\n", err)
	}
	req.Header.Set("accept", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("request to login/oauth/access_token failed: %v\n", err)
	}

	if err != nil {
		return false, nil, fmt.Errorf("Failed to read response's body: %v\n", err)
	}

	var t OAuthAccessResponse
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return false, nil, fmt.Errorf("could not parse JSON response: %v\n", err)
	}

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
		log.Printf("could create github client: %v", err)
		return false, nil, fmt.Errorf("could not create github client: %v", err)
	}

	installationIdMatch := false
	// list all installations for the user
	var matchedInstallation *github.Installation
	installations, _, err := client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		log.Printf("could not retrieve installations: %v", err)
		return false, nil, fmt.Errorf("could not retrieve installations: %v", installationId)
	}
	log.Printf("installations %v", installations)
	for _, v := range installations {
		log.Printf("installation id: %v\n", *v.ID)
		if *v.ID == installationId {
			matchedInstallation = v
			installationIdMatch = true
		}
	}
	if !installationIdMatch {
		return false, nil, fmt.Errorf("InstallationId %v doesn't match any id for specified user\n", installationId)
	}

	return true, matchedInstallation, nil
}
