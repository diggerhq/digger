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

	// Code parameter is optional - GitHub doesn't always send it on re-authorization flows
	codeParams, codeExists := c.Request.URL.Query()["code"]
	code := ""
	if codeExists && len(codeParams) > 0 && len(codeParams[0]) > 0 {
		code = codeParams[0]
	} else {
		slog.Debug("No code parameter found, probably a setup update, going to return success since we are relying on webhooks now")
		c.HTML(http.StatusOK, "github_success.tmpl", gin.H{})
		return
	}

	appId := c.Request.URL.Query().Get("state")

	slog.Info("Processing GitHub app callback", "installationId", installationId, "appId", appId, "hasCode", code != "")

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		slog.Error("Failed to parse installation ID",
			"installationId", installationId,
			"error", err,
		)
		c.String(http.StatusInternalServerError, "Failed to parse installation_id.")
		return
	}

	// vcsOwner is used for analytics; we'll populate it if we can validate via OAuth
	var vcsOwner string

	// If we have a code parameter, validate the callback via OAuth
	// This provides additional security by confirming the user authorized the installation
	if code != "" {
		clientId, clientSecret, _, _, err := d.GithubClientProvider.FetchCredentials(appId)
		if err != nil {
			slog.Error("Could not fetch credentials for GitHub app", "appId", appId, "error", err)
			c.String(http.StatusInternalServerError, "could not find credentials for github app")
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

		if installation != nil && installation.Account != nil && installation.Account.Login != nil {
			vcsOwner = *installation.Account.Login
		}
	} else {
		slog.Info("No code parameter provided, skipping OAuth validation (repos will sync via webhook)",
			"installationId", installationId64,
		)
	}

	slog.Debug("Looking up GitHub app installation link", "installationId", installationId64)

	var link *models.GithubAppInstallationLink
	link, err = models.DB.GetGithubAppInstallationLink(installationId64)
	if err != nil {
		slog.Error("Error getting GitHub app installation link",
			"installationId", installationId64,
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

	// Track the installation event for analytics
	segment.Track(*org, vcsOwner, "", "github", "vcs_repo_installed", map[string]string{})

	// Ensure the installation link exists (idempotent operation)
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
