package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func PolicyOrgGetApi(c *gin.Context) {
	policyType := c.Param("policy_type")

	if policyType != "plan" && policyType != "access" {
		slog.Warn("Invalid policy type requested", "policyType", policyType)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy type requested: " + policyType})
		return
	}
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"status": "Could not find organisation: " + organisationId})
		} else {
			slog.Error("Database error while finding organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Internal server error"})
		}
		return
	}

	var policy models.Policy
	query := JoinedOrganisationRepoProjectQuery()
	err = query.
		Where("organisations.id = ? AND (repos.id IS NULL AND projects.id IS NULL) AND policies.type = ? ", org.ID, policyType).
		First(&policy).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Policy not found for organisation", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType)
			c.JSON(http.StatusNotFound, gin.H{"error": "Could not find policy for organisation ext ID: " + organisationId})
		} else {
			slog.Error("Error fetching policy", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unknown error occurred while fetching database"})
		}
		return
	}

	slog.Debug("Policy found", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType)
	c.JSON(http.StatusOK, gin.H{"result": policy.Policy})
}

func PolicyOrgUpsertApi(c *gin.Context) {
	type PolicyUpsertRequest struct {
		PolicyType string `json:"policy_type"`
		PolicyText string `json:"policy_text"`
	}

	var request PolicyUpsertRequest
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
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.JSON(http.StatusNotFound, gin.H{"status": "Could not find organisation: " + organisationId})
		} else {
			slog.Error("Database error while finding organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Internal server error"})
		}
		return
	}

	policyType := request.PolicyType
	policyData := request.PolicyText

	policy := models.Policy{}

	policyResult := models.DB.GormDB.Where("organisation_id = ? AND (repo_id IS NULL AND project_id IS NULL) AND type = ?", org.ID, policyType).Take(&policy)

	if policyResult.RowsAffected == 0 {
		err := models.DB.GormDB.Create(&models.Policy{
			OrganisationID: org.ID,
			Type:           policyType,
			Policy:         policyData,
		}).Error
		if err != nil {
			slog.Error("Error creating policy", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType, "error", err)
			c.String(http.StatusInternalServerError, "Error creating policy")
			return
		}
		slog.Info("Created new policy", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType)
	} else {
		policy.Policy = policyData
		err := models.DB.GormDB.Save(policy).Error
		if err != nil {
			slog.Error("Error updating policy", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating policy"})
			return
		}
		slog.Info("Updated existing policy", "organisationId", organisationId, "orgId", org.ID, "policyType", policyType)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
