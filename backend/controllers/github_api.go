package controllers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func LinkGithubInstallationToOrgApi(c *gin.Context) {
	type LinkInstallationRequest struct {
		InstallationId string `json:"installation_id"`
	}

	var request LinkInstallationRequest
	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("Error binding JSON", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request format"})
		return
	}

	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err = models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Could not find organisation", "organisationId", organisationId, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"status": "Could not find organisation: " + organisationId})
		} else {
			slog.Error("Database error while finding organisation", "organisationId", organisationId, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Internal server error"})
		}
		return
	}

	installationId, err := strconv.ParseInt(request.InstallationId, 10, 64)
	if err != nil {
		slog.Error("Failed to convert InstallationId to int64", "installationId", request.InstallationId, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "installationID should be a valid integer"})
		return
	}

	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		slog.Error("Could not get installation link", "installationId", installationId, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Could not get installation link"})
		return
	}

	// if there is an existing link it should already belong to existing org id
	if link != nil {
		if link.OrganisationId == org.ID {
			slog.Info("Installation already linked to this org", "installationId", installationId, "orgId", org.ID)
			c.JSON(http.StatusOK, gin.H{"status": "already linked to this org"})
			return
		} else {
			slog.Warn("Installation ID already linked to another org",
				"installationId", installationId,
				"currentOrgId", link.OrganisationId,
				"requestedOrgId", org.ID,
				"requestedOrgExtId", org.ExternalId)
			c.JSON(http.StatusBadRequest, gin.H{"status": "installationID already linked to another org and cant be linked to this one unless the app is uninstalled"})
			return
		}
	}

	_, err = models.DB.CreateGithubInstallationLink(&org, installationId)
	if err != nil {
		slog.Error("Failed to create installation link", "installationId", installationId, "orgId", org.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed to create installation link"})
		return
	}

	slog.Info("Successfully created Github installation link", "installationId", installationId, "orgId", org.ID)
	// Return status 200
	c.JSON(http.StatusOK, gin.H{"status": "Successfully created Github installation link"})
}
