package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-substrate/strate/backend/middleware"
	"github.com/go-substrate/strate/backend/models"
	"gorm.io/gorm"
)

func ListReposApi(c *gin.Context) {
	// assume all exists as validated in middleware
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

	var repos []models.Repo

	err = models.DB.GormDB.Preload("Organisation").
		Joins("LEFT JOIN organisations ON repos.organisation_id = organisations.id").
		Where("repos.organisation_id = ?", org.ID).
		Order("name").
		Find(&repos).Error
	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		return
	}

	marshalledRepos := make([]interface{}, 0)

	for _, r := range repos {
		marshalled := r.MapToJsonStruct()
		marshalledRepos = append(marshalledRepos, marshalled)
	}

	response := make(map[string]interface{})
	response["result"] = marshalledRepos

	c.JSON(http.StatusOK, response)
}
