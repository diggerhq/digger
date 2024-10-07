package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	"github.com/diggerhq/digger/ee/drift/middleware"
	"github.com/diggerhq/digger/ee/drift/model"
	"github.com/diggerhq/digger/ee/drift/tasks"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func (mc MainController) GithubAppWebHook(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	gh := mc.GithubClientProvider
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

func handlePushEvent(gh utils.GithubClientProvider, payload *github.PushEvent) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoFullName := *payload.Repo.FullName
	repoOwner := *payload.Repo.Owner.Login
	cloneURL := *payload.Repo.CloneURL
	ref := *payload.Ref
	defaultBranch := *payload.Repo.DefaultBranch

	if strings.HasSuffix(ref, defaultBranch) {
		go tasks.LoadProjectsFromGithubRepo(gh, strconv.FormatInt(installationId, 10), repoFullName, repoOwner, repoName, cloneURL, defaultBranch)
	}

	return nil
}

func (mc MainController) GithubAppCallbackPage(c *gin.Context) {
	installationIds := c.Request.URL.Query()["installation_id"]
	if len(installationIds) == 0 {
		log.Printf("installationId parameter missing in callback")
		c.String(http.StatusBadRequest, "installation ID parameter is missing")
		return
	}
	installationId := installationIds[0]

	codes := c.Request.URL.Query()["code"]
	if len(codes) == 0 {
		log.Printf("code parameter missing in callback")
		c.String(http.StatusBadRequest, "code parameter missing in callback")
		return
	}
	code := codes[0]

	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		log.Printf("err: %v", err)
		c.String(http.StatusInternalServerError, "Failed to parse installation_id.")
		return
	}

	result, installation, err := validateGithubCallback(mc.GithubClientProvider, clientId, clientSecret, code, installationId64)
	if !result {
		log.Printf("Failed to validated installation id, %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to validate installation_id.")
		return
	}

	// retrive org for current orgID
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		log.Printf("missing argument orgId in github callback")
		c.String(http.StatusBadRequest, "missing orgID in request")
		return
	}
	org, err := dbmodels.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	// create a github installation link (org ID matched to installation ID)
	_, err = dbmodels.DB.CreateGithubInstallationLink(org.ID, installationId)
	if err != nil {
		log.Printf("Error saving GithubInstallationLink to database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating GitHub installation"})
		return
	}

	client, _, err := mc.GithubClientProvider.Get(*installation.AppID, installationId64)
	if err != nil {
		log.Printf("Error retriving github client: %v", err)
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
	err = dbmodels.DB.GormDB.Delete(ExistingRepos, "organisation_id=?", orgId).Error
	if err != nil {
		log.Printf("could not delete repos: %v", err)
		c.String(http.StatusInternalServerError, "could not delete repos: %v", err)
		return
	}

	// here we mark repos that are available one by one
	for _, repo := range repos {
		cloneUrl := *repo.CloneURL
		defaultBranch := *repo.DefaultBranch
		repoFullName := *repo.FullName
		repoOwner := strings.Split(*repo.FullName, "/")[0]
		repoName := *repo.Name
		repoUrl := fmt.Sprintf("https://github.com/%v", repoFullName)

		_, _, err = dbmodels.CreateOrGetDiggerRepoForGithubRepo(repoFullName, repoOwner, repoName, repoUrl, installationId, *installation.AppID, *installation.Account.ID, *installation.Account.Login, defaultBranch, cloneUrl)
		if err != nil {
			log.Printf("createOrGetDiggerRepoForGithubRepo error: %v", err)
			c.String(http.StatusInternalServerError, "createOrGetDiggerRepoForGithubRepo error: %v", err)
			return
		}

		go tasks.LoadProjectsFromGithubRepo(mc.GithubClientProvider, installationId, repoFullName, repoOwner, repoName, cloneUrl, defaultBranch)
	}

	c.String(http.StatusOK, "success", gin.H{})
}

// why this validation is needed: https://roadie.io/blog/avoid-leaking-github-org-data/
// validation based on https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app , step 3
func validateGithubCallback(githubClientProvider next_utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, *github.Installation, error) {
	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := "github.com"
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
