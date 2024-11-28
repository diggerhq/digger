package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/segment"
	backend_utils "github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/services"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"github.com/samber/lo"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
)

type DiggerController struct {
	CiBackendProvider    ci_backends.CiBackendProvider
	GithubClientProvider next_utils.GithubClientProvider
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

		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event)
			if err != nil {
				log.Printf("Failed to handle webhook event. %v", err)
				return
			}
		}

		c.String(http.StatusAccepted, "ok")
		return

	case *github.IssueCommentEvent:
		log.Printf("IssueCommentEvent, action: %v\n", *event.Action)
	case *github.PullRequestEvent:
		log.Printf("Got pull request event for %d", *event.PullRequest.ID)
		err := handlePullRequestEvent(gh, event, d.CiBackendProvider)
		if err != nil {
			log.Printf("handlePullRequestEvent error: %v", err)
		}
		c.String(http.StatusAccepted, "ok")
		return

	case *github.PushEvent:
		log.Printf("Got push event for %d", event.Repo.URL)
		handlePushEventApplyAfterMerge(gh, event)
		if err != nil {
			log.Printf("handlePushEvent error: %v", err)
		}
		c.String(http.StatusAccepted, "ok")
		return

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
		SetupOnUpdate         bool              `json:"setup_on_update"`
		URL                   string            `json:"url"`
		Webhook               *githubWebhook    `json:"hook_attributes"`
	}

	host := os.Getenv("DIGGER_HOSTNAME")
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

	_, err = dbmodels.DB.CreateGithubApp(cfg.GetName(), cfg.GetID(), cfg.GetHTMLURL())
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

func createOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId int64) (*model.Repo, *model.Organization, error) {
	link, err := dbmodels.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		log.Printf("Error fetching installation link: %v", err)
		return nil, nil, err
	}
	orgId := link.OrganizationID
	org, err := dbmodels.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation by id: %v, error: %v\n", orgId, err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo model.Repo
	r := dbmodels.DB.GormDB.Unscoped().Where("organization_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

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
		dbmodels.DB.GormDB.Save(&existingRepo)
		log.Printf("Digger repo already exists: %v", existingRepo)
		return &existingRepo, org, nil
	}

	repo, err := dbmodels.DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, `
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

func handleInstallationDeletedEvent(installation *github.InstallationEvent) error {
	installationId := *installation.Installation.ID
	appId := *installation.Installation.AppID

	link, err := dbmodels.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		return err
	}
	_, err = dbmodels.DB.MakeGithubAppInstallationLinkInactive(link)
	if err != nil {
		return err
	}

	for _, repo := range installation.Repositories {
		repoFullName := *repo.FullName
		log.Printf("Removing an installation %d for repo: %s", installationId, repoFullName)
		_, err := dbmodels.DB.GithubRepoRemoved(installationId, appId, repoFullName, link.OrganizationID)
		if err != nil {
			return err
		}
	}
	return nil
}

func handlePullRequestEvent(gh next_utils.GithubClientProvider, payload *github.PullRequestEvent, ciBackendProvider ci_backends.CiBackendProvider) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoOwner := *payload.Repo.Owner.Login
	repoFullName := *payload.Repo.FullName
	//cloneURL := *payload.Repo.CloneURL
	prNumber := *payload.PullRequest.Number
	isDraft := payload.PullRequest.GetDraft()
	commitSha := payload.PullRequest.Head.GetSHA()
	sourceBranch := payload.PullRequest.Head.GetRef()
	targetBranch := payload.PullRequest.Base.GetRef()

	link, err := dbmodels.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	organisationId := link.OrganizationID
	segment.Track(organisationId, "backend_trigger_job")

	ghService, _, err := next_utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("GetGithubService error: %v", err)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	// impacated projects should be fetched from a query
	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.RepoFullName.Eq(repoFullName), r.OrganizationID.Eq(organisationId)).First()
	if err != nil {
		log.Printf("could not find repo: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error could not find repository for org: %v", err))
		return fmt.Errorf("could not find reop: %v", err)
	}
	p := dbmodels.DB.Query.Project
	projects, err := dbmodels.DB.Query.Project.Where(p.RepoID.Eq(repo.ID)).Find()

	var dgprojects []dg_configuration.Project = []dg_configuration.Project{}
	for _, proj := range projects {
		projectBranch := proj.Branch
		if targetBranch == projectBranch {
			dgprojects = append(dgprojects, dbmodels.ToDiggerProject(proj))
		}
	}

	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(dgprojects)
	workflows, err := services.GetWorkflowsForRepoAndBranch(gh, repo.ID, sourceBranch, commitSha)
	if err != nil {
		log.Printf("error getting workflows from config: %v", err)
		return fmt.Errorf("error getting workflows from config")
	}
	var config *dg_configuration.DiggerConfig = &dg_configuration.DiggerConfig{
		ApplyAfterMerge:   true,
		AllowDraftPRs:     false,
		CommentRenderMode: "",
		DependencyConfiguration: dg_configuration.DependencyConfiguration{
			Mode: dg_configuration.DependencyConfigurationHard,
		},
		PrLocks:                    false,
		Projects:                   dgprojects,
		AutoMerge:                  false,
		Telemetry:                  false,
		Workflows:                  workflows,
		MentionDriftedProjectsInPR: false,
		TraverseToNestedProjects:   false,
	}

	impactedProjects, _, _, err := dg_github.ProcessGitHubPullRequestEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, _, err := dg_github.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config, false)
	if err != nil {
		log.Printf("Error converting event to jobsForImpactedProjects: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
		return fmt.Errorf("error converting event to jobsForImpactedProjects")
	}

	if len(jobsForImpactedProjects) == 0 {
		// do not report if no projects are impacted to minimise noise in the PR thread
		// TODO use status checks instead: https://github.com/diggerhq/digger/issues/1135
		log.Printf("No projects impacted; not starting any jobs")
		// This one is for aggregate reporting
		err = backend_utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
		return nil
	}

	diggerCommand, err := orchestrator_scheduler.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		log.Printf("could not determine digger command from job: %v", jobsForImpactedProjects[0].Commands)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	if *diggerCommand == orchestrator_scheduler.DiggerCommandNoop {
		log.Printf("job is of type noop, no actions top perform")
		return nil
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("Draft PRs are disabled, skipping PR: %v", prNumber)
		return nil
	}

	commentReporter, err := backend_utils.InitCommentReporter(ghService, prNumber, ":construction_worker: Digger starting...")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	err = backend_utils.ReportInitialJobsStatus(commentReporter, jobsForImpactedProjects)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	err = backend_utils.SetPRStatusForJobs(ghService, prNumber, jobsForImpactedProjects)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
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
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: could not handle commentId: %v", err))
	}
	batchId, _, err := services.ConvertJobsToDiggerJobs(*diggerCommand, dbmodels.DiggerVCSGithub, organisationId, impactedJobsMap, impactedProjectsMap, projectsGraph, installationId, sourceBranch, prNumber, repoOwner, repoName, repoFullName, commitSha, commentId, "", 0, dbmodels.DiggerBatchPullRequestEvent)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
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
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, *batchId, prNumber, ghService, gh)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		backend_utils.InitCommentReporter(ghService, prNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	return nil
}

func TriggerDiggerJobs(ciBackend ci_backends.CiBackend, repoFullName string, repoOwner string, repoName string, batchId string, prNumber int, prService ci.PullRequestService, gh next_utils.GithubClientProvider) error {
	_, err := dbmodels.DB.GetDiggerBatch(batchId)
	if err != nil {
		log.Printf("failed to get digger batch, %v\n", err)
		return fmt.Errorf("failed to get digger batch, %v\n", err)
	}
	diggerJobs, err := dbmodels.DB.GetPendingParentDiggerJobs(batchId)

	if err != nil {
		log.Printf("failed to get pending digger jobs, %v\n", err)
		return fmt.Errorf("failed to get pending digger jobs, %v\n", err)
	}

	log.Printf("number of diggerJobs:%v\n", len(diggerJobs))

	for _, job := range diggerJobs {
		if job.JobSpec == nil {
			return fmt.Errorf("GitHub job can't be nil")
		}
		jobString := string(job.JobSpec)
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

func getDiggerConfigForBranch(gh next_utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {
	ghService, token, err := next_utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
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
	err = backend_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, func(dir string) error {
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

// TODO: Refactor this func to receive ghService as input
func getDiggerConfigForPR(gh next_utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, prNumber int) (string, *dg_github.GithubService, *dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], *string, *string, error) {
	ghService, _, err := next_utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
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

func GetRepoByInstllationId(installationId int64, repoOwner string, repoName string) (*model.Repo, error) {
	link, err := dbmodels.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return nil, fmt.Errorf("error getting github app link")
	}

	if link == nil {
		log.Printf("Failed to find GithubAppInstallationLink for installationId: %v", installationId)
		return nil, fmt.Errorf("error getting github app installation link")
	}

	diggerRepoName := repoOwner + "-" + repoName
	repo, err := dbmodels.DB.GetRepo(link.OrganizationID, diggerRepoName)
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

func (d DiggerController) GithubAppCallbackPage(c *gin.Context) {
	installationId := c.Request.URL.Query()["installation_id"][0]
	//setupAction := c.Request.URL.Query()["setup_action"][0]
	code := c.Request.URL.Query()["code"][0]
	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")

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

	// retrieve org for current orgID
	orgId := c.GetString(middleware.ORGANISATION_ID_KEY)
	org, err := dbmodels.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	// create a github installation link (org ID matched to installation ID)
	_, err = dbmodels.DB.CreateGithubInstallationLink(org, installationId64)
	if err != nil {
		log.Printf("Error saving GithubInstallationLink to database: %v", err)
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

	// reset all existing repos (soft delete)
	var ExistingRepos []model.Repo
	err = dbmodels.DB.GormDB.Delete(ExistingRepos, "organization_id=?", orgId).Error
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
		repoUrl := fmt.Sprintf("https://github.com/%v", repoFullName)

		// reset the GithubAppInstallation for this repo before creating a new one
		var AppInstallation model.GithubAppInstallation
		err = dbmodels.DB.GormDB.Model(&AppInstallation).Where("repo=?", repoFullName).Update("status", dbmodels.GithubAppInstallDeleted).Error
		if err != nil {
			log.Printf("Failed to update github installations: %v", err)
			c.String(http.StatusInternalServerError, "Failed to update github installations: %v", err)
			return
		}

		_, err := dbmodels.DB.GithubRepoAdded(installationId64, *installation.AppID, *installation.Account.Login, *installation.Account.ID, repoFullName)
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

	link, err := dbmodels.DB.GetGithubInstallationLinkForOrg(orgId)
	if err != nil {
		log.Printf("GetGithubInstallationLinkForOrg error: %v\n", err)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	installations, err := dbmodels.DB.GetGithubAppInstallations(link.GithubInstallationID)
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
	client, _, err := gh.Get(installations[0].GithubAppID, installations[0].GithubInstallationID)
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
func validateGithubCallback(githubClientProvider next_utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, *github.Installation, error) {
	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := getGithubHostname()
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
