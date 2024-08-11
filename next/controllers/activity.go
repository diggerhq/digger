package controllers

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/middleware"
	"github.com/diggerhq/digger/next/model"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

type CreateProjectRunRequest struct {
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Status    string    `json:"status"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
}

func CreateRunForProject(c *gin.Context) {
	repoName := c.Param("repo")
	projectName := c.Param("projectName")
	orgId := c.GetString(middleware.ORGANISATION_ID_KEY)

	if orgId == "" {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	org, err := dbmodels.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("Error fetching organisation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		return
	}

	var repo model.Repo

	err = dbmodels.DB.GormDB.Where("name = ? AND organisation_id = ?", repoName, orgId).First(&repo).Error

	if err != nil {
		log.Printf("Error fetching repo: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching repo"})
		return
	}

	var project model.Project

	err = dbmodels.DB.GormDB.Where("name = ? AND repo_id = ? AND organisation_id = ?", projectName, repo.ID, org.ID).First(&project).Error

	if err != nil {
		log.Printf("Error fetching project: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching project"})
		return
	}

	var request CreateProjectRunRequest

	err = c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	run := model.ProjectRun{
		StartedAt: request.StartedAt.UnixMilli(),
		EndedAt:   request.EndedAt.UnixMilli(),
		Status:    request.Status,
		Command:   request.Command,
		Output:    request.Output,
		ProjectID: project.ID,
	}

	err = models.DB.GormDB.Create(&run).Error

	if err != nil {
		log.Printf("Error creating run: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating run"})
		return
	}

	c.JSON(http.StatusOK, map[string]string{"success": "true"})
}
