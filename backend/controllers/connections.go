package controllers

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/samber/lo"

	"github.com/diggerhq/digger/backend/utils"
	"gorm.io/gorm"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
)

func ListVCSConnectionsApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		log.Printf("could not fetch organisation: %v err: %v", organisationId, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Could not fetch organisation"})
		return
	}

	var connections []models.VCSConnection
	err = models.DB.GormDB.Where("organisation_id = ?", org.ID).Find(&connections).Error
	if err != nil {
		log.Printf("could not fetch VCS connections: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch VCS connections"})
		return
	}

	connectionsSlim := lo.Map(connections, func(c models.VCSConnection, i int) gin.H {
		return gin.H{
			"connection_id":   c.ID,
			"vcs":             "bitbucket",
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
		log.Printf("could not fetch organisation: %v err: %v", organisationId, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Could not fetch organisation"})
		return
	}

	type CreateVCSConnectionRequest struct {
		VCS                    string `json:"type" binding:"required"`
		Name                   string `json:"connection_name"`
		BitbucketAccessToken   string `json:"bitbucket_access_token"`
		BitbucketWebhookSecret string `json:"bitbucket_webhook_secret"`
	}

	var request CreateVCSConnectionRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if request.VCS != "bitbucket" {
		log.Printf("VCS type not supported: %v", request.VCS)
		c.JSON(http.StatusBadRequest, gin.H{"error": "VCS type not supported"})
		return
	}

	secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
	if secret == "" {
		log.Printf("ERROR: no encryption secret specified")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt access token"})
		return
	}

	bitbucketAccessTokenEncrypted, err := utils.AESEncrypt([]byte(secret), request.BitbucketAccessToken)
	if err != nil {
		log.Printf("could not encrypt access token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt access token"})
		return
	}

	bitbucketWebhookSecretEncrypted, err := utils.AESEncrypt([]byte(secret), request.BitbucketWebhookSecret)
	if err != nil {
		log.Printf("could not encrypt webhook secret: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encrypt webhook secret"})
		return
	}

	connection, err := models.DB.CreateVCSConnection(
		request.Name,
		0,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		bitbucketAccessTokenEncrypted,
		bitbucketWebhookSecretEncrypted,
		org.ID,
	)
	if err != nil {
		log.Printf("")
	}

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
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			log.Printf("could not fetch organisation: %v err: %v", organisationId, err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	var connection models.VCSConnection
	err = models.DB.GormDB.Where("id = ? AND organisation_id = ?", connectionId, org.ID).First(&connection).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Could not find connection: "+connectionId)
		} else {
			log.Printf("could not fetch connection: %v err: %v", connectionId, err)
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
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			log.Printf("could not fetch organisation: %v err: %v", organisationId, err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	var connection models.VCSConnection
	err = models.DB.GormDB.Where("id = ? AND organisation_id = ?", connectionId, org.ID).First(&connection).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Could not find connection: "+connectionId)
		} else {
			log.Printf("could not fetch connection: %v err: %v", connectionId, err)
			c.String(http.StatusInternalServerError, "Could not fetch connection")
		}
		return
	}

	err = models.DB.GormDB.Delete(&connection).Error
	if err != nil {
		log.Printf("could not delete connection: %v err: %v", connectionId, err)
		c.String(http.StatusInternalServerError, "Could not delete connection")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}
