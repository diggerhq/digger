package controllers

import (
	"log/slog"
	"net/http"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/gin-gonic/gin"
)

func (d DiggerController) UpsertOrgInternal(c *gin.Context) {
	type OrgCreateRequest struct {
		Name           string  `json:"org_name"`
		ExternalSource string  `json:"external_source"`
		ExternalId     string  `json:"external_id"`
		AdminEmail     *string `json:"admin_email,omitempty"`
	}

	var request OrgCreateRequest
	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("Error binding JSON", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error binding JSON"})
		return
	}

	name := request.Name
	externalSource := request.ExternalSource
	externalId := request.ExternalId
	adminEmail := request.AdminEmail

	slog.Info("Creating or updating organization",
		"name", name,
		"externalSource", externalSource,
		"externalId", externalId)

	var org *models.Organisation
	org, err = models.DB.GetOrganisation(externalId)
	if err != nil {
		slog.Error("Error while retrieving org", "externalId", externalId, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating org"})
		return
	}

	if org == nil {
		slog.Info("Organization not found, creating new one", "externalId", externalId)
		org, err = models.DB.CreateOrganisation(name, externalSource, externalId, adminEmail)
		if err != nil {
			slog.Error("Error creating org", "name", name, "externalId", externalId, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating org"})
			return
		}
	}

	slog.Info("Successfully upserted organization", "orgId", org.ID, "externalId", externalId)
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
		slog.Error("Error binding JSON", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error binding JSON"})
		return
	}

	extUserId := request.UserExternalId
	extUserSource := request.UserExternalSource
	userEmail := request.UserEmail
	externalOrgId := request.OrgExternalId

	slog.Info("Creating user",
		"email", userEmail,
		"externalSource", extUserSource,
		"externalId", extUserId,
		"orgExternalId", externalOrgId)

	org, err := models.DB.GetOrganisation(externalOrgId)
	if err != nil {
		slog.Error("Error retrieving org", "externalOrgId", externalOrgId, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving org"})
		return
	}

	existingUser, err := models.DB.GetUserByEmail(userEmail)
	if existingUser != nil && err == nil {
		slog.Error("User email already exists", "email", userEmail)
		c.JSON(http.StatusConflict, gin.H{"error": "User email already exists"})
		return
	}

	// for now using email for username since we want to deprecate that field
	username := userEmail
	user, err := models.DB.CreateUser(userEmail, extUserSource, extUserId, org.ID, username)
	if err != nil {
		slog.Error("Error creating user",
			"email", userEmail,
			"externalId", extUserId,
			"orgId", org.ID,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	segment.IdentifyClient(userEmail, userEmail, userEmail, userEmail, org.Name, org.ExternalId, string(org.BillingPlan))

	slog.Info("Successfully created user", "userId", user.ID, "email", userEmail, "orgId", org.ID)
	c.JSON(http.StatusOK, gin.H{"status": "success", "user_id": user.ID})
}
