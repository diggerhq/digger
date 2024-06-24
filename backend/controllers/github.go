package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/services"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/uuid"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
	"golang.org/x/oauth2"
)

type DiggerController struct {
	CiBackendProvider    ci_backends.CiBackendProvider
	GithubClientProvider utils.GithubClientProvider
}

func (d DiggerController) GithubAppWebHook(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	gh := d.GithubClientProvider
	log.Printf("GithubAppWebHook")

	payload, err := github.ValidatePayload(c.Request, []byte(os.Getenv("GITHUB_WEBHOOK_SECRET")))
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

	switch event := event.(type) {
	case *github.InstallationEvent:
		log.Printf("InstallationEvent, action: %v\n", *event.Action)
		if *event.Action == "created" {
			err := handleInstallationCreatedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle webhook event.")
				return
			}
		}

		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle webhook event.")
				return
			}
		}
	case *github.InstallationRepositoriesEvent:
		log.Printf("InstallationRepositoriesEvent, action: %v\n", *event.Action)
		if *event.Action == "added" {
			err := handleInstallationRepositoriesAddedEvent(gh, event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle installation repo added event.")
			}
		}
		if *event.Action == "removed" {
			err := handleInstallationRepositoriesDeletedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle installation repo deleted event.")
			}
		}
	case *github.IssueCommentEvent:
		log.Printf("IssueCommentEvent, action: %v\n", *event.Action)
		if event.Sender.Type != nil && *event.Sender.Type == "Bot" {
			c.String(http.StatusOK, "OK")
			return
		}
		err := handleIssueCommentEvent(gh, event, d.CiBackendProvider)
		if err != nil {
			log.Printf("handleIssueCommentEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *github.PullRequestEvent:
		log.Printf("Got pull request event for %d", *event.PullRequest.ID)
		err := handlePullRequestEvent(gh, event, d.CiBackendProvider)
		if err != nil {
			log.Printf("handlePullRequestEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *github.PushEvent:
		log.Printf("Got push event for %d", event.Repo.URL)
		err := handlePushEvent(gh, event)
		if err != nil {
			log.Printf("handlePushEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	default:
		log.Printf("Unhandled event, event type %v", reflect.TypeOf(event))
	}

	c.JSON(200, "ok")
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
		SetupOnUpdate         bool              `json:"setup_on_update"'`
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

	githubHostname := getGithubHostname()
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

func getGithubHostname() string {
	githubHostname := os.Getenv("DIGGER_GITHUB_HOSTNAME")
	if githubHostname == "" {
		githubHostname = "github.com"
	}
	return githubHostname
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

	_, err = models.DB.CreateGithubApp(cfg.GetName(), cfg.GetID(), cfg.GetHTMLURL())
	if err != nil {
		c.Error(fmt.Errorf("Failed to create github app record on callback"))
	}

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

	repo, err := models.DB.GetRepo(orgId, diggerRepoName)

	if err != nil {
		log.Printf("Error fetching repo: %v", err)
		return nil, nil, err
	}

	if repo != nil {
		log.Printf("Digger repo already exists: %v", repo)
		return repo, org, nil
	}

	repo, err = models.DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, `
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

func handleInstallationRepositoriesAddedEvent(ghClientProvider utils.GithubClientProvider, payload *github.InstallationRepositoriesEvent) error {
	installationId := *payload.Installation.ID
	login := *payload.Installation.Account.Login
	accountId := *payload.Installation.Account.ID
	appId := *payload.Installation.AppID

	for _, repo := range payload.RepositoriesAdded {
		repoFullName := *repo.FullName
		repoOwner := strings.Split(*repo.FullName, "/")[0]
		repoName := *repo.Name
		repoUrl := fmt.Sprintf("https://github.com/%v", repoFullName)
		_, err := models.DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
		if err != nil {
			log.Printf("GithubRepoAdded failed, error: %v\n", err)
			return err
		}

		_, _, err = createOrGetDiggerRepoForGithubRepo(repoFullName, repoOwner, repoName, repoUrl, installationId)
		if err != nil {
			log.Printf("createOrGetDiggerRepoForGithubRepo failed, error: %v\n", err)
			return err
		}
	}
	return nil
}

func handleInstallationRepositoriesDeletedEvent(payload *github.InstallationRepositoriesEvent) error {
	installationId := *payload.Installation.ID
	appId := *payload.Installation.AppID
	for _, repo := range payload.RepositoriesRemoved {
		repoFullName := *repo.FullName
		_, err := models.DB.GithubRepoRemoved(installationId, appId, repoFullName)
		if err != nil {
			return err
		}

		// todo: change the status of DiggerRepo to InActive
	}
	return nil
}

func handleInstallationCreatedEvent(installation *github.InstallationEvent) error {
	installationId := *installation.Installation.ID
	login := *installation.Installation.Account.Login
	accountId := *installation.Installation.Account.ID
	appId := *installation.Installation.AppID

	for _, repo := range installation.Repositories {
		repoFullName := *repo.FullName
		repoOwner := strings.Split(*repo.FullName, "/")[0]
		repoName := *repo.Name
		repoUrl := fmt.Sprintf("https://github.com/%v", repoFullName)

		log.Printf("Adding a new installation %d for repo: %s", installationId, repoFullName)
		_, err := models.DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
		if err != nil {
			return err
		}
		_, _, err = createOrGetDiggerRepoForGithubRepo(repoFullName, repoOwner, repoName, repoUrl, installationId)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleInstallationDeletedEvent(installation *github.InstallationEvent) error {
	installationId := *installation.Installation.ID
	appId := *installation.Installation.AppID

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

func handlePushEvent(gh utils.GithubClientProvider, payload *github.PushEvent) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoFullName := *payload.Repo.FullName
	repoOwner := *payload.Repo.Owner.Login
	cloneURL := *payload.Repo.CloneURL
	ref := *payload.Ref
	defaultBranch := *payload.Repo.DefaultBranch

	if strings.HasSuffix(ref, defaultBranch) {
		link, err := models.DB.GetGithubAppInstallationLink(installationId)
		if err != nil {
			log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
			return fmt.Errorf("error getting github app link")
		}

		orgId := link.OrganisationId
		diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
		repo, err := models.DB.GetRepo(orgId, diggerRepoName)
		if err != nil {
			log.Printf("Error getting Repo: %v", err)
			return fmt.Errorf("error getting github app link")
		}
		if repo == nil {
			log.Printf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
			return fmt.Errorf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
		}

		_, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
		if err != nil {
			log.Printf("Error getting github service: %v", err)
			return fmt.Errorf("error getting github service")
		}
		utils.CloneGitRepoAndDoAction(cloneURL, defaultBranch, *token, func(dir string) error {
			dat, err := os.ReadFile(path.Join(dir, "digger.yml"))
			if err != nil {
				log.Printf("ERROR fetching digger.yml file: %v", err)
				return err
			}
			config, _, _, err := dg_configuration.LoadDiggerConfig(dir, true, nil)
			if err != nil {
				log.Printf("failed to load digger config: %v", err)
				return err
			}
			err = models.DB.UpdateRepoDiggerConfig(link.OrganisationId, *config, string(dat), repo)
			if err != nil {
				log.Printf("failed to update database with digger config: %v", err)
			}
			return nil
		})
	}

	return nil
}

func handlePullRequestEvent(gh utils.GithubClientProvider, payload *github.PullRequestEvent, ciBackendProvider ci_backends.CiBackendProvider) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	prNumber := *payload.PullRequest.Number
	isDraft := payload.PullRequest.GetDraft()
	commitSha := payload.PullRequest.Head.GetSHA()
	branch := payload.PullRequest.Head.GetRef()

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	organisationId := link.OrganisationId

	diggerYmlStr, ghService, config, projectsGraph, _, _, err := getDiggerConfigForPR(gh, installationId, repoFullName, repoOwner, repoName, cloneURL, prNumber)
	if err != nil {
		log.Printf("getDiggerConfigForPR error: %v", err)
		return fmt.Errorf("error getting digger config")
	}

	impactedProjects, impactedProjectsSourceMapping, _, err := dg_github.ProcessGitHubPullRequestEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, _, err := dg_github.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config)
	if err != nil {
		log.Printf("Error converting event to jobsForImpactedProjects: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
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

	diggerCommand, err := orchestrator.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		log.Printf("could not determine digger command from job: %v", jobsForImpactedProjects[0].Commands)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unkown digger command in comment %v", err)
	}

	if *diggerCommand == orchestrator.DiggerCommandNoop {
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
			err = PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed to perform lock action on project: %v, %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == orchestrator.DiggerCommandUnlock ||
		*diggerCommand == orchestrator.DiggerCommandLock {
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("Draft PRs are disabled, skipping PR: %v", prNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(ghService, prNumber, ":construction_worker: Digger starting...")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	err = utils.ReportInitialJobsStatus(commentReporter, jobsForImpactedProjects)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	err = utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]orchestrator.Job)
	for _, j := range jobsForImpactedProjects {
		impactedJobsMap[j.ProjectName] = j
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, organisationId, impactedJobsMap, impactedProjectsMap, projectsGraph, installationId, branch, prNumber, repoOwner, repoName, repoFullName, commitSha, commentReporter.CommentId, diggerYmlStr)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule {
		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, prNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}
		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(organisationId)), "backend_trigger_job")

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
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	err = TriggerDiggerJobs(ciBackend, repoOwner, repoName, batchId, prNumber, ghService)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggerring Digger Jobs")
	}

	return nil
}

func getDiggerConfigForBranch(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	ghService, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return "", nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var config *dg_configuration.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, dg_configuration.Project]

	changedFiles, err := ghService.GetChangedFiles(prNumber)
	if err != nil {
		log.Printf("Error getting changed files: %v", err)
		return "", nil, nil, nil, fmt.Errorf("error getting changed files")
	}
	err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, *token, func(dir string) error {
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
		return "", nil, nil, nil, fmt.Errorf("error cloning and loading config")
	}

	log.Printf("Digger config loadded successfully\n")
	return diggerYmlStr, ghService, config, dependencyGraph, nil
}

func getDiggerConfigForPR(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], *string, *string, error) {
	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return "", nil, nil, nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var prBranch string
	prBranch, prCommitSha, err := ghService.GetBranchName(prNumber)
	if err != nil {
		log.Printf("Error getting branch name: %v", err)
		return "", nil, nil, nil, nil, nil, fmt.Errorf("error getting branch name")
	}

	diggerYmlStr, ghService, config, dependencyGraph, err := getDiggerConfigForBranch(gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, prBranch, prNumber)
	if err != nil {
		log.Printf("Error loading digger.yml: %v", err)
		return "", nil, nil, nil, nil, nil, fmt.Errorf("error loading digger.yml")
	}

	log.Printf("Digger config loadded successfully\n")
	return diggerYmlStr, ghService, config, dependencyGraph, &prBranch, &prCommitSha, nil
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

func getBatchType(jobs []orchestrator.Job) orchestrator_scheduler.DiggerBatchType {
	allJobsContainApply := lo.EveryBy(jobs, func(job orchestrator.Job) bool {
		return lo.Contains(job.Commands, "digger apply")
	})
	if allJobsContainApply == true {
		return orchestrator_scheduler.BatchTypeApply
	} else {
		return orchestrator_scheduler.BatchTypePlan
	}
}

func handleIssueCommentEvent(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	cloneURL := *payload.Repo.CloneURL
	issueNumber := *payload.Issue.Number
	isDraft := payload.Issue.GetDraft()
	commentId := *payload.GetComment().ID

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

	diggerYmlStr, ghService, config, projectsGraph, branch, commitSha, err := getDiggerConfigForPR(gh, installationId, repoFullName, repoOwner, repoName, cloneURL, issueNumber)
	if err != nil {
		log.Printf("getDiggerConfigForPR error: %v", err)
		return fmt.Errorf("error getting digger config")
	}

	err = ghService.CreateCommentReaction(commentId, string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", issueNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(ghService, issueNumber, ":construction_worker: Digger starting....")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	diggerCommand, err := orchestrator.GetCommandFromComment(*payload.Comment.Body)
	if err != nil {
		log.Printf("unkown digger command in comment: %v", *payload.Comment.Body)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unkown digger command in comment %v", err)
	}

	prBranchName, _, err := ghService.GetBranchName(issueNumber)
	if err != nil {
		log.Printf("GetBranchName error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	impactedProjects, impactedProjectsSourceMapping, requestedProject, _, err := dg_github.ProcessGitHubIssueCommentEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}
	log.Printf("GitHub IssueComment event processed successfully\n")

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
			err = PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == orchestrator.DiggerCommandUnlock ||
		*diggerCommand == orchestrator.DiggerCommandLock {
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	jobs, _, err := dg_github.ConvertGithubIssueCommentEventToJobs(payload, impactedProjects, requestedProject, config.Workflows, prBranchName)
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

	if len(jobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]orchestrator.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, orgId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, installationId, *branch, issueNumber, repoOwner, repoName, repoFullName, *commitSha, commentReporter.CommentId, diggerYmlStr)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == orchestrator.DiggerCommandPlan || *diggerCommand == orchestrator.DiggerCommandApply) {

		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(orgId)), "backend_trigger_job")

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
	err = TriggerDiggerJobs(ciBackend, repoOwner, repoName, batchId, issueNumber, ghService)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggerring Digger Jobs")
	}
	return nil
}

func PerformLockingActionFromCommand(prLock dg_locking.PullRequestLock, command orchestrator.DiggerCommand) error {
	var err error
	switch command {
	case orchestrator.DiggerCommandUnlock:
		_, err = prLock.Unlock()
		if err != nil {
			err = fmt.Errorf("failed to unlock project: %v", err)
		}
	case orchestrator.DiggerCommandPlan:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	case orchestrator.DiggerCommandApply:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	case orchestrator.DiggerCommandLock:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	}
	return err
}

func TriggerDiggerJobs(ciBackend ci_backends.CiBackend, repoOwner string, repoName string, batchId *uuid.UUID, prNumber int, prService *dg_github.GithubService) error {
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
		err = services.ScheduleJob(ciBackend, repoOwner, repoName, batchId, &job)
		if err != nil {
			log.Printf("failed to trigger github workflow, %v\n", err)
			return fmt.Errorf("failed to trigger github workflow, %v\n", err)
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
	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")

	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		log.Printf("err: %v", err)
		c.String(http.StatusInternalServerError, "Failed to parse installation_id.")
		return
	}

	result, err := validateGithubCallback(d.GithubClientProvider, clientId, clientSecret, code, installationId64)
	if !result {
		log.Printf("Failed to validated installation id, %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to validate installation_id.")
		return
	}

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	_, err = models.DB.CreateGithubInstallationLink(org, installationId64)
	if err != nil {
		log.Printf("Error saving CreateGithubInstallationLink to database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating GitHub installation"})
		return
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
func validateGithubCallback(githubClientProvider utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, error) {
	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := getGithubHostname()
	reqURL := fmt.Sprintf("https://%v/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s", githubHostname, clientId, clientSecret, code)
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("could not create HTTP request: %v\n", err)
	}
	req.Header.Set("accept", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request to login/oauth/access_token failed: %v\n", err)
	}

	if err != nil {
		return false, fmt.Errorf("Failed to read response's body: %v\n", err)
	}

	var t OAuthAccessResponse
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return false, fmt.Errorf("could not parse JSON response: %v\n", err)
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
		return false, fmt.Errorf("could not create github client: %v", err)
	}

	installationIdMatch := false
	// list all installations for the user
	installations, _, err := client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		log.Printf("could not retrieve installations: %v", err)
		return false, fmt.Errorf("could not retrieve installations: %v", installationId)
	}
	log.Printf("installations %v", installations)
	for _, v := range installations {
		log.Printf("installation id: %v\n", *v.ID)
		if *v.ID == installationId {
			installationIdMatch = true
		}
	}
	if !installationIdMatch {
		return false, fmt.Errorf("InstallationId %v doesn't match any id for specified user\n", installationId)
	}
	return true, nil
}
