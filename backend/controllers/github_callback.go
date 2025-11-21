package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci/github"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (d DiggerController) GithubAppCallbackPage(c *gin.Context) {
	installationIdParams, installationIdExists := c.Request.URL.Query()["installation_id"]
	if !installationIdExists || len(installationIdParams) == 0 {
		slog.Error("There was no installation_id in the url query parameters")
		c.String(http.StatusBadRequest, "could not find the installation_id query parameter for github app")
		return
	}
	installationId := installationIdParams[0]
	if len(installationId) < 1 {
		slog.Error("Installation_id parameter is empty")
		c.String(http.StatusBadRequest, "installation_id parameter for github app is empty")
		return
	}

	//setupAction := c.Request.URL.Query()["setup_action"][0]
	codeParams, codeExists := c.Request.URL.Query()["code"]

	// Code parameter is only provided for fresh installations, not for updates
	// If code is missing, this is likely an update - just show success page
	// The actual repository changes will be handled by the InstallationRepositoriesEvent webhook
	if !codeExists || len(codeParams) == 0 {
		slog.Info("No code parameter - likely an installation update, showing success page")
		c.HTML(http.StatusOK, "github_success.tmpl", gin.H{})
		return
	}

	code := codeParams[0]
	if len(code) < 1 {
		slog.Error("Code parameter is empty")
		c.String(http.StatusBadRequest, "code parameter for github app is empty")
		return
	}
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

		org, err := models.DB.CreateOrganisation(name, "digger", externalId, nil)
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

	var vcsOwner string = ""
	if installation.Account.Login != nil {
		vcsOwner = *installation.Account.Login
	}
	// we have multiple repos here, we don't really want to send an track event for each repo, so we just send the vcs owner
	segment.Track(*org, vcsOwner, "", "github", "vcs_repo_installed", map[string]string{})

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

	repos, err := github.ListGithubRepos(client)
	if err != nil {
		slog.Error("Failed to list existing repositories",
			"installationId", installationId64,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to list existing repos: %v", err)
		return
	}

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
