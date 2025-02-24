package controllers

import (
	"errors"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
)

func LinkGithubInstallationToOrgApi(c *gin.Context) {
	type LinkInstallationRequest struct {
		InstallationId string `json:"installation_id"`
	}

	var request LinkInstallationRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request format"})
		return
	}

	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err = models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("could not find organisation: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"status": "Could not find organisation: " + organisationId})
		}
		return
	}

	installationId, err := strconv.ParseInt(request.InstallationId, 10, 64)
	if err != nil {
		log.Printf("Failed to convert InstallationId to int64: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "installationID should be a valid integer"})
		return
	}

	link, err := models.DB.GetGithubAppInstallationLink(installationId)

	if err != nil {
		log.Printf("could not get installation link: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Could not get installation link"})
		return
	}

	// if there is an existing link it should already belong to existing org id
	if link != nil {
		if link.OrganisationId == org.ID {
			c.JSON(200, gin.H{"status": "already linked to this org"})
			return
		} else {
			log.Printf("installation id %v is already linked to %v", installationId, org.ExternalId)
			c.JSON(http.StatusBadRequest, gin.H{"status": "installationID already linked to another org and cant be linked to this one unless the app is uninstalled"})
			return
		}
	}

	_, err = models.DB.CreateGithubInstallationLink(&org, installationId)
	if err != nil {
		log.Printf("failed to create installation link: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed to create installation link"})
		return
	}

	// Return status 200
	c.JSON(http.StatusOK, gin.H{"status": "Successfully created Github installation link"})
	return
}
