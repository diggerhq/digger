package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
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
	if !codeExists || len(codeParams) == 0 {
		slog.Error("There was no code in the url query parameters")
		c.String(http.StatusBadRequest, "could not find the code query parameter for github app")
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

	slog.Info("GitHub app callback processed",
		"installationId", installationId64,
		"orgId", orgId,
	)

	c.HTML(http.StatusOK, "github_success.tmpl", gin.H{})
}
