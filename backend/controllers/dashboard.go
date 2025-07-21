package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
)

func GetDashboardStatusApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Organisation not found", "organisationId", organisationId, "source", organisationSource)
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		} else {
			slog.Error("Could not fetch organisation", "organisationId", organisationId, "source", organisationSource, "error", err)
			c.String(http.StatusNotFound, "Could not fetch organisation: "+organisationId)
		}
		return
	}

	response := make(map[string]interface{})

	slog.Debug("Fetching dashboard status", "organisationId", organisationId, "orgId", org.ID)
	c.JSON(http.StatusOK, response)
}
