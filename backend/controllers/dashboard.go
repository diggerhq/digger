package controllers

import (
	"errors"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

func GetDashboardStatusApi(c *gin.Context) {
	organisationId := c.GetString(middleware.ORGANISATION_ID_KEY)
	organisationSource := c.GetString(middleware.ORGANISATION_SOURCE_KEY)

	var org models.Organisation
	err := models.DB.GormDB.Where("external_id = ? AND external_source = ?", organisationId, organisationSource).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Could not find organisation: "+organisationId)
		}
		return
	}

	response := make(map[string]interface{})

	c.JSON(http.StatusOK, response)
}
