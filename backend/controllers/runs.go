package controllers

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
)

func RunsForProject(c *gin.Context) {
	currentOrg, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	projectIdStr := c.Param("project_id")

	if projectIdStr == "" {
		c.String(http.StatusBadRequest, "ProjectId not specified")
		return
	}

	projectId, err := strconv.Atoi(projectIdStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ProjectId")
		return
	}

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var org models.Organisation
	err = models.DB.GormDB.Where("id = ?", currentOrg).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, fmt.Sprintf("Could not find organisation: %v", currentOrg))
		} else {
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	project, err := models.DB.GetProject(uint(projectId))
	if err != nil {
		log.Printf("could not fetch project: %v", err)
		c.String(http.StatusInternalServerError, "Could not fetch project")
		return
	}

	runs, err := models.DB.ListDiggerRunsForProject(project.Name, project.Repo.ID)
	if err != nil {
		log.Printf("could not fetch runs: %v", err)
		c.String(http.StatusInternalServerError, "Could not fetch runs")
		return
	}

	serializedRuns := make([]interface{}, 0)
	for _, run := range runs {
		serializedRun, err := run.MapToJsonStruct()
		if err != nil {
			log.Printf("could not unmarshal run: %v", err)
			c.String(http.StatusInternalServerError, "Could not unmarshal runs")
			return
		}
		serializedRuns = append(serializedRuns, serializedRun)
	}
	c.JSON(http.StatusOK, serializedRuns)
}
