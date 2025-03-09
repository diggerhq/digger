package controllers

import (
	"errors"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func PolicyOrgGetApi(c *gin.Context) {
	policyType := c.Param("policy_type")

	if policyType != "plan" && policyType != "access" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid policy type requested: " + policyType})
		return
	}
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("could not find organisation: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"status": "Could not find organisation: " + organisationId})
		} else {
			log.Printf("database error while finding organisation: %v", err)
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Could not find policy for organisation ext ID: " + organisationId})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unknown error occurred while fetching database"})
		}
		return
	}

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
		} else {
			log.Printf("database error while finding organisation: %v", err)
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
			log.Printf("Error creating policy: %v", err)
			c.String(http.StatusInternalServerError, "Error creating policy")
			return
		}
	} else {
		policy.Policy = policyData
		err := models.DB.GormDB.Save(policy).Error
		if err != nil {
			log.Printf("Error updating policy: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating policy"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})

}
