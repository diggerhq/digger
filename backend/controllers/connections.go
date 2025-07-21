package controllers

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
)

func ListVCSConnectionsApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		slog.Error("Could not fetch organisation", "organisationId", organisationId, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Could not fetch organisation"})
		return
	}

	var connections []models.VCSConnection
	err = models.DB.GormDB.Where("organisation_id = ?", org.ID).Find(&connections).Error
	if err != nil {
		slog.Error("Could not fetch VCS connections", "organisationId", organisationId, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch VCS connections"})
		return
	}

	connectionsSlim := lo.Map(connections, func(c models.VCSConnection, i int) gin.H {
		return gin.H{
			"connection_id":   c.ID,
			"vcs":             c.VCSType,
			"connection_name": c.Name,
		}
	})
	c.JSON(http.StatusOK, gin.H{
		"result": connectionsSlim,
	})
}

func CreateVCSConnectionApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		slog.Error("Could not fetch organisation", "organisationId", organisationId, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Could not fetch organisation"})
		return
	}

	type CreateVCSConnectionRequest struct {
		VCS                    string `json:"type" binding:"required"`
		Name                   string `json:"connection_name"`
		BitbucketAccessToken   string `json:"bitbucket_access_token"`
		BitbucketWebhookSecret string `json:"bitbucket_webhook_secret"`
		GitlabAccessToken      string `json:"gitlab_access_token"`
		GitlabWebhookSecret    string `json:"gitlab_webhook_secret"`
	}

	var request CreateVCSConnectionRequest
	if err := c.BindJSON(&request); err != nil {
		slog.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if request.VCS != string(models.DiggerVCSBitbucket) &&
		request.VCS != string(models.DiggerVCSGitlab) {
		slog.Error("VCS type not supported", "type", request.VCS)
		c.JSON(http.StatusBadRequest, gin.H{"error": "VCS type not supported"})
		return
	}

	secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
	if secret == "" {
		slog.Error("No encryption secret specified")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt access token"})
		return
	}

	bitbucketAccessTokenEncrypted, err := utils.AESEncrypt([]byte(secret), request.BitbucketAccessToken)
	if err != nil {
		slog.Error("Could not encrypt bitbucket access token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt bitbucket access token"})
		return
	}

	bitbucketWebhookSecretEncrypted, err := utils.AESEncrypt([]byte(secret), request.BitbucketWebhookSecret)
	if err != nil {
		slog.Error("Could not encrypt bitbucket webhook secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt bitbucket webhook secret"})
		return
	}

	gitlabAccessTokenEncrypted, err := utils.AESEncrypt([]byte(secret), request.GitlabAccessToken)
	if err != nil {
		slog.Error("Could not encrypt gitlab access secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt gitlab access token"})
		return
	}

	gitlabWebhookSecret, err := utils.AESEncrypt([]byte(secret), request.GitlabWebhookSecret)
	if err != nil {
		slog.Error("Could not encrypt gitlab webhook secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt gitlab webhook secret"})
		return
	}

	connection, err := models.DB.CreateVCSConnection(request.Name, models.DiggerVCSType(request.VCS), 0, "", "", "", "", "", "", "", bitbucketAccessTokenEncrypted, bitbucketWebhookSecretEncrypted, gitlabWebhookSecret, gitlabAccessTokenEncrypted, org.ID)
	if err != nil {
		slog.Error("Could not create VCS connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create VCS connection"})
		return

	}

	slog.Info("Created VCS connection", "connectionId", connection.ID, "organisationId", org.ID)
	c.JSON(http.StatusCreated, gin.H{
		"connection": connection.ID,
	})
}

func GetVCSConnection(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)
	connectionId := c.Param("id")

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Could not fetch organisation", "organisationId", organisationId, "error", err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	var connection models.VCSConnection
	err = models.DB.GormDB.Where("id = ? AND organisation_id = ?", connectionId, org.ID).First(&connection).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Connection not found", "connectionId", connectionId, "organisationId", org.ID)
			c.String(http.StatusNotFound, "Could not find connection: "+connectionId)
		} else {
			slog.Error("Could not fetch connection", "connectionId", connectionId, "error", err)
			c.String(http.StatusInternalServerError, "Could not fetch connection")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connection_name": connection.Name,
		"connection_id":   connection.ID,
	})
}

func DeleteVCSConnection(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)
	connectionId := c.Param("id")

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Could not fetch organisation", "organisationId", organisationId, "error", err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	var connection models.VCSConnection
	err = models.DB.GormDB.Where("id = ? AND organisation_id = ?", connectionId, org.ID).First(&connection).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Connection not found", "connectionId", connectionId, "organisationId", org.ID)
			c.String(http.StatusNotFound, "Could not find connection: "+connectionId)
		} else {
			slog.Error("Could not fetch connection", "connectionId", connectionId, "error", err)
			c.String(http.StatusInternalServerError, "Could not fetch connection")
		}
		return
	}

	err = models.DB.GormDB.Delete(&connection).Error
	if err != nil {
		slog.Error("Could not delete connection", "connectionId", connectionId, "error", err)
		c.String(http.StatusInternalServerError, "Could not delete connection")
		return
	}

	slog.Info("Successfully deleted VCS connection", "connectionId", connectionId, "organisationId", org.ID)
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}
