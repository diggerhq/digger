package controllers

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

func GetActivity(c *gin.Context) {
	loggedInOrganisation, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err := models.DB.GormDB.Where("id = ?", loggedInOrganisation).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", loggedInOrganisation))
		} else {
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	runs, err := models.DB.GetProjectRunsForOrg(int(loggedInOrganisation.(uint)))
	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while fetching activity from database")
		return
	}

	marshalledRuns := make([]interface{}, 0)

	for _, run := range runs {
		marshalled := run.MapToJsonStruct()
		marshalledRuns = append(marshalledRuns, marshalled)
	}

	response := make(map[string]interface{})
	response["runs"] = marshalledRuns

	if err != nil {
		c.String(http.StatusInternalServerError, "Unknown error occurred while marshalling response")
		return
	}

	c.JSON(http.StatusOK, response)
}
