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

func ListReposApi(c *gin.Context) {
	// assume all exists as validated in middleware
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

	var repos []models.Repo

	err = models.DB.GormDB.Preload("Organisation").
		Joins("LEFT JOIN organisations ON repos.organisation_id = organisations.id").
		Where("repos.organisation_id = ?", org.ID).
		Order("name").
		Find(&repos).Error

	if err != nil {
		slog.Error("Error fetching repos", "organisationId", organisationId, "orgId", org.ID, "error", err)
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		return
	}

	marshalledRepos := make([]interface{}, 0)

	for _, r := range repos {
		marshalled := r.MapToJsonStruct()
		marshalledRepos = append(marshalledRepos, marshalled)
	}

	slog.Info("Successfully fetched repositories", "organisationId", organisationId, "orgId", org.ID, "repoCount", len(repos))

	response := make(map[string]interface{})
	response["result"] = marshalledRepos

	c.JSON(http.StatusOK, response)
}
