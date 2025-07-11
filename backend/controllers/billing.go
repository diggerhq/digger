package controllers

import (
	"errors"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
)

func BillingSettingsApi(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"drift_enabled":     org.DriftEnabled,
		"drift_cron_tab":    org.DriftCronTab,
		"drift_webhook_url": org.DriftWebhookUrl,
	})
}
