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

func BillingStatusApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Error fetching organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.String(http.StatusInternalServerError, "Error fetching organisation")
		}
		return
	}

	monitoredProjectsCount, remainingFreeProjects, billableProjectsCount, err := models.DB.GetProjectsRemainingInFreePLan(org.ID)
	if err != nil {
		slog.Error("Error fetching remaining free projects", "error", err)
		c.String(http.StatusInternalServerError, "Error fetching remaining free projects")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"billing_plan":                   org.BillingPlan,
		"billing_stripe_subscription_id": org.BillingStripeSubscriptionId,
		"remaining_free_projects":        remainingFreeProjects,
		"monitored_projects_count":       monitoredProjectsCount,
		"billable_projects_count":        billableProjectsCount,
		"monitored_projects_limit":       models.MaxFreePlanProjectsPerOrg,
	})
}
