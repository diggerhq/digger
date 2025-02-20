package controllers

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func (d DiggerController) CreateOrgInternal(c *gin.Context) {
	type OrgCreateRequest struct {
		Name           string `json:"org_name"`
		ExternalSource string `json:"external_source"`
		ExternalId     string `json:"external_id"`
	}

	var request OrgCreateRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error binding JSON"})
		return
	}

	name := request.Name
	externalSource := request.ExternalSource
	externalId := request.ExternalId

	log.Printf("creating org for %v %v %v", name, externalSource, externalId)
	org, err := models.DB.CreateOrganisation(name, externalSource, externalId)
	if err != nil {
		log.Printf("Error creating org: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating org"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "org_id": org.ID})
}

func (d DiggerController) CreateUserInternal(c *gin.Context) {
	type UserCreateRequest struct {
		UserExternalSource string `json:"external_source"`
		UserExternalId     string `json:"external_id"`
		UserEmail          string `json:"email"`
		OrgExternalId      string `json:"external_org_id"`
	}

	var request UserCreateRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	extUserId := request.UserExternalId
	extUserSource := request.UserExternalSource
	userEmail := request.UserEmail
	externalOrgId := request.OrgExternalId

	org, err := models.DB.GetOrganisation(externalOrgId)
	if err != nil {
		log.Printf("Error retrieving org: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving org"})
	}

	// for now using email for username since we want to deprecate that field
	username := userEmail
	user, err := models.DB.CreateUser(userEmail, extUserSource, extUserId, org.ID, username)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	c.JSON(200, gin.H{"status": "success", "user_id": user.ID})
}
